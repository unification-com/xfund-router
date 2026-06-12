package dex

import (
	"go-ooo/ooo_api/dex/families/messari"
	"go-ooo/ooo_api/dex/families/univ4"
)

// Compile-time checks that each schema family satisfies the SchemaFamily interface. The uniswap
// and messari families are also exercised by the manifest builder; the checks here keep every
// family honest and built even before a manifest source references it. univ4 (Uniswap v4) is
// asserted because its sources are seeded on the dex-pair-verify side and become live once the
// manifest flips them to priceable (see MODERN_DEX_SUPPORT.md).
var (
	_ SchemaFamily = messari.Family{}
	_ SchemaFamily = univ4.Family{}
)
