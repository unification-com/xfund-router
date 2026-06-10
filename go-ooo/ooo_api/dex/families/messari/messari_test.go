package messari

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
	raw, err := New().GeneratePairsQuery(`"0xABC"`)
	if err != nil {
		t.Fatalf("GeneratePairsQuery: %v", err)
	}
	q := queryText(t, raw)
	for _, want := range []string{"liquidityPools(", "lastPriceUSD", "totalValueLockedUSD", `id_in: ["0xabc"]`} {
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
			raw, n, err := New().GenerateDexPricesQuery(`"0xabc"`, c.minutes, c.currentBlock, c.blocksPerMin)
			if err != nil {
				t.Fatalf("GenerateDexPricesQuery: %v", err)
			}
			if n != c.want {
				t.Errorf("numQueries = %d, want %d", n, c.want)
			}
			if q := queryText(t, raw); !strings.Contains(q, "p0:") || !strings.Contains(q, "liquidityPools(") {
				t.Errorf("price query malformed: %s", q)
			}
		})
	}
}

// Real arbitrum/sushiswap response captured 2026-06-10 (the WETH/FTDex pool). Maps to the
// canonical DexPair, with liquidity from totalValueLockedUSD and the two inputTokens.
func TestProcessPairsQueryResult_RealPool(t *testing.T) {
	fixture := []byte(`{"data":{"liquidityPools":[{
		"id":"0x48b4cf6d13ccb8113f36488a74a2c2bd6914045c",
		"name":"SushiSwap Wrapped Ether/FTDex",
		"symbol":"Wrapped Ether/FTDex",
		"totalValueLockedUSD":"21681497271086118701084.80466084228",
		"inputTokens":[
			{"id":"0x82af49447d8a07e3bd95bd0d56f35241523fbab1","symbol":"WETH","name":"Wrapped Ether","decimals":18,"lastPriceUSD":"1638.117649447512158148814861754781"},
			{"id":"0x8a77984d1a5659893473b7fd01c551e4d9ff6f4c","symbol":"FTD","name":"FTDex","decimals":9,"lastPriceUSD":"0.00650444918132583557496692536839557"}
		]
	}]}}`)

	pairs, err := New().ProcessPairsQueryResult(fixture)
	if err != nil {
		t.Fatalf("ProcessPairsQueryResult: %v", err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	p := pairs[0]
	if p.Id != "0x48b4cf6d13ccb8113f36488a74a2c2bd6914045c" {
		t.Errorf("id = %q", p.Id)
	}
	if p.Token0.Symbol != "WETH" || p.Token0.Contract != "0x82af49447d8a07e3bd95bd0d56f35241523fbab1" {
		t.Errorf("token0 = %q/%q, want WETH/0x82af...", p.Token0.Symbol, p.Token0.Contract)
	}
	if p.Token1.Symbol != "FTD" {
		t.Errorf("token1 symbol = %q, want FTD", p.Token1.Symbol)
	}
	if p.ReserveUSD != "21681497271086118701084.80466084228" {
		t.Errorf("reserveUSD (from TVL) = %q", p.ReserveUSD)
	}
}

func TestProcessPairsQueryResult_SkipsSingleSided(t *testing.T) {
	fixture := []byte(`{"data":{"liquidityPools":[{"id":"0xabc","inputTokens":[{"symbol":"WETH","lastPriceUSD":"1638"}]}]}}`)
	pairs, err := New().ProcessPairsQueryResult(fixture)
	if err != nil {
		t.Fatalf("ProcessPairsQueryResult: %v", err)
	}
	if len(pairs) != 0 {
		t.Errorf("single-token pool should be skipped, got %d pairs", len(pairs))
	}
}

// The pricing formula: price(base,target) = lastPriceUSD(base)/lastPriceUSD(target). WETH's
// lastPriceUSD of ~1638 was confirmed live to match the cross-DEX ETH consensus, so a
// WETH/USDC pool (USDC ~ 1.0) must price ~1638 "USDC per WETH".
func TestProcessDexPricesResult_FormulaAndConvention(t *testing.T) {
	fixture := []byte(`{"data":{"p0":[{
		"id":"0xpool",
		"inputTokens":[{"symbol":"WETH","lastPriceUSD":"1638.12"},{"symbol":"USDC","lastPriceUSD":"1.0"}]
	}]}}`)

	got, err := New().ProcessDexPricesResult("WETH", "USDC", 1, fixture)
	if err != nil {
		t.Fatalf("ProcessDexPricesResult: %v", err)
	}
	if len(got) != 1 || math.Abs(got[0]-1638.12) > 1e-6 {
		t.Errorf("WETH/USDC = %v, want [1638.12]", got)
	}

	got, err = New().ProcessDexPricesResult("USDC", "WETH", 1, fixture)
	if err != nil {
		t.Fatalf("ProcessDexPricesResult: %v", err)
	}
	if len(got) != 1 || math.Abs(got[0]-(1.0/1638.12)) > 1e-9 {
		t.Errorf("USDC/WETH = %v, want [%v]", got, 1.0/1638.12)
	}
}

// Real captured prices: WETH/FTD ratio must land in a sane band.
func TestProcessDexPricesResult_RealRatio(t *testing.T) {
	fixture := []byte(`{"data":{"p0":[{
		"id":"0x48b4cf6d13ccb8113f36488a74a2c2bd6914045c",
		"inputTokens":[{"symbol":"WETH","lastPriceUSD":"1638.117649447512158148814861754781"},{"symbol":"FTD","lastPriceUSD":"0.00650444918132583557496692536839557"}]
	}]}}`)

	got, err := New().ProcessDexPricesResult("WETH", "FTD", 1, fixture)
	if err != nil {
		t.Fatalf("ProcessDexPricesResult: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 price, got %v", got)
	}
	// 1638.1176.../0.0065044... ≈ 251,846
	if got[0] < 251000 || got[0] > 253000 {
		t.Errorf("WETH/FTD = %v, want ~251846", got[0])
	}
}

func TestProcessDexPricesResult_SkipsMissingOrUnpriced(t *testing.T) {
	// Pool lacks the target token entirely.
	missing := []byte(`{"data":{"p0":[{"id":"0xa","inputTokens":[{"symbol":"WETH","lastPriceUSD":"1638"},{"symbol":"DAI","lastPriceUSD":"1"}]}]}}`)
	got, err := New().ProcessDexPricesResult("WETH", "USDC", 1, missing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("pool without the target token should yield no price, got %v", got)
	}

	// Target present but lastPriceUSD empty -> skip, not error.
	unpriced := []byte(`{"data":{"p0":[{"id":"0xa","inputTokens":[{"symbol":"WETH","lastPriceUSD":"1638"},{"symbol":"USDC","lastPriceUSD":""}]}]}}`)
	got, err = New().ProcessDexPricesResult("WETH", "USDC", 1, unpriced)
	if err != nil {
		t.Fatalf("unpriced token must not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("unpriced target should yield no price, got %v", got)
	}
}

func TestProcessDexPricesResult_GraphQLError(t *testing.T) {
	fixture := []byte(`{"errors":[{"message":"bad indexers"}]}`)
	_, err := New().ProcessDexPricesResult("WETH", "USDC", 1, fixture)
	if err == nil || !strings.Contains(err.Error(), "bad indexers") {
		t.Fatalf("expected GraphQL error to surface, got: %v", err)
	}
}
