package dex

import (
	"sort"
	"testing"

	"go-ooo/database/models"
	"go-ooo/ooo_api/dex/chains"
	"go-ooo/ooo_api/dex/types"
	"go-ooo/ooo_api/export"

	"github.com/stretchr/testify/require"
)

// testManifestWithAliases is a small ETH/USD/BTC alias manifest used across the alias tests.
func testManifestWithAliases() *export.Manifest {
	return &export.Manifest{
		AliasGroups: []export.ManifestAliasGroup{
			{Symbol: "ETH", CgIds: []string{"ethereum", "weth"}},
			{Symbol: "USD", CgIds: []string{"usd-coin", "tether"}},
			{Symbol: "BTC", CgIds: []string{"bitcoin", "wrapped-bitcoin"}},
		},
		AliasPairs: []export.ManifestAliasPair{
			{Pair: "ETH.USD", CanonicalKeys: []string{"ethereum:usd-coin", "tether:weth"}},
			{Pair: "BTC.USD", CanonicalKeys: []string{"bitcoin:usd-coin"}},
		},
	}
}

func TestBuildAliasIndexAndLookup(t *testing.T) {
	idx := buildAliasIndex(testManifestWithAliases())
	require.NotNil(t, idx)

	// cg -> class, case/space-insensitive.
	require.Equal(t, "ETH", idx.aliasSymbol("weth"))
	require.Equal(t, "ETH", idx.aliasSymbol("  Ethereum "))
	require.Equal(t, "USD", idx.aliasSymbol("tether"))
	require.Equal(t, "", idx.aliasSymbol("some-random-token"))

	// A DB-reconstructed manifest (no alias groups) yields a nil index so ApplyManifest preserves the
	// prior one rather than wiping it.
	require.Nil(t, buildAliasIndex(&export.Manifest{}))

	// lookupAliasPair: order-independent, distinct classes, must have backing coverage.
	keys, ok := idx.lookupAliasPair("ETH", "USD")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"ethereum:usd-coin", "tether:weth"}, keys)
	_, ok = idx.lookupAliasPair("usd", "eth") // case-insensitive + reversed
	require.True(t, ok)
	_, ok = idx.lookupAliasPair("ETH", "ETH") // same class -> not a pair
	require.False(t, ok)
	_, ok = idx.lookupAliasPair("ETH", "BTC") // no backing coverage in this manifest
	require.False(t, ok)
	_, ok = (*aliasIndex)(nil).lookupAliasPair("ETH", "USD") // nil-safe
	require.False(t, ok)
}

// TestAliasPersistRoundTrip locks the resilience floor: PersistManifest persists the alias payload and
// LoadManifestFromDB restores it, so a fresh process (e.g. the testapp `price` command, or a restarted
// oracle before its first live sync) can recognise + price an alias query from the DB alone.
func TestAliasPersistRoundTrip(t *testing.T) {
	dm := newDexTestManager(t)

	m := testManifestWithAliases()
	m.SupportedSources = []export.ManifestSource{{Chain: "eth", Dex: "uniswap_v3", SubgraphSchemaFamily: "univ3"}}
	_, err := dm.PersistManifest(m)
	require.NoError(t, err)

	restored, err := dm.LoadManifestFromDB()
	require.NoError(t, err)
	require.Len(t, restored.AliasGroups, 3)
	require.Len(t, restored.AliasPairs, 2)

	// The restored manifest builds the same working index, so IsAliasPair works from the DB alone.
	idx := buildAliasIndex(restored)
	require.NotNil(t, idx)
	_, ok := idx.lookupAliasPair("ETH", "USD")
	require.True(t, ok)

	// A later manifest with no alias data (a DB reconstruction) must NOT wipe the persisted payload.
	require.NoError(t, dm.persistAliases(&export.Manifest{}))
	groups, pairs, found, err := dm.db.GetAliasConfig()
	require.NoError(t, err)
	require.True(t, found)
	require.NotEmpty(t, groups)
	require.NotEmpty(t, pairs)
}

