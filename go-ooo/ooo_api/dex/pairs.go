package dex

import (
	"go-ooo/logger"
)

func (dm *Manager) UpdateAllPairsAndTokens() {
	for _, module := range dm.modules {

		logger.InfoWithFields("dex", "UpdateAllPairsAndTokens", "", "start update pairs", logger.Fields{
			"dex": module.Name(),
		})

		skip := uint64(0)
		hasMore := true

		for hasMore {
			query, err := module.GeneratePairsQuery(skip)
			if err != nil {
				logger.ErrorWithFields("dex", "UpdateAllPairsAndTokens", "generate pairs query", err.Error(), logger.Fields{
					"dex": module.Name(),
				})

				hasMore = false
				continue
			}

			res, err := runQuery(query, module.SubgraphUrl())
			if err != nil {
				logger.ErrorWithFields("dex", "UpdateAllPairsAndTokens", "run pairs query", err.Error(), logger.Fields{
					"dex": module.Name(),
				})
				hasMore = false
				continue
			}

			if res == nil {
				logger.ErrorWithFields("dex", "UpdateAllPairsAndTokens", "run pairs query", "empty response", logger.Fields{
					"dex": module.Name(),
				})
				hasMore = false
				continue
			}

			// ToDo: migrate to Graph Network & GRT fees
			// ToDo: implement fallback query URLs
			pairs, more, err := module.ProcessPairsQueryResult(res)

			if err != nil {
				logger.ErrorWithFields("dex", "UpdateAllPairsAndTokens", "process pairs query", err.Error(), logger.Fields{
					"dex": module.Name(),
				})
				hasMore = false
				continue
			}

			hasMore = more
			skip += 1000

			logger.InfoWithFields("dex", "UpdateAllPairsAndTokens", "", "found pairs", logger.Fields{
				"dex":       module.Name(),
				"num_pairs": len(pairs),
			})

			dm.updatePairsInDb(pairs, module.Name(), module.Chain())
		}

		logger.InfoWithFields("dex", "UpdateAllPairsAndTokens", "", "no more pairs", logger.Fields{
			"dex": module.Name(),
		})
	}
}
