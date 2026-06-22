package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"time"

	"go-ooo/chain"
	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/logger"
	"go-ooo/ooo_api"
	"go-ooo/ooo_api/export"
	"go-ooo/ooo_router"
	go_ooo_types "go-ooo/types"
	"go-ooo/utils"
	"go-ooo/utils/walletworker"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Service is the process-level supervisor. It owns the shared spine - the database, the DEX
// pricing engine (origin-chain-agnostic), the HTTP/admin + Prometheus servers and the pair
// catalogue refresh - and supervises one chain worker per configured network. Each worker runs
// its own event watchers and serialises its own chain's transactions; the supervisor wires them
// together and routes admin tasks to the right one.
type Service struct {
	db                *database.DB
	ctx               context.Context
	cfg               *config.Config
	updatePairsTicker *time.Ticker
	workers           []*chain.OoORouterService

	echoService *echo.Echo
	oooApi      *ooo_api.OOOApi

	// Each task carries its own reply channel (Resp), so concurrent requests can't read
	// each other's responses - no shared response channel.
	adminTasks     chan go_ooo_types.AdminTask
	analyticsTasks chan go_ooo_types.AnalyticsTask

	adminTokenHash string
}

func NewService(ctx context.Context, cfg *config.Config, oraclePrivateKey []byte,
	db *database.DB, adminTokenHash string) (*Service, error) {

	// How often to refresh the DEX source catalogue + pairs from the dex-pair-verify export.
	pairsPollInterval := cfg.Jobs.DexExport.PollIntervalSec
	if pairsPollInterval == 0 {
		pairsPollInterval = 3600
	}

	oooApi, err := ooo_api.NewApi(ctx, cfg, db)
	if err != nil {
		return nil, err
	}

	// Provider export auth (T8): when no static EXPORT_API_TOKEN is configured but a provider key + chain
	// are available, authenticate the dex-pair-verify export pulls with the oracle wallet (the on-chain
	// registered provider) via challenge-response instead of a shared secret. A configured static token
	// (the Manager's default) takes precedence as the operator break-glass.
	// The export wallet-auth is process-level (one OOOApi); the provider (one key) is registered on
	// every chain, so any configured chain's id works for the challenge - use the first.
	authChainId := cfg.ChainList()[0].NetworkId
	exportCfg := cfg.Jobs.DexExport
	if exportCfg.ApiToken == "" && exportCfg.BaseUrl != "" && len(oraclePrivateKey) > 0 && authChainId > 0 {
		priv, kerr := crypto.HexToECDSA(utils.RemoveHexPrefix(string(oraclePrivateKey)))
		if kerr != nil {
			logger.ErrorWithFields("service", "NewService", "provider export auth", kerr.Error(), logger.Fields{})
		} else {
			signer := walletworker.NewEthSigner(priv)
			oooApi.SetExportAuthenticator(export.NewWalletAuth(exportCfg.BaseUrl, authChainId, signer, nil))
			logger.InfoWithFields("service", "NewService", "", "using provider wallet-auth for the dex-pair-verify export", logger.Fields{
				"provider": signer.Address(),
				"chain_id": authChainId,
			})
		}
	}

	logger.Info("service", "NewService", "", "init chain workers")
	var workers []*chain.OoORouterService
	for _, chainCfg := range cfg.ChainList() {
		worker, werr := buildChainWorker(ctx, cfg, chainCfg, oraclePrivateKey, db, oooApi)
		if werr != nil {
			// Start-up failure isolation: a chain that can't be built logs + is skipped, so one bad
			// RPC/config can't abort the whole fleet. Fatal only if NONE come up.
			logger.ErrorWithFields("service", "NewService", "build chain worker", werr.Error(), logger.Fields{
				"network_id": chainCfg.NetworkId,
			})
			continue
		}
		workers = append(workers, worker)
	}
	if len(workers) == 0 {
		return nil, errors.New("no chain workers could be started - check the [chain]/[[chains]] RPC endpoints")
	}

	return &Service{
		ctx: ctx,
		cfg: cfg,
		db:  db,
		// https://stackoverflow.com/questions/16903348/scheduled-polling-task-in-go
		updatePairsTicker: time.NewTicker(time.Second * time.Duration(pairsPollInterval)),
		workers:           workers,
		adminTasks:        make(chan go_ooo_types.AdminTask),
		analyticsTasks:    make(chan go_ooo_types.AnalyticsTask),
		echoService:       echo.New(),
		oooApi:            oooApi,
		adminTokenHash:    adminTokenHash,
	}, nil
}

