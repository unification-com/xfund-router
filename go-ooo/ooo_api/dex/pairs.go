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

		// The source owns its transport: FetchPairsMetadata runs query + fetch + parse and returns
		// the refreshed pair metadata, so this loop stays transport-blind. Any failure (generate /
		// fetch / empty / parse) returns an error and the source is skipped for this cycle.
		pairs, err := module.FetchPairsMetadata(contractAddressesStr)

		if err != nil {
			logger.ErrorWithFields("dex", "UpdateAllPairsMetaDataFromDexs", "fetch pairs metadata", err.Error(), logger.Fields{
				"chain": module.Chain(),
				"dex":   module.Dex(),
			})
			continue
		}

		dm.updatePairsInDb(pairs, module.Chain(), module.Dex())
	}
}
