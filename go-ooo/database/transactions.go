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

func (d *DB) InsertNewRequest(chainId int64, provider string,
	consumer string, requestId string,
	endpoint string, endpointDecoded string,
	txHash string, gasUsed uint64, gasPrice uint64,
	fee uint64, blockNumber uint64) (err error) {
	err = d.Omit("FulfilTx").Create(&models.DataRequests{
		ChainId:             chainId,
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
		JobStatus:           models.JOB_STATUS_PENDING,
	}).Error
	return
}

// updateRequest loads a request by id, applies mutate, and saves it - factoring the
// load + save boilerplate the simple Update* methods share. The read-modify-write is fine
// here: fulfilment runs on the single serialised job ticker, so there's no concurrent writer.
func (d *DB) updateRequest(chainId int64, requestId string, mutate func(*models.DataRequests)) error {
	req := models.DataRequests{}
	if err := d.Where("chain_id = ? AND request_id = ?", chainId, requestId).First(&req).Error; err != nil {
		return err
	}
	mutate(&req)
	return d.Save(&req).Error
}

func (d *DB) UpdateFulfillmentSuccess(chainId int64, requestId string, blockNumber uint64,
	txHash string, gasUsed uint64, gasPrice uint64) error {
	return d.updateRequest(chainId, requestId, func(req *models.DataRequests) {
		req.RequestStatus = models.REQUEST_STATUS_SUCCESS
		req.JobStatus = models.JOB_STATUS_SUCCESS
		req.FulfillConfirmedBlockNumber = blockNumber
		req.FulfillTxHash = txHash
		req.FulfillGasUsed = gasUsed
		req.FulfillGasPrice = gasPrice
	})
}

func (d *DB) UpdateFulfillmentSent(chainId int64, requestId string, txHash string, blockNumber uint64, nonce uint64, gasPrice uint64, gasTipCap uint64) error {
	return d.updateRequest(chainId, requestId, func(req *models.DataRequests) {
		req.FulfillTxHash = txHash
		req.LastFulfillSentBlockNumber = blockNumber
		req.FulfillNonce = nonce
		req.FulfillGasPrice = gasPrice
		req.FulfillGasTipCap = gasTipCap
	})
}

func (d *DB) IncrementFulfillmentAttempts(chainId int64, requestId string) error {
	// Atomic increment in one statement, rather than read-modify-write (Save), so the
	// counter can't lose an update and it's a single round-trip.
	return d.Model(&models.DataRequests{}).
		Where("chain_id = ? AND request_id = ?", chainId, requestId).
		Update("fulfillment_attempts", gorm.Expr("fulfillment_attempts + ?", 1)).Error
}

