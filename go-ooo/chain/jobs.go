package chain

import (
	"fmt"
	"math/big"

	"go-ooo/config"
	"go-ooo/database/models"
	"go-ooo/logger"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	solsha3 "github.com/miguelmota/go-solidity-sha3"
	"github.com/spf13/viper"
)

func (o *OoORouterService) ProcessPendingJobQueue() {

	logger.Info("chain", "ProcessPendingJobQueue", "check job queue", "")

	// get pending requests from data_requests table
	requests, err := o.db.GetPendingJobs()

	if err != nil {
		logger.Error("chain", "ProcessPendingJobQueue", "get job queue", err.Error())
		return
	}

	if len(requests) > 0 {
		// for checking sent fulfilments etc.
		currentBlockNum, err := o.client.BlockNumber(o.context)

		if err != nil {
			logger.Error("chain", "ProcessPendingJobQueue", "get block num", err.Error())

			return
		}

		for _, request := range requests {
			// process
			o.preProcessPendingJob(request, currentBlockNum)
		}
	}
}

func (o *OoORouterService) preProcessPendingJob(job models.DataRequests, currentBlockNum uint64) {
	requestId := job.GetRequestId()
	logger.InfoWithFields("chain", "preProcessPendingJob", "preprocess job", "", logger.Fields{
		"request_id": requestId,
		"status":     job.GetRequestStatusString(),
	})

	// get request Tx receipt from chain
	requestTxReceipt, err := o.client.TransactionReceipt(o.context, common.HexToHash(job.GetRequestTxHash()))
	if err != nil {
		// possibly not in Tx pool yet
		logger.ErrorWithFields("chain", "preProcessPendingJob", "get tx receipt from chain",
			err.Error(), logger.Fields{
				"request_id": requestId,
				"status":     job.GetRequestStatusString(),
			})

		return
	}

	requestBlockDiff := currentBlockNum - requestTxReceipt.BlockNumber.Uint64()
	switch job.GetRequestStatus() {
	case models.REQUEST_STATUS_INITIALISED:
		waitConfirmations := viper.GetUint64(config.JobsWaitConfirmations)
		if requestBlockDiff >= waitConfirmations {
			go func(o *OoORouterService, job models.DataRequests) {
				o.processFulfillmentFetchData(job, currentBlockNum)
			}(o, job)
		} else {
			// log it
			logger.WarnWithFields("chain", "preProcessPendingJob", "check confirmations for initialised job",
				"not enough block confirmations to fulfill request",
				logger.Fields{
					"request_id":    requestId,
					"request_block": requestTxReceipt.BlockNumber.Uint64(),
					"current_block": currentBlockNum,
					"block_diff":    requestBlockDiff,
					"wait_config":   waitConfirmations,
				})
		}
		return
	case models.REQUEST_STATUS_DATA_READY_TO_SEND:
		o.sendFulfillmentTx(job, currentBlockNum)
		return
	case models.REQUEST_STATUS_TX_FAILED:
		o.processSendFailedJob(job, currentBlockNum)
		return
	case models.REQUEST_STATUS_API_ERROR:
		o.processSendFailedJob(job, currentBlockNum)
		return
	case models.REQUEST_STATUS_FETCHING_DATA:
		o.processPossiblyStuckDataFetch(job, currentBlockNum)
		return
	case models.REQUEST_STATUS_TX_SENT:
		o.processPossiblyStuckSentTx(job, currentBlockNum)
		return
	default:
		return
	}

}

