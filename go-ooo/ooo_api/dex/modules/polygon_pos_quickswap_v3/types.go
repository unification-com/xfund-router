package polygon_pos_quickswap_v3

import (
	"go-ooo/ooo_api/dex/types"
)

const (
	ModuleName = "polygon_pos_quickswap_v3"
	// HostedSubgraphUrl will be deprecated
	HostedSubgraphUrl      = "https://api.thegraph.com/subgraphs/name/sameepsi/quickswap06"
	GraphNetworkSubgraphId = "FqsRcH1XqSjqVx9GRTvEJe959aCbKrcyGgDWBrUkG24g"
	Chain                  = types.ChainPolygon
	Dex                    = "quickswap_v3"

	// MinLiquidity - min liquidity a pair should have for the DEX pair search
	MinLiquidity = types.MinLiquidity

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
	Id                 string
	Token0             GraphQlToken
	Token1             GraphQlToken
	Token0Price        string `json:"token0Price"`
	Token1Price        string `json:"token1Price"`
	ReserveUSD         string `json:"totalValueLockedUSD,omitempty"`
	VolumeUSD          string `json:"volumeUSD,omitempty"`
	TxCount            string `json:"txCount,omitempty"`
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
