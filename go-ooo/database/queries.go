package database

import (
	"errors"
	"fmt"
	"time"

	"go-ooo/database/models"

	"gorm.io/gorm"
)

/*
  ToBlocks Queries
*/

func (d DB) GetLastBlockNumQueried() (models.ToBlocks, error) {
	toBlock := models.ToBlocks{}
	err := d.Last(&toBlock).Error
	return toBlock, err
}

/*
  DataRequests Queries
*/

// FindByRequestId looks up a request. The bool reports whether a row was found, so callers
// can tell "not found" apart from a real DB error (GORM's First conflates them via the zero
// struct) and not treat a transient failure as a brand-new request.
func (d *DB) FindByRequestId(requestId string) (models.DataRequests, bool, error) {
	result := models.DataRequests{}
	err := d.Where("request_id = ?", requestId).First(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, false, nil
	}
	return result, err == nil, err
}

func (d *DB) GetPendingJobs() ([]models.DataRequests, error) {
	var jobs = []models.DataRequests{}
	err := d.Where("job_status = ?",
		models.JOB_STATUS_PENDING).Order(fmt.Sprintf("id %s", "asc")).Find(&jobs).Error
	return jobs, err
}

// CountFulfilmentsSent counts requests that have been broadcast at least once (have a fulfil tx
// hash). Used to warm-start the fulfilment metrics from history.
func (d *DB) CountFulfilmentsSent() (int64, error) {
	var n int64
	err := d.Model(&models.DataRequests{}).Where("fulfill_tx_hash <> ''").Count(&n).Error
	return n, err
}

// CountRequestsByStatus counts requests in a given RequestStatus.
func (d *DB) CountRequestsByStatus(status int) (int64, error) {
	var n int64
	err := d.Model(&models.DataRequests{}).Where("request_status = ?", status).Count(&n).Error
	return n, err
}

// CountFailedFulfilments counts failed (reverted) fulfilment-tx attempts recorded in history.
func (d *DB) CountFailedFulfilments() (int64, error) {
	var n int64
	err := d.Model(&models.FailedFulfilment{}).Count(&n).Error
	return n, err
}

func (d *DB) GetLastXSuccessfulRequests(limit int, consumer string) ([]models.DataRequests, error) {
	var requests = []models.DataRequests{}
	var err error

	where := map[string]interface{}{"job_status": models.JOB_STATUS_SUCCESS}
	if len(consumer) > 0 {
		where = map[string]interface{}{"job_status": models.JOB_STATUS_SUCCESS, "consumer": consumer}
	}

	if limit > 0 {
		err = d.Where(where).Order(fmt.Sprintf("id %s", "desc")).Limit(limit).Find(&requests).Error
	} else {
		err = d.Where(where).Order(fmt.Sprintf("id %s", "desc")).Find(&requests).Error
	}
	return requests, err
}

func (d *DB) GetMostGasUsed() (models.DataRequests, error) {
	request := models.DataRequests{}
	err := d.Where("job_status = ?", models.JOB_STATUS_SUCCESS).Order(fmt.Sprintf("fulfill_gas_used %s", "desc")).Limit(1).First(&request).Error
	return request, err
}

func (d *DB) GetLeastGasUsed() (models.DataRequests, error) {
	request := models.DataRequests{}
	err := d.Where("job_status = ?", models.JOB_STATUS_SUCCESS).Order(fmt.Sprintf("fulfill_gas_used %s", "asc")).Limit(1).First(&request).Error
	return request, err
}

/*
  DexPairs queries
*/

func (d *DB) FindByDexPairName(base, target, chain, dexName string) ([]models.DexPairs, error) {
	pair := fmt.Sprintf("%s-%s", base, target)
	pairRev := fmt.Sprintf("%s-%s", target, base)
	var result []models.DexPairs
	err := d.Where(
		"(pair = ? OR pair = ?) AND chain = ? AND dex = ? AND verified = ?", pair, pairRev, chain, dexName, true,
	).Order("reserve_usd desc").Find(&result).Error
	return result, err
}

func (d *DB) FindByDexChainAddress(chain, dex, contractAddress string) (models.DexPairs, error) {
	result := models.DexPairs{}
	err := d.Where(
		"chain = ? AND dex = ? AND contract_address = ?", chain, dex, contractAddress,
	).Order("reserve_usd desc").First(&result).Error
	return result, err
}

func (d *DB) Get100PairsForDataRefresh(chain, dex string) ([]models.DexPairs, error) {
	var res []models.DexPairs
	duration, _ := time.ParseDuration("-6h")
	qTime := time.Now().Add(duration)
	err := d.Where("chain = ? AND dex = ? AND updated_at <= ? AND verified = ?", chain, dex, qTime, true).Limit(100).Find(&res).Error
	return res, err
}

/*
  SupportedSources queries
*/

// GetSupportedSources returns the persisted DEX source catalogue (the last-synced manifest),
// excluding any source soft-deleted because it dropped out of a later manifest.
func (d *DB) GetSupportedSources() ([]models.SupportedSource, error) {
	var sources []models.SupportedSource
	err := d.Order("chain asc, dex asc").Find(&sources).Error
	return sources, err
}

// FindSupportedSourceByChainDex looks up a single source by its (chain, dex) key. A zero-ID result
// means no row was found (the upsert path treats that as "insert").
func (d *DB) FindSupportedSourceByChainDex(chain, dex string) (models.SupportedSource, error) {
	result := models.SupportedSource{}
	err := d.Where("chain = ? AND dex = ?", chain, dex).First(&result).Error
	return result, err
}

/*
  TokenContracts queries
*/

func (d *DB) FindByTokenAndAddress(symbol string, address string) (models.TokenContracts, error) {
	result := models.TokenContracts{}
	err := d.Where("token_symbol = ? AND contract_address = ?", symbol, address).First(&result).Error
	return result, err
}

func (d *DB) FindByChainAndAddress(chain string, address string) (models.TokenContracts, error) {
	result := models.TokenContracts{}
	err := d.Where("chain = ? AND contract_address = ?", chain, address).First(&result).Error
	return result, err
}

func (d *DB) FindByTokenAll(symbol string, address string, chain string) (models.TokenContracts, error) {
	result := models.TokenContracts{}
	err := d.Where("token_symbol = ? AND contract_address = ? AND chain = ?", symbol, address, chain).First(&result).Error
	return result, err
}

func (d *DB) FindTokenAddressByRowId(id uint) (string, error) {
	result := models.TokenContracts{}
	err := d.Where("id = ?", id).First(&result).Error
	return result.ContractAddress, err
}