func (o *OoORouterService) processFulfillmentFetchData(job models.DataRequests, currentBlockNum uint64) {

	requestId := job.GetRequestId()

	logger.Debug("chain", "processFulfillmentFetchData", "", "begin fetching data",
		logger.Fields{
			"request_id": requestId,
		})

	err := o.RenewTransactOpts()
	if err != nil {
		logger.ErrorWithFields("chain", "processFulfillmentFetchData", "RenewTransactOpts",
			err.Error(),
			logger.Fields{
				"request_id": requestId,
			})

		return
	}

	err = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_FETCHING_DATA, "")

	if err != nil {
		// possibly not in Tx pool yet
		logger.ErrorWithFields("chain", "processFulfillmentFetchData", "update processing status in db",
			err.Error(),
			logger.Fields{
				"request_id": requestId,
			})

		return
	}

	err = o.db.IncrementFulfillmentAttempts(requestId)

	if err != nil {
		// possibly not in Tx pool yet
		logger.ErrorWithFields("chain", "processFulfillmentFetchData", "update fulfilment attempts in db",
			err.Error(),
			logger.Fields{
				"request_id": requestId,
			})
		return
	}

	err = o.db.UpdateLastDataFetchBlockNumber(requestId, currentBlockNum)

	if err != nil {
		// possibly not in Tx pool yet
		logger.ErrorWithFields("chain", "processFulfillmentFetchData", "update last fetch blocknum in db",
			err.Error(),
			logger.Fields{
				"request_id": requestId,
			})

		return
	}

	endpoint := job.GetEndpointDecoded()

	price, err := o.oooApi.RouteQuery(endpoint, requestId)

	if err != nil {
		logger.ErrorWithFields("chain", "processFulfillmentFetchData", "run api query",
			err.Error(),
			logger.Fields{
				"request_id": requestId,
			})

		_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_API_ERROR, err.Error())
		return
	}

	if price == "" {
		// no price returned
		logger.ErrorWithFields("chain", "processFulfillmentFetchData", "api query result",
			"empty price returned",
			logger.Fields{
				"request_id": requestId,
			})

		_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_API_ERROR, "empty price returned")
		return
	}

	logger.Debug("chain", "processFulfillmentFetchData", "",
		"price fetched",
		logger.Fields{
			"request_id": requestId,
			"endpoint":   job.Endpoint,
			"price":      price,
		})

	_ = o.db.UpdateDataFetched(requestId, price)

	return
}

func (o *OoORouterService) sendFulfillmentTx(job models.DataRequests, currentBlockNum uint64) {
	requestId := job.GetRequestId()
	price := job.GetPriceResult()

	logger.Debug("chain", "sendFulfillmentTx", "",
		"begin send fulfillment transaction",
		logger.Fields{
			"request_id": requestId,
			"endpoint":   job.Endpoint,
			"price":      price,
		})

	// https://ethereum.stackexchange.com/questions/51566/from-golang-sha3-to-solidity-sha3
	priceBigInt := big.NewInt(0)
	priceBigInt.SetString(price, 10)

	reqIdBytes := common.FromHex(requestId)
	reqIdBytes32 := [32]byte{}
	copy(reqIdBytes32[:], reqIdBytes)

	hash := solsha3.SoliditySHA3(
		solsha3.Bytes32(reqIdBytes),
		solsha3.Uint256(price),
		solsha3.Address(job.Consumer),
	)

	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n32%s", hash)
	msgHash := crypto.Keccak256Hash([]byte(msg))

	signatureBytes, err := crypto.Sign(msgHash.Bytes(), o.oraclePrivateKey)

	if err != nil {
		logger.ErrorWithFields("chain", "sendFulfillmentTx", "sign message",
			err.Error(),
			logger.Fields{
				"request_id": requestId,
			})

		_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_TX_FAILED, err.Error())
		return
	}

	// grr - https://ethereum.stackexchange.com/questions/45580/validating-go-ethereum-key-signature-with-ecrecover
	signatureBytes[64] = uint8(int(signatureBytes[64])) + 27

	tx, err := o.contractInstance.FulfillRequest(o.transactOpts, reqIdBytes32, priceBigInt, signatureBytes)

	if err != nil {
		logger.ErrorWithFields("chain", "sendFulfillmentTx", "send tx",
			err.Error(),
			logger.Fields{
				"request_id": requestId,
			})

		_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_TX_FAILED, err.Error())
		return
	}

	logger.InfoWithFields("chain", "sendFulfillmentTx", "send tx",
		"fulfill tx sent",
		logger.Fields{
			"request_id": requestId,
			"tx":         tx.Hash().Hex(),
		})

	_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_TX_SENT, "")
	_ = o.db.UpdateFulfillmentSent(requestId, tx.Hash().Hex(), currentBlockNum)

	o.setNextTxNonce(tx.Nonce(), false)

	_ = o.RenewTransactOpts()
}

