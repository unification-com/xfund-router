package service

import (
	"context"
	"github.com/labstack/echo/v4"
	"sync/atomic"
	"time"

	"go-ooo/chain"
	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/logger"
	"go-ooo/ooo_api"
	"go-ooo/ooo_router"
	go_ooo_types "go-ooo/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Service struct {
	contractAddress   common.Address
	client            *ethclient.Client
	contractInstance  *ooo_router.OooRouter
	db                *database.DB
	ctx               context.Context
	cfg               *config.Config
	jobTicker         *time.Ticker // periodic jobTicker
	updatePairsTicker *time.Ticker
	oooRouterService  *chain.OoORouterService

	echoService *echo.Echo
	oooApi      *ooo_api.OOOApi

	// Each task carries its own reply channel (Resp), so concurrent requests can't read
	// each other's responses - no shared response channel.
	adminTasks     chan go_ooo_types.AdminTask
	analyticsTasks chan go_ooo_types.AnalyticsTask

	adminTokenHash string

	// pairsRefreshing guards the periodic pair refresh so a slow run can't pile up a
	// new goroutine on every tick.
	pairsRefreshing atomic.Bool
}

func NewService(ctx context.Context, cfg *config.Config, oraclePrivateKey []byte,
	db *database.DB, adminTokenHash string) (*Service, error) {

	contractAddress := common.HexToAddress(cfg.Chain.ContractAddress)
	logger.InfoWithFields("service", "NewService", "", "dial eth client", logger.Fields{
		"address": cfg.Chain.EthWsHost,
	})
	client, err := ethclient.Dial(cfg.Chain.EthWsHost)

	if err != nil {
		return nil, err
	}

	var pollInterval = time.Duration(30)
	checkDuration := cfg.Jobs.CheckDuration
	if checkDuration != 0 {
		pollInterval = time.Duration(checkDuration)
	}

	logger.Debug("service", "NewService", "", "poll service", logger.Fields{
		"poll_interval": time.Second * pollInterval,
	})

	if err != nil {
		return nil, err
	}

	logger.InfoWithFields("service", "NewService", "", "create ooo router instance", logger.Fields{
		"contract": contractAddress,
	})
	oooRouterInstance, err := ooo_router.NewOooRouter(contractAddress, client)
	if err != nil {
		return nil, err
	}

	oooApi, err := ooo_api.NewApi(ctx, cfg, db)

	if err != nil {
		return nil, err
	}

	logger.Info("service", "NewService", "", "init ooo router service")
	oooRouterService, err := chain.NewOoORouter(ctx, cfg, client, oooRouterInstance, contractAddress, oraclePrivateKey, db, oooApi)

	if err != nil {
		return nil, err
	}

	return &Service{
		ctx:              ctx,
		cfg:              cfg,
		client:           client,
		contractAddress:  contractAddress,
		contractInstance: oooRouterInstance,
		db:               db,
		// https://stackoverflow.com/questions/16903348/scheduled-polling-task-in-go
		jobTicker:         time.NewTicker(time.Second * pollInterval),
		updatePairsTicker: time.NewTicker(time.Minute * 30),
		oooRouterService:  oooRouterService,
		adminTasks:        make(chan go_ooo_types.AdminTask),
		analyticsTasks:    make(chan go_ooo_types.AnalyticsTask),
		echoService:       echo.New(),
		oooApi:            oooApi,
		adminTokenHash:    adminTokenHash,
	}, nil
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

	// refresh the DEX pair catalogue - initial refresh at startup
	go s.refreshPairs()

	// pick up from the last block we know about to process
	// any historical events missed. This will run and complete
	// before the event subscriptions initialise in order to
	// process any potentially missed and/or processed requests
	s.oooRouterService.GetHistoricalEvents()

	go func(s *Service) {
		s.oooRouterService.RunEventWatchers()
	}(s)

	// The event watcher signals this channel when it detects a new request, so we process it
	// immediately instead of waiting for the next jobTicker tick. A nudge buffered during the
	// historical-event catch-up above is drained on the first iteration, so startup backlog is
	// processed at once too.
	jobNudge := s.oooRouterService.JobNudge()

	for {
		select {
		case <-s.ctx.Done():
			logger.Info("service", "Run", "", "context cancelled - shutting down")
			s.Stop()
			return
		case <-s.jobTicker.C:
			s.oooRouterService.ProcessPendingJobQueue("ticker")
		case <-jobNudge:
			s.oooRouterService.ProcessPendingJobQueue("event")
		case <-s.updatePairsTicker.C:
			go s.refreshPairs()
		case t := <-s.analyticsTasks:
			t.Resp <- s.ProcessAnalyticsTask(t)
		case t := <-s.adminTasks:
			// At any time we can process a request to add a new admin task
			// such as changing fees etc.
			t.Resp <- s.oooRouterService.ProcessAdminTask(t)
		}
	}
}

// refreshPairs updates the DEX pair catalogue, skipping the run if a previous refresh is still
// in progress - so a slow refresh can't pile up a goroutine on every updatePairsTicker tick.
// Shared by the initial refresh + the ticker (DRY).
func (s *Service) refreshPairs() {
	if !s.pairsRefreshing.CompareAndSwap(false, true) {
		logger.Info("service", "refreshPairs", "", "skipping pair refresh - previous run still in progress")
		return
	}
	defer s.pairsRefreshing.Store(false)

	s.oooApi.UpdateDexPairs()
}

func (s *Service) Stop() {
	// clean up and shut down
	logger.Info("service", "Stop", "", "shutting down jobTicker")
	s.jobTicker.Stop()

	logger.Info("service", "Stop", "", "shutting down updatePairsTicker")
	s.updatePairsTicker.Stop()

	logger.Info("service", "Stop", "", "shutting down oooRouterService")
	s.oooRouterService.Shutdown()

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
