package dex

import "go-ooo/ooo_api/dex/families/messari"

// Compile-time checks that each schema family satisfies the SchemaFamily interface. The
// uniswap family is also checked implicitly where seed.go constructs it; messari is asserted
// here because its sources are not yet wired into a running module - arbitrum/sushiswap and
// optimism/velodrome need the new-chain plumbing (arbitrum, optimism) plus the dex-pair-verify
// manifest flipping them to priceable, which lands with the manifest-driven builder (see
// MODULAR_DEX_DISPATCH step 5). The assertion keeps the family honest and built in the meantime.
var _ SchemaFamily = messari.Family{}
