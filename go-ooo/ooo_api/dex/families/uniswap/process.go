package uniswap

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"

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
			price, err := f.priceFor(base, target, pair)
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
// otherwise - the same convention as the hand-written modules. A sqrtPrice schema has neither
// field, so the same choice is made on prices derived from sqrtPrice.
func (f Family) priceFor(base, target string, pair pricePair) (float64, error) {
	forward := base == pair.Token0.Symbol && target == pair.Token1.Symbol
	if f.params.SqrtPrice {
		return sqrtPriceFor(pair, forward)
	}

	raw := pair.Token0Price
	if forward {
		raw = pair.Token1Price
	}

	bf, err := utils.ParseBigFloat(raw)
	if err != nil {
		return 0, err
	}

	price, _ := bf.Float64()
	return price, nil
}

// q96 is 2^96, the fixed-point scale of a Uniswap-v3-style sqrtPriceX96.
var q96 = new(big.Float).SetInt(new(big.Int).Lsh(big.NewInt(1), 96))

// sqrtPriceFor derives token1Price (forward) or token0Price from a pool's sqrtPrice, which is
// sqrt(token1/token0) in raw token units as a Q64.96 fixed-point number:
//
//	token1 per token0 = (sqrtPrice / 2^96)^2 x 10^(decimals0 - decimals1)
//
// A pool with no price yet (sqrtPrice 0) yields 0, which the caller drops.
func sqrtPriceFor(pair pricePair, forward bool) (float64, error) {
	sqrtPrice, err := utils.ParseBigFloat(pair.SqrtPrice)
	if err != nil {
		return 0, err
	}
	dec0, err := tokenDecimals(pair.Token0)
	if err != nil {
		return 0, err
	}
	dec1, err := tokenDecimals(pair.Token1)
	if err != nil {
		return 0, err
	}
	if sqrtPrice.Sign() <= 0 {
		return 0, nil
	}

	ratio := new(big.Float).Quo(sqrtPrice, q96)
	price := new(big.Float).Mul(ratio, ratio)
	shift := dec0 - dec1
	if shift < 0 {
		shift = -shift
	}
	scale := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(shift)), nil))
	if dec0 >= dec1 {
		price.Mul(price, scale)
	} else {
		price.Quo(price, scale)
	}
	if !forward {
		price.Quo(new(big.Float).SetInt64(1), price)
	}

	p, _ := price.Float64()
	return p, nil
}

// tokenDecimals parses a token's ERC-20 decimals, which fit a uint8.
func tokenDecimals(t priceToken) (int, error) {
	d, err := strconv.ParseUint(t.Decimals, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("token %s decimals %q: %w", t.Symbol, t.Decimals, err)
	}
	return int(d), nil
}
