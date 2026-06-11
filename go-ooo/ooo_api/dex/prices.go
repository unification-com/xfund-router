package dex

import (
	"errors"
	"fmt"
	"strings"

	"go-ooo/logger"
	"go-ooo/ooo_api/dex/types"
)

type DexInfo struct {
	CurrentBlock      uint64
	BlockPerMin       uint64
	ContractAddresses string
}

// PoolSample is one pool's contribution to a price query: its identity, its backing
// liquidity (ReserveUsd, for weighting) and its snapshot prices across the queried blocks.
// GetPricesFromDexModules returns these per-pool rather than a flat price list so the
// aggregator can reduce and liquidity-weight each pool independently.
type PoolSample struct {
	Chain     string
	Dex       string
	Contract  string
	Liquidity float64
	Prices    []float64
}

// DexResult carries one DEX's per-pool price groups back from a fetch goroutine.
type DexResult struct {
	Chain string
	Dex   string
	Pools []types.PoolPrices
}

func (dm *Manager) GetPricesFromDexModules(base, target string, minutes uint64) []PoolSample {
	var samples []PoolSample

	resCh := make(chan DexResult)
	errCh := make(chan error)
	validMods := make(map[string]DexInfo)
	// liquidity per module, keyed "chain|dex" -> lower(contract) -> reserveUsd, to join onto
	// the per-pool prices the family returns in the receive loop.
	liquidityByMod := make(map[string]map[string]float64)
	dexSuccess := 0
	dexFail := 0
	dexNoData := 0

	// Cache each chain's current block number for this query: several modules share a chain,
	// so querying once avoids N identical RPC round-trips, and a chain whose RPC is down is
	// recorded once (chainRpcFailed) and its modules skipped, rather than re-failing per module.
	blockByChain := make(map[string]uint64)
	chainRpcFailed := make(map[string]bool)

	// get a list of valid modules to send query to
	for _, module := range dm.modules {

		logger.InfoWithFields("dex", "GetPricesFromDexModules", "check valid", "get prices", logger.Fields{
			"dex":     module.Name(),
			"chain":   module.Chain(),
			"base":    base,
			"target":  target,
			"minutes": minutes,
		})

		chain := module.Chain()

		if chainRpcFailed[chain] {
			continue
		}

		currentBlock, cached := blockByChain[chain]
		if !cached {
			cb, err := dm.chains[chain].EthClient.BlockNumber(dm.ctx)
			if err != nil {
				logger.ErrorWithFields("dex", "GetPricesFromDexModules", "get current block", err.Error(), logger.Fields{
					"chain": chain,
					"dex":   module.Dex(),
				})

				// Mark the chain failed so its other modules are skipped without re-querying.
				chainRpcFailed[chain] = true
				continue
			}
			blockByChain[chain] = cb
			currentBlock = cb
		}

		blocksPerMin := uint64(dm.chains[chain].BlocksPerMin)

		dbPairRes, _ := dm.db.FindByDexPairName(base, target, chain, module.Dex())

		if len(dbPairRes) == 0 {
			logger.WarnWithFields("dex", "GetPricesFromDexModules", "check pair exists in db",
				"pair not found in database for this dex",
				logger.Fields{
					"chain":  chain,
					"dex":    module.Dex(),
					"base":   base,
					"target": target,
				})

			continue
		}

		liq := make(map[string]float64)
		var contractAddresses []string
		for _, p := range dbPairRes {
			if p.ReserveUsd < float64(module.MinLiquidity()) {
				logger.WarnWithFields("dex", "GetPricesFromDexModules", "check liquidity",
					"liquidity too low. Skipping",
					logger.Fields{
						"chain":         chain,
						"dex":           module.Dex(),
						"base":          base,
						"target":        target,
						"reserve_usd":   p.ReserveUsd,
						"min_liquidity": module.MinLiquidity(),
					})

				continue
			}

			contractAddresses = append(contractAddresses, p.ContractAddress)
			liq[strings.ToLower(p.ContractAddress)] = p.ReserveUsd
		}

		if len(contractAddresses) == 0 {
			logger.WarnWithFields("dex", "GetPricesFromDexModules", "check contract address array",
				"no contract addresses to query",
				logger.Fields{
					"chain":  chain,
					"dex":    module.Dex(),
					"base":   base,
					"target": target,
				})

			continue
		}

		logger.Debug("dex", "GetPricesFromDexModules", "number contracts", "",
			logger.Fields{
				"chain":                  chain,
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
		liquidityByMod[chain+"|"+module.Dex()] = liq

		go getPrices(module, base, target, minutes, dexInfo, resCh, errCh)
	}

	for range validMods {
		r := <-resCh // receive result from channel resCh
		err := <-errCh

		if err != nil {
			logger.Error("dex", "GetPricesFromDexModules", "getPrices",
				err.Error(),
			)
			dexFail++
			continue
		}

		logger.Debug("dex", "GetPricesFromDexModules", "getPrices", "prices result",
			logger.Fields{
				"chain":     r.Chain,
				"dex":       r.Dex,
				"base":      base,
				"target":    target,
				"num_pools": len(r.Pools),
			})

		if len(r.Pools) > 0 {
			liq := liquidityByMod[r.Chain+"|"+r.Dex]
			for _, pool := range r.Pools {
				samples = append(samples, PoolSample{
					Chain:     r.Chain,
					Dex:       r.Dex,
					Contract:  pool.Contract,
					Liquidity: liq[strings.ToLower(pool.Contract)],
					Prices:    pool.Prices,
				})
			}
			dexSuccess++
		} else {
			dexNoData++
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
			"num_pools":   len(samples),
		})

	return samples
}

func getPrices(module Module, base, target string, minutes uint64, dexInfo DexInfo, resCh chan<- DexResult, errCh chan<- error) {
	// A malformed/partial subgraph response can make a module's query parsing panic
	// (e.g. an unchecked type assertion). As this runs in its own goroutine, an
	// un-recovered panic would crash the whole oracle; and because the caller reads
	// exactly one (result, error) pair per module, a panicking goroutine that never
	// sends would deadlock the caller. Recover here and send the empty result + error
	// so one bad DEX degrades to "no data" instead of taking the process down. The
	// recover runs before any of the normal sends below, so it sends exactly once.
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorWithFields("dex", "getPrices", "recovered from panic", fmt.Sprintf("%v", r), logger.Fields{
				"chain":  module.Chain(),
				"dex":    module.Dex(),
				"base":   base,
				"target": target,
			})
			dexQueryTotal.WithLabelValues(module.Chain(), module.Dex(), "error").Inc()
			resCh <- DexResult{}
			errCh <- fmt.Errorf("%s, %s, %s, %s. getPrices recovered from panic: %v", module.Chain(), module.Dex(), base, target, r)
		}
	}()

	query, numQueries, err := module.GenerateDexPricesQuery(dexInfo.ContractAddresses, minutes, dexInfo.CurrentBlock, dexInfo.BlockPerMin)
	if err != nil {
		dexQueryTotal.WithLabelValues(module.Chain(), module.Dex(), "error").Inc()
		errMsg := fmt.Sprintf(`%s, %s, %s, %s. getPrices generate query error: %s`, module.Chain(), module.Dex(), base, target, err.Error())
		resCh <- DexResult{}
		errCh <- errors.New(errMsg)
		return
	}

	dexResult, err := runQuery(query, module.SubgraphUrl())
	if err != nil {
		dexQueryTotal.WithLabelValues(module.Chain(), module.Dex(), "error").Inc()
		errMsg := fmt.Sprintf(`%s, %s, %s, %s. getPrices run query error: %s`, module.Chain(), module.Dex(), base, target, err.Error())
		resCh <- DexResult{}
		errCh <- errors.New(errMsg)
		return
	}

	dexPools, err := module.ProcessDexPricesResult(base, target, numQueries, dexResult)

	if err != nil {
		dexQueryTotal.WithLabelValues(module.Chain(), module.Dex(), "error").Inc()
		errMsg := fmt.Sprintf(`%s, %s, %s, %s. getPrices process query results error: %s`, module.Chain(), module.Dex(), base, target, err.Error())
		resCh <- DexResult{}
		errCh <- errors.New(errMsg)
		return
	}

	result := "success"
	if len(dexPools) == 0 {
		result = "no_data"
	}
	dexQueryTotal.WithLabelValues(module.Chain(), module.Dex(), result).Inc()

	resCh <- DexResult{
		Chain: module.Chain(),
		Dex:   module.Dex(),
		Pools: dexPools,
	}
	errCh <- nil

}
