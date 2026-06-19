package dex

import (
	"path/filepath"
	"testing"

	"go-ooo/database"
	"go-ooo/database/models"
	"go-ooo/ooo_api/export"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newDexTestManager(t *testing.T) *Manager {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.sqlite")), &gorm.Config{})
	require.NoError(t, err)
	db := &database.DB{DB: gdb}
	require.NoError(t, db.Migrate())
	return &Manager{db: db}
}

func tok(chain, sym, addr string) *export.ExportToken {
	return &export.ExportToken{Chain: chain, Symbol: sym, ContractAddress: addr}
}

func TestProcessPairFeed(t *testing.T) {
	dm := newDexTestManager(t)

	feed := export.PairFeed{
		Chain: "eth", Dex: "uniswap_v3", MinLiquidityUsd: 1000,
		Pairs: []export.ExportPair{
			{ContractAddress: "0x1111111111111111111111111111111111111111", ReserveUsd: 5e6, TxCount: 100,
				Confidence: 0.9, CanonicalKey: "weth:usdc", Token0: tok("eth", "WETH", "0xa"), Token1: tok("eth", "USDC", "0xb")},
			{ContractAddress: "0x2222222222222222222222222222222222222222", ReserveUsd: 2e6, TxCount: 50,
				Confidence: 0.7, CanonicalKey: "", Token0: tok("eth", "BONE", "0xc"), Token1: tok("eth", "WETH", "0xa")},
			// token-less pair is skipped.
			{ContractAddress: "0x9999999999999999999999999999999999999999", Token0: nil, Token1: nil},
		},
	}

	require.Equal(t, 2, dm.processPairFeed(feed)) // the token-less pair is skipped

	var pairs []models.DexPairs
	require.NoError(t, dm.db.Where("chain = ? AND dex = ?", "eth", "uniswap_v3").Find(&pairs).Error)
	require.Len(t, pairs, 2)
	var p1 models.DexPairs
	for _, p := range pairs {
		require.True(t, p.Verified)
		if p.Confidence == 0.9 {
			p1 = p
		}
	}
	require.Equal(t, "weth:usdc", p1.CanonicalKey)
	require.Equal(t, 5e6, p1.ReserveUsd)
	require.EqualValues(t, 100, p1.TxCount)

	// A later feed that drops pair 2 soft-deletes it; pair 1 stays and updates.
	feed2 := export.PairFeed{Chain: "eth", Dex: "uniswap_v3", Pairs: []export.ExportPair{
		{ContractAddress: "0x1111111111111111111111111111111111111111", ReserveUsd: 9e6, TxCount: 200,
			Confidence: 0.95, CanonicalKey: "weth:usdc", Token0: tok("eth", "WETH", "0xa"), Token1: tok("eth", "USDC", "0xb")},
	}}
	require.Equal(t, 1, dm.processPairFeed(feed2))

	var active []models.DexPairs
	require.NoError(t, dm.db.Where("chain = ? AND dex = ?", "eth", "uniswap_v3").Find(&active).Error)
	require.Len(t, active, 1) // pair 2 soft-deleted (default scope excludes it)
	require.Equal(t, 9e6, active[0].ReserveUsd)
	require.Equal(t, 0.95, active[0].Confidence)
}

func TestProcessPairFeedEmptyFeedDoesNotWipe(t *testing.T) {
	dm := newDexTestManager(t)
	feed := export.PairFeed{Chain: "eth", Dex: "shibaswap", Pairs: []export.ExportPair{
		{ContractAddress: "0x3333333333333333333333333333333333333333", ReserveUsd: 1e6, TxCount: 10,
			Token0: tok("eth", "SHIB", "0xd"), Token1: tok("eth", "WETH", "0xa")},
	}}
	require.Equal(t, 1, dm.processPairFeed(feed))

	// A transient empty feed must NOT wipe the source's existing pair.
	require.Equal(t, 0, dm.processPairFeed(export.PairFeed{Chain: "eth", Dex: "shibaswap"}))
	var active []models.DexPairs
	require.NoError(t, dm.db.Where("chain = ? AND dex = ?", "eth", "shibaswap").Find(&active).Error)
	require.Len(t, active, 1, "empty feed must not soft-delete the existing pair")
}

// TestNormalisePoolKey guards the v4 regression: common.HexToAddress silently truncates a 32-byte
// poolId to its last 20 bytes, which corrupted every Uniswap v4 pool key so no subgraph pool ever
// matched it. A poolId must pass through intact (lower-cased); a 20-byte address is still checksummed.
func TestNormalisePoolKey(t *testing.T) {
	// Real Uniswap v4 ETH/USDC poolId — must survive whole, not be cut to its last 20 bytes.
	poolId := "0xdce6394339af00981949f5f3baf27e3610c76326a700af57e4b3e3ae4977f78d"
	if got := normalisePoolKey(poolId); got != poolId {
		t.Errorf("poolId mangled: got %q (len %d), want it intact (len %d)", got, len(got), len(poolId))
	}
	// A mixed-case poolId is lower-cased (so it matches the subgraph + the price-query form).
	if got := normalisePoolKey("0xDCE6394339AF00981949F5F3BAF27E3610C76326A700AF57E4B3E3AE4977F78D"); got != poolId {
		t.Errorf("poolId not lower-cased: got %q", got)
	}
	// A 20-byte pool address is still checksummed (unchanged v2/v3 behaviour).
	if got := normalisePoolKey("0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"); got != "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2" {
		t.Errorf("20-byte address checksum: got %q", got)
	}
	// A Cosmos bech32 pool address is not hex - common.HexToAddress would collapse it (and every other
	// non-hex address) onto the zero address, making all of a chain's pairs upsert onto one row. It must
	// pass through intact so distinct pools stay distinct.
	neutronA := "neutron1nfns3ck2ykrs0fknckrzd9728cyf77devuzernhwcwrdxw7ssk2s3tjf8r"
	neutronB := "neutron18c8qejysp4hgcfuxdpj4wf29mevzwllz5yh8uayjxamwtrs0n9fshq9vtv"
	if got := normalisePoolKey(neutronA); got != neutronA {
		t.Errorf("cosmos address mangled: got %q, want %q", got, neutronA)
	}
	if normalisePoolKey(neutronA) == normalisePoolKey(neutronB) {
		t.Errorf("distinct cosmos addresses collapsed to the same key %q", normalisePoolKey(neutronA))
	}
}
