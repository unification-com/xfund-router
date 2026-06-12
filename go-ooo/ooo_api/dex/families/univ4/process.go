package univ4

import (
	"encoding/json"
	"errors"
	"fmt"

	"go-ooo/ooo_api/dex/types"
	"go-ooo/utils"
)

// ProcessPairsQueryResult decodes a metadata response into the canonical []types.DexPair. Hooked
// pools are skipped (this family does not price them) and the native currency's symbol is
// normalised to the chain's wrapped symbol so the stored pair matches the WETH.USDC form.
func (f Family) ProcessPairsQueryResult(result []byte) ([]types.DexPair, error) {
	var decoded pairsResponse
	if err := json.Unmarshal(result, &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Errors) > 0 {
		return nil, fmt.Errorf("error from GraphQL API: %s", decoded.Errors[0].Message)
	}

	var pairs []types.DexPair
	for _, pool := range decoded.Data[entity] {
		if isHooked(pool.Hooks) {
			// A hooked pool's price may be non-canonical/manipulable - never store it as a
			// priceable pair (oracle hooks-safety policy: no-hook pools only for now).
			continue
		}
		pairs = append(pairs, types.DexPair{
			Id:                 pool.Id,
			Contract:           pool.Id,
			Token0:             f.toDexToken(pool.Token0),
			Token1:             f.toDexToken(pool.Token1),
			Token0Price:        pool.Token0Price,
			Token1Price:        pool.Token1Price,
			ReserveUSD:         pool.TotalValueLockedUSD,
			VolumeUSD:          pool.VolumeUSD,
			TxCount:            pool.TxCount,
			UntrackedVolumeUSD: pool.UntrackedVolumeUSD,
		})
	}

	return pairs, nil
}

func (f Family) toDexToken(t token) types.DexToken {
	return types.DexToken{
		Id:       t.Id,
		Contract: t.Id,
		Name:     t.Name,
		Symbol:   f.symbolFor(t.Id, t.Symbol),
	}
}

// ProcessDexPricesResult walks the p0..p{numQueries-1} aliases and returns the positive prices
// grouped by pool (so each pool's snapshot series can be reduced and weighted independently),
// skipping hooked pools. Typed decoding means a malformed or partial reply yields an error here
// rather than a panic in the price goroutine.
func (f Family) ProcessDexPricesResult(base, target string, numQueries uint64, result []byte) ([]types.PoolPrices, error) {
	var decoded pricesResponse
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
			if isHooked(pool.Hooks) {
				continue
			}
			price, err := f.priceFor(base, target, pool)
			if err != nil {
				return nil, err
			}
			if price > 0 {
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

// priceFor picks token1Price when (base,target) matches (token0,token1) and token0Price
// otherwise - the same convention as the uniswap family - after normalising the native currency
// symbol so a WETH.USDC request matches a pool whose native side the subgraph reports as "ETH".
func (f Family) priceFor(base, target string, pool pricePool) (float64, error) {
	token0Symbol := f.symbolFor(pool.Token0.Id, pool.Token0.Symbol)
	token1Symbol := f.symbolFor(pool.Token1.Id, pool.Token1.Symbol)

	raw := pool.Token0Price
	if base == token0Symbol && target == token1Symbol {
		raw = pool.Token1Price
	}

	bf, err := utils.ParseBigFloat(raw)
	if err != nil {
		return 0, err
	}

	price, _ := bf.Float64()
	return price, nil
}
