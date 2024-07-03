package dex

import (
	"errors"
	"fmt"
	"strings"

	"go-ooo/logger"
)

type DexInfo struct {
	CurrentBlock      uint64
	BlockPerMin       uint64
	ContractAddresses string
}

type DexResult struct {
	Chain  string
	Dex    string
	Prices []float64
}

func (dm *Manager) GetPricesFromDexModules(base, target string, minutes uint64) []float64 {
	var prices []float64

	resCh := make(chan DexResult)
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
				"chain": module.Chain(),
				"dex":   module.Dex(),
			})

			continue
		}

		dbPairRes, _ := dm.db.FindByDexPairName(base, target, module.Chain(), module.Dex())

		if len(dbPairRes) == 0 {
			logger.WarnWithFields("dex", "GetPricesFromDexModules", "check pair exists in db",
				"pair not found in database for this dex",
				logger.Fields{
					"chain":  module.Chain(),
					"dex":    module.Dex(),
					"base":   base,
					"target": target,
				})

			continue
		}

		var contractAddresses []string
		for _, p := range dbPairRes {
			if p.ReserveUsd < float64(module.MinLiquidity()) {
				logger.WarnWithFields("dex", "GetPricesFromDexModules", "check liquidity",
					"liquidity too low. Skipping",
					logger.Fields{
						"chain":         module.Chain(),
						"dex":           module.Dex(),
						"base":          base,
						"target":        target,
						"reserve_usd":   p.ReserveUsd,
						"min_liquidity": module.MinLiquidity(),
					})

				continue
			}

			contractAddresses = append(contractAddresses, p.ContractAddress)
		}

		if len(contractAddresses) == 0 {
			logger.WarnWithFields("dex", "GetPricesFromDexModules", "check contract address array",
				"no contract addresses to query",
				logger.Fields{
					"chain":  module.Chain(),
					"dex":    module.Dex(),
					"base":   base,
					"target": target,
				})

			continue
		}

		logger.Debug("dex", "GetPricesFromDexModules", "number contracts", "",
			logger.Fields{
				"chain":                  module.Chain(),
				"dex":                    module.Dex(),
				"base":                   base,
				"target":                 target,
				"num_contract_addresses": len(contractAddresses),
			})

		contractAddressesStr := fmt.Sprintf(`"%s"`, strings.Join(contractAddresses, `","`))

		dexInfo := DexInfo{
			CurrentBlock:      currentBlock,
			BlockPerMin:       blocksPerMin,
			ContractAddresses: contractAddressesStr,
		}

		validMods[module.Name()] = dexInfo

		go getPrices(module, base, target, minutes, dexInfo, resCh, errCh)
	}

	for _ = range validMods {
		r := <-resCh // receive result from channel resCh
		err := <-errCh

		if err != nil {
			logger.Error("dex", "GetPricesFromDexModules", "getPrices",
				err.Error(),
			)
			dexFail++
		} else {
			logger.Debug("dex", "GetPricesFromDexModules", "getPrices", "prices result",
				logger.Fields{
					"chain":      r.Chain,
					"dex":        r.Dex,
					"base":       base,
					"target":     target,
					"num_prices": len(r.Prices),
				})

			if len(r.Prices) > 0 {
				prices = append(prices, r.Prices...)
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

func getPrices(module Module, base, target string, minutes uint64, dexInfo DexInfo, resCh chan<- DexResult, errCh chan<- error) {
	query, numQueries, err := module.GenerateDexPricesQuery(dexInfo.ContractAddresses, minutes, dexInfo.CurrentBlock, dexInfo.BlockPerMin)
	if err != nil {
		errMsg := fmt.Sprintf(`%s, %s, %s, %s. getPrices generate query error: %s`, module.Chain(), module.Dex(), base, target, err.Error())
		resCh <- DexResult{}
		errCh <- errors.New(errMsg)
		return
	}

	dexResult, err := runQuery(query, module.SubgraphUrl())
	if err != nil {
		errMsg := fmt.Sprintf(`%s, %s, %s, %s. getPrices run query error: %s`, module.Chain(), module.Dex(), base, target, err.Error())
		resCh <- DexResult{}
		errCh <- errors.New(errMsg)
		return
	}

	dexPrices, err := module.ProcessDexPricesResult(base, target, numQueries, dexResult)

	if err != nil {
		errMsg := fmt.Sprintf(`%s, %s, %s, %s. getPrices process query results error: %s`, module.Chain(), module.Dex(), base, target, err.Error())
		resCh <- DexResult{}
		errCh <- errors.New(errMsg)
		return
	}

	resCh <- DexResult{
		Chain:  module.Chain(),
		Dex:    module.Dex(),
		Prices: dexPrices,
	}
	errCh <- nil

}
