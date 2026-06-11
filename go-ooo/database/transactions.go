package database

import (
	"errors"
	"fmt"

	"go-ooo/database/models"
	"go-ooo/logger"

	"gorm.io/gorm"
)

/*
  DataRequests table
*/

func (d *DB) InsertNewRequest(provider string,
	consumer string, requestId string,
	endpoint string, endpointDecoded string,
	txHash string, gasUsed uint64, gasPrice uint64,
	fee uint64, blockNumber uint64, isAdhoc bool) (err error) {
	err = d.Omit("FulfilTx").Create(&models.DataRequests{
		Provider:            provider,
		Consumer:            consumer,
		RequestId:           requestId,
		Endpoint:            endpoint,
		EndpointDecoded:     endpointDecoded,
		RequestTxHash:       txHash,
		RequestGasUsed:      gasUsed,
		RequestGasPrice:     gasPrice,
		RequestBlockNumber:  blockNumber,
		Fee:                 fee,
		RequestStatus:       models.REQUEST_STATUS_INITIALISED,
		FulfillmentAttempts: 0,
		IsAdhoc:             isAdhoc,
		JobStatus:           models.JOB_STATUS_PENDING,
	}).Error
	return
}

// updateRequest loads a request by id, applies mutate, and saves it - factoring the
// load + save boilerplate the simple Update* methods share. The read-modify-write is fine
// here: fulfilment runs on the single serialised job ticker, so there's no concurrent writer.
func (d *DB) updateRequest(requestId string, mutate func(*models.DataRequests)) error {
	req := models.DataRequests{}
	if err := d.Where("request_id = ?", requestId).First(&req).Error; err != nil {
		return err
	}
	mutate(&req)
	return d.Save(&req).Error
}

func (d *DB) UpdateFulfillmentSuccess(requestId string, blockNumber uint64,
	txHash string, gasUsed uint64, gasPrice uint64) error {
	return d.updateRequest(requestId, func(req *models.DataRequests) {
		req.RequestStatus = models.REQUEST_STATUS_SUCCESS
		req.JobStatus = models.JOB_STATUS_SUCCESS
		req.FulfillConfirmedBlockNumber = blockNumber
		req.FulfillTxHash = txHash
		req.FulfillGasUsed = gasUsed
		req.FulfillGasPrice = gasPrice
	})
}

func (d *DB) UpdateFulfillmentSent(requestId string, txHash string, blockNumber uint64, nonce uint64, gasPrice uint64, gasTipCap uint64) error {
	return d.updateRequest(requestId, func(req *models.DataRequests) {
		req.FulfillTxHash = txHash
		req.LastFulfillSentBlockNumber = blockNumber
		req.FulfillNonce = nonce
		req.FulfillGasPrice = gasPrice
		req.FulfillGasTipCap = gasTipCap
	})
}

func (d *DB) IncrementFulfillmentAttempts(requestId string) error {
	// Atomic increment in one statement, rather than read-modify-write (Save), so the
	// counter can't lose an update and it's a single round-trip.
	return d.Model(&models.DataRequests{}).
		Where("request_id = ?", requestId).
		Update("fulfillment_attempts", gorm.Expr("fulfillment_attempts + ?", 1)).Error
}

func (d *DB) UpdateRequestStatus(requestId string, status int, reason string) error {
	req := models.DataRequests{}
	err := d.Where("request_id = ?", requestId).First(&req).Error
	if err != nil {
		return err
	}

	from := req.RequestStatus

	// Terminal states are inert: never re-drive a SUCCESS / FULFILMENT_FAILED job.
	if models.IsTerminalRequestStatus(from) && status != from {
		logger.WarnWithFields("database", "UpdateRequestStatus", "guard",
			"refusing to move a job out of a terminal status",
			logger.Fields{"request_id": requestId, "from": from, "to": status})
		return nil
	}

	// Observe (but still apply) an unforeseen transition, so an unexpected edge shows
	// up in the logs without stalling the job.
	if !models.IsExpectedTransition(from, status) {
		logger.WarnWithFields("database", "UpdateRequestStatus", "guard",
			"unexpected status transition (applied)",
			logger.Fields{"request_id": requestId, "from": from, "to": status})
	}

	req.RequestStatus = status
	req.StatusReason = reason
	req.JobStatus = models.JobStatusForRequestStatus(status)

	return d.Save(&req).Error
}

func (d *DB) UpdateJobStatus(requestId string, status int) error {
	return d.updateRequest(requestId, func(req *models.DataRequests) {
		req.JobStatus = status
	})
}

