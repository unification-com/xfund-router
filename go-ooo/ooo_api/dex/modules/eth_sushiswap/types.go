package eth_sushiswap

import (
	"go-ooo/ooo_api/dex/types"
)

const (
	ModuleName = "eth_sushiswap"
	// HostedSubgraphUrl will be deprecated
	HostedSubgraphUrl      = "https://api.thegraph.com/subgraphs/name/sushiswap/exchange"
	GraphNetworkSubgraphId = "6NUtT5mGjZ1tSshKLf5Q3uEEJtjBZJo1TpL5MXsUBqrT"
	Chain                  = types.ChainEth
	Dex                    = "sushiswap"
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
