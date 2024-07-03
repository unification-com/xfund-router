package dex

import (
	"encoding/json"
	"fmt"
	"go-ooo/logger"
	"go-ooo/ooo_api/dex/types"
	"strings"
)

func (dm *Manager) GetSupportedPairs() {
	for _, module := range dm.modules {
		var pairMetaData types.PairMetaData

		dataUrl := fmt.Sprintf(`https://raw.githubusercontent.com/unification-com/ooo-adhoc/main/data/%s/%s.json`, module.Chain(), module.Dex())

		logger.InfoWithFields("dex", "GetSupportedPairs", "", "refresh pairs", logger.Fields{
			"dex": module.Name(),
			"url": dataUrl,
		})

		res, err := runQuery(nil, dataUrl)

		if err != nil {
			logger.ErrorWithFields("dex", "GetSupportedPairs", "run refresh pairs query", err.Error(), logger.Fields{
				"dex": module.Name(),
			})
			continue
		}

		err = json.Unmarshal(res, &pairMetaData)

		if err != nil {
			logger.ErrorWithFields("dex", "GetSupportedPairs", "unmarshal result", err.Error(), logger.Fields{
				"dex": module.Name(),
			})
			continue
		}

		dm.processPairMetaData(pairMetaData)
	}
}

func (dm *Manager) UpdateAllPairsMetaDataFromDexs() {
	for _, module := range dm.modules {

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