func TestAliasOrient(t *testing.T) {
	idx := buildAliasIndex(testManifestWithAliases())

	// token0 is the base class, token1 the target class -> symbols returned in (base, target) order.
	b, tg, ok := idx.orient(dexPairRow("eth", "uniswap_v3", "0xa", "WETH", "USDC", "weth", "usd-coin", 1), "ETH", "USD")
	require.True(t, ok)
	require.Equal(t, "WETH", b)
	require.Equal(t, "USDC", tg)

	// token0 is the target class, token1 the base class -> symbols swapped so base stays base.
	b, tg, ok = idx.orient(dexPairRow("eth", "uniswap_v3", "0xb", "USDT", "WETH", "tether", "weth", 1), "ETH", "USD")
	require.True(t, ok)
	require.Equal(t, "WETH", b)
	require.Equal(t, "USDT", tg)

	// A pool whose tokens don't map to exactly this class pair is rejected.
	_, _, ok = idx.orient(dexPairRow("eth", "uniswap_v3", "0xc", "WETH", "DAI", "weth", "dai", 1), "ETH", "USD")
	require.False(t, ok) // dai is not a curated USD member in this manifest
}

func TestIsAliasPair(t *testing.T) {
	dm := newDexTestManager(t)
	dm.aliases = buildAliasIndex(testManifestWithAliases())

	require.True(t, dm.IsAliasPair("ETH", "USD"))
	require.True(t, dm.IsAliasPair("usd", "eth")) // order + case insensitive
	require.True(t, dm.IsAliasPair("BTC", "USD"))
	require.False(t, dm.IsAliasPair("WETH", "USDC")) // token symbols, not class symbols -> direct path
	require.False(t, dm.IsAliasPair("ETH", "BTC"))   // no coverage
	require.False(t, dm.IsAliasPair("ETH", "ETH"))   // same class
}

