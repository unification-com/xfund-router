package dex

import (
	"fmt"
	"strings"

	"go-ooo/database/models"
	"go-ooo/logger"
	"go-ooo/ooo_api/dex/types"
)

type DexInfo struct {
	CurrentBlock      uint64
	BlockPerMin       uint64
	ContractAddresses string
	// ContractAddressList is the same pool contracts as a raw slice (ContractAddresses is the
	// GraphQL-query-formatted string a subgraph source needs). A REST source (Cosmos SQS) echoes its
	// price under each of these so the orchestrator joins per-pool liquidity/confidence; a subgraph
	// source ignores it.
	ContractAddressList []string
}

// PoolSample is one pool's contribution to a price query: its identity, its backing
// liquidity (ReserveUsd) + trust score (for weighting), and its snapshot prices across the queried
// blocks. GetPricesFromDexModules returns these per-pool rather than a flat price list so the
// aggregator can reduce and weight each pool independently.
type PoolSample struct {
	Chain     string
	Dex       string
	Contract  string
	Liquidity float64
	// Confidence is the pool's dex-pair-verify trust score in [0,1] (XR1): it scales the pool's
	// weight in the aggregator's weighted mean. 0 means unscored (legacy/pre-trust rows) and is
	// treated as neutral by the aggregator, not zero-trust.
	Confidence float64
	Prices     []float64
}

// poolMeta carries the per-pool metadata the aggregator needs but the subgraph price query does not
// return - backing liquidity + trust score - joined back onto the prices by contract address.
type poolMeta struct {
	liquidity  float64
	confidence float64
}

// DexResult carries one DEX's per-pool price groups back from a fetch goroutine.
type DexResult struct {
	Chain string
	Dex   string
	Pools []types.PoolPrices
}

// poolQuery is one oriented price query against a single module within a price gather: the base/target
// SYMBOLS the query must use to orient the price, and the candidate verified pool rows for that
// orientation. The direct path yields at most one per module (the pools that literally trade
// base/target); the alias path (S7) yields one per distinct oriented symbol pair among the alias's
// member pools on that module.
type poolQuery struct {
	base   string
	target string
	pools  []models.DexPairs
}

// GetPricesFromDexModules collects per-pool price samples for an exact base/target symbol pair across
// every priceable module that lists it. It is the direct (non-alias) sample source; the alias path
// (GetAliasPricesFromDexModules, S7) feeds the same gatherSamples driver with a different pool plan.
func (dm *Manager) GetPricesFromDexModules(base, target string, minutes uint64) []PoolSample {
	return dm.gatherSamples(base, target, minutes, func(module Module) []poolQuery {
		rows, _ := dm.db.FindByDexPairName(base, target, module.Chain(), module.Dex())
		if len(rows) == 0 {
			logger.WarnWithFields("dex", "GetPricesFromDexModules", "check pair exists in db",
				"pair not found in database for this dex",
				logger.Fields{"chain": module.Chain(), "dex": module.Dex(), "base": base, "target": target})
			return nil
		}
		return []poolQuery{{base: base, target: target, pools: rows}}
	})
}

