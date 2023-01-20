package dex

import (
	"github.com/sirupsen/logrus"
)

func (dm *Manager) UpdateAllPairsAndTokens() {
	for _, module := range dm.modules {

		dm.logger.WithFields(logrus.Fields{
			"package":  "dex",
			"function": "UpdateAllPairsAndTokens",
			"dex":      module.Name(),
		}).Info("start update pairs")

		skip := uint64(0)
		hasMore := true

		for hasMore {
			query, err := module.GeneratePairsQuery(skip)
			if err != nil {
				dm.logger.WithFields(logrus.Fields{
					"package":  "dex",
					"function": "UpdateAllPairsAndTokens",
					"action":   "generate pairs query",
					"dex":      module.Name(),
				}).Error(err.Error())
				hasMore = false
				continue
			}

			res, err := runQuery(query, module.SubgraphUrl())
			if err != nil {
				dm.logger.WithFields(logrus.Fields{
					"package":  "dex",
					"function": "UpdateAllPairsAndTokens",
					"action":   "run pairs query",
					"dex":      module.Name(),
				}).Error(err.Error())
				hasMore = false
				continue
			}

			if res == nil {
				dm.logger.WithFields(logrus.Fields{
					"package":  "dex",
					"function": "UpdateAllPairsAndTokens",
					"action":   "run pairs query",
					"dex":      module.Name(),
				}).Error("empty response")
				hasMore = false
				continue
			}

			pairs, more, err := module.ProcessPairsQueryResult(res)

			if err != nil {
				dm.logger.WithFields(logrus.Fields{
					"package":  "dex",
					"function": "UpdateAllPairsAndTokens",
					"action":   "process pairs query result",
					"dex":      module.Name(),
				}).Error(err.Error())
				hasMore = false
				continue
			}

			hasMore = more
			skip += 1000

			dm.logger.WithFields(logrus.Fields{
				"package":   "ooo_api",
				"function":  "UpdateAllPairsAndTokens",
				"dex":       module.Name(),
				"num_pairs": len(pairs),
			}).Info("found pairs")

			dm.updatePairsInDb(pairs, module.Name(), module.Chain())
		}

		dm.logger.WithFields(logrus.Fields{
			"package":  "dex",
			"function": "UpdateAllPairsAndTokens",
			"dex":      module.Name(),
		}).Info("no more pairs")
	}
}
