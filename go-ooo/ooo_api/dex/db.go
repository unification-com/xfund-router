package dex

import (
	"go-ooo/logger"
	"go-ooo/ooo_api/dex/types"
	"go-ooo/utils"
	"strings"
)

func pairIsClean(pair types.DexPair) bool {
	// check if Token symbols are empty/only whitespace
	if strings.TrimSpace(pair.Token0.Symbol) == "" || pair.Token0.Symbol == " " {
		return false
	}
	if strings.TrimSpace(pair.Token1.Symbol) == "" || pair.Token1.Symbol == " " {
		return false
	}

	return true
}

func (dm *Manager) updatePairsInDb(pairs []types.DexPair, dex, chain string) {
	for _, pair := range pairs {
		if pairIsClean(pair) {
			// pair.Token0.Id is the token's contract address
			t0Db, _ := dm.db.FindOrInsertNewTokenContract(pair.Token0.Symbol, pair.Token0.Id, chain)
			t1Db, _ := dm.db.FindOrInsertNewTokenContract(pair.Token1.Symbol, pair.Token1.Id, chain)

			t0DtDb, _ := dm.db.FindOrInsertNewDexToken(pair.Token0.Symbol, t0Db.ID, dex, chain)
			t1DtDb, _ := dm.db.FindOrInsertNewDexToken(pair.Token1.Symbol, t1Db.ID, dex, chain)

			// store liquidity
			reserve, _ := utils.ParseBigFloat(pair.ReserveUSD)
			reserveUsd, _ := reserve.Float64()

			// todo - update latest liquidity
			_, _ = dm.db.FindOrInsertNewDexPair(pair.Token0.Symbol, pair.Token1.Symbol, pair.Id, dex, t0DtDb.ID, t1DtDb.ID, reserveUsd)
		} else {
			logger.Debug("dex", "updatePairsInDb", "", "pair not clean", logger.Fields{
				"Token0.Symbol": pair.Token0.Symbol,
				"Token1.Symbol": pair.Token1.Symbol,
			})
		}
	}
}
