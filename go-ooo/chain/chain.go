package chain

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"time"

	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/logger"
	"go-ooo/ooo_api"
	"go-ooo/ooo_router"
	"go-ooo/utils"
	"go-ooo/utils/walletworker"

	"github.com/cenkalti/backoff/v4"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
)

type OoORouterService struct {
	contractAddress  common.Address
	client           *ethclient.Client
	contractInstance *ooo_router.OooRouter
	context          context.Context
	cfg              *config.Config

	transactOpts *bind.TransactOpts
	callOpts     *bind.CallOpts

	logDataRequestedHash    common.Hash
	logRequestFulfilledHash common.Hash
	contractAbi             abi.ABI

	oracleAddress    common.Address
	oraclePrivateKey *ecdsa.PrivateKey

	db *database.DB

	oooApi *ooo_api.OOOApi

	watchOpts            *bind.WatchOpts
	chanDataRequests     chan *ooo_router.OooRouterDataRequested
	chanRequestFulfilled chan *ooo_router.OooRouterRequestFulfilled

	// historical data
	historicalFilterOpts *bind.FilterOpts

	lastBlockNumber uint64

	subscriptionDr event.Subscription
	subscriptionRf event.Subscription

	prevTxNonce uint64
}

func NewOoORouter(ctx context.Context, cfg *config.Config, client *ethclient.Client,
	contractInstance *ooo_router.OooRouter, contractAddress common.Address,
	oraclePrivateKey []byte, db *database.DB, oooApi *ooo_api.OOOApi) (*OoORouterService, error) {

	logDataRequestedHash := crypto.Keccak256Hash([]byte("DataRequested(address,address,uint256,bytes32,bytes32)"))
	logRequestFulfilledHash := crypto.Keccak256Hash([]byte("RequestFulfilled(address,address,bytes32,uint256)"))

	contractAbi, err := abi.JSON(strings.NewReader(ooo_router.OooRouterMetaData.ABI))
	if err != nil {
		return nil, err
	}

	oraclePrivateKeyECDSA, err := crypto.HexToECDSA(utils.RemoveHexPrefix(string(oraclePrivateKey)))
	if err != nil {
		return nil, err
	}

	oraclePublicKey := oraclePrivateKeyECDSA.Public()

	ECDSAoraclePublicKey, err := crypto.UnmarshalPubkey(crypto.FromECDSAPub(oraclePublicKey.(*ecdsa.PublicKey)))
	if err != nil || ECDSAoraclePublicKey == nil {
		return nil, err
	}
	_, oracleAddressStr := walletworker.GenerateAddress(ECDSAoraclePublicKey)
	oracleAddress := common.HexToAddress(oracleAddressStr)

	logger.InfoWithFields("chain", "NewOoORouter", "", "set our wallet address", logger.Fields{
		"address": oracleAddressStr,
	})

	transactOpts, err := bind.NewKeyedTransactorWithChainID(oraclePrivateKeyECDSA, big.NewInt(cfg.Chain.NetworkId))
	if err != nil {
		return nil, err
	}

	nonce, err := client.PendingNonceAt(ctx, oracleAddress)
	if err != nil {
		return nil, err
	}

	transactOpts.Nonce = big.NewInt(int64(nonce))
	transactOpts.Value = big.NewInt(0)

	transactOpts.GasPrice = nil
	transactOpts.GasLimit = cfg.Chain.GasLimit // in units
	transactOpts.Context = ctx

	callOpts := &bind.CallOpts{From: common.HexToAddress(oracleAddressStr), Context: ctx}

	// fromBlock - set first to 0
	initialFromBlock := uint64(0)

	// todo - have a cmd flag to use from block to override all
	// check conf
	firstBlockFromConf := cfg.Chain.FirstBlock
	if firstBlockFromConf > 0 {
		initialFromBlock = firstBlockFromConf
	}

	// check DB
	tb, err := db.GetLastBlockNumQueried()
	if err == nil {
		if tb.GetBlockNum() > firstBlockFromConf {
			initialFromBlock = tb.GetBlockNum()
		}
	}

	logger.InfoWithFields("chain", "NewOoORouter", "", "set initial query from block", logger.Fields{
		"initial_block": initialFromBlock,
	})

	watchOpts := &bind.WatchOpts{Context: ctx, Start: &initialFromBlock}

	chanDataRequests := make(chan *ooo_router.OooRouterDataRequested)
	chanRequestFulfilled := make(chan *ooo_router.OooRouterRequestFulfilled)

	historicalFilterOpts := &bind.FilterOpts{Context: ctx, Start: initialFromBlock, End: nil}

	return &OoORouterService{
		contractAddress:         contractAddress,
		client:                  client,
		contractInstance:        contractInstance,
		context:                 ctx,
		cfg:                     cfg,
		logDataRequestedHash:    logDataRequestedHash,
		logRequestFulfilledHash: logRequestFulfilledHash,
		contractAbi:             contractAbi,
		oracleAddress:           oracleAddress,
		transactOpts:            transactOpts,
		callOpts:                callOpts,
		db:                      db,
		oooApi:                  oooApi,
		oraclePrivateKey:        oraclePrivateKeyECDSA,
		watchOpts:               watchOpts,
		chanDataRequests:        chanDataRequests,
		chanRequestFulfilled:    chanRequestFulfilled,
		historicalFilterOpts:    historicalFilterOpts,
		lastBlockNumber:         initialFromBlock,
		prevTxNonce:             nonce,
	}, nil
}

