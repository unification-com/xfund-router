package univ4

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func queryText(t *testing.T, raw []byte) string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("query is not valid JSON: %v", err)
	}
	return m["query"]
}

func TestGeneratePairsQuery(t *testing.T) {
	raw, err := New("eth").GeneratePairsQuery(`"0xABC"`)
	if err != nil {
		t.Fatalf("GeneratePairsQuery: %v", err)
	}
	q := queryText(t, raw)
	for _, want := range []string{"pools(", "hooks", "totalValueLockedUSD", "token0Price", `id_in: ["0xabc"]`} {
		if !strings.Contains(q, want) {
			t.Errorf("pairs query missing %q, got: %s", want, q)
		}
	}
}

func TestGenerateDexPricesQuery_QueryCount(t *testing.T) {
	cases := []struct {
		name         string
		minutes      uint64
		currentBlock uint64
		blocksPerMin uint64
		want         uint64
	}{
		{"latest only", 0, 1000, 10, 1},
		{"three history", 3, 1000, 10, 4},
		{"underflow stops early", 3, 15, 10, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, n, err := New("eth").GenerateDexPricesQuery(`"0xabc"`, c.minutes, c.currentBlock, c.blocksPerMin)
			if err != nil {
				t.Fatalf("GenerateDexPricesQuery: %v", err)
			}
			if n != c.want {
				t.Errorf("numQueries = %d, want %d", n, c.want)
			}
			q := queryText(t, raw)
			if !strings.Contains(q, "p0:") || !strings.Contains(q, "pools(") || !strings.Contains(q, "hooks") {
				t.Errorf("price query malformed: %s", q)
			}
		})
	}
}

