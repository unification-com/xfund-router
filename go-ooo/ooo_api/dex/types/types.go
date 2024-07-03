package types

const (
	// MinLiquidity ToDo - make configurable in config.toml
	// MinLiquidity - min liquidity a pair should have for the DEX pair search
	MinLiquidity = 30000
	// MinTxCount ToDo - make configurable in config.toml
	// MinTxCount - min tx count a pair should have for the DEX pair search
	MinTxCount = 250

	ChainEth       = "eth"
	ChainPolygon   = "polygon_pos"
	ChainBsc       = "bsc"
	ChainXdai      = "xdai"
	ChainShibarium = "shibarium"
)

type DexToken struct {
	Id             string
	Contract       string
	Name           string
	Symbol         string
	TotalLiquidity string
	TxCount        string
	Typename       string
}

type DexPair struct {
	Id                 string
	Contract           string
	Token0             DexToken
	Token1             DexToken
	Token0Price        string
	Token1Price        string
	ReserveUSD         string
	VolumeUSD          string
	TxCount            string
	Typename           string
	UntrackedVolumeUSD string
}

type MetaDexToken struct {
	Chain           string `json:"chain,omitempty"`
	Symbol          string `json:"symbol,omitempty"`
	Name            string `json:"name,omitempty"`
	ContractAddress string `json:"contractAddress,omitempty"`
}

type MetaDexPair struct {
	ContractAddress string       `json:"contractAddress,omitempty"`
	Pair            string       `json:"pair,omitempty"`
	ReserveUsd      float64      `json:"reserveUsd,omitempty"`
	VolumeUsd       float64      `json:"volumeUsd,omitempty"`
	TxCount         uint64       `json:"txCount,omitempty"`
	Token0          MetaDexToken `json:"token0,omitempty"`
	Token1          MetaDexToken `json:"token1,omitempty"`
}

type PairMetaData struct {
	Pairs []MetaDexPair `json:"pairs,omitempty"`
	Chain string        `json:"chain,omitempty"`
	Dex   string        `json:"dex,omitempty"`
}
