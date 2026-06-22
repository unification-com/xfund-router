package chain

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/logger"
	"go-ooo/ooo_api"
	"go-ooo/ooo_router"
	go_ooo_types "go-ooo/types"
	"go-ooo/utils"
	"go-ooo/utils/walletworker"

	"github.com/cenkalti/backoff/v4"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/event"
)

type OoORouterService struct {
	contractAddress common.Address
	// client + contractInstance are the HTTP transport: every call (nonce, receipts, gas, getLogs),
	// the poll-mode event detection and the fulfilment/admin tx sends go over HTTP.
	client           *ethclient.Client
	contractInstance *ooo_router.OooRouter
	// wsClient + wsContractInstance are the OPTIONAL websocket transport, used only for live event
	// subscriptions. Both are nil when no eth_ws_host is configured (or it couldn't be dialled), in
	// which case the worker detects events by polling getLogs over HTTP instead.
	wsClient           *ethclient.Client
	wsContractInstance *ooo_router.OooRouter
	context            context.Context
	// cfg is the process config (used for the global jobs settings); chainCfg is THIS worker's chain
	// block - gas/eip1559/first_block/RPC/poll settings - so every chain reads its own values.
	cfg      *config.Config
	chainCfg config.ChainConfig

	// networkId is the EVM chain id this worker is bound to. With one worker per chain in a
	// single process, it identifies the worker (logging, admin-task routing, future metrics label).
	networkId int64

	baseTransactOpts *bind.TransactOpts
	callOpts         *bind.CallOpts

	logDataRequestedHash    common.Hash
	logRequestFulfilledHash common.Hash
	contractAbi             abi.ABI

	oracleAddress    common.Address
	oraclePrivateKey *ecdsa.PrivateKey

	// useEip1559 is the effective decision (config AND chain support) on whether fulfilment
	// and admin txs are priced as EIP-1559 dynamic-fee txs rather than legacy gas-price txs.
	useEip1559 bool

	db *database.DB

	oooApi *ooo_api.OOOApi

	watchOpts            *bind.WatchOpts
	chanDataRequests     chan *ooo_router.OooRouterDataRequested
	chanRequestFulfilled chan *ooo_router.OooRouterRequestFulfilled

	// jobNudge signals this worker's job loop to process the pending queue immediately when a
	// new request is detected, instead of waiting for the periodic ticker. Buffered (cap 1) +
	// non-blocking send, so it coalesces bursts and never blocks the event watcher.
	jobNudge chan struct{}

	// adminTasks carries admin operations (register/set_fee/withdraw/…) onto this worker's own
	// run loop, so they are serialised with this chain's fulfilment transactions - the provider
	// account's nonce on this chain is never raced. The supervisor routes each task here.
	adminTasks chan go_ooo_types.AdminTask

	// historical data
	historicalFilterOpts *bind.FilterOpts

	lastBlockNumber uint64

	subscriptionDr event.Subscription
	subscriptionRf event.Subscription
}

