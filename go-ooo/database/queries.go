package database

import (
	"fmt"
	"go-ooo/database/models"
	"time"
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

func (d *DB) FindByRequestId(requestId string) (models.DataRequests, error) {
	result := models.DataRequests{}
	err := d.Where("request_id = ?", requestId).First(&result).Error
	return result, err
}

func (d *DB) GetPendingJobs() ([]models.DataRequests, error) {
	var jobs = []models.DataRequests{}
	err := d.Where("job_status = ?",
		models.JOB_STATUS_PENDING).Order(fmt.Sprintf("id %s", "asc")).Find(&jobs).Error
	return jobs, err
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
  SupportedPairs queries
*/

func (d *DB) PairIsSupportedByPairName(pair string) (models.SupportedPairs, error) {
	supported := models.SupportedPairs{}
	err := d.Where("name = ?", pair).First(&supported).Error
	return supported, err
}

func (d *DB) PairIsSupportedByBaseAndTarget(base string, target string) (models.SupportedPairs, error) {
	supported := models.SupportedPairs{}
	err := d.Where("base = ? AND target = ?", base, target).First(&supported).Error
	return supported, err
}

func (d *DB) PairsNoLongerSupported(pairs []string) ([]models.SupportedPairs, error) {
	res := []models.SupportedPairs{}
	err := d.Not(map[string]interface{}{"name": pairs}).Find(&res).Error
	return res, err
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

/*
 VersionInfo queries
*/

func (d *DB) getCurrentDbSchemaVersion() (models.VersionInfo, error) {
	result := models.VersionInfo{}
	err := d.Where("version_type = ?", models.VERSION_TYPE_DB_SCHEMA).First(&result).Error
	return result, err
}
