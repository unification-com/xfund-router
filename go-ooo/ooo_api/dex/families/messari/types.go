package messari

// graphErr is a single GraphQL error entry.
type graphErr struct {
	Message string `json:"message"`
}

// inputToken is one of a pool's constituent tokens. lastPriceUSD is the Messari-computed USD
// price of the token and is the basis for all pricing in this family. It can be empty/null
// for an unpriced token, in which case the pool is simply skipped (not an error).
type inputToken struct {
	Id           string `json:"id"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Decimals     int    `json:"decimals"`
	LastPriceUSD string `json:"lastPriceUSD"`
}

// liquidityPool is a Messari pool. The pairs query populates every field; the lighter prices
// query populates only id + inputTokens{symbol,lastPriceUSD}, leaving the rest zero-valued.
type liquidityPool struct {
	Id                  string       `json:"id"`
	Name                string       `json:"name"`
	Symbol              string       `json:"symbol"`
	TotalValueLockedUSD string       `json:"totalValueLockedUSD"`
	InputTokens         []inputToken `json:"inputTokens"`
}

// response decodes both the pairs query (entity key "liquidityPools") and the prices query
// (alias keys p0..pN) - the entity/alias is read dynamically from the map, the same approach
// the uniswap family uses.
type response struct {
	Data   map[string][]liquidityPool `json:"data"`
	Errors []graphErr                 `json:"errors"`
}
