package dex

import (
	"fmt"
	"go-ooo/logger"
	"strings"
)

// UpdateAllPairsMetaDataFromDexs refreshes the live reserve/tx metadata of the persisted pairs by
// querying each active source's subgraph - keeping the figures the price-path liquidity gate + the
// liquidity-weighting depend on current, independent of how stale the dex-pair-verify feed's own
// reserves are.
func (dm *Manager) UpdateAllPairsMetaDataFromDexs() {
	for _, module := range dm.snapshotModules() {

		var contractAddresses []string
		pairsDb, _ := dm.db.Get100PairsForDataRefresh(module.Chain(), module.Dex())

		if len(pairsDb) == 0 {
			logger.InfoWithFields("dex", "UpdateAllPairsMetaDataFromDexs", "", "no pairs to update", logger.Fields{
				"chain": module.Chain(),
				"dex":   module.Dex(),
			})
			continue
		}

		for _, p := range pairsDb {
			contractAddresses = append(contractAddresses, p.ContractAddress)
		}

		contractAddressesStr := fmt.Sprintf(`"%s"`, strings.Join(contractAddresses, `","`))

		logger.InfoWithFields("dex", "UpdateAllPairsMetaDataFromDexs", "", "start update pairs", logger.Fields{
			"chain":     module.Chain(),
			"dex":       module.Dex(),
			"num_pairs": len(pairsDb),
		})

		query, err := module.GeneratePairsQuery(contractAddressesStr)

		if err != nil {
			logger.ErrorWithFields("dex", "UpdateAllPairsMetaDataFromDexs", "generate pairs query", err.Error(), logger.Fields{
				"chain": module.Chain(),
				"dex":   module.Dex(),
			})
			continue
		}

		res, err := runQuery(query, module.SubgraphUrl())

		if err != nil {
			logger.ErrorWithFields("dex", "UpdateAllPairsMetaDataFromDexs", "run pairs query", err.Error(), logger.Fields{
				"chain": module.Chain(),
				"dex":   module.Dex(),
			})
			continue
		}

		if res == nil {
			logger.ErrorWithFields("dex", "UpdateAllPairsMetaDataFromDexs", "run pairs query", "empty response", logger.Fields{
				"chain": module.Chain(),
				"dex":   module.Dex(),
			})
			continue
		}

		pairs, err := module.ProcessPairsQueryResult(res)

		if err != nil {
			logger.ErrorWithFields("dex", "UpdateAllPairsMetaDataFromDexs", "process pairs query", err.Error(), logger.Fields{
				"chain": module.Chain(),
				"dex":   module.Dex(),
			})
			continue
		}

		dm.updatePairsInDb(pairs, module.Chain(), module.Dex())
	}
}