func TestFindByCanonicalKeys(t *testing.T) {
	dm := newDexTestManager(t)
	seedAliasPools(t, dm)

	rows, err := dm.db.FindByCanonicalKeys([]string{"ethereum:usd-coin", "tether:weth"})
	require.NoError(t, err)
	require.Len(t, rows, 3) // the two WETH/USDC pools + the USDT/WETH pool; the BTC pool is excluded

	rows, err = dm.db.FindByCanonicalKeys(nil)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestProcessPairFeedPersistsCgIds(t *testing.T) {
	dm := newDexTestManager(t)
	feed := export.PairFeed{Chain: "eth", Dex: "uniswap_v3", Pairs: []export.ExportPair{
		{ContractAddress: "0x1111111111111111111111111111111111111111", ReserveUsd: 5e6, Confidence: 0.9,
			CanonicalKey: "ethereum:usd-coin",
			Token0:       &export.ExportToken{Chain: "eth", Symbol: "WETH", ContractAddress: "0xa", CoinGeckoCoinId: "weth"},
			Token1:       &export.ExportToken{Chain: "eth", Symbol: "USDC", ContractAddress: "0xb", CoinGeckoCoinId: "usd-coin"}},
	}}
	require.Equal(t, 1, dm.processPairFeed(feed))

	row, err := dm.db.FindByDexChainAddress("eth", "uniswap_v3", "0x1111111111111111111111111111111111111111")
	require.NoError(t, err)
	require.Equal(t, "weth", row.T0Cg)
	require.Equal(t, "usd-coin", row.T1Cg)
}

// TestGetAliasPricesFromDexModules is the S7 end-to-end: an ETH.USD query expands to every fungible
// member pool across modules, orients each (so the cross price is base-per-target), and returns one
// combined sample set the robust aggregator prices - covering the multi-query-per-module grouping and
// the per-pool liquidity/confidence join.
func TestGetAliasPricesFromDexModules(t *testing.T) {
	dm := newDexTestManager(t)
	seedAliasPools(t, dm)
	dm.aliases = buildAliasIndex(testManifestWithAliases())

	// Price is keyed by the oriented target symbol so we can prove orientation flows through: a USDC
	// quote prices ETH at 2000, a USDT quote at 2010. Every pool's base resolves to WETH.
	priceFor := func(_, target string) float64 {
		if target == "USDT" {
			return 2010
		}
		return 2000
	}
	dm.modules = map[string]Module{
		"eth_uniswap_v3": fakePriceSource{chain: "eth", dex: "uniswap_v3", minLiq: 1000, price: priceFor},
		"eth_sushiswap":  fakePriceSource{chain: "eth", dex: "sushiswap", minLiq: 1000, price: priceFor},
	}
	// eth has no EVM client in the test, so the block lookup is skipped (as for a Cosmos source).
	dm.chains = map[string]*chains.ChainDef{"eth": {ChainShort: "eth", ChainName: "eth"}}

	samples := dm.GetAliasPricesFromDexModules("ETH", "USD", 0)
	require.Len(t, samples, 3) // 2 on uniswap_v3 (WETH/USDC + USDT/WETH), 1 on sushiswap (WETH/USDC)

	byContract := make(map[string]PoolSample)
	for _, s := range samples {
		byContract[s.Contract] = s
	}
	// WETH/USDC on uniswap_v3 -> 2000, liquidity 5e6.
	require.Equal(t, []float64{2000}, byContract["0xa"].Prices)
	require.Equal(t, 5e6, byContract["0xa"].Liquidity)
	// USDT/WETH on uniswap_v3 -> oriented to WETH/USDT -> 2010, liquidity 3e6.
	require.Equal(t, []float64{2010}, byContract["0xb"].Prices)
	require.Equal(t, 3e6, byContract["0xb"].Liquidity)
	// WETH/USDC on sushiswap -> 2000, liquidity 2e6.
	require.Equal(t, []float64{2000}, byContract["0xc"].Prices)
	require.Equal(t, 2e6, byContract["0xc"].Liquidity)

	// A non-alias query on the same data takes the direct path (no member expansion).
	require.False(t, dm.IsAliasPair("WETH", "USDC"))
}

// --- test helpers ---

// dexPairRow builds an in-memory DexPairs row for the orientation unit tests.
func dexPairRow(chain, dex, contract, t0sym, t1sym, t0cg, t1cg string, reserve float64) models.DexPairs {
	return models.DexPairs{
		Chain:           chain,
		Dex:             dex,
		ContractAddress: contract,
		T0Symbol:        t0sym,
		T1Symbol:        t1sym,
		T0Cg:            t0cg,
		T1Cg:            t1cg,
		ReserveUsd:      reserve,
	}
}

// seedAliasPools writes the standard alias-test pool set into the DB: two ETH/USD pools on different
// DEXs plus a USDT/WETH pool (reverse orientation), and one unrelated BTC/USD pool.
func seedAliasPools(t *testing.T, dm *Manager) {
	t.Helper()
	pools := []struct {
		chain, dex, contract, t0sym, t1sym, t0cg, t1cg, ck string
		reserve                                            float64
	}{
		{"eth", "uniswap_v3", "0xa", "WETH", "USDC", "weth", "usd-coin", "ethereum:usd-coin", 5e6},
		{"eth", "uniswap_v3", "0xb", "USDT", "WETH", "tether", "weth", "tether:weth", 3e6},
		{"eth", "sushiswap", "0xc", "WETH", "USDC", "weth", "usd-coin", "ethereum:usd-coin", 2e6},
		{"eth", "uniswap_v3", "0xd", "WBTC", "USDC", "wrapped-bitcoin", "usd-coin", "bitcoin:usd-coin", 4e6},
	}
	for _, p := range pools {
		_, err := dm.db.UpsertExportDexPair(p.chain, p.dex, p.contract, p.t0sym, p.t1sym, 0, 0,
			p.reserve, 10, 0.9, p.ck, p.t0cg, p.t1cg)
		require.NoError(t, err)
	}
}

// fakePriceSource is a PriceSource that echoes a (base,target)-derived price under each queried pool
// contract, like a Cosmos REST source - so a test can exercise gatherSamples without a real subgraph.
type fakePriceSource struct {
	chain, dex string
	minLiq     uint64
	price      func(base, target string) float64
}

func (m fakePriceSource) Name() string         { return m.chain + "_" + m.dex }
func (m fakePriceSource) Chain() string        { return m.chain }
func (m fakePriceSource) Dex() string          { return m.dex }
func (m fakePriceSource) MinLiquidity() uint64 { return m.minLiq }
func (m fakePriceSource) MinTxCount() uint64   { return 0 }

func (m fakePriceSource) FetchPoolPrices(base, target string, dexInfo DexInfo, _ uint64) ([]types.PoolPrices, error) {
	p := m.price(base, target)
	out := make([]types.PoolPrices, 0, len(dexInfo.ContractAddressList))
	contracts := append([]string(nil), dexInfo.ContractAddressList...)
	sort.Strings(contracts) // deterministic order for the test
	for _, c := range contracts {
		out = append(out, types.PoolPrices{Contract: c, Prices: []float64{p}})
	}
	return out, nil
}

func (m fakePriceSource) FetchPairsMetadata(string) ([]types.DexPair, error) { return nil, nil }