func (o *OoORouterService) GetPrevTxNonce() uint64 {
	return o.prevTxNonce
}

func (o *OoORouterService) GetNonceFromTransactOpts() uint64 {
	return o.transactOpts.Nonce.Uint64()
}

func (o *OoORouterService) GetProviderAddress() common.Address {
	return o.oracleAddress
}

func (o *OoORouterService) setLastBlockNumber(blockNumber uint64) {

	if blockNumber > o.lastBlockNumber {
		logger.Debug("chain", "setLastBlockNumber", "", "set last block number in db", logger.Fields{
			"block_num": blockNumber,
		})

		o.lastBlockNumber = blockNumber
		err := o.db.InsertNewToBlock(blockNumber)

		if err != nil {
			logger.ErrorWithFields("chain", "setLastBlockNumber", "update db", err.Error(), logger.Fields{
				"block_num": blockNumber,
			})
		}
	}
}

func (o *OoORouterService) Shutdown() {
	currentBlockNum, err := o.client.BlockNumber(o.context)

	if err != nil {
		logger.Error("chain", "Shutdown", "get block num", err.Error())
	}

	if o.subscriptionDr != nil {
		logger.Info("chain", "Shutdown", "", "unsubscribe from DataRequest events")
		o.subscriptionDr.Unsubscribe()

		// to pick up where it left - only for DataRequests.
		// We want to check historical events for DRs first.
		if err == nil {
			o.setLastBlockNumber(currentBlockNum)
		}
	}
	if o.subscriptionRf != nil {
		logger.Info("chain", "Shutdown", "", "unsubscribe from RequestFulfilled events")
		o.subscriptionRf.Unsubscribe()
	}
}

func (o *OoORouterService) GetHistoricalEvents() {

	logger.Info("chain", "GetHistoricalEvents", "", "get event history")

	me := make([]common.Address, 0, 1)
	me = append(me, o.oracleAddress)

	itrDr, err := o.contractInstance.FilterDataRequested(o.historicalFilterOpts, nil, me, nil)

	if err != nil {
		logger.Error("chain", "GetHistoricalEvents", "get FilterDataRequested events", err.Error())

		return
	}

	for itrDr.Next() {
		ev := itrDr.Event
		o.processIncomingRequests(ev)
	}

	itrFr, err := o.contractInstance.FilterRequestFulfilled(o.historicalFilterOpts, nil, me, nil)

	if err != nil {
		logger.Error("chain", "GetHistoricalEvents", "get FilterRequestFulfilled events", err.Error())

		return
	}

	for itrFr.Next() {
		ev := itrFr.Event
		o.processIncomingFulfilments(ev)
	}

}

