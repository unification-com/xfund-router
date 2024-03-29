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

	adminTasks     chan go_ooo_types.AdminTask
	adminTasksResp chan go_ooo_types.AdminTaskResponse

	// todo - analytics channels, functions and echo endpoint
	analyticsTasks     chan go_ooo_types.AnalyticsTask
	analyticsTasksResp chan go_ooo_types.AnalyticsTaskResponse

	authToken string
}

func NewService(ctx context.Context, cfg *config.Config, oraclePrivateKey []byte,
	db *database.DB, authToken string) (*Service, error) {

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
		jobTicker:          time.NewTicker(time.Second * pollInterval),
		updatePairsTicker:  time.NewTicker(time.Minute * 30),
		oooRouterService:   oooRouterService,
		adminTasks:         make(chan go_ooo_types.AdminTask),
		adminTasksResp:     make(chan go_ooo_types.AdminTaskResponse),
		analyticsTasks:     make(chan go_ooo_types.AnalyticsTask),
		analyticsTasksResp: make(chan go_ooo_types.AnalyticsTaskResponse),
		echoService:        echo.New(),
		oooApi:             oooApi,
		authToken:          authToken,
	}, nil
}

func (s *Service) Run() {

	go func(s *Service) {
		s.initEcho()
	}(s)

	go func(s *Service) {
		s.initPrometheus()
	}(s)

	// update supported pairs from the Finchains API
	go func(s *Service) {
		s.oooApi.UpdateSupportedPairs()
		s.oooApi.UpdateDexPairs()
	}(s)

	// pick up from the last block we know about to process
	// any historical events missed. This will run and complete
	// before the event subscriptions initialise in order to
	// process any potentially missed and/or processed requests
	s.oooRouterService.GetHistoricalEvents()

	go func(s *Service) {
		s.oooRouterService.RunEventWatchers()
	}(s)

	for {
		select {
		case <-s.jobTicker.C:
			s.oooRouterService.ProcessPendingJobQueue()
		case <-s.updatePairsTicker.C:
			go func(s *Service) {
				s.oooApi.UpdateSupportedPairs()
				s.oooApi.UpdateDexPairs()
			}(s)
		case t := <-s.analyticsTasks:
			s.analyticsTasksResp <- s.ProcessAnalyticsTask(t)
		case t := <-s.adminTasks:
			// At any time we can process a request to add a new admin task
			// such as changing fees etc.
			s.adminTasksResp <- s.oooRouterService.ProcessAdminTask(t)
		}
	}
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
	err := s.echoService.Shutdown(s.ctx)

	if err != nil {
		logger.Error("service", "Stop", "shutting down echo", err.Error())
	}

	err = s.echoService.Close()

	if err != nil {
		logger.Error("service", "Stop", "closing echo", err.Error())
	}
}