func (d *DB) UpdateRequestStatus(chainId int64, requestId string, status int, reason string) error {
	req := models.DataRequests{}
	err := d.Where("chain_id = ? AND request_id = ?", chainId, requestId).First(&req).Error
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

func (d *DB) UpdateJobStatus(chainId int64, requestId string, status int) error {
	return d.updateRequest(chainId, requestId, func(req *models.DataRequests) {
		req.JobStatus = status
	})
}

func (d *DB) UpdateDataFetched(chainId int64, requestId string, price string) error {
	return d.updateRequest(chainId, requestId, func(req *models.DataRequests) {
		req.RequestStatus = models.REQUEST_STATUS_DATA_READY_TO_SEND
		req.PriceResult = price
	})
}

func (d *DB) UpdateLastDataFetchBlockNumber(chainId int64, requestId string, blockNum uint64) error {
	return d.updateRequest(chainId, requestId, func(req *models.DataRequests) {
		req.LastDataFetchBlockNumber = blockNum
	})
}

/*
  ToBlocks table
*/

func (d *DB) InsertNewToBlock(chainId int64, toBlock uint64) error {
	last, err := d.GetLastBlockNumQueried(chainId)
	// An empty table (no rows yet) is fine - that's the first insert. Any other error
	// means we don't know the current head, so don't write a resume point on top of it.
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if last.GetBlockNum() < toBlock {
		return d.Create(&models.ToBlocks{ChainId: chainId, BlockNum: toBlock}).Error
	}

	return nil
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

// UpsertExportDexPair inserts or updates a dex pair from the dex-pair-verify export, keyed by
// (chain, dex, contractAddress). It carries the export's trust score (confidence), canonical key and
// per-token CoinGecko coin ids (t0Cg/t1Cg, for alias orientation - S7) and marks the pair Verified -
// the export only ever emits verified pairs.
func (d *DB) UpsertExportDexPair(chain, dex, contractAddress, t0Symbol, t1Symbol string,
	t0DbId, t1DbId uint, reserveUsd float64, txCount uint64, confidence float64, canonicalKey, t0Cg, t1Cg string) (models.DexPairs, error) {

	pair, _ := d.FindByDexChainAddress(chain, dex, contractAddress)
	if pair.ID == 0 {
		data := models.DexPairs{
			Chain:           chain,
			Dex:             dex,
			Pair:            fmt.Sprintf("%s-%s", t0Symbol, t1Symbol),
			T0TokenId:       t0DbId,
			T1TokenId:       t1DbId,
			T0Symbol:        t0Symbol,
			T1Symbol:        t1Symbol,
			ContractAddress: contractAddress,
			ReserveUsd:      reserveUsd,
			TxCount:         txCount,
			Verified:        true,
			Confidence:      confidence,
			CanonicalKey:    canonicalKey,
			T0Cg:            t0Cg,
			T1Cg:            t1Cg,
		}
		err := d.Create(&data).Error
		return data, err
	}

	pair.ReserveUsd = reserveUsd
	pair.TxCount = txCount
	pair.Confidence = confidence
	pair.CanonicalKey = canonicalKey
	pair.T0Cg = t0Cg
	pair.T1Cg = t1Cg
	pair.Verified = true
	err := d.Save(&pair).Error
	return pair, err
}

// SoftDeleteDexPairsNotIn soft-deletes the (chain, dex) dex pairs whose contract address is not in
// keepContracts (the export's current set), so a pair dropped from the export stops being priced.
// An empty keepContracts is a no-op: a transient empty feed must NOT wipe a source's whole pair set
// (mirrors the supported_pairs NOT-IN-empty data-loss guard). Returns the number soft-deleted.
func (d *DB) SoftDeleteDexPairsNotIn(chain, dex string, keepContracts []string) (int64, error) {
	if len(keepContracts) == 0 {
		return 0, nil
	}
	res := d.Where("chain = ? AND dex = ? AND contract_address NOT IN ?", chain, dex, keepContracts).
		Delete(&models.DexPairs{})
	return res.RowsAffected, res.Error
}

/*
  SupportedSources
*/

// UpsertSupportedSource inserts or updates one DEX source from the dex-pair-verify manifest, keyed
// by (chain, dex). The caller maps a manifest source into the model (marshalling endpoints to JSON);
// this just persists it, overwriting the manifest-derived fields on an existing row while preserving
// its id + timestamps.
func (d *DB) UpsertSupportedSource(src models.SupportedSource) (models.SupportedSource, error) {
	existing, _ := d.FindSupportedSourceByChainDex(src.Chain, src.Dex)
	if existing.ID == 0 {
		err := d.Create(&src).Error
		return src, err
	}

	existing.SubgraphSchemaFamily = src.SubgraphSchemaFamily
	existing.SourceType = src.SourceType
	existing.Endpoints = src.Endpoints
	existing.FactoryAddress = src.FactoryAddress
	existing.RpcUrl = src.RpcUrl
	existing.BlocksPerMin = src.BlocksPerMin
	existing.PairCount = src.PairCount
	existing.ExportUrl = src.ExportUrl
	existing.SourceUpdatedAt = src.SourceUpdatedAt
	existing.SourceVerifiedAt = src.SourceVerifiedAt
	err := d.Save(&existing).Error
	return existing, err
}

// UpsertAliasConfig persists the singleton asset-class alias payload (S7): the curated alias groups +
// active alias pairs as opaque JSON. It keeps exactly one row - the first existing row is overwritten,
// otherwise one is created - so the alias index can be rebuilt on restart without a live manifest
// fetch (the same resilience floor supported_sources gives the module catalogue).
func (d *DB) UpsertAliasConfig(groupsJSON, pairsJSON string) error {
	var existing models.AliasConfig
	err := d.First(&existing).Error
	if err != nil {
		// No row yet (or read error) - create the singleton.
		return d.Create(&models.AliasConfig{Groups: groupsJSON, Pairs: pairsJSON}).Error
	}
	existing.Groups = groupsJSON
	existing.Pairs = pairsJSON
	return d.Save(&existing).Error
}

// UpdateSourceMinLiquidity records a source's curation floor (its pair feed's minLiquidityUsd, XR2)
// on the supported_sources row. Kept separate from UpsertSupportedSource because the floor arrives
// with the pair feed, not the manifest, so the manifest upsert must not clobber a known floor.
func (d *DB) UpdateSourceMinLiquidity(chain, dex string, minLiquidityUsd float64) error {
	return d.Model(&models.SupportedSource{}).
		Where("chain = ? AND dex = ?", chain, dex).
		Update("min_liquidity_usd", minLiquidityUsd).Error
}

// SoftDeleteSupportedSourcesNotIn soft-deletes the sources whose "chain|dex" key is not in keepKeys
// (the manifest's current set), so a source dropped from the manifest stops driving dispatch. An
// empty keepKeys is a no-op: a transient empty/failed manifest must NOT wipe the whole catalogue
// (mirrors the SoftDeleteDexPairsNotIn data-loss guard). Returns the number soft-deleted.
func (d *DB) SoftDeleteSupportedSourcesNotIn(keepKeys []string) (int64, error) {
	if len(keepKeys) == 0 {
		return 0, nil
	}
	// Match on the composite key via a SQL concat so a (chain, dex) pair is compared as a unit.
	res := d.Where("chain || '|' || dex NOT IN ?", keepKeys).Delete(&models.SupportedSource{})
	return res.RowsAffected, res.Error
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