// buildChainWorker dials one chain (chainCfg) and constructs its worker. NewService calls it once per
// entry in the config's chain list, so the supervisor ends up with one independent worker per network.
//
// Two transports: the HTTP endpoint (required) carries every call, the getLogs event detection and the
// tx sends; the websocket endpoint (optional) is used only for live event subscriptions. A blank or
// unreachable websocket is NOT fatal - the worker degrades to HTTP polling, since most operators run on
// free RPCs where WSS is flaky or absent.
func buildChainWorker(ctx context.Context, cfg *config.Config, chainCfg config.ChainConfig,
	oraclePrivateKey []byte, db *database.DB, oooApi *ooo_api.OOOApi) (*chain.OoORouterService, error) {

	contractAddress := common.HexToAddress(chainCfg.ContractAddress)

	logger.InfoWithFields("service", "buildChainWorker", "", "dial eth http client", logger.Fields{
		"address": chainCfg.EthHttpHost,
	})
	httpClient, err := ethclient.Dial(chainCfg.EthHttpHost)
	if err != nil {
		return nil, err
	}
	httpContract, err := ooo_router.NewOooRouter(contractAddress, httpClient)
	if err != nil {
		return nil, err
	}

	// Optional websocket transport for live subscriptions. A failed dial degrades to polling.
	var wsClient *ethclient.Client
	var wsContract *ooo_router.OooRouter
	if chainCfg.EthWsHost != "" {
		logger.InfoWithFields("service", "buildChainWorker", "", "dial eth ws client", logger.Fields{
			"address": chainCfg.EthWsHost,
		})
		wsClient, err = ethclient.Dial(chainCfg.EthWsHost)
		if err != nil {
			logger.WarnWithFields("service", "buildChainWorker", "",
				"could not connect the websocket endpoint - the worker will detect events via HTTP polling", logger.Fields{
					"address": chainCfg.EthWsHost,
					"error":   err.Error(),
				})
			wsClient = nil
		} else if wsContract, err = ooo_router.NewOooRouter(contractAddress, wsClient); err != nil {
			logger.WarnWithFields("service", "buildChainWorker", "",
				"could not bind the router on the websocket client - the worker will detect events via HTTP polling", logger.Fields{
					"error": err.Error(),
				})
			wsClient = nil
			wsContract = nil
		}
	} else {
		logger.Info("service", "buildChainWorker", "", "no eth_ws_host configured - the worker will detect events via HTTP polling")
	}

	return chain.NewOoORouter(ctx, cfg, chainCfg, httpClient, httpContract, wsClient, wsContract, contractAddress, oraclePrivateKey, db, oooApi)
}