func (o *OoORouterService) subscribeToDataRequested(me []common.Address) {

	if o.subscriptionDr != nil {
		o.subscriptionDr.Unsubscribe()
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 10 * time.Minute

	var sub event.Subscription

	retryable := func() error {
		var subErr error
		sub, subErr = o.contractInstance.WatchDataRequested(o.watchOpts, o.chanDataRequests, nil, me, nil)
		return subErr
	}

	notify := func(err error, t time.Duration) {
		logger.Error("chain", "subscribeToDataRequested", "init subscription", err.Error())
	}

	err := backoff.RetryNotify(retryable, b, notify)

	if err != nil {
		// no point continuing if we can't connect after retrying
		panic(err)
	}

	o.subscriptionDr = sub
}

func (o *OoORouterService) subscribeToRequestFulfilled(me []common.Address) {

	if o.subscriptionRf != nil {
		o.subscriptionRf.Unsubscribe()
	}

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 10 * time.Minute

	var sub event.Subscription

	retryable := func() error {
		var subErr error
		sub, subErr = o.contractInstance.WatchRequestFulfilled(o.watchOpts, o.chanRequestFulfilled, nil, me, nil)
		return subErr
	}

	notify := func(err error, t time.Duration) {
		logger.Error("chain", "subscribeToRequestFulfilled", "init subscription", err.Error())
	}

	err := backoff.RetryNotify(retryable, b, notify)

	if err != nil {
		// no point continuing if we can't connect after retrying
		panic(err)
	}

	o.subscriptionRf = sub
}

func (o *OoORouterService) RunEventWatchers() {
	logger.Info("chain", "RunEventWatchers", "", "initialise event subscriptions")

	me := make([]common.Address, 0, 1)
	me = append(me, o.oracleAddress)

	o.subscribeToDataRequested(me)
	defer o.subscriptionDr.Unsubscribe()

	o.subscribeToRequestFulfilled(me)
	defer o.subscriptionRf.Unsubscribe()

	for {
		select {
		case ev := <-o.chanDataRequests:
			o.processIncomingRequests(ev)
		case ev := <-o.chanRequestFulfilled:
			o.processIncomingFulfilments(ev)
		case subErr := <-o.subscriptionDr.Err():
			if subErr != nil {
				logger.Error("chain", "RunEventWatchers", "DataRequested subscription connection error", subErr.Error())
				o.subscribeToDataRequested(me)
			}

		case subErr := <-o.subscriptionRf.Err():
			if subErr != nil {
				logger.Error("chain", "RunEventWatchers", "RequestFulfilled subscription connection error", subErr.Error())
				o.subscribeToRequestFulfilled(me)
			}
		}
	}
}

func (o *OoORouterService) processIncomingRequests(event *ooo_router.OooRouterDataRequested) {
	consumer := event.Consumer
	provider := event.Provider
	requestId := common.Bytes2Hex(event.RequestId[:])
	endpointStr := string(common.TrimRightZeroes(event.Data[:]))

	logger.InfoWithFields("chain", "processIncomingRequests", "", "got data request event for me", logger.Fields{
		"requestId": requestId,
	})

	gasPrice, gasUsed := o.processGasUsage(event.Raw)

	// check status and if requests already exists
	reqDbRes, _ := o.db.FindByRequestId(requestId)

	if reqDbRes.ID == 0 {
		logger.InfoWithFields("chain", "processIncomingRequests", "add job to db", "new request", logger.Fields{
			"requestId": requestId,
		})

		isAdHoc, err := ooo_api.IsAdhoc(endpointStr)

		if err != nil {
			// possibly not in Tx pool yet
			logger.ErrorWithFields("chain", "processIncomingRequests", "parse and check adhoc", err.Error(),
				logger.Fields{
					"requestId": requestId,
				})
		}

		_ = o.db.InsertNewRequest(
			provider.Hex(),
			consumer.Hex(),
			requestId,
			common.Bytes2Hex(event.Data[:]),
			endpointStr,
			event.Raw.TxHash.Hex(),
			gasUsed,
			gasPrice,
			event.Fee.Uint64(),
			event.Raw.BlockNumber,
			isAdHoc,
		)
	} else {
		logger.InfoWithFields("chain", "processIncomingRequests", "check db for request", "request already in db",
			logger.Fields{
				"request_id": reqDbRes.RequestId,
				"status":     reqDbRes.GetRequestStatusString(),
			})
	}

	o.setLastBlockNumber(event.Raw.BlockNumber)

}

func (o *OoORouterService) processIncomingFulfilments(event *ooo_router.OooRouterRequestFulfilled) {

	requestId := common.Bytes2Hex(event.RequestId[:])

	logger.InfoWithFields("chain", "processIncomingFulfilments", "", "got request fulfilment event for me",
		logger.Fields{
			"request_id": requestId,
		})

	gasPrice, gasUsed := o.processGasUsage(event.Raw)
	// check status and if requests already exists
	reqDbRes, _ := o.db.FindByRequestId(requestId)

	if reqDbRes.ID != 0 {
		logger.InfoWithFields("chain", "processIncomingFulfilments", "confirm fulfillment",
			"confirmed request fulfilment for request",
			logger.Fields{
				"request_id": requestId,
			})

		err := o.db.UpdateFulfillmentSuccess(
			requestId,
			event.Raw.BlockNumber,
			event.Raw.TxHash.Hex(),
			gasUsed,
			gasPrice,
		)
		if err != nil {
			logger.ErrorWithFields("chain", "processIncomingFulfilments", "UpdateFulfillmentSuccess",
				err.Error(),
				logger.Fields{
					"request_id": requestId,
				})
		}
	}

	o.setLastBlockNumber(event.Raw.BlockNumber)

}

func (o *OoORouterService) processGasUsage(evLog types.Log) (uint64, uint64) {
	gasPrice := uint64(0)
	gasUsed := uint64(0)

	txRec, err := o.client.TransactionReceipt(o.context, evLog.TxHash)
	if err == nil {
		// todo - need to clean up and gather any missing data if Tx query above fails
		gasUsed = txRec.GasUsed
	} else {
		logger.ErrorWithFields("chain", "processEventLog", "get TransactionReceipt", err.Error(), logger.Fields{
			"tx_hash": evLog.TxHash,
		})
	}

	tx, _, err := o.client.TransactionByHash(o.context, evLog.TxHash)
	if err == nil {
		// todo - need to clean up and gather any missing data if Tx query above fails
		gasPrice = tx.GasPrice().Uint64()
	} else {
		logger.ErrorWithFields("chain", "processEventLog", "get TransactionByHash", err.Error(), logger.Fields{
			"tx_hash": evLog.TxHash,
		})
	}

	return gasPrice, gasUsed
}
