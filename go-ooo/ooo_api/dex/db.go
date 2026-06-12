package dex

import (
	"strconv"

	"github.com/ethereum/go-ethereum/common"

	"go-ooo/logger"
	"go-ooo/ooo_api/dex/types"
	"go-ooo/ooo_api/export"
)

// processPairFeed persists one source's dex-pair-verify pair feed to dex_pairs: it resolves each
// pair's tokens, upserts the row (carrying the export's confidence + canonical key), then
// soft-deletes any pair for this (chain, dex) that has dropped out of the export. Token-less or
// otherwise invalid pairs are skipped + logged. Returns the number of pairs upserted.
func (dm *Manager) processPairFeed(feed export.PairFeed) int {
	chain, dex := feed.Chain, feed.Dex
	kept := make([]string, 0, len(feed.Pairs))
	upserted := 0

	for _, p := range feed.Pairs {
		if p.Token0 == nil || p.Token1 == nil {
			logger.WarnWithFields("dex", "processPairFeed", "skip pair", "export pair missing token data", logger.Fields{
				"chain": chain, "dex": dex, "contract": p.ContractAddress,
			})
			continue
		}

		// Normalise to a checksummed address so the upsert lookup + the soft-delete NOT-IN set match.
		contract := common.HexToAddress(p.ContractAddress).Hex()

		t0, err := dm.db.FindOrInsertNewTokenContract(p.Token0.Symbol, p.Token0.ContractAddress, chain)
		if err != nil {
			logger.ErrorWithFields("dex", "processPairFeed", "token0", err.Error(), logger.Fields{"chain": chain, "dex": dex, "contract": contract})
			continue
		}
		t1, err := dm.db.FindOrInsertNewTokenContract(p.Token1.Symbol, p.Token1.ContractAddress, chain)
		if err != nil {
			logger.ErrorWithFields("dex", "processPairFeed", "token1", err.Error(), logger.Fields{"chain": chain, "dex": dex, "contract": contract})
			continue
		}

		if _, err := dm.db.UpsertExportDexPair(chain, dex, contract, p.Token0.Symbol, p.Token1.Symbol,
			t0.ID, t1.ID, p.ReserveUsd, uint64(p.TxCount), p.Confidence, p.CanonicalKey); err != nil {
			logger.ErrorWithFields("dex", "processPairFeed", "upsert", err.Error(), logger.Fields{"chain": chain, "dex": dex, "contract": contract})
			continue
		}

		kept = append(kept, contract)
		upserted++
	}

	if removed, err := dm.db.SoftDeleteDexPairsNotIn(chain, dex, kept); err != nil {
		logger.ErrorWithFields("dex", "processPairFeed", "soft-delete dropped", err.Error(), logger.Fields{"chain": chain, "dex": dex})
	} else if removed > 0 {
		logger.InfoWithFields("dex", "processPairFeed", "", "soft-deleted pairs dropped from the export", logger.Fields{
			"chain": chain, "dex": dex, "removed": removed,
		})
	}

	logger.InfoWithFields("dex", "processPairFeed", "", "processed export pair feed", logger.Fields{
		"chain": chain, "dex": dex, "upserted": upserted, "in_feed": len(feed.Pairs),
	})

	return upserted
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