// Real Uniswap v4 Ethereum mainnet metadata response captured 2026-06-12 (the deepest no-hook
// ETH/USDC pool). The native side (currency id 0x0, subgraph symbol "ETH") must normalise to
// WETH, and the liquidity must come from totalValueLockedUSD.
func TestProcessPairsQueryResult_RealNoHookPool(t *testing.T) {
	fixture := []byte(`{"data":{"pools":[{
		"id":"0xdce6394339af00981949f5f3baf27e3610c76326a700af57e4b3e3ae4977f78d",
		"hooks":"0x0000000000000000000000000000000000000000",
		"totalValueLockedUSD":"28210754.80762743181210710052353435",
		"volumeUSD":"1234.5",
		"txCount":"224601",
		"token0Price":"0.0005995550572066507491162008032366646",
		"token1Price":"1667.90353609731371978391944795638",
		"token0":{"id":"0x0000000000000000000000000000000000000000","symbol":"ETH","name":"Ether","decimals":"18"},
		"token1":{"id":"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48","symbol":"USDC","name":"USD Coin","decimals":"6"}
	}]}}`)

	pairs, err := New("eth").ProcessPairsQueryResult(fixture)
	if err != nil {
		t.Fatalf("ProcessPairsQueryResult: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	p := pairs[0]
	if p.Id != "0xdce6394339af00981949f5f3baf27e3610c76326a700af57e4b3e3ae4977f78d" {
		t.Errorf("id = %q (want the 32-byte poolId)", p.Id)
	}
	if p.Token0.Symbol != "WETH" {
		t.Errorf("token0 symbol = %q, want WETH (native ETH normalised)", p.Token0.Symbol)
	}
	if p.Token0.Contract != "0x0000000000000000000000000000000000000000" {
		t.Errorf("token0 contract = %q, want the native zero address", p.Token0.Contract)
	}
	if p.Token1.Symbol != "USDC" {
		t.Errorf("token1 symbol = %q, want USDC", p.Token1.Symbol)
	}
	if p.ReserveUSD != "28210754.80762743181210710052353435" {
		t.Errorf("reserveUSD (from TVL) = %q", p.ReserveUSD)
	}
}

// A pool whose hook acts on swaps must never be returned as a priceable pair (oracle hooks-safety
// policy); a no-hook pool and one whose hook has no swap permissions are. Real captured pools: a
// no-hook and a beforeSwap-hooked ETH/USDC pool, plus a hook with only liquidity/init permissions.
func TestProcessPairsQueryResult_SkipsSwapHooks(t *testing.T) {
	fixture := []byte(`{"data":{"pools":[
		{"id":"0xdce6","hooks":"0x0000000000000000000000000000000000000000","totalValueLockedUSD":"28210754","token0Price":"0.0006","token1Price":"1667.9","token0":{"id":"0x0000000000000000000000000000000000000000","symbol":"ETH"},"token1":{"id":"0xa0b8","symbol":"USDC"}},
		{"id":"0x91438ef00ff5e0b85cd964aa4e9b28ed088943e0268df3ed762e54e60e5c3884","hooks":"0x13ba8523f62decaee6489464935fbd8c3f505080","totalValueLockedUSD":"270933","token0Price":"0.0006","token1Price":"1660.73","token0":{"id":"0x0000000000000000000000000000000000000000","symbol":"ETH"},"token1":{"id":"0xa0b8","symbol":"USDC"}},
		{"id":"0x727b","hooks":"0x23ab2e6d4ab0c5f872567098671f1ffb46fd2500","totalValueLockedUSD":"48211","token0Price":"0.0006","token1Price":"1660.1","token0":{"id":"0x0000000000000000000000000000000000000000","symbol":"ETH"},"token1":{"id":"0xa0b8","symbol":"USDC"}}
	]}}`)

	pairs, err := New("eth").ProcessPairsQueryResult(fixture)
	if err != nil {
		t.Fatalf("ProcessPairsQueryResult: %v", err)
	}
	if len(pairs) != 2 || pairs[0].Id != "0xdce6" || pairs[1].Id != "0x727b" {
		t.Errorf("expected the no-hook and the no-swap-permission pools, got %+v", pairs)
	}
}

func TestActsOnSwaps(t *testing.T) {
	for hooks, want := range map[string]bool{
		"": false,
		"0x0000000000000000000000000000000000000000": false,
		"0x23ab2e6d4ab0c5f872567098671f1ffb46fd2500": false, // beforeInitialize, afterAdd/RemoveLiquidity only
		"0xd53006d1e3110fd319a79aeec4c527a0d265e080": true,  // beforeSwap
		"0x6b70fef40d3925881251c018164dbcec6bc94040": true,  // afterSwap
		"0xbdf938149ac6a781f94faa0ed45e6a0e984c6544": true,  // afterSwap + afterSwapReturnDelta
		"0x0000000000000000000000000000000000000008": true,  // beforeSwapReturnDelta alone
		"0xnot-hex": true, // unreadable: unsafe
	} {
		if got := actsOnSwaps(hooks); got != want {
			t.Errorf("actsOnSwaps(%q) = %v, want %v", hooks, got, want)
		}
	}
}

// The v3 convention applied to a native-ETH v4 pool: a WETH.USDC request must resolve to
// token1Price (~1667.9 "USDC per WETH"), which only works once the native "ETH" symbol is
// normalised to "WETH". The reverse request resolves to token0Price.
func TestProcessDexPricesResult_NativeOrientation(t *testing.T) {
	fixture := []byte(`{"data":{"p0":[{
		"id":"0xdce6394339af00981949f5f3baf27e3610c76326a700af57e4b3e3ae4977f78d",
		"hooks":"0x0000000000000000000000000000000000000000",
		"token0":{"id":"0x0000000000000000000000000000000000000000","symbol":"ETH"},
		"token1":{"id":"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48","symbol":"USDC"},
		"token0Price":"0.0005995550572066507491162008032366646",
		"token1Price":"1667.90353609731371978391944795638"
	}]}}`)

	got, err := New("eth").ProcessDexPricesResult("WETH", "USDC", 1, fixture)
	if err != nil {
		t.Fatalf("ProcessDexPricesResult: %v", err)
	}
	if len(got) != 1 || len(got[0].Prices) != 1 || math.Abs(got[0].Prices[0]-1667.90353609731372) > 1e-4 {
		t.Fatalf("WETH/USDC = %+v, want ~1667.9035", got)
	}

	got, err = New("eth").ProcessDexPricesResult("USDC", "WETH", 1, fixture)
	if err != nil {
		t.Fatalf("ProcessDexPricesResult: %v", err)
	}
	if len(got) != 1 || len(got[0].Prices) != 1 || math.Abs(got[0].Prices[0]-0.0005995550572066507) > 1e-12 {
		t.Fatalf("USDC/WETH = %+v, want ~0.00059955", got)
	}
}

// A hooked pool must not be priced even if it is somehow queried.
func TestProcessDexPricesResult_SkipsHookedPool(t *testing.T) {
	fixture := []byte(`{"data":{"p0":[{
		"id":"0x91438ef0",
		"hooks":"0x13ba8523f62decaee6489464935fbd8c3f505080",
		"token0":{"id":"0x0000000000000000000000000000000000000000","symbol":"ETH"},
		"token1":{"id":"0xa0b8","symbol":"USDC"},
		"token0Price":"0.0006","token1Price":"1660.73"
	}]}}`)

	got, err := New("eth").ProcessDexPricesResult("WETH", "USDC", 1, fixture)
	if err != nil {
		t.Fatalf("ProcessDexPricesResult: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("hooked pool must not be priced, got %+v", got)
	}
}

// On a chain whose wrapped-native symbol is not yet mapped, the native side is NOT rewritten, so
// a WETH.USDC request no longer matches and falls to the inverted token0Price. This locks in the
// "verify the wrapped symbol before enabling v4 on a new chain" requirement.
func TestProcessDexPricesResult_UnmappedChainDoesNotNormalise(t *testing.T) {
	fixture := []byte(`{"data":{"p0":[{
		"id":"0xdce6",
		"hooks":"0x0000000000000000000000000000000000000000",
		"token0":{"id":"0x0000000000000000000000000000000000000000","symbol":"ETH"},
		"token1":{"id":"0xa0b8","symbol":"USDC"},
		"token0Price":"0.0006","token1Price":"1667.9"
	}]}}`)

	got, err := New("some_unmapped_chain").ProcessDexPricesResult("WETH", "USDC", 1, fixture)
	if err != nil {
		t.Fatalf("ProcessDexPricesResult: %v", err)
	}
	// No rewrite -> base "WETH" != token0 "ETH" -> defaults to token0Price (0.0006), not 1667.9.
	if len(got) != 1 || math.Abs(got[0].Prices[0]-0.0006) > 1e-9 {
		t.Errorf("unmapped chain should not normalise native; got %+v, want ~0.0006", got)
	}
}

func TestProcessDexPricesResult_GraphQLError(t *testing.T) {
	fixture := []byte(`{"errors":[{"message":"bad indexers"}]}`)
	_, err := New("eth").ProcessDexPricesResult("WETH", "USDC", 1, fixture)
	if err == nil || !strings.Contains(err.Error(), "bad indexers") {
		t.Fatalf("expected GraphQL error to surface, got: %v", err)
	}
}