// gatherSamples is the shared per-module price-fetch driver behind both the direct and the alias
// (S7) sample sources. For each priceable module, queriesFor supplies the oriented price queries to
// run against it (direct: one for base/target; alias: one per oriented symbol pair among the member
// pools). The driver owns everything transport-/orchestration-shared: snapshotting the module set,
// caching each chain's current block (one RPC per chain, a failed chain skipped once), honouring each
// module's MinLiquidity floor, launching one transport-blind getPrices goroutine per query, and
// joining each pool's liquidity + trust score back onto the returned prices. base/target are the
// queried pair (or alias) and are used for logging only - the per-query orientation comes from each
// poolQuery. The aggregation in dexprice.go is identical regardless of which source produced the
// samples.
func (dm *Manager) gatherSamples(base, target string, minutes uint64, queriesFor func(module Module) []poolQuery) []PoolSample {
	var samples []PoolSample

	resCh := make(chan DexResult)
	errCh := make(chan error)
	// per-pool metadata (liquidity + trust score) keyed "chain|dex" -> lower(contract), to join onto
	// the per-pool prices the source returns in the receive loop. A module may launch more than one
	// query (alias path), so its meta accumulates across them.
	metaByMod := make(map[string]map[string]poolMeta)
	launched := 0
	dexSuccess := 0
	dexFail := 0
	dexNoData := 0

	// Cache each chain's current block number for this query: several modules share a chain,
	// so querying once avoids N identical RPC round-trips, and a chain whose RPC is down is
	// recorded once (chainRpcFailed) and its modules skipped, rather than re-failing per module.
	blockByChain := make(map[string]uint64)
	chainRpcFailed := make(map[string]bool)

	// snapshotModules guards against a concurrent SetModules (the manifest refresh) swapping the set
	// out mid-iteration.
	for _, module := range dm.snapshotModules() {
		queries := queriesFor(module)
		if len(queries) == 0 {
			continue
		}

		chain := module.Chain()
		if chainRpcFailed[chain] {
			continue
		}

		// A manifest-driven module may reference a chain go-ooo has no block/RPC config for; skip
		// it rather than nil-panicking below. Read under the lock (chainDef) so a concurrent
		// ApplyManifest swapping the chain set in can't race this lookup.
		chainDef := dm.chainDef(chain)
		if chainDef == nil {
			logger.WarnWithFields("dex", "gatherSamples", "check chain configured",
				"module's chain is not configured (no block/rpc) - skipping",
				logger.Fields{"chain": chain, "dex": module.Dex()})
			continue
		}

		// EVM chains query (and cache) the current block for the historical-block price window. A
		// non-EVM source (nil EthClient, e.g. the Cosmos SQS source) has no block query - currentBlock
		// stays 0 and the source ignores it, since Cosmos spot prices are current.
		var currentBlock uint64
		if chainDef.EthClient != nil {
			cb, cached := blockByChain[chain]
			if !cached {
				fetched, err := chainDef.EthClient.BlockNumber(dm.ctx)
				if err != nil {
					logger.ErrorWithFields("dex", "gatherSamples", "get current block", err.Error(), logger.Fields{
						"chain": chain,
						"dex":   module.Dex(),
					})

					// Mark the chain failed so its other modules are skipped without re-querying.
					chainRpcFailed[chain] = true
					continue
				}
				blockByChain[chain] = fetched
				cb = fetched
			}
			currentBlock = cb
		}

		blocksPerMin := uint64(chainDef.BlocksPerMin)
		modKey := chain + "|" + module.Dex()

		for _, q := range queries {
			meta := metaByMod[modKey]
			if meta == nil {
				meta = make(map[string]poolMeta)
				metaByMod[modKey] = meta
			}

			var contractAddresses []string
			for _, p := range q.pools {
				if p.ReserveUsd < float64(module.MinLiquidity()) {
					logger.WarnWithFields("dex", "gatherSamples", "check liquidity",
						"liquidity too low. Skipping",
						logger.Fields{
							"chain":         chain,
							"dex":           module.Dex(),
							"base":          q.base,
							"target":        q.target,
							"reserve_usd":   p.ReserveUsd,
							"min_liquidity": module.MinLiquidity(),
						})

					continue
				}

				contractAddresses = append(contractAddresses, p.ContractAddress)
				meta[strings.ToLower(p.ContractAddress)] = poolMeta{liquidity: p.ReserveUsd, confidence: p.Confidence}
			}

			if len(contractAddresses) == 0 {
				continue
			}

			logger.Debug("dex", "gatherSamples", "number contracts", "",
				logger.Fields{
					"chain":                  chain,
					"dex":                    module.Dex(),
					"base":                   q.base,
					"target":                 q.target,
					"num_contract_addresses": len(contractAddresses),
				})

			contractAddressesStr := fmt.Sprintf(`"%s"`, strings.Join(contractAddresses, `","`))

			dexInfo := DexInfo{
				CurrentBlock:        currentBlock,
				BlockPerMin:         blocksPerMin,
				ContractAddresses:   contractAddressesStr,
				ContractAddressList: contractAddresses,
			}

			launched++
			go getPrices(module, q.base, q.target, minutes, dexInfo, resCh, errCh)
		}
	}

	for i := 0; i < launched; i++ {
		r := <-resCh // receive result from channel resCh
		err := <-errCh

		if err != nil {
			logger.Error("dex", "gatherSamples", "getPrices",
				err.Error(),
			)
			dexFail++
			continue
		}

		logger.Debug("dex", "gatherSamples", "getPrices", "prices result",
			logger.Fields{
				"chain":     r.Chain,
				"dex":       r.Dex,
				"base":      base,
				"target":    target,
				"num_pools": len(r.Pools),
			})

		if len(r.Pools) > 0 {
			meta := metaByMod[r.Chain+"|"+r.Dex]
			for _, pool := range r.Pools {
				m := meta[strings.ToLower(pool.Contract)]
				samples = append(samples, PoolSample{
					Chain:      r.Chain,
					Dex:        r.Dex,
					Contract:   pool.Contract,
					Liquidity:  m.liquidity,
					Confidence: m.confidence,
					Prices:     pool.Prices,
				})
			}
			dexSuccess++
		} else {
			dexNoData++
		}
	}

	logger.Debug("dex", "gatherSamples", "", "",
		logger.Fields{
			"base":        base,
			"target":      target,
			"dex_success": dexSuccess,
			"dex_fail":    dexFail,
			"dex_no_data": dexNoData,
			"num_queries": launched,
			"num_pools":   len(samples),
		})

	return samples
}

