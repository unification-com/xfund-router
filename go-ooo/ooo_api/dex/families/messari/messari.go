// Package messari implements the SchemaFamily strategy for subgraphs built on the Messari
// standardised dex-amm schema (https://github.com/messari/subgraphs). Unlike the Uniswap
// families it has no per-pair price ratio (token0Price/token1Price); instead each pool lists
// its inputTokens with a per-token lastPriceUSD, and a pair price is derived as the ratio
// lastPriceUSD(base) / lastPriceUSD(target). This unlocks the many DEXs Messari standardises
// (verified live 2026-06-10 against arbitrum/sushiswap + optimism/velodrome).
//
// It satisfies the dex package's SchemaFamily interface structurally and does not import the
// dex package (which constructs messari.Family values) to avoid an import cycle.
package messari

// entity is the Messari top-level pool collection. It is fixed for the family (there is no
// v2/v3-style variant), so no Params are needed yet. Velodrome's stable/volatile curve flag
// does not affect lastPriceUSD-based pricing, so it is not modelled here.
const entity = "liquidityPools"

// Family is the SchemaFamily implementation for Messari-schema subgraphs; construct with New.
type Family struct{}

// New returns a Messari schema family.
func New() Family {
	return Family{}
}
