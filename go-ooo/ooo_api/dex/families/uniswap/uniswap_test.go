package uniswap

import (
	"encoding/json"
	"strings"
	"testing"
)

// queryText unmarshals a generated GraphQL request and returns its "query" string.
func queryText(t *testing.T, raw []byte) string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("query is not valid JSON: %v", err)
	}
	return m["query"]
}

func TestGeneratePairsQuery_EntityAndLiquidityField(t *testing.T) {
	v2, err := New(V2).GeneratePairsQuery(`"0xABC"`)
	if err != nil {
		t.Fatalf("v2 GeneratePairsQuery: %v", err)
	}
	q := queryText(t, v2)
	if !strings.Contains(q, "pairs(") {
		t.Errorf("v2 query should select the pairs entity, got: %s", q)
	}
	if !strings.Contains(q, "reserveUSD") {
		t.Errorf("v2 query should request reserveUSD, got: %s", q)
	}
	if !strings.Contains(q, `id_in: ["0xabc"]`) {
		t.Errorf("v2 query should lower-case and embed the addresses, got: %s", q)
	}

	v3, err := New(V3).GeneratePairsQuery(`"0xABC"`)
	if err != nil {
		t.Fatalf("v3 GeneratePairsQuery: %v", err)
	}
	q = queryText(t, v3)
	if !strings.Contains(q, "pools(") {
		t.Errorf("v3 query should select the pools entity, got: %s", q)
	}
	if !strings.Contains(q, "totalValueLockedUSD") {
		t.Errorf("v3 query should request totalValueLockedUSD, got: %s", q)
	}
	if strings.Contains(q, "reserveUSD") {
		t.Errorf("v3 query should not request reserveUSD, got: %s", q)
	}
}

