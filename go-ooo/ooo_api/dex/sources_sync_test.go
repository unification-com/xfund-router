package dex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/database/models"
	"go-ooo/ooo_api/export"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// exportToken is a tiny helper for the feed fixtures.
func exportToken(sym, addr string) *export.ExportToken {
	return &export.ExportToken{Chain: "eth", Symbol: sym, ContractAddress: addr}
}

// TestSyncManifestAndFeeds exercises the whole consumer against a stub dex-pair-verify API: it
// fetches the manifest, persists + applies it (sources + modules), and pulls each source's pair feed
// into dex_pairs - the end-to-end loop, minus the live subgraph metadata refresh.
func TestSyncManifestAndFeeds(t *testing.T) {
	manifest := export.Manifest{
		SchemaVersion: export.ManifestSchemaVersion,
		SupportedSources: []export.ManifestSource{
			{Chain: "eth", Dex: "uniswap_v3", SubgraphSchemaFamily: "univ3", RPCURL: "http://127.0.0.1:8545", BlocksPerMin: 5,
				Endpoints: []export.ManifestEndpoint{{Provider: "studio", URLTemplate: "http://subgraph.local/univ3", Tier: "free"}}},
			{Chain: "eth", Dex: "sushiswap", SubgraphSchemaFamily: "univ2", RPCURL: "http://127.0.0.1:8545", BlocksPerMin: 5,
				Endpoints: []export.ManifestEndpoint{{Provider: "studio", URLTemplate: "http://subgraph.local/sushi", Tier: "free"}}},
			// An unknown schema family must be skipped, not crash the sync (univ4 is now
			// recognised, so this uses "infinity" - a family go-ooo does not yet implement).
			{Chain: "eth", Dex: "futureswap", SubgraphSchemaFamily: "infinity", RPCURL: "http://127.0.0.1:8545", BlocksPerMin: 5,
				Endpoints: []export.ManifestEndpoint{{Provider: "studio", URLTemplate: "http://subgraph.local/future", Tier: "free"}}},
		},
	}

	feeds := map[string]export.PairFeed{
		"/export/eth/uniswap_v3": {SchemaVersion: export.PairSchemaVersion, Chain: "eth", Dex: "uniswap_v3", MinLiquidityUsd: 1000,
			Pairs: []export.ExportPair{{ContractAddress: "0x1111111111111111111111111111111111111111", Pair: "WETH-USDC",
				ReserveUsd: 5e6, TxCount: 100, Confidence: 0.9, CanonicalKey: "weth:usdc",
				Token0: exportToken("WETH", "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"), Token1: exportToken("USDC", "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")}}},
		"/export/eth/sushiswap": {SchemaVersion: export.PairSchemaVersion, Chain: "eth", Dex: "sushiswap", MinLiquidityUsd: 1000,
			Pairs: []export.ExportPair{{ContractAddress: "0x2222222222222222222222222222222222222222", Pair: "WETH-DAI",
				ReserveUsd: 2e6, TxCount: 50, Confidence: 0.8, CanonicalKey: "weth:dai",
				Token0: exportToken("WETH", "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"), Token1: exportToken("DAI", "0x6B175474E89094C44Da98b954EedeAC495271d0F")}}},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/export/manifest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(manifest)
	})
	for path, feed := range feeds {
		feed := feed
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(feed)
		})
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.sqlite")), &gorm.Config{})
	require.NoError(t, err)
	db := &database.DB{DB: gdb}
	require.NoError(t, db.Migrate())

	cfg := config.DefaultConfig()
	cfg.Jobs.DexExport.BaseUrl = srv.URL
	cfg.Subchain.EthHttpRpc = "http://127.0.0.1:8545"

	dm := NewDexManager(context.Background(), cfg, db)
	require.NoError(t, dm.syncManifestAndFeeds())

	// The two recognised sources are persisted; the infinity one is skipped (persisted but not a module).
	sources, err := db.GetSupportedSources()
	require.NoError(t, err)
	require.Len(t, sources, 3)
	require.Len(t, dm.snapshotModules(), 2) // infinity family unrecognised -> no module

	// XR2: the module's liquidity floor reflects the feed's per-source minLiquidityUsd (1000), not
	// go-ooo's hard-coded default - applied via the re-apply after the feeds land.
	var uniMod Module
	for _, m := range dm.snapshotModules() {
		if m.Dex() == "uniswap_v3" {
			uniMod = m
		}
	}
	require.NotNil(t, uniMod)
	require.EqualValues(t, 1000, uniMod.MinLiquidity())

	// Each recognised source's feed landed in dex_pairs, carrying the export's confidence + key.
	var pairs []models.DexPairs
	require.NoError(t, db.Where("chain = ?", "eth").Find(&pairs).Error)
	require.Len(t, pairs, 2)

	var uni models.DexPairs
	require.NoError(t, db.Where("chain = ? AND dex = ?", "eth", "uniswap_v3").First(&uni).Error)
	require.Equal(t, 0.9, uni.Confidence)
	require.Equal(t, "weth:usdc", uni.CanonicalKey)
	require.True(t, uni.Verified)
}

// TestSyncManifestAndFeedsNoBaseUrl confirms an empty base URL is a no-op (the persisted catalogue
// is authoritative) rather than an error.
func TestSyncManifestAndFeedsNoBaseUrl(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.sqlite")), &gorm.Config{})
	require.NoError(t, err)
	db := &database.DB{DB: gdb}
	require.NoError(t, db.Migrate())

	cfg := config.DefaultConfig()
	cfg.Jobs.DexExport.BaseUrl = "" // exercise the no-base-url path (the default is now the prod dpv)
	dm := NewDexManager(context.Background(), cfg, db)
	require.NoError(t, dm.syncManifestAndFeeds())

	sources, err := db.GetSupportedSources()
	require.NoError(t, err)
	require.Empty(t, sources)
}
