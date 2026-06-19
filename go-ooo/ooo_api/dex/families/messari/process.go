package messari

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"go-ooo/ooo_api/dex/types"
	"go-ooo/utils"
)

// ProcessPairsQueryResult decodes a metadata response into the canonical []types.DexPair.
// A Messari pool can hold more than two inputTokens; the canonical DexPair models exactly
// two, so the first two are mapped (sushiswap/velodrome pools are 2-token). Pricing does not
// depend on this mapping - it matches tokens by symbol - so multi-asset pools still price.
func (f Family) ProcessPairsQueryResult(result []byte) ([]types.DexPair, error) {
	var decoded response
	if err := json.Unmarshal(result, &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Errors) > 0 {
		return nil, fmt.Errorf("error from GraphQL API: %s", decoded.Errors[0].Message)
	}

	var pairs []types.DexPair
	for _, pool := range decoded.Data[entity] {
		if len(pool.InputTokens) < 2 {
			// Not a tradeable pair (single-sided / malformed) - skip.
			continue
		}
		pairs = append(pairs, types.DexPair{
			Id:         pool.Id,
			Contract:   pool.Id,
			Token0:     toDexToken(pool.InputTokens[0]),
			Token1:     toDexToken(pool.InputTokens[1]),
			ReserveUSD: pool.TotalValueLockedUSD,
			// Messari has no native pair ratio - Token0Price/Token1Price are left empty; the
			// prices path derives a price from inputTokens[].lastPriceUSD instead.
		})
	}

	return pairs, nil
}

func toDexToken(t inputToken) types.DexToken {
	return types.DexToken{
		Id:       t.Id,
		Contract: t.Id,
		Name:     t.Name,
		Symbol:   t.Symbol,
	}
}

// ProcessDexPricesResult walks the p0..p{numQueries-1} aliases and returns the prices that
// could be derived for the requested pair, grouped by pool so each pool's snapshot series can
// be reduced and weighted independently.
func (f Family) ProcessDexPricesResult(base, target string, numQueries uint64, result []byte) ([]types.PoolPrices, error) {
	var decoded response
	if err := json.Unmarshal(result, &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Errors) > 0 {
		return nil, errors.New(decoded.Errors[0].Message)
	}

	byPool := make(map[string][]float64)
	var order []string // preserve first-seen pool order for deterministic output
	for i := uint64(0); i < numQueries; i++ {
		for _, pool := range decoded.Data[fmt.Sprintf("p%d", i)] {
			if price, ok := priceFor(base, target, pool); ok && price > 0 {
				if _, seen := byPool[pool.Id]; !seen {
					order = append(order, pool.Id)
				}
				byPool[pool.Id] = append(byPool[pool.Id], price)
			}
		}
	}

	pools := make([]types.PoolPrices, 0, len(order))
	for _, id := range order {
		pools = append(pools, types.PoolPrices{Contract: id, Prices: byPool[id]})
	}

	return pools, nil
}

// priceFor derives base/target as the ratio of the two tokens' lastPriceUSD within the pool:
// (base in USD) / (target in USD) = how many target per base, matching the Uniswap families'
// "1 base = N target" convention. It returns ok=false (rather than an error) when the pool
// lacks either token or a usable lastPriceUSD, so one unpriced pool never fails the whole DEX.
func priceFor(base, target string, pool liquidityPool) (float64, bool) {
	var basePrice, targetPrice *big.Float

	for _, t := range pool.InputTokens {
		switch t.Symbol {
		case base:
			if basePrice == nil {
				if p, err := utils.ParseBigFloat(t.LastPriceUSD); err == nil {
					basePrice = p
				}
			}
		case target:
			if targetPrice == nil {
				if p, err := utils.ParseBigFloat(t.LastPriceUSD); err == nil {
					targetPrice = p
				}
			}
		}
	}

	if basePrice == nil || targetPrice == nil || targetPrice.Sign() <= 0 {
		return 0, false
	}

	price, _ := new(big.Float).Quo(basePrice, targetPrice).Float64()
	return price, true
}
