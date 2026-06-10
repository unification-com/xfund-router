package types

const (
	// DefaultMinLiquidity - min liquidity a pair should have for the DEX pair search. Can be overridden in config.toml
	DefaultMinLiquidity = 30000
	// DefaultMinTxCount - min tx count a pair should have for the DEX pair search. Can be overridden in config.toml
	DefaultMinTxCount = 250

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

// PoolPrices is the per-pool price series a SchemaFamily extracts from a prices response:
// one entry per pool (keyed by its contract address), holding that pool's snapshot prices
// across the queried blocks. Grouping by pool - rather than flattening every pool's snapshots
// into one bag - lets the aggregator reduce and liquidity-weight each pool independently.
type PoolPrices struct {
	Contract string
	Prices   []float64
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
