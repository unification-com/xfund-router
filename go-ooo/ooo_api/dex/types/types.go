package types

const (
	// MinLiquidity ToDo - make configurable in config.toml
	// MinLiquidity - min liquidity a pair should have for the DEX pair search
	MinLiquidity = 30000
	// MinTxCount ToDo - make configurable in config.toml
	// MinTxCount - min tx count a pair should have for the DEX pair search
	MinTxCount = 250

	ChainEth       = "eth"
	ChainPolygon   = "polygon"
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
