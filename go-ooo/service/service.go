package service

import (
	"context"
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
	exportCfg := cfg.Jobs.DexExport
	if exportCfg.ApiToken == "" && exportCfg.BaseUrl != "" && len(oraclePrivateKey) > 0 && cfg.Chain.NetworkId > 0 {
		priv, kerr := crypto.HexToECDSA(utils.RemoveHexPrefix(string(oraclePrivateKey)))
		if kerr != nil {
			logger.ErrorWithFields("service", "NewService", "provider export auth", kerr.Error(), logger.Fields{})
		} else {
			signer := walletworker.NewEthSigner(priv)
			oooApi.SetExportAuthenticator(export.NewWalletAuth(exportCfg.BaseUrl, cfg.Chain.NetworkId, signer, nil))
			logger.InfoWithFields("service", "NewService", "", "using provider wallet-auth for the dex-pair-verify export", logger.Fields{
				"provider": signer.Address(),
				"chain_id": cfg.Chain.NetworkId,
			})
		}
	}

	logger.Info("service", "NewService", "", "init chain workers")
	worker, err := buildChainWorker(ctx, cfg, oraclePrivateKey, db, oooApi)
	if err != nil {
		return nil, err
	}

	return &Service{
		ctx: ctx,
		cfg: cfg,
		db:  db,
		// https://stackoverflow.com/questions/16903348/scheduled-polling-task-in-go
		updatePairsTicker: time.NewTicker(time.Second * time.Duration(pairsPollInterval)),
		workers:           []*chain.OoORouterService{worker},
		adminTasks:        make(chan go_ooo_types.AdminTask),
		analyticsTasks:    make(chan go_ooo_types.AnalyticsTask),
		echoService:       echo.New(),
		oooApi:            oooApi,
		adminTokenHash:    adminTokenHash,
	}, nil
}

// buildChainWorker dials a chain, binds the Router contract and constructs the per-chain worker.
// One worker is built per configured network; today there is a single [chain] block, so one
// worker. Running several networks from one process loops this over the configured chains.
func buildChainWorker(ctx context.Context, cfg *config.Config, oraclePrivateKey []byte,
	db *database.DB, oooApi *ooo_api.OOOApi) (*chain.OoORouterService, error) {

	contractAddress := common.HexToAddress(cfg.Chain.ContractAddress)
	logger.InfoWithFields("service", "buildChainWorker", "", "dial eth client", logger.Fields{
		"address": cfg.Chain.EthWsHost,
	})
	client, err := ethclient.Dial(cfg.Chain.EthWsHost)
	if err != nil {
		return nil, err
	}

	logger.InfoWithFields("service", "buildChainWorker", "", "create ooo router instance", logger.Fields{
		"contract": contractAddress,
	})
	oooRouterInstance, err := ooo_router.NewOooRouter(contractAddress, client)
	if err != nil {
		return nil, err
	}

	return chain.NewOoORouter(ctx, cfg, client, oooRouterInstance, contractAddress, oraclePrivateKey, db, oooApi)
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

// dispatchAdminTask routes an admin task to the worker for its target chain and returns at once;
// the worker serialises it with that chain's fulfilment transactions and replies on t.Resp. With
// a single chain configured the routing is unambiguous; selecting by the task's target network
// when several chains run in one process arrives with the --network work (MULTI_NETWORK_CLIENT
// Phase 3).
func (s *Service) dispatchAdminTask(t go_ooo_types.AdminTask) {
	if len(s.workers) == 0 {
		t.Resp <- go_ooo_types.AdminTaskResponse{AdminTask: t, Success: false, Error: "no chain workers running"}
		return
	}
	worker := s.workers[0]
	go worker.SubmitAdminTask(t)
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
