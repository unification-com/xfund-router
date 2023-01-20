package dex

import (
	"errors"
	"fmt"

	"go-ooo/logger"
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
	dexSuccess := 0
	dexFail := 0
	dexNoData := 0

	// get a list of valid modules to send query to
	for _, module := range dm.modules {

		logger.InfoWithFields("dex", "GetPricesFromDexModules", "check valid", "get prices", logger.Fields{
			"dex":     module.Name(),
			"chain":   module.Chain(),
			"base":    base,
			"target":  target,
			"minutes": minutes,
		})

		currentBlock, err := dm.chains[module.Chain()].EthClient.BlockNumber(dm.ctx)
		blocksPerMin := uint64(dm.chains[module.Chain()].BlocksPerMin)

		if err != nil {
			logger.ErrorWithFields("dex", "GetPricesFromDexModules", "get current block", err.Error(), logger.Fields{
				"dex":   module.Name(),
				"chain": module.Chain(),
			})

			continue
		}

		dbPairRes, _ := dm.db.FindByDexPairName(base, target, module.Name())

		if dbPairRes.ID == 0 {
			logger.WarnWithFields("dex", "GetPricesFromDexModules", "check pair exists in db",
				"pair not found in database for this dex",
				logger.Fields{
					"dex":    module.Name(),
					"base":   base,
					"target": target,
				})

			continue
		}

		if dbPairRes.ReserveUsd < float64(module.MinLiquidity()) {
			logger.WarnWithFields("dex", "GetPricesFromDexModules", "check liquidity",
				"liquidity too low. Skipping",
				logger.Fields{
					"dex":           module.Name(),
					"base":          base,
					"target":        target,
					"reserve_usd":   dbPairRes.ReserveUsd,
					"min_liquidity": module.MinLiquidity(),
				})

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

	for modName := range validMods {
		p := <-resCh // receive result from channel resCh
		err := <-errCh

		if err != nil {
			logger.ErrorWithFields("dex", "GetPricesFromDexModules", "getPrices",
				err.Error(),
				logger.Fields{
					"dex":        modName,
					"base":       base,
					"target":     target,
					"num_prices": len(p),
				})
			dexFail++
		} else {
			logger.Debug("dex", "GetPricesFromDexModules", "getPrices", "prices result",
				logger.Fields{
					"dex":        modName,
					"base":       base,
					"target":     target,
					"num_prices": len(p),
				})

			if len(p) > 0 {
				prices = append(prices, p...)
				dexSuccess++
			} else {
				dexNoData++
			}
		}
	}

	logger.Debug("dex", "GetPricesFromDexModules", "", "",
		logger.Fields{
			"base":        base,
			"target":      target,
			"dex_success": dexSuccess,
			"dex_fail":    dexFail,
			"dex_no_data": dexNoData,
			"num_dexes":   len(validMods),
			"num_prices":  len(prices),
		})

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
