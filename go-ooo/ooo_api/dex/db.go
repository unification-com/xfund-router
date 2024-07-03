package dex

import (
	"github.com/ethereum/go-ethereum/common"
	"go-ooo/logger"
	"strconv"
	"strings"
	"unicode/utf8"

	"go-ooo/ooo_api/dex/types"
)

func (dm *Manager) pairIsValid(pair types.DexPair) bool {

	// check if token symbols contain nothing by empty space, null characters etc.
	r := strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "\x00", "")
	if r.Replace(pair.Token0.Symbol) == "" {
		return false
	}
	if r.Replace(pair.Token1.Symbol) == "" {
		return false
	}

	// check if Token symbols are valid UTF-8 strings
	if !utf8.Valid([]byte(pair.Token0.Symbol)) {
		return false
	}
	if !utf8.Valid([]byte(pair.Token1.Symbol)) {
		return false
	}

	return true
}

func (dm *Manager) processPairMetaData(data types.PairMetaData) {
	chain := data.Chain
	dex := data.Dex
	for _, pair := range data.Pairs {
		token0 := pair.Token0
		token1 := pair.Token1

		t0Db, err := dm.db.FindOrInsertNewTokenContract(token0.Symbol, token0.ContractAddress, chain)

		if err != nil {
			// log error and continue
			logger.ErrorWithFields("dex", "processPairMetaData", "FindOrInsertNewTokenContract", err.Error(), logger.Fields{
				"chain":         chain,
				"dex":           dex,
				"what":          "token0",
				"symbol":        token0.Symbol,
				"contract":      token0.ContractAddress,
				"pair_contract": pair.ContractAddress,
			})

			continue
		}

		t1Db, err := dm.db.FindOrInsertNewTokenContract(token1.Symbol, token1.ContractAddress, chain)

		if err != nil {
			// log error and continue
			logger.ErrorWithFields("dex", "processPairMetaData", "FindOrInsertNewTokenContract", err.Error(), logger.Fields{
				"chain":         chain,
				"dex":           dex,
				"what":          "token1",
				"symbol":        token1.Symbol,
				"contract":      token1.ContractAddress,
				"pair_contract": pair.ContractAddress,
			})

			continue
		}

		_, err = dm.db.FindOrInsertNewDexPair(token0.Symbol, token1.Symbol, chain, pair.ContractAddress, dex, t0Db.ID, t1Db.ID, pair.ReserveUsd, pair.TxCount)

		if err != nil {
			// log error and continue
			logger.ErrorWithFields("dex", "updatePairsInDb", "FindOrInsertNewDexPair", err.Error(), logger.Fields{
				"chain":         chain,
				"dex":           dex,
				"token0_symbol": pair.Token0.Symbol,
				"token1_symbol": pair.Token1.Symbol,
				"token0_row_id": t0Db.ID,
				"token1_row_id": t1Db.ID,
				"pair_contract": pair.ContractAddress,
				"reserve_usd":   pair.ReserveUsd,
				"tx_count":      pair.TxCount,
			})
		}
	}
}

func (dm *Manager) updatePairsInDb(pairs []types.DexPair, chain, dex string) {

	for _, p := range pairs {
		contractAddress := common.HexToAddress(p.Contract)
		txCount, err := strconv.Atoi(p.TxCount)
		if err != nil {
			txCount = 0
		}
		reserveUsd, err := strconv.ParseFloat(p.ReserveUSD, 64)

		if err != nil {
			reserveUsd = 0.0
		}

		err = dm.db.UpdateDexPairMetaData(chain, dex, contractAddress.Hex(), reserveUsd, uint64(txCount))

		if err != nil {
			// log error and continue
			logger.ErrorWithFields("dex", "updatePairsInDb", "FindOrInsertNewDexPair", err.Error(), logger.Fields{
				"chain":            chain,
				"dex":              dex,
				"contract_address": contractAddress.Hex(),
			})
		}
	}
}
