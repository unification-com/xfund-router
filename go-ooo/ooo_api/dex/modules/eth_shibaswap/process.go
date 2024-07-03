package eth_shibaswap

import (
	"encoding/json"
	"errors"
	"fmt"
	"go-ooo/ooo_api/dex/types"
	"go-ooo/utils"
	"math/big"
)

func (d DexModule) processPairs(result []byte) ([]types.DexPair, error) {
	var decodedResponse GraphQlPairsResponse
	var pairs []types.DexPair

	err := json.Unmarshal(result, &decodedResponse)

	if err != nil {
		return nil, err
	}

	if decodedResponse.Errors != nil {
		return nil, errors.New(fmt.Sprintf("Error from GraphQL API: %s", decodedResponse.Errors[0].Message))
	}

	for _, pair := range decodedResponse.Data.Pairs {
		standardPair := types.DexPair{
			Id:       pair.Id,
			Contract: pair.Id,
			Token0: types.DexToken{
				Id:             pair.Token0.Id,
				Contract:       pair.Token0.Id,
				Name:           pair.Token0.Name,
				Symbol:         pair.Token0.Symbol,
				TotalLiquidity: pair.Token0.TotalLiquidity,
				TxCount:        pair.Token0.TxCount,
				Typename:       pair.Token0.Typename,
			},
			Token1: types.DexToken{
				Id:             pair.Token1.Id,
				Contract:       pair.Token1.Id,
				Name:           pair.Token1.Name,
				Symbol:         pair.Token1.Symbol,
				TotalLiquidity: pair.Token1.TotalLiquidity,
				TxCount:        pair.Token1.TxCount,
				Typename:       pair.Token1.Typename,
			},
			Token0Price:        pair.Token0Price,
			Token1Price:        pair.Token1Price,
			ReserveUSD:         pair.ReserveUSD,
			VolumeUSD:          pair.VolumeUSD,
			TxCount:            pair.TxCount,
			Typename:           pair.Typename,
			UntrackedVolumeUSD: pair.UntrackedVolumeUSD,
		}

		pairs = append(pairs, standardPair)
	}

	return pairs, nil
}

func (d DexModule) processPrices(base, target string, numQueries uint64, result []byte) ([]float64, error) {
	var decodedResponse map[string]any
	var prices []float64

	err := json.Unmarshal(result, &decodedResponse)

	if err != nil {
		return nil, err
	}

	if decodedResponse["errors"] != nil {
		retErrors := decodedResponse["errors"].([]interface{})
		retErr := retErrors[0].(map[string]any)
		return nil, errors.New(retErr["message"].(string))
	}

	pairPricesRes := decodedResponse["data"].(map[string]any)

	if pairPricesRes != nil {
		for i := 0; i < int(numQueries); i++ {
			priceResArray := pairPricesRes[fmt.Sprintf(`p%d`, i)].([]interface{})
			for _, pInst := range priceResArray {
				price, err := d.getPrice(base, target, pInst.(map[string]any))
				if err != nil {
					return prices, err
				}
				if price > 0 {
					prices = append(prices, price)
				}
			}
		}
	}
	return prices, nil
}

func (d DexModule) getPrice(base string, target string, pair map[string]any) (float64, error) {
	price := float64(0)

	var err error
	priceBf := big.NewFloat(0)

	t0 := pair["token0"].(map[string]any)
	t1 := pair["token1"].(map[string]any)

	if base == t0["symbol"] && target == t1["symbol"] {
		priceBf, err = utils.ParseBigFloat(pair["token1Price"].(string))
	} else {
		priceBf, err = utils.ParseBigFloat(pair["token0Price"].(string))
	}

	if err != nil {
		return price, err
	}

	price, _ = priceBf.Float64()

	return price, nil
}