func TestProcessPairsQueryResult_V2(t *testing.T) {
	fixture := []byte(`{"data":{"pairs":[{
		"id":"0xpair",
		"token0":{"id":"0xweth","symbol":"WETH","name":"Wrapped Ether","__typename":"Token"},
		"token1":{"id":"0xusdc","symbol":"USDC","name":"USD Coin","__typename":"Token"},
		"token0Price":"0.0006","token1Price":"1636.81",
		"reserveUSD":"1000000","volumeUSD":"50000","txCount":"1234",
		"untrackedVolumeUSD":"49000","__typename":"Pair"
	}]}}`)

	pairs, err := New(V2).ProcessPairsQueryResult(fixture)
	if err != nil {
		t.Fatalf("ProcessPairsQueryResult: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	p := pairs[0]
	if p.Id != "0xpair" || p.Contract != "0xpair" {
		t.Errorf("id/contract = %q/%q, want 0xpair/0xpair", p.Id, p.Contract)
	}
	if p.Token0.Symbol != "WETH" || p.Token0.Contract != "0xweth" {
		t.Errorf("token0 = %q/%q, want WETH/0xweth", p.Token0.Symbol, p.Token0.Contract)
	}
	if p.Token1.Symbol != "USDC" {
		t.Errorf("token1 symbol = %q, want USDC", p.Token1.Symbol)
	}
	if p.ReserveUSD != "1000000" {
		t.Errorf("reserveUSD = %q, want 1000000", p.ReserveUSD)
	}
	if p.Token0Price != "0.0006" || p.Token1Price != "1636.81" {
		t.Errorf("prices = %q/%q, want 0.0006/1636.81", p.Token0Price, p.Token1Price)
	}
	if p.TxCount != "1234" || p.Typename != "Pair" {
		t.Errorf("txCount/typename = %q/%q, want 1234/Pair", p.TxCount, p.Typename)
	}
}

func TestProcessPairsQueryResult_V3_LiquidityFromTVL(t *testing.T) {
	// v3 schema: entity "pools", liquidity arrives as totalValueLockedUSD (no reserveUSD).
	fixture := []byte(`{"data":{"pools":[{
		"id":"0xpool",
		"token0":{"id":"0xweth","symbol":"WETH","__typename":"Token"},
		"token1":{"id":"0xusdc","symbol":"USDC","__typename":"Token"},
		"token0Price":"0.0006","token1Price":"1636.81",
		"totalValueLockedUSD":"2000000","__typename":"Pool"
	}]}}`)

	pairs, err := New(V3).ProcessPairsQueryResult(fixture)
	if err != nil {
		t.Fatalf("ProcessPairsQueryResult: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pairs))
	}
	if pairs[0].ReserveUSD != "2000000" {
		t.Errorf("reserveUSD (from TVL) = %q, want 2000000", pairs[0].ReserveUSD)
	}
}

func TestProcessPairsQueryResult_GraphQLError(t *testing.T) {
	fixture := []byte(`{"errors":[{"message":"bad subgraph"}]}`)
	_, err := New(V2).ProcessPairsQueryResult(fixture)
	if err == nil || !strings.Contains(err.Error(), "bad subgraph") {
		t.Fatalf("expected a GraphQL error to surface, got: %v", err)
	}
}

func TestGenerateDexPricesQuery_QueryCount(t *testing.T) {
	cases := []struct {
		name         string
		minutes      uint64
		currentBlock uint64
		blocksPerMin uint64
		wantQueries  uint64
	}{
		{"latest only", 0, 1000, 10, 1},
		{"three history points", 3, 1000, 10, 4},
		{"underflow stops early", 3, 15, 10, 2}, // p0 + p1(block 5); i=2 sub=20 >= 15 breaks
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, n, err := New(V2).GenerateDexPricesQuery(`"0xabc"`, c.minutes, c.currentBlock, c.blocksPerMin)
			if err != nil {
				t.Fatalf("GenerateDexPricesQuery: %v", err)
			}
			if n != c.wantQueries {
				t.Errorf("numQueries = %d, want %d", n, c.wantQueries)
			}
			q := queryText(t, raw)
			if !strings.Contains(q, "p0:") {
				t.Errorf("query should always include the p0 alias, got: %s", q)
			}
			if !strings.Contains(q, "pairs(") {
				t.Errorf("v2 price query should select the pairs entity, got: %s", q)
			}
		})
	}
}

func TestGenerateDexPricesQuery_V3Entity(t *testing.T) {
	raw, _, err := New(V3).GenerateDexPricesQuery(`"0xabc"`, 0, 1000, 10)
	if err != nil {
		t.Fatalf("GenerateDexPricesQuery: %v", err)
	}
	if q := queryText(t, raw); !strings.Contains(q, "pools(") {
		t.Errorf("v3 price query should select the pools entity, got: %s", q)
	}
}

func TestProcessDexPricesResult_Ordering(t *testing.T) {
	// token0=WETH, token1=USDC: base==token0 && target==token1 uses token1Price (WETH priced
	// in USDC), otherwise token0Price. Prices come back grouped by pool.
	fixture := []byte(`{"data":{"p0":[{
		"id":"0xpool",
		"token0":{"symbol":"WETH"},"token1":{"symbol":"USDC"},
		"token0Price":"0.0006","token1Price":"1636.81"
	}]}}`)

	got, err := New(V2).ProcessDexPricesResult("WETH", "USDC", 1, fixture)
	if err != nil {
		t.Fatalf("ProcessDexPricesResult: %v", err)
	}
	if len(got) != 1 || got[0].Contract != "0xpool" || len(got[0].Prices) != 1 || got[0].Prices[0] != 1636.81 {
		t.Errorf("WETH/USDC = %+v, want pool 0xpool [1636.81]", got)
	}

	got, err = New(V2).ProcessDexPricesResult("USDC", "WETH", 1, fixture)
	if err != nil {
		t.Fatalf("ProcessDexPricesResult: %v", err)
	}
	if len(got) != 1 || len(got[0].Prices) != 1 || got[0].Prices[0] != 0.0006 {
		t.Errorf("USDC/WETH = %+v, want [0.0006]", got)
	}
}

func TestProcessDexPricesResult_MultiBlockGroupsByPool(t *testing.T) {
	// The same pool across two blocks groups into one pool with two snapshot prices.
	fixture := []byte(`{"data":{
		"p0":[{"id":"0xpool","token0":{"symbol":"WETH"},"token1":{"symbol":"USDC"},"token0Price":"0.0006","token1Price":"1600"}],
		"p1":[{"id":"0xpool","token0":{"symbol":"WETH"},"token1":{"symbol":"USDC"},"token0Price":"0.0006","token1Price":"1650"}]
	}}`)

	got, err := New(V2).ProcessDexPricesResult("WETH", "USDC", 2, fixture)
	if err != nil {
		t.Fatalf("ProcessDexPricesResult: %v", err)
	}
	if len(got) != 1 || len(got[0].Prices) != 2 {
		t.Fatalf("expected 1 pool with 2 prices, got %+v", got)
	}
}

func TestProcessDexPricesResult_GraphQLError(t *testing.T) {
	fixture := []byte(`{"errors":[{"message":"rate limited"}]}`)
	_, err := New(V2).ProcessDexPricesResult("WETH", "USDC", 1, fixture)
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("expected GraphQL error to surface, got: %v", err)
	}
}

// A malformed/partial reply (here a missing price string) must return an error, not panic
// the price goroutine - the regression that the typed decode replaces unchecked assertions
// to prevent.
func TestProcessDexPricesResult_MalformedDoesNotPanic(t *testing.T) {
	fixture := []byte(`{"data":{"p0":[{
		"token0":{"symbol":"WETH"},"token1":{"symbol":"USDC"},"token1Price":""
	}]}}`)

	_, err := New(V2).ProcessDexPricesResult("WETH", "USDC", 1, fixture)
	if err == nil {
		t.Fatal("expected an error for the empty price string, got nil")
	}
}