func getPrices(source PriceSource, base, target string, minutes uint64, dexInfo DexInfo, resCh chan<- DexResult, errCh chan<- error) {
	// A malformed/partial source response can make the source's parsing panic (e.g. an unchecked
	// type assertion). As this runs in its own goroutine, an un-recovered panic would crash the
	// whole oracle; and because the caller reads exactly one (result, error) pair per source, a
	// panicking goroutine that never sends would deadlock the caller. Recover here and send the
	// empty result + error so one bad DEX degrades to "no data" instead of taking the process down.
	// The recover runs before any of the normal sends below, so it sends exactly once.
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorWithFields("dex", "getPrices", "recovered from panic", fmt.Sprintf("%v", r), logger.Fields{
				"chain":  source.Chain(),
				"dex":    source.Dex(),
				"base":   base,
				"target": target,
			})
			dexQueryTotal.WithLabelValues(source.Chain(), source.Dex(), "error").Inc()
			resCh <- DexResult{}
			errCh <- fmt.Errorf("%s, %s, %s, %s. getPrices recovered from panic: %v", source.Chain(), source.Dex(), base, target, r)
		}
	}()

	// The source owns its transport (subgraph GraphQL, REST, ...): FetchPoolPrices runs query +
	// fetch + parse and returns the per-pool prices, so this orchestrator stays transport-blind.
	dexPools, err := source.FetchPoolPrices(base, target, dexInfo, minutes)
	if err != nil {
		dexQueryTotal.WithLabelValues(source.Chain(), source.Dex(), "error").Inc()
		resCh <- DexResult{}
		errCh <- fmt.Errorf("%s, %s, %s, %s. getPrices fetch error: %s", source.Chain(), source.Dex(), base, target, err.Error())
		return
	}

	result := "success"
	if len(dexPools) == 0 {
		result = "no_data"
	}
	dexQueryTotal.WithLabelValues(source.Chain(), source.Dex(), result).Inc()

	resCh <- DexResult{
		Chain: source.Chain(),
		Dex:   source.Dex(),
		Pools: dexPools,
	}
	errCh <- nil

}
