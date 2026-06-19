package uniswap

import (
	"encoding/json"
	"errors"
	"fmt"

	"go-ooo/ooo_api/dex/types"
	"go-ooo/utils"
)

// ProcessPairsQueryResult decodes a metadata response into the canonical []types.DexPair.
// The entity key (pairs|pools) and the liquidity field are selected from the family's
// Params, so one parser serves every Uniswap-style schema.
func (f Family) ProcessPairsQueryResult(result []byte) ([]types.DexPair, error) {
	var decoded pairsResponse
	if err := json.Unmarshal(result, &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Errors) > 0 {
		return nil, fmt.Errorf("error from GraphQL API: %s", decoded.Errors[0].Message)
	}

	var pairs []types.DexPair
	for _, pair := range decoded.Data[f.params.Entity] {
		liquidity := pair.ReserveUSD
		if f.params.LiquidityField == "totalValueLockedUSD" {
			liquidity = pair.TotalValueLockedUSD
		}

		pairs = append(pairs, types.DexPair{
			Id:                 pair.Id,
			Contract:           pair.Id,
			Token0:             toDexToken(pair.Token0),
			Token1:             toDexToken(pair.Token1),
			Token0Price:        pair.Token0Price,
			Token1Price:        pair.Token1Price,
			ReserveUSD:         liquidity,
			VolumeUSD:          pair.VolumeUSD,
			TxCount:            pair.TxCount,
			Typename:           pair.Typename,
			UntrackedVolumeUSD: pair.UntrackedVolumeUSD,
		})
	}

	return pairs, nil
}

func toDexToken(t token) types.DexToken {
	return types.DexToken{
		Id:             t.Id,
		Contract:       t.Id,
		Name:           t.Name,
		Symbol:         t.Symbol,
		TotalLiquidity: t.TotalLiquidity,
		TxCount:        t.TxCount,
		Typename:       t.Typename,
	}
}

// ProcessDexPricesResult walks the p0..p{numQueries-1} aliases and returns the positive
// prices grouped by pool (so each pool's snapshot series can be reduced and weighted
// independently). Typed decoding means a malformed or partial reply yields an error here
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
		for _, pair := range decoded.Data[fmt.Sprintf("p%d", i)] {
			price, err := priceFor(base, target, pair)
			if err != nil {
				return nil, err
			}
			if price > 0 {
				if _, seen := byPool[pair.Id]; !seen {
					order = append(order, pair.Id)
				}
				byPool[pair.Id] = append(byPool[pair.Id], price)
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
// otherwise - the same convention as the hand-written modules.
func priceFor(base, target string, pair pricePair) (float64, error) {
	raw := pair.Token0Price
	if base == pair.Token0.Symbol && target == pair.Token1.Symbol {
		raw = pair.Token1Price
	}

	bf, err := utils.ParseBigFloat(raw)
	if err != nil {
		return 0, err
	}

	price, _ := bf.Float64()
	return price, nil
}
