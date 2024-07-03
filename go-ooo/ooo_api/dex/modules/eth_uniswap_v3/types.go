package eth_uniswap_v3

import (
	"go-ooo/ooo_api/dex/types"
)

const (
	ModuleName = "eth_uniswap_v3"
	// HostedSubgraphUrl will be deprecated
	HostedSubgraphUrl      = "https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v3"
	GraphNetworkSubgraphId = "5zvR82QoaXYFyDEKLZ9t6v9adgnptxYpKpSbxtgVENFV"
	Chain                  = types.ChainEth
	Dex                    = "uniswap_v3"

	// MinLiquidity - min liquidity a pair should have for the DEX pair search
	MinLiquidity = 75000 // types.MinLiquidity

	// MinTxCount - min tx count a pair should have for the DEX pair search
	MinTxCount = types.MinTxCount
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
	Id                  string
	Token0              GraphQlToken
	Token1              GraphQlToken
	Token0Price         string `json:"token0Price"`
	Token1Price         string `json:"token1Price"`
	VolumeUSD           string `json:"volumeUSD,omitempty"`
	TotalValueLockedUSD string `json:"totalValueLockedUSD,omitempty"`
	TxCount             string `json:"txCount,omitempty"`
	Typename            string `json:"__typename,omitempty"`
	UntrackedVolumeUSD  string `json:"untrackedVolumeUSD,omitempty"`
}

type GraphQlPairs struct {
	Pools []GraphQlPairContent `json:"pools,omitempty"`
}

type GraphQlErrors struct {
	Message string
}

type GraphQlPairsResponse struct {
	Data   GraphQlPairs
	Errors []GraphQlErrors
}