func (o *OoORouterService) processPossiblyStuckDataFetch(job models.DataRequests, currentBlockNum uint64) {
	requestId := job.GetRequestId()

	logger.InfoWithFields("chain", "processPossiblyStuckDataFetch", "start",
		"begin processing possibly stuck data fetch",
		logger.Fields{
			"request_id": requestId,
		})

	// at some point, we just have to stop trying...
	if job.GetFulfillmentAttempts() >= 3 {
		// too many fails
		logger.WarnWithFields("chain", "processPossiblyStuckDataFetch", "check num attempts",
			"too many fails",
			logger.Fields{
				"request_id":   requestId,
				"num_attempts": job.GetFulfillmentAttempts(),
			})

		_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_FULFILMENT_FAILED, "too many failed attempts")
		return
	}

	lastFetchBlockDiff := currentBlockNum - job.LastDataFetchBlockNumber

	// still relatively new - ignore
	if lastFetchBlockDiff < 5 {
		logger.InfoWithFields("chain", "processPossiblyStuckDataFetch", "check request age",
			"request < 5 blocks. Wait for data fetch timeout",
			logger.Fields{
				"request_id": requestId,
			})

		return
	}

	requestBlockDiff := currentBlockNum - job.RequestBlockNumber

	// is the request > 1 hour old?
	if requestBlockDiff > 250 {
		logger.WarnWithFields("chain", "processPossiblyStuckDataFetch", "check request age",
			"request too old",
			logger.Fields{
				"request_id": requestId,
				"age_blocks": requestBlockDiff,
			})

		_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_FULFILMENT_FAILED, "request too old")
		return
	}

	// finally, try to re-fetch data for fulfillment
	o.processFulfillmentFetchData(job, currentBlockNum)
}

// processSendFailedJob will try to see why a fulfilment didn't even send, and resend
func (o *OoORouterService) processSendFailedJob(job models.DataRequests, currentBlockNum uint64) {

	requestId := job.GetRequestId()
	logger.Debug("chain", "processSendFailedJob", "start",
		"begin processing send failed job",
		logger.Fields{
			"request_id": requestId,
		})

	// Add fail info to failed Tx history table
	_ = o.db.InsertNewFailedFulfilment(requestId, "", 0, 0, job.GetStatusReason())

	// at some point, we just have to stop trying...
	if job.GetFulfillmentAttempts() >= 3 {
		// too many fails
		logger.WarnWithFields("chain", "processSendFailedJob", "check num attempts",
			"too many failed attempts",
			logger.Fields{
				"request_id":   requestId,
				"num_attempts": job.GetFulfillmentAttempts(),
			})

		_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_FULFILMENT_FAILED, "too many failed attempts")
		return
	}

	requestBlockDiff := currentBlockNum - job.RequestBlockNumber

	// is the request > 1 hour old?
	if requestBlockDiff > 250 {
		logger.WarnWithFields("chain", "processSendFailedJob", "check request age",
			"request too old",
			logger.Fields{
				"request_id": requestId,
				"age_blocks": requestBlockDiff,
			})

		_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_FULFILMENT_FAILED, "request too old")
		return
	}

	// finally, try to re-fetch data for fulfillment
	o.processFulfillmentFetchData(job, currentBlockNum)

}

