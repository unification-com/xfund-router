package dex

import (
	"go-ooo/logger"
)

func (dm *Manager) UpdateAllPairsAndTokens() {
	for _, module := range dm.modules {

		dm.logger.InfoWithFields("dex", "UpdateAllPairsAndTokens", "", "start update pairs", logger.Fields{
			"dex": module.Name(),
		})

		skip := uint64(0)
		hasMore := true

		for hasMore {
			query, err := module.GeneratePairsQuery(skip)
			if err != nil {
				dm.logger.ErrorWithFields("dex", "UpdateAllPairsAndTokens", "generate pairs query", err.Error(), logger.Fields{
					"dex": module.Name(),
				})

				hasMore = false
				continue
			}

			res, err := runQuery(query, module.SubgraphUrl())
			if err != nil {
				dm.logger.ErrorWithFields("dex", "UpdateAllPairsAndTokens", "run pairs query", err.Error(), logger.Fields{
					"dex": module.Name(),
				})
				hasMore = false
				continue
			}

			if res == nil {
				dm.logger.ErrorWithFields("dex", "UpdateAllPairsAndTokens", "run pairs query", "empty response", logger.Fields{
					"dex": module.Name(),
				})
				hasMore = false
				continue
			}

			pairs, more, err := module.ProcessPairsQueryResult(res)

			if err != nil {
				dm.logger.ErrorWithFields("dex", "UpdateAllPairsAndTokens", "process pairs query", err.Error(), logger.Fields{
					"dex": module.Name(),
				})
				hasMore = false
				continue
			}

			hasMore = more
			skip += 1000

			dm.logger.InfoWithFields("dex", "UpdateAllPairsAndTokens", "", "found pairs", logger.Fields{
				"dex":       module.Name(),
				"num_pairs": len(pairs),
			})

			dm.updatePairsInDb(pairs, module.Name(), module.Chain())
		}

		dm.logger.InfoWithFields("dex", "UpdateAllPairsAndTokens", "", "no more pairs", logger.Fields{
			"dex": module.Name(),
		})
	}
}
