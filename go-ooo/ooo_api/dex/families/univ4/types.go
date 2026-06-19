package univ4

// graphErr is a single GraphQL error entry.
type graphErr struct {
	Message string `json:"message"`
}

// token mirrors the token fields the pools query selects. id is needed to detect the native
// currency (id 0x0) for symbol normalisation.
type token struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals string `json:"decimals"`
}

// poolContent is one pool entry from the metadata query. hooks gates the pool (hooked pools are
// not priced); the liquidity figure is totalValueLockedUSD (as for Uniswap v3).
type poolContent struct {
	Id                  string `json:"id"`
	Hooks               string `json:"hooks"`
	Token0              token  `json:"token0"`
	Token1              token  `json:"token1"`
	Token0Price         string `json:"token0Price"`
	Token1Price         string `json:"token1Price"`
	TotalValueLockedUSD string `json:"totalValueLockedUSD"`
	VolumeUSD           string `json:"volumeUSD"`
	TxCount             string `json:"txCount"`
	UntrackedVolumeUSD  string `json:"untrackedVolumeUSD"`
}

// pairsResponse decodes the metadata (pools) query; the entity key "pools" is read dynamically
// from the data map, the same approach the uniswap family uses.
type pairsResponse struct {
	Data   map[string][]poolContent `json:"data"`
	Errors []graphErr               `json:"errors"`
}

// pricePool decodes one pool in the historical-price query. id + hooks gate the pool; the token
// id/symbol pairs drive native normalisation and the orientation match. Typed decoding means a
// malformed reply yields an error here rather than a panic in the price goroutine.
type pricePool struct {
	Id          string `json:"id"`
	Hooks       string `json:"hooks"`
	Token0      token  `json:"token0"`
	Token1      token  `json:"token1"`
	Token0Price string `json:"token0Price"`
	Token1Price string `json:"token1Price"`
}

type pricesResponse struct {
	Data   map[string][]pricePool `json:"data"`
	Errors []graphErr             `json:"errors"`
}