func NewOoORouter(ctx context.Context, cfg *config.Config, chainCfg config.ChainConfig,
	client *ethclient.Client, contractInstance *ooo_router.OooRouter, wsClient *ethclient.Client,
	wsContractInstance *ooo_router.OooRouter, contractAddress common.Address,
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
	if err != nil {
		return nil, err
	}
	if ECDSAoraclePublicKey == nil {
		// Never return (nil, nil): the caller only checks err and would proceed with a nil service.
		return nil, errors.New("could not derive the oracle public key")
	}
	_, oracleAddressStr := walletworker.GenerateAddress(ECDSAoraclePublicKey)
	oracleAddress := common.HexToAddress(oracleAddressStr)

	logger.InfoWithFields("chain", "NewOoORouter", "", "set our wallet address", logger.Fields{
		"address": oracleAddressStr,
	})

	transactOpts, err := bind.NewKeyedTransactorWithChainID(oraclePrivateKeyECDSA, big.NewInt(chainCfg.NetworkId))
	if err != nil {
		return nil, err
	}

	// Base template only: the per-send Nonce and GasPrice are set by buildTransactOpts
	// (chain-anchored), so this struct is never mutated after construction.
	transactOpts.Value = big.NewInt(0)
	transactOpts.GasLimit = chainCfg.GasLimit // in units
	transactOpts.Context = ctx

	callOpts := &bind.CallOpts{From: common.HexToAddress(oracleAddressStr), Context: ctx}

	// fromBlock - set first to 0
	initialFromBlock := uint64(0)

	// check conf
	firstBlockFromConf := chainCfg.FirstBlock
	if firstBlockFromConf > 0 {
		initialFromBlock = firstBlockFromConf
	}

	// check DB
	tb, err := db.GetLastBlockNumQueried(chainCfg.NetworkId)
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

	// Decide legacy vs EIP-1559 pricing once at startup: the config toggle gated by an actual
	// base-fee probe, so a pre-London chain (or an RPC hiccup) safely falls back to legacy.
	useEip1559 := determineEip1559(ctx, client, chainCfg.Eip1559)

	return &OoORouterService{
		contractAddress:         contractAddress,
		client:                  client,
		contractInstance:        contractInstance,
		wsClient:                wsClient,
		wsContractInstance:      wsContractInstance,
		context:                 ctx,
		cfg:                     cfg,
		chainCfg:                chainCfg,
		networkId:               chainCfg.NetworkId,
		logDataRequestedHash:    logDataRequestedHash,
		logRequestFulfilledHash: logRequestFulfilledHash,
		contractAbi:             contractAbi,
		oracleAddress:           oracleAddress,
		useEip1559:              useEip1559,
		baseTransactOpts:        transactOpts,
		callOpts:                callOpts,
		db:                      db,
		oooApi:                  oooApi,
		oraclePrivateKey:        oraclePrivateKeyECDSA,
		watchOpts:               watchOpts,
		chanDataRequests:        chanDataRequests,
		chanRequestFulfilled:    chanRequestFulfilled,
		jobNudge:                make(chan struct{}, 1),
		adminTasks:              make(chan go_ooo_types.AdminTask),
		historicalFilterOpts:    historicalFilterOpts,
		lastBlockNumber:         initialFromBlock,
	}, nil
}

// JobNudge returns the channel the service's job loop selects on to process the pending queue
// as soon as a new request arrives, rather than waiting for the periodic ticker.
func (o *OoORouterService) JobNudge() <-chan struct{} {
	return o.jobNudge
}

// nudgeJobQueue asks the job loop to run ProcessPendingJobQueue now. The send is non-blocking
// onto a cap-1 buffer, so a burst of new requests coalesces into a single pending nudge (one
// run picks up every just-inserted request) and a busy job loop never blocks the event watcher.
func (o *OoORouterService) nudgeJobQueue() {
	select {
	case o.jobNudge <- struct{}{}:
	default:
	}
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
		err := o.db.InsertNewToBlock(o.networkId, blockNumber)

		if err != nil {
			logger.ErrorWithFields("chain", "setLastBlockNumber", "update db", err.Error(), logger.Fields{
				"block_num": blockNumber,
			})
		}
	}
}

