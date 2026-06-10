package uniswap

// graphErr is a single GraphQL error entry.
type graphErr struct {
	Message string `json:"message"`
}

// token mirrors the token fields the pairs/pools query selects.
type token struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	Symbol         string `json:"symbol"`
	TotalLiquidity string `json:"totalLiquidity"`
	TxCount        string `json:"txCount"`
	Typename       string `json:"__typename"`
}

// pairContent is one pair (v2) / pool (v3) entry. A single struct captures both schemas:
// the liquidity figure arrives as reserveUSD (v2) or totalValueLockedUSD (v3), and
// Params.LiquidityField selects which one to read.
type pairContent struct {
	Id                  string `json:"id"`
	Token0              token  `json:"token0"`
	Token1              token  `json:"token1"`
	Token0Price         string `json:"token0Price"`
	Token1Price         string `json:"token1Price"`
	VolumeUSD           string `json:"volumeUSD"`
	ReserveUSD          string `json:"reserveUSD"`
	TotalValueLockedUSD string `json:"totalValueLockedUSD"`
	TxCount             string `json:"txCount"`
	Typename            string `json:"__typename"`
	UntrackedVolumeUSD  string `json:"untrackedVolumeUSD"`
}

// pairsResponse decodes the metadata (pairs/pools) query. The entity key (pairs|pools) is
// read dynamically via Params.Entity, so the same struct serves every Uniswap-style schema
// - removing the per-DEX json:"pairs"-vs-"pools" copy bug that silently broke metadata
// refresh for the hand-written v3 modules.
type pairsResponse struct {
	Data   map[string][]pairContent `json:"data"`
	Errors []graphErr               `json:"errors"`
}

// priceToken / pricePair / pricesResponse decode the historical-price query. Decoding into
// typed structs (rather than map[string]any + type assertions) removes the class of
// unchecked-assertion panics that previously crashed the price goroutine on a partial reply.
type priceToken struct {
	Symbol string `json:"symbol"`
}

type pricePair struct {
	Id          string     `json:"id"`
	Token0      priceToken `json:"token0"`
	Token1      priceToken `json:"token1"`
	Token0Price string     `json:"token0Price"`
	Token1Price string     `json:"token1Price"`
}

type pricesResponse struct {
	Data   map[string][]pricePair `json:"data"`
	Errors []graphErr             `json:"errors"`
}
