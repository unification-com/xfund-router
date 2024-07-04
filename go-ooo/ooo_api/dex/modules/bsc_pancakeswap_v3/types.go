package bsc_pancakeswap_v3

import (
	"go-ooo/ooo_api/dex/types"
)

const (
	ModuleName             = "bsc_pancakeswap_v3"
	HostedSubgraphUrl      = ""
	GraphNetworkSubgraphId = "A1fvJWQLBeUAggX2WQTMm3FKjXTekNXo77ZySun4YN2m"
	Chain                  = types.ChainBsc
	Dex                    = "pancakeswap_v3"
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