func (o *OoORouterService) Shutdown() {
	// Use a fresh short-lived context: o.context is already cancelled during a
	// graceful shutdown, so reusing it here would fail this last block-number read
	// and lose the resume point for the next start.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	currentBlockNum, err := o.client.BlockNumber(ctx)

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

// GetHistoricalEvents replays this provider's DataRequested + RequestFulfilled events emitted while the
// oracle was offline, from the resume cursor up to the current head. It runs once at startup, before
// live detection begins, so a request raised (and maybe fulfilled) during downtime is reconciled first.
// The scan is batched (see drainFrom), so a wide gap after a long outage never asks the RPC for an
// unbounded getLogs range.
func (o *OoORouterService) GetHistoricalEvents() {
	logger.Info("chain", "GetHistoricalEvents", "", "get event history")
	o.drainFrom(o.lastBlockNumber)
}

const (
	// maxScanRetries is how many times a single getLogs batch is retried (exponential backoff) before
	// the catch-up pass gives up for this tick. Lets a transient RPC throttle clear without dropping
	// the whole pass; the next poll tick retries from the same cursor regardless.
	maxScanRetries uint64 = 3
	// scanBatchPause spaces consecutive getLogs batches during a multi-batch catch-up, so a wide
	// catch-up doesn't storm a rate-limited RPC. Single-batch steady-state polling never hits it.
	scanBatchPause = 250 * time.Millisecond
)

// scanBatchBlocks is the configured eth_getLogs block-range batch size, defaulted.
func (o *OoORouterService) scanBatchBlocks() uint64 {
	if n := o.chainCfg.EventScanBatchBlocks; n > 0 {
		return n
	}
	return config.DefaultEventScanBatchBlocks
}

// scanEventRange fetches and processes this provider's DataRequested + RequestFulfilled events in the
// inclusive block range [from, to] via eth_getLogs over HTTP. It is the shared detection primitive for
// the startup catch-up and the HTTP poll loop (the websocket path uses live subscriptions instead).
// Re-processing an already-seen request is harmless - inserts dedupe on request_id.
//
// It filters by the Router address + the two event signatures (topic0) ONLY - NOT by the indexed
// provider - then matches the provider in Go. Filtering on an indexed arg makes go-ethereum send a
// positional topic filter with empty-array wildcards (e.g. [[sig],[],[provider],[]]), which some
// limited RPCs (notably block-explorer eth-rpc proxies, e.g. Puppynet's) reject with a non-standard
// error. A single-position topic0 filter is universally supported, and one getLogs fetches both event
// types (mirrors the dpv fulfilment-watcher).
func (o *OoORouterService) scanEventRange(from, to uint64) error {
	logs, err := o.client.FilterLogs(o.context, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{o.contractAddress},
		Topics:    [][]common.Hash{{o.logDataRequestedHash, o.logRequestFulfilledHash}},
	})
	if err != nil {
		return fmt.Errorf("getLogs [%d,%d]: %w", from, to, err)
	}

	for i := range logs {
		lg := logs[i]
		if len(lg.Topics) == 0 {
			continue
		}
		switch lg.Topics[0] {
		case o.logDataRequestedHash:
			ev, perr := o.contractInstance.ParseDataRequested(lg)
			if perr != nil {
				logger.ErrorWithFields("chain", "scanEventRange", "parse DataRequested", perr.Error(), logger.Fields{"tx": lg.TxHash.Hex()})
				continue
			}
			if ev.Provider != o.oracleAddress {
				continue // another provider's request - not ours to fulfil
			}
			o.processIncomingRequests(ev)
		case o.logRequestFulfilledHash:
			ev, perr := o.contractInstance.ParseRequestFulfilled(lg)
			if perr != nil {
				logger.ErrorWithFields("chain", "scanEventRange", "parse RequestFulfilled", perr.Error(), logger.Fields{"tx": lg.TxHash.Hex()})
				continue
			}
			if ev.Provider != o.oracleAddress {
				continue
			}
			o.processIncomingFulfilments(ev)
		}
	}
	return nil
}