func (s *Service) Run() {

	go func(s *Service) {
		s.initEcho()
	}(s)

	// Seed the cumulative fulfilment counters from DB history BEFORE /metrics starts serving,
	// so the all-time totals (and forward rates) are correct from the first scrape.
	chain.WarmStartFulfilmentMetrics(s.db)

	go func(s *Service) {
		s.initPrometheus()
	}(s)

	// refresh the DEX pair catalogue - initial refresh at startup. The catalogue is shared across
	// all chains (pricing is origin-chain-agnostic), so one refresh serves every worker.
	go s.refreshPairs()

	// Start each chain's worker on its own goroutine. Each replays missed events, runs its event
	// watchers and serialises its own fulfilment + admin transactions; the chains run concurrently
	// (independent nonce spaces + RPC clients), so none blocks another.
	for _, w := range s.workers {
		go w.Run()
	}

	for {
		select {
		case <-s.ctx.Done():
			logger.Info("service", "Run", "", "context cancelled - shutting down")
			s.Stop()
			return
		case <-s.updatePairsTicker.C:
			go s.refreshPairs()
		case t := <-s.analyticsTasks:
			t.Resp <- s.ProcessAnalyticsTask(t)
		case t := <-s.adminTasks:
			// At any time we can process a request to add a new admin task such as changing fees.
			// It is routed to the worker for its target chain, which serialises the send with that
			// chain's fulfilments and replies on t.Resp.
			s.dispatchAdminTask(t)
		}
	}
}

// dispatchAdminTask routes an admin task to the worker for its target chain (t.Network) and returns at
// once; the worker serialises it with that chain's fulfilment transactions and replies on t.Resp.
func (s *Service) dispatchAdminTask(t go_ooo_types.AdminTask) {
	worker := s.workerForTask(t)
	if worker == nil {
		msg := fmt.Sprintf("no chain worker for network %d", t.Network)
		if t.Network == 0 {
			msg = "several chains are running - specify --chain <name|network_id>"
		}
		t.Resp <- go_ooo_types.AdminTaskResponse{AdminTask: t, Success: false, Error: msg}
		return
	}
	go worker.SubmitAdminTask(t)
}

// workerForTask resolves the worker for an admin task's target network, or nil if none applies (the
// caller turns that into an "ask for --chain" error). The routing decision itself lives in the pure
// routeWorkerIndex so it can be unit-tested without dialling a chain.
func (s *Service) workerForTask(t go_ooo_types.AdminTask) *chain.OoORouterService {
	ids := make([]int64, len(s.workers))
	for i, w := range s.workers {
		ids[i] = w.NetworkId()
	}
	if idx := routeWorkerIndex(ids, t.Network); idx >= 0 {
		return s.workers[idx]
	}
	return nil
}

// routeWorkerIndex picks the worker (by index into networkIds) that should handle a task for the target
// network, or -1 if none applies. Target 0 (unspecified) routes to the sole worker when exactly one
// chain runs; with several it is ambiguous (-1). A set target matches a worker by network id.
func routeWorkerIndex(networkIds []int64, target int64) int {
	if target == 0 {
		if len(networkIds) == 1 {
			return 0
		}
		return -1
	}
	for i, id := range networkIds {
		if id == target {
			return i
		}
	}
	return -1
}

// refreshPairs refreshes the DEX source catalogue + pair set. Shared by the initial refresh + the
// ticker; overlapping runs are coalesced by SyncDexSources' own guard, so this stays a thin seam.
func (s *Service) refreshPairs() {
	s.oooApi.UpdateDexPairs()
}

func (s *Service) Stop() {
	// clean up and shut down
	logger.Info("service", "Stop", "", "shutting down updatePairsTicker")
	s.updatePairsTicker.Stop()

	// Each worker's own job loop stops itself on the shared context cancellation; Shutdown
	// unsubscribes its event watchers and saves its resume block.
	for _, w := range s.workers {
		logger.InfoWithFields("service", "Stop", "", "shutting down chain worker", logger.Fields{
			"network_id": w.NetworkId(),
		})
		w.Shutdown()
	}

	logger.Info("service", "Stop", "", "shutting down echo")
	// s.ctx is already cancelled here, so use a fresh bounded context to let echo
	// drain in-flight requests rather than aborting immediately.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := s.echoService.Shutdown(shutdownCtx)

	if err != nil {
		logger.Error("service", "Stop", "shutting down echo", err.Error())
	}

	err = s.echoService.Close()

	if err != nil {
		logger.Error("service", "Stop", "closing echo", err.Error())
	}
}
