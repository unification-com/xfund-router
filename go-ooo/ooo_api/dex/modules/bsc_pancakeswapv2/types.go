package bsc_pancakeswapv2

import (
	"fmt"
	"github.com/spf13/viper"
	"go-ooo/config"
	"go-ooo/ooo_api/dex/types"
)

const (
	ModuleName = "bsc_pancakeswapv2"
	Chain      = types.ChainBsc

	// MinLiquidity - min liquidity a pair should have for the DEX pair search
	MinLiquidity = types.MinLiquidity

	// MinTxCount - min tx count a pair should have for the DEX pair search
	MinTxCount = types.MinTxCount
)

var (

	// SubGraphUrl see https://docs.nodereal.io/reference/pancakeswap-graphql-api
	// SubGraphUrl requires Nodereal API key
	SubGraphUrl = fmt.Sprintf(`https://data-platform.nodereal.io/graph/v1/%s/projects/pancakeswap`, viper.GetString(config.ApiKeysNodereal))
)

type GraphQlToken struct {
	Id             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	Symbol         string `json:"symbol,omitempty"`
	TotalLiquidity string `json:"totalLiquidity,omitempty"`
	TxCount        string `json:"txCount,omitempty"`
	Typename       string `json:"__typename,omitempty"`
}

type GraphQlPairContent struct {
	Id                 string
	Token0             GraphQlToken
	Token1             GraphQlToken
	Token0Price        string `json:"token0Price"`
	Token1Price        string `json:"token1Price"`
	ReserveUSD         string `json:"reserveUSD,omitempty"`
	VolumeUSD          string `json:"volumeUSD,omitempty"`
	TxCount            string `json:"totalTransactions,omitempty"`
	Typename           string `json:"__typename,omitempty"`
	UntrackedVolumeUSD string `json:"untrackedVolumeUSD,omitempty"`
}

type GraphQlPairs struct {
	Pairs []GraphQlPairContent `json:"pairs,omitempty"`
}

type GraphQlErrors struct {
	Message string
}

type GraphQlPairsResponse struct {
	Data   GraphQlPairs
	Errors []GraphQlErrors
}