// drainFrom scans every block from start up to the current head, in bounded batches, advancing the
// resume cursor per batch (so progress survives a mid-scan RPC error - the next call retries from the
// cursor). Shared by the startup catch-up (start = cursor, inclusive) and each poll tick (start =
// cursor + 1, strictly-new blocks - so a confirmed fulfilment, whose re-processing would re-count
// success metrics, is never re-scanned).
func (o *OoORouterService) drainFrom(start uint64) {
	head, err := o.client.BlockNumber(o.context)
	if err != nil {
		logger.Error("chain", "drainFrom", "get block number", err.Error())
		return
	}
	batch := o.scanBatchBlocks()
	for from := start; from <= head; {
		to := from + batch - 1
		if to > head {
			to = head
		}
		if err := o.scanRangeWithRetry(from, to); err != nil {
			// Still failing after retries: stop this pass without advancing the cursor, so the next
			// poll tick resumes from here. Avoids skipping events on a persistent RPC error.
			logger.ErrorWithFields("chain", "drainFrom", "scan event range", err.Error(), logger.Fields{
				"from": from, "to": to,
			})
			return
		}
		o.setLastBlockNumber(to)
		from = to + 1
		// Pace a multi-batch catch-up so a rate-limited RPC isn't stormed; interruptible by shutdown.
		if from <= head && !o.sleepCtx(scanBatchPause) {
			return
		}
	}
}

// scanRangeWithRetry runs scanEventRange with bounded exponential backoff, so a transient RPC error
// (a rate-limit throttle, a blip) retries within the tick instead of dropping the whole catch-up.
func (o *OoORouterService) scanRangeWithRetry(from, to uint64) error {
	b := backoff.WithContext(backoff.WithMaxRetries(backoff.NewExponentialBackOff(), maxScanRetries), o.context)
	attempt := 0
	return backoff.RetryNotify(
		func() error { return o.scanEventRange(from, to) },
		b,
		func(err error, d time.Duration) {
			attempt++
			logger.WarnWithFields("chain", "scanRangeWithRetry", "getLogs error - retrying", err.Error(), logger.Fields{
				"from": from, "to": to, "attempt": attempt, "backoff": d.String(),
			})
		},
	)
}

// sleepCtx waits for d, returning false if the context is cancelled first.
func (o *OoORouterService) sleepCtx(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-o.context.Done():
		return false
	}
}

// subscribe (re)establishes one event subscription with retry/backoff: it unsubscribes any existing
// subscription and retries watch until it connects. Unlike a hard dependency it RETURNS an error if it
// still can't connect within the backoff window, so the caller can fall back to HTTP polling rather
// than taking the oracle down. name is the log tag; watch performs the specific WatchXxx call.
func (o *OoORouterService) subscribe(name string, existing event.Subscription, watch func() (event.Subscription, error)) (event.Subscription, error) {
	if existing != nil {
		existing.Unsubscribe()
	}

	b := backoff.NewExponentialBackOff()
	// Keep this short: HTTP polling is a ready fallback, so it is better to degrade to polling quickly
	// than to spend many minutes retrying a websocket a free RPC may simply not offer.
	b.MaxElapsedTime = 2 * time.Minute

	var sub event.Subscription
	retryable := func() error {
		s, err := watch()
		sub = s
		return err
	}
	notify := func(err error, _ time.Duration) {
		logger.Error("chain", name, "init subscription", err.Error())
	}

	if err := backoff.RetryNotify(retryable, b, notify); err != nil {
		return nil, err
	}

	return sub, nil
}

func (o *OoORouterService) subscribeToDataRequested(me []common.Address) error {
	sub, err := o.subscribe("subscribeToDataRequested", o.subscriptionDr, func() (event.Subscription, error) {
		return o.wsContractInstance.WatchDataRequested(o.watchOpts, o.chanDataRequests, nil, me, nil)
	})
	if err != nil {
		return err
	}
	o.subscriptionDr = sub
	return nil
}

func (o *OoORouterService) subscribeToRequestFulfilled(me []common.Address) error {
	sub, err := o.subscribe("subscribeToRequestFulfilled", o.subscriptionRf, func() (event.Subscription, error) {
		return o.wsContractInstance.WatchRequestFulfilled(o.watchOpts, o.chanRequestFulfilled, nil, me, nil)
	})
	if err != nil {
		return err
	}
	o.subscriptionRf = sub
	return nil
}

