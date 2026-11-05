// Package univ4 implements the SchemaFamily strategy for Uniswap-v4-style subgraphs (the
// official Uniswap/v4-subgraph schema and its forks). A v4 pool prices like a v3 pool - it
// exposes token0Price/token1Price and totalValueLockedUSD - but differs in two oracle-relevant
// ways this family handles:
//
//   - Hooks. A v4 pool can attach hook logic that acts on swaps (custom curves, dynamic fees,
//     taxes) and can make its reported price non-canonical or manipulable, and - with no factory
//     gatekeeping - such pools are cheap to spin up (more spoof surface). This family prices pools
//     with no hook, or with a hook that has no swap permissions (see actsOnSwaps), and skips the
//     rest, both when refreshing pair metadata and when pricing. An audited hooks allow-list can
//     widen this later.
//   - Native currency. v4 pools can hold native ETH as currency address 0x0; the subgraph
//     reports it with id 0x0000...0000 and symbol "ETH". dex-pair-verify normalises the native
//     side to the wrapped token (WETH) for identity and pricing, so a consumer queries
//     WETH.USDC; this family rewrites a token whose id is the zero address to the chain's
//     wrapped-native symbol so the symbol-based base/target orientation match lines up.
//
// Pool ids are 32-byte poolIds rather than 20-byte pool addresses; this family is agnostic to
// that - it queries and groups by whatever id dex-pair-verify curated for the pool.
//
// It satisfies the dex package's SchemaFamily interface structurally and deliberately does not
// import the dex package (which constructs univ4.Family values) to avoid an import cycle.
package univ4

import (
	"strconv"
	"strings"
)

// zeroAddress is both the no-hook sentinel (Pool.hooks) and the native-currency id (token.id)
// in the Uniswap v4 schema.
const zeroAddress = "0x0000000000000000000000000000000000000000"

// wrappedNativeByChain maps a chain to the symbol dex-pair-verify normalises its native currency
// to. The ETH-native chains wrap to WETH; Polygon's native POL wraps to WPOL; BSC's native BNB
// wraps to WBNB. A chain not listed here gets no native rewrite - its native-currency pools, if
// any, would price with the wrong orientation, so its wrapped symbol must be added here (and
// verified live) before a v4-style source is enabled on it. This is the "verify before seeding"
// discipline applied to native pricing.
var wrappedNativeByChain = map[string]string{
	"eth":         "WETH",
	"base":        "WETH",
	"arbitrum":    "WETH",
	"optimism":    "WETH",
	"polygon_pos": "WPOL",
	"bsc":         "WBNB",
}

// Family is the SchemaFamily implementation for Uniswap-v4-style subgraphs; construct with New.
type Family struct {
	wrappedNative string // the chain's wrapped-native symbol, or "" to disable native rewriting
}

// New returns a Family for the given chain. The chain selects the wrapped-native symbol used to
// normalise native-currency (id 0x0) tokens so symbol-based orientation matches the WETH.USDC
// form dex-pair-verify exports. An unmapped chain disables native rewriting (see
// wrappedNativeByChain).
func New(chain string) Family {
	return Family{wrappedNative: wrappedNativeByChain[strings.ToLower(chain)]}
}

// swapPermissionBits are the swap permissions of a v4 hook. A hook's permissions are the lowest 14
// bits of its address (Uniswap v4 Hooks.sol); these are beforeSwap (1<<7), afterSwap (1<<6),
// beforeSwapReturnDelta (1<<3) and afterSwapReturnDelta (1<<2). A hook with none of them can only act
// when the pool is created, on liquidity changes or on donations, so its pool prices like a plain one.
const swapPermissionBits = 1<<7 | 1<<6 | 1<<3 | 1<<2

// actsOnSwaps reports whether a pool's hook can act on swaps, making its price untrustworthy, so the
// pool must not be priced (mirrors dex-pair-verify's hookActsOnSwaps). No hook, or a hook without
// swap permissions, prices like a plain pool; an address that can't be read is treated as unsafe.
func actsOnSwaps(hooks string) bool {
	h := strings.ToLower(hooks)
	if h == "" || h == zeroAddress {
		return false
	}
	if len(h) < 4 {
		return true
	}
	low, err := strconv.ParseUint(h[len(h)-4:], 16, 16)
	return err != nil || low&swapPermissionBits != 0
}

// symbolFor returns the token's symbol, rewriting the native currency (id 0x0) to the chain's
// wrapped-native symbol when one is configured so orientation matching lines up with WETH.USDC.
func (f Family) symbolFor(id, symbol string) string {
	if f.wrappedNative != "" && strings.ToLower(id) == zeroAddress {
		return f.wrappedNative
	}
	return symbol
}
