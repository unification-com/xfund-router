// Package uniswap implements the SchemaFamily strategy for Uniswap-V2- and
// Uniswap-V3-style subgraphs. The two variants differ only in the GraphQL entity name
// (pairs vs pools) and the liquidity field (reserveUSD vs totalValueLockedUSD), so a
// single parameterised implementation serves every member DEX (uniswap_v2/v3, sushiswap,
// shibaswap, honeyswap, pancakeswap_v3, quickswap_v3, ...) instead of one hand-copied
// module each. Minimal v3-style subgraphs that expose only sqrtPrice are served by the same
// code with V3Sqrt.
//
// It satisfies the dex package's SchemaFamily interface structurally and deliberately does
// not import the dex package - the dex package constructs uniswap.Family values, so an
// import here would create a cycle.
package uniswap

// Params parameterises the schema variant. Entity and LiquidityField are the entire difference
// between a Uniswap-v2 and a Uniswap-v3 style subgraph (confirmed against the live modules,
// 2026-06-10); SqrtPrice covers minimal v3-style subgraphs that carry no derived prices.
type Params struct {
	Entity         string // GraphQL entity: "pairs" (v2) or "pools" (v3)
	LiquidityField string // liquidity field:  "reserveUSD" (v2) or "totalValueLockedUSD" (v3); "" if the schema has none
	SqrtPrice      bool   // price from the pool's sqrtPrice and token decimals instead of token0Price/token1Price
}

var (
	// V2 - Uniswap-v2-style subgraphs (pairs / reserveUSD).
	V2 = Params{Entity: "pairs", LiquidityField: "reserveUSD"}
	// V3 - Uniswap-v3-style subgraphs (pools / totalValueLockedUSD).
	V3 = Params{Entity: "pools", LiquidityField: "totalValueLockedUSD"}
	// V3Sqrt - minimal concentrated-liquidity subgraphs whose Pool has sqrtPrice but no
	// token0Price/token1Price and no USD liquidity (e.g. Pharaoh v3, Aerodrome Slipstream 2).
	// Prices are derived from sqrtPrice and the token decimals; with no liquidity figure to
	// refresh, the source's reserves come from the dex-pair-verify feed alone.
	V3Sqrt = Params{Entity: "pools", SqrtPrice: true}
)

// Family is the SchemaFamily implementation for a given variant; construct with New.
type Family struct {
	params Params
}

// New returns a Family configured for the given schema variant (uniswap.V2 / uniswap.V3).
func New(p Params) Family {
	return Family{params: p}
}