// unsubscribeAll tears down any live event subscriptions. Called when degrading from websocket to poll
// mode so a half-established subscription doesn't dangle. Unsubscribe is idempotent, so Shutdown
// calling it again later is harmless.
func (o *OoORouterService) unsubscribeAll() {
	if o.subscriptionDr != nil {
		o.subscriptionDr.Unsubscribe()
	}
	if o.subscriptionRf != nil {
		o.subscriptionRf.Unsubscribe()
	}
}

// RunEventWatchers detects on-chain events for this worker until the context is cancelled, choosing the
// transport automatically: live websocket subscriptions when a working eth_ws_host is configured,
// otherwise periodic HTTP polling. If the websocket path is configured but fails - at startup, or by
// dropping mid-run and not re-establishing within the backoff - it degrades to polling rather than
// taking the worker down. Most operators run on free RPCs where WSS is flaky or absent.
func (o *OoORouterService) RunEventWatchers() {
	if o.wsContractInstance != nil {
		logger.Info("chain", "RunEventWatchers", "", "detecting events via websocket subscriptions")
		if o.runSubscribeMode() {
			return // clean shutdown (context cancelled)
		}
		o.unsubscribeAll()
		logger.Warn("chain", "RunEventWatchers", "",
			"websocket subscriptions unavailable - falling back to HTTP polling")
	} else {
		logger.Info("chain", "RunEventWatchers", "",
			"no websocket endpoint configured - detecting events via HTTP polling")
	}
	o.runPollMode()
}

// runSubscribeMode runs the live websocket event loop. It returns true when it exits cleanly (context
// cancelled) and false when the websocket fails terminally (initial subscribe or a re-subscribe gives
// up), signalling the caller to fall back to polling.
//
// Teardown (Unsubscribe + save resume block) on a clean shutdown is owned by Shutdown(), which Stop()
// calls on the same cancellation - so there is a single unsubscribe path and no race there.
func (o *OoORouterService) runSubscribeMode() bool {
	me := []common.Address{o.oracleAddress}

	if err := o.subscribeToDataRequested(me); err != nil {
		logger.Error("chain", "runSubscribeMode", "subscribe to DataRequested", err.Error())
		return false
	}
	if err := o.subscribeToRequestFulfilled(me); err != nil {
		logger.Error("chain", "runSubscribeMode", "subscribe to RequestFulfilled", err.Error())
		return false
	}

	for {
		select {
		case <-o.context.Done():
			logger.Info("chain", "runSubscribeMode", "", "context cancelled - stopping event watchers")
			return true
		case ev := <-o.chanDataRequests:
			o.processIncomingRequests(ev)
		case ev := <-o.chanRequestFulfilled:
			o.processIncomingFulfilments(ev)
		case subErr := <-o.subscriptionDr.Err():
			if subErr != nil {
				logger.Error("chain", "runSubscribeMode", "DataRequested subscription connection error", subErr.Error())
				if err := o.subscribeToDataRequested(me); err != nil {
					logger.Error("chain", "runSubscribeMode", "re-subscribe to DataRequested", err.Error())
					return false
				}
			}
		case subErr := <-o.subscriptionRf.Err():
			if subErr != nil {
				logger.Error("chain", "runSubscribeMode", "RequestFulfilled subscription connection error", subErr.Error())
				if err := o.subscribeToRequestFulfilled(me); err != nil {
					logger.Error("chain", "runSubscribeMode", "re-subscribe to RequestFulfilled", err.Error())
					return false
				}
			}
		}
	}
}

