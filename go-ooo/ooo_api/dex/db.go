package dex

import (
	"strings"
	"unicode/utf8"

	"go-ooo/logger"
	"go-ooo/ooo_api/dex/types"
	"go-ooo/utils"
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

func (dm *Manager) updatePairsInDb(pairs []types.DexPair, dex, chain string) {
	for _, pair := range pairs {
		// check if pair is valid & clean
		if !dm.pairIsValid(pair) {
			// Log, ignore and continue
			logger.Debug("dex", "updatePairsInDb", "", "pair not valid - ignoring", logger.Fields{
				"token0_symbol":   pair.Token0.Symbol,
				"token1_symbol":   pair.Token1.Symbol,
				"token0_contract": pair.Token0.Id,
				"token1_contract": pair.Token1.Id,
				"pair_contract":   pair.Id,
				"dex":             dex,
			})

			continue
		}

		// pair.Token0.Id is the token's contract address
		t0Db, err := dm.db.FindOrInsertNewTokenContract(pair.Token0.Symbol, pair.Token0.Id, chain)

		if err != nil {
			// log error and continue
			logger.ErrorWithFields("dex", "updatePairsInDb", "FindOrInsertNewTokenContract", err.Error(), logger.Fields{
				"chain":         chain,
				"dex":           dex,
				"what":          "token0",
				"symbol":        pair.Token0.Symbol,
				"contract":      pair.Token0.Id,
				"pair_contract": pair.Id,
			})

			continue
		}

		t1Db, err := dm.db.FindOrInsertNewTokenContract(pair.Token1.Symbol, pair.Token1.Id, chain)

		if err != nil {
			// log error and continue
			logger.ErrorWithFields("dex", "updatePairsInDb", "FindOrInsertNewTokenContract", err.Error(), logger.Fields{
				"chain":         chain,
				"dex":           dex,
				"what":          "token1",
				"symbol":        pair.Token1.Symbol,
				"contract":      pair.Token1.Id,
				"pair_contract": pair.Id,
			})

			continue
		}

		t0DtDb, err := dm.db.FindOrInsertNewDexToken(pair.Token0.Symbol, t0Db.ID, dex, chain)

		if err != nil {
			// log error and continue
			logger.ErrorWithFields("dex", "updatePairsInDb", "FindOrInsertNewDexToken", err.Error(), logger.Fields{
				"chain":           chain,
				"dex":             dex,
				"what":            "token0",
				"symbol":          pair.Token0.Symbol,
				"contract_row_id": t0Db.ID,
				"pair_contract":   pair.Id,
			})

			continue
		}

		t1DtDb, err := dm.db.FindOrInsertNewDexToken(pair.Token1.Symbol, t1Db.ID, dex, chain)

		if err != nil {
			// log error and continue
			logger.ErrorWithFields("dex", "updatePairsInDb", "FindOrInsertNewDexToken", err.Error(), logger.Fields{
				"chain":           chain,
				"dex":             dex,
				"what":            "token1",
				"symbol":          pair.Token1.Symbol,
				"contract_row_id": t1Db.ID,
				"pair_contract":   pair.Id,
			})

			continue
		}

		// store liquidity
		reserve, err := utils.ParseBigFloat(pair.ReserveUSD)

		if err != nil {
			// log error and continue
			logger.ErrorWithFields("dex", "updatePairsInDb", "ParseBigFloat", err.Error(), logger.Fields{
				"chain":         chain,
				"dex":           dex,
				"pair_contract": pair.Id,
			})

			continue
		}

		reserveUsd, _ := reserve.Float64()

		_, err = dm.db.FindOrInsertNewDexPair(pair.Token0.Symbol, pair.Token1.Symbol, pair.Id, dex, t0DtDb.ID, t1DtDb.ID, reserveUsd)

		if err != nil {
			// log error and continue
			logger.ErrorWithFields("dex", "updatePairsInDb", "FindOrInsertNewDexPair", err.Error(), logger.Fields{
				"chain":         chain,
				"dex":           dex,
				"token0_symbol": pair.Token0.Symbol,
				"token1_symbol": pair.Token1.Symbol,
				"token0_row_id": t0DtDb.ID,
				"token1_row_id": t1DtDb.ID,
				"pair_contract": pair.Id,
				"reserve_usd":   reserveUsd,
			})
		}
	}
}