func (d *DB) UpdateDataFetched(requestId string, price string) error {
	return d.updateRequest(requestId, func(req *models.DataRequests) {
		req.RequestStatus = models.REQUEST_STATUS_DATA_READY_TO_SEND
		req.PriceResult = price
	})
}

func (d *DB) UpdateLastDataFetchBlockNumber(requestId string, blockNum uint64) error {
	return d.updateRequest(requestId, func(req *models.DataRequests) {
		req.LastDataFetchBlockNumber = blockNum
	})
}

/*
  ToBlocks table
*/

func (d *DB) InsertNewToBlock(toBlock uint64) error {
	last, err := d.GetLastBlockNumQueried()
	// An empty table (no rows yet) is fine - that's the first insert. Any other error
	// means we don't know the current head, so don't write a resume point on top of it.
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if last.GetBlockNum() < toBlock {
		return d.Create(&models.ToBlocks{BlockNum: toBlock}).Error
	}

	return nil
}

/*
  SupportedPairs table
*/

func (d *DB) AddNewSupportedPair(name string, base string, target string) (err error) {
	err = d.Create(&models.SupportedPairs{
		Name:   name,
		Base:   base,
		Target: target,
	}).Error
	return
}

/*
  FailedFulfillments table
*/

func (d *DB) InsertNewFailedFulfilment(requestId string, txHash string, gasUsed uint64, gasPrice uint64, reason string) (err error) {
	err = d.Create(&models.FailedFulfilment{
		RequestId:  requestId,
		TxHash:     txHash,
		GasUsed:    gasUsed,
		GasPrice:   gasPrice,
		FailReason: reason,
	}).Error
	return
}

/*
  DexPairs
*/

func (d *DB) FindOrInsertNewDexPair(t0Symbol string, t1Symbol string, chain string,
	contractAddress string, dexName string, t0DbId uint, t1DbId uint, reserveUsd float64, txCount uint64) (models.DexPairs, error) {

	pair, err := d.FindByDexChainAddress(chain, dexName, contractAddress)

	if pair.ID == 0 {
		return d.InsertNewDexPair(t0Symbol, t1Symbol, chain, contractAddress, dexName, t0DbId, t1DbId, reserveUsd, txCount)
	} else {
		// update Reserve value
		pair.ReserveUsd = reserveUsd
		err = d.Save(&pair).Error
	}

	return pair, err
}

func (d *DB) InsertNewDexPair(t0Symbol string, t1Symbol string, chain string,
	contractAddress string, dexName string, t0DbId uint, t1DbId uint, reserveUsd float64, txCount uint64) (models.DexPairs, error) {

	data := models.DexPairs{
		Chain:           chain,
		Dex:             dexName,
		Pair:            fmt.Sprintf("%s-%s", t0Symbol, t1Symbol),
		T0TokenId:       t0DbId,
		T1TokenId:       t1DbId,
		T0Symbol:        t0Symbol,
		T1Symbol:        t1Symbol,
		ContractAddress: contractAddress,
		ReserveUsd:      reserveUsd,
		TxCount:         txCount,
		Verified:        true,
	}

	err := d.Create(&data).Error

	return data, err
}

func (d *DB) UpdateDexPairMetaData(chain string, dex string,
	contractAddress string, reserveUsd float64, txCount uint64) (err error) {

	pair, err := d.FindByDexChainAddress(chain, dex, contractAddress)

	if pair.ID == 0 {
		return fmt.Errorf(`%s, %s, %s not found in db`, chain, dex, contractAddress)
	}

	pair.ReserveUsd = reserveUsd
	pair.TxCount = txCount
	return d.Save(&pair).Error
}

/*
  TokenContracts
*/

func (d *DB) FindOrInsertNewTokenContract(symbol string, contractAddress string, chain string) (models.TokenContracts, error) {
	res, err := d.FindByChainAndAddress(chain, contractAddress)
	if res.ID == 0 {
		return d.InsertNewTokenContract(symbol, contractAddress, chain)
	}
	return res, err
}

func (d *DB) UpdateOrInsertNewTokenContract(symbol string, contractAddress string, chain string) (models.TokenContracts, error) {
	res, err := d.FindByTokenAndAddress(symbol, contractAddress)
	if res.ID == 0 {
		return d.InsertNewTokenContract(symbol, contractAddress, chain)
	} else {
		res.ContractAddress = contractAddress
		err = d.Save(&res).Error
	}
	return res, err
}

func (d *DB) InsertNewTokenContract(symbol string, contractAddress string, chain string) (models.TokenContracts, error) {

	data := models.TokenContracts{
		TokenSymbol:     symbol,
		ContractAddress: contractAddress,
		Chain:           chain,
	}

	err := d.Create(&data).Error

	return data, err
}
