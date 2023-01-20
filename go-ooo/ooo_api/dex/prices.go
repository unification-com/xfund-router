package dex

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
)

type DexInfo struct {
	CurrentBlock    uint64
	BlockPerMin     uint64
	ContractAddress string
}

func (dm *Manager) GetPricesFromDexModules(base, target string, minutes uint64) []float64 {
	var prices []float64

	resCh := make(chan []float64)
	errCh := make(chan error)
	validMods := make(map[string]DexInfo)

	// get a list of valid modules to send query to
	for _, module := range dm.modules {

		dm.logger.WithFields(logrus.Fields{
			"package":  "dex",
			"function": "GetPricesFromDexModules",
			"action":   "check valid",
			"dex":      module.Name(),
			"base":     base,
			"target":   target,
			"minutes":  minutes,
		}).Info("get prices")

		currentBlock, err := dm.chains[module.Chain()].EthClient.BlockNumber(dm.ctx)
		blocksPerMin := uint64(dm.chains[module.Chain()].BlocksPerMin)

		if err != nil {
			dm.logger.WithFields(logrus.Fields{
				"package":  "dex",
				"function": "GetPricesFromDexModules",
				"action":   "get current block",
				"dex":      module.Name(),
				"chain":    module.Chain(),
			}).Error(err.Error())

			continue
		}

		dbPairRes, _ := dm.db.FindByDexPairName(base, target, module.Name())

		if dbPairRes.ID == 0 {
			dm.logger.WithFields(logrus.Fields{
				"package":  "dex",
				"function": "GetPricesFromDexModules",
				"action":   "check pair exists in db",
				"dex":      module.Name(),
				"base":     base,
				"target":   target,
			}).Warn("pair not found in database for this dex")
			continue
		}

		if dbPairRes.ReserveUsd < float64(module.MinLiquidity()) {
			dm.logger.WithFields(logrus.Fields{
				"package":       "dex",
				"function":      "GetPricesFromDexModules",
				"action":        "check liquidity",
				"dex":           module.Name(),
				"base":          base,
				"target":        target,
				"reserve_usd":   dbPairRes.ReserveUsd,
				"min_liquidity": module.MinLiquidity(),
			}).Warn("liquidity too low. Skipping")
			continue
		}

		dexInfo := DexInfo{
			CurrentBlock:    currentBlock,
			BlockPerMin:     blocksPerMin,
			ContractAddress: dbPairRes.ContractAddress,
		}

		validMods[module.Name()] = dexInfo

		go getPrices(module, base, target, minutes, dexInfo, resCh, errCh)
	}

	for modName, _ := range validMods {
		p := <-resCh // receive result from channel resCh
		err := <-errCh

		if err != nil {
			dm.logger.WithFields(logrus.Fields{
				"package":    "dex",
				"function":   "getPrices",
				"dex":        modName,
				"base":       base,
				"target":     target,
				"num_prices": len(p),
			}).Error(err.Error())
		} else {
			dm.logger.WithFields(logrus.Fields{
				"package":    "dex",
				"function":   "getPrices",
				"dex":        modName,
				"base":       base,
				"target":     target,
				"num_prices": len(p),
			}).Debug("prices result")

			if len(p) > 0 {
				prices = append(prices, p...)
			}
		}
	}

	return prices
}

func getPrices(module Module, base, target string, minutes uint64, dexInfo DexInfo, resCh chan<- []float64, errCh chan<- error) {
	query, err := module.GenerateDexPricesQuery(dexInfo.ContractAddress, minutes, dexInfo.CurrentBlock, dexInfo.BlockPerMin)
	if err != nil {
		errMsg := fmt.Sprintf(`getPrices generate query error: %s`, err.Error())
		resCh <- []float64{}
		errCh <- errors.New(errMsg)
		return
	}

	dexResult, err := runQuery(query, module.SubgraphUrl())
	if err != nil {
		errMsg := fmt.Sprintf(`getPrices run query error: %s`, err.Error())
		resCh <- []float64{}
		errCh <- errors.New(errMsg)
		return
	}

	dexPrices, err := module.ProcessDexPricesResult(base, target, minutes, dexResult)

	if err != nil {
		errMsg := fmt.Sprintf(`getPrices process query results error: %s`, err.Error())
		resCh <- []float64{}
		errCh <- errors.New(errMsg)
		return
	}

	resCh <- dexPrices
	errCh <- nil

}