func (o *OoORouterService) processPossiblyStuckSentTx(job models.DataRequests, currentBlockNum uint64) {
	requestId := job.GetRequestId()
	logger.Debug("chain", "processPossiblyStuckSentTx", "start",
		"begin processing possibly stuck sent tx",
		logger.Fields{
			"request_id": requestId,
		})

	lastFulfillSentBlockDiff := currentBlockNum - job.GetLastFulfillSentBlockNumber()
	if lastFulfillSentBlockDiff < 3 {
		// too soon - may take a while for Tx to be broadcast/picked up
		logger.InfoWithFields("chain", "processPossiblyStuckSentTx", "check block diff since fulfill tx sent",
			"not enough blocks since last sent. Wait.",
			logger.Fields{
				"request_id": requestId,
				"block_diff": lastFulfillSentBlockDiff,
			})

		return
	}

	fulfilTxHash := common.HexToHash(job.GetFulfillTxHash())
	// check if it's pending
	_, isPending, err := o.client.TransactionByHash(o.context, fulfilTxHash)

	if err != nil {
		// possibly not in Tx pool yet
		logger.ErrorWithFields("chain", "processPossiblyStuckSentTx", "get fulfill tx",
			err.Error(),
			logger.Fields{
				"request_id": requestId,
				"tx_hash":    job.GetFulfillTxHash(),
			})

		return
	}

	// no point continuing if it's still pending. Log it and move on.
	if isPending {
		logger.InfoWithFields("chain", "processPossiblyStuckSentTx", "check fulfill tx pending",
			"tx still pending - ignore",
			logger.Fields{
				"request_id": requestId,
				"tx_hash":    job.GetFulfillTxHash(),
			})
		return
	}

	// try and get the receipt
	fulfillReceipt, err := o.client.TransactionReceipt(o.context, fulfilTxHash)
	if err != nil {
		logger.ErrorWithFields("chain", "processPossiblyStuckSentTx", "get fulfil tx receipt",
			err.Error(),
			logger.Fields{
				"request_id": job.GetRequestId(),
				"tx_hash":    job.GetFulfillTxHash(),
			})
		return
	}

	if fulfillReceipt.Status == 1 {
		// Tx was successful. double check for RandomnessRequestFulfilled event
		// in case it was missed
		logger.InfoWithFields("chain", "processPossiblyStuckSentTx", "check fulfill tx status",
			"tx was successful. check for RequestFulfilled event",
			logger.Fields{
				"request_id": job.GetRequestId(),
			})

		reqIdBytes := common.FromHex(requestId)
		reqIdBytes32 := [32]byte{}
		copy(reqIdBytes32[:], reqIdBytes)
		reqArr := make([][32]byte, 0, 1)
		reqArr = append(reqArr, reqIdBytes32)
		opts := o.historicalFilterOpts
		opts.Start = job.RequestBlockNumber
		itrFr, err := o.contractInstance.FilterRequestFulfilled(opts, nil, nil, reqArr)
		if err != nil {
			logger.Error("chain", "processPossiblyStuckSentTx", "get FilterRequestFulfilled events",
				err.Error())
			return
		}

		for itrFr.Next() {
			event := itrFr.Event
			o.processIncomingFulfilments(event)
		}
		return
	}

	// Tx has failed - process
	// used later to store failed fulfill tx history
	failedGasUsed := job.GetFulfillGasUsed()
	failedGasPrice := job.GetFulfillGasPrice()
	failReason := "tx reverted" // todo - try to get revert reason from receipt

	// Add fail info to failed Tx history table
	_ = o.db.InsertNewFailedFulfilment(requestId, fulfilTxHash.Hex(), failedGasUsed, failedGasPrice, failReason)

	// at some point, we just have to stop trying...
	if job.GetFulfillmentAttempts() >= 3 {
		// too many fails
		logger.WarnWithFields("chain", "processPossiblyStuckSentTx", "check num attempts",
			"too many failed attempts",
			logger.Fields{
				"request_id":   requestId,
				"num_attempts": job.GetFulfillmentAttempts(),
			})

		_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_FULFILMENT_FAILED, "too many failed attempts")
		return
	}

	requestBlockDiff := currentBlockNum - job.RequestBlockNumber

	// is the request > 1 hour?
	if requestBlockDiff > 250 {
		logger.WarnWithFields("chain", "processPossiblyStuckSentTx", "check request age",
			"request too old",
			logger.Fields{
				"request_id": requestId,
				"age_blocks": requestBlockDiff,
			})

		_ = o.db.UpdateRequestStatus(requestId, models.REQUEST_STATUS_FULFILMENT_FAILED, "request too old")
		return
	}

	// finally, try to send a new fulfillment
	o.sendFulfillmentTx(job, currentBlockNum)

	return
}