// runPollMode detects events by scanning eth_getLogs over HTTP on a ticker, from the last-seen block up
// to head. It is the fallback when no websocket is available; a few seconds' detection latency is
// immaterial given the confirmation wait before fulfilment. In poll mode the resume cursor is persisted
// every tick, so there is no separate save on shutdown. Returns when the context is cancelled.
func (o *OoORouterService) runPollMode() {
	interval := o.eventPollInterval()
	logger.InfoWithFields("chain", "runPollMode", "", "polling for events over HTTP", logger.Fields{
		"interval":   interval.String(),
		"network_id": o.networkId,
	})

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Catch up immediately, then on every tick.
	o.drainFrom(o.lastBlockNumber + 1)
	for {
		select {
		case <-o.context.Done():
			logger.Info("chain", "runPollMode", "", "context cancelled - stopping event poller")
			return
		case <-ticker.C:
			o.drainFrom(o.lastBlockNumber + 1)
		}
	}
}

// eventPollInterval is the HTTP event-poll cadence from chain.event_poll_interval_sec, defaulted.
func (o *OoORouterService) eventPollInterval() time.Duration {
	secs := o.chainCfg.EventPollIntervalSec
	if secs == 0 {
		secs = config.DefaultEventPollIntervalSec
	}
	return time.Second * time.Duration(secs)
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

	// check status and if request already exists
	reqDbRes, found, err := o.db.FindByRequestId(o.networkId, requestId)
	if err != nil {
		// A real DB error (not just not-found): don't risk a duplicate insert by treating it
		// as new - log + skip. The block number isn't advanced, so the event is re-seen.
		logger.ErrorWithFields("chain", "processIncomingRequests", "find request in db", err.Error(),
			logger.Fields{"requestId": requestId})
		return
	}

	if !found {
		logger.InfoWithFields("chain", "processIncomingRequests", "add job to db", "new request", logger.Fields{
			"requestId": requestId,
		})

		err = o.db.InsertNewRequest(
			o.networkId,
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
		)

		if err != nil {
			logger.ErrorWithFields("chain", "processIncomingRequests", "insert new request", err.Error(),
				logger.Fields{"requestId": requestId})
		} else {
			// Process the new request immediately rather than waiting up to a full ticker
			// interval for the periodic sweep to pick it up.
			o.nudgeJobQueue()
		}
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
	// check status and if request already exists
	request, found, err := o.db.FindByRequestId(o.networkId, requestId)
	if err != nil {
		// A real DB error (not just not-found): log + skip so the block isn't advanced and the
		// confirmation is retried, rather than the fulfilment being silently lost.
		logger.ErrorWithFields("chain", "processIncomingFulfilments", "find request in db", err.Error(),
			logger.Fields{"request_id": requestId})
		return
	}

	if found {
		logger.InfoWithFields("chain", "processIncomingFulfilments", "confirm fulfillment",
			"confirmed request fulfilment for request",
			logger.Fields{
				"request_id": requestId,
			})

		err := o.db.UpdateFulfillmentSuccess(
			o.networkId,
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

		fulfilmentResultTotal.WithLabelValues(o.chainLabel(), "success").Inc()
		if gasUsed > 0 {
			fulfilmentGasUsed.WithLabelValues(o.chainLabel()).Observe(float64(gasUsed))
		}
		if request.RequestBlockNumber > 0 && event.Raw.BlockNumber >= request.RequestBlockNumber {
			fulfilmentBlocks.WithLabelValues(o.chainLabel()).Observe(float64(event.Raw.BlockNumber - request.RequestBlockNumber))
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
		logger.ErrorWithFields("chain", "processGasUsage", "get TransactionReceipt", err.Error(), logger.Fields{
			"tx_hash": evLog.TxHash,
		})
	}

	tx, _, err := o.client.TransactionByHash(o.context, evLog.TxHash)
	if err == nil {
		// todo - need to clean up and gather any missing data if Tx query above fails
		gasPrice = tx.GasPrice().Uint64()
	} else {
		logger.ErrorWithFields("chain", "processGasUsage", "get TransactionByHash", err.Error(), logger.Fields{
			"tx_hash": evLog.TxHash,
		})
	}

	return gasPrice, gasUsed
}
