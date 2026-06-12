package dex

import (
	"testing"

	"go-ooo/config"
	"go-ooo/ooo_api/export"

	"github.com/stretchr/testify/require"
)

// TestSourceModelRoundTrip locks the manifest<->row mapping, including the endpoint list surviving
// the JSON round-trip intact.
func TestSourceModelRoundTrip(t *testing.T) {
	orig := export.ManifestSource{
		Chain: "eth",
		Dex:   "uniswap_v3",
		Endpoints: []export.ManifestEndpoint{
			{Provider: "thegraph", URLTemplate: "https://gw/{API_KEY}/id/abc", Tier: "paid"},
			{Provider: "studio", URLTemplate: "https://studio/abc", Tier: "free"},
		},
		SubgraphSchemaFamily: "univ3",
		FactoryAddress:       "0xfac",
		RPCURL:               "https://eth.example/rpc",
		BlocksPerMin:         5,
		PairCount:            42,
		ExportURL:            "/api/ooo/v1/export/eth/uniswap_v3",
		LastUpdated:          111,
		LastVerifiedAt:       222,
	}

	row, err := sourceToModel(orig)
	require.NoError(t, err)
	back, err := modelToSource(row)
	require.NoError(t, err)
	require.Equal(t, orig, back)
}

// TestPersistAndLoadManifest covers the full persist->restore loop the resilience floor relies on,
// plus the drop handling: a source absent from a later manifest is soft-deleted and no longer
// restored.
func TestPersistAndLoadManifest(t *testing.T) {
	dm := newDexTestManager(t)

	m := &export.Manifest{
		SchemaVersion: export.ManifestSchemaVersion,
		SupportedSources: []export.ManifestSource{
			{Chain: "eth", Dex: "uniswap_v3", SubgraphSchemaFamily: "univ3",
				Endpoints: []export.ManifestEndpoint{{Provider: "thegraph", URLTemplate: "u", Tier: "paid"}}},
			{Chain: "eth", Dex: "sushiswap", SubgraphSchemaFamily: "univ2"},
		},
	}
	n, err := dm.PersistManifest(m)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	restored, err := dm.LoadManifestFromDB()
	require.NoError(t, err)
	require.Len(t, restored.SupportedSources, 2)

	// A later manifest dropping sushiswap soft-deletes it; only uniswap_v3 is restored.
	m2 := &export.Manifest{SupportedSources: []export.ManifestSource{m.SupportedSources[0]}}
	n, err = dm.PersistManifest(m2)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	restored, err = dm.LoadManifestFromDB()
	require.NoError(t, err)
	require.Len(t, restored.SupportedSources, 1)
	require.Equal(t, "uniswap_v3", restored.SupportedSources[0].Dex)
}

// TestBuildChains covers 4.D: a known chain takes its RPC from config, a new chain is driven entirely
// by the manifest's rpcUrl/blocksPerMin, a chain with no RPC anywhere is skipped, and an unchanged
// RPC reuses the existing dialled client rather than re-dialling.
func TestBuildChains(t *testing.T) {
	dm := &Manager{cfg: &config.Config{Subchain: config.SubchainConfig{EthHttpRpc: "http://127.0.0.1:8545"}}}

	m := &export.Manifest{SupportedSources: []export.ManifestSource{
		{Chain: "eth", Dex: "uniswap_v3"}, // known chain, RPC from config
		{Chain: "arbitrum", Dex: "camelot_v3", RPCURL: "http://127.0.0.1:8546", BlocksPerMin: 240}, // new chain via manifest
		{Chain: "mystery", Dex: "x"}, // unknown + no RPC -> skipped
	}}

	out := dm.buildChains(nil, m)
	require.Len(t, out, 2)
	require.Contains(t, out, "eth")
	require.Contains(t, out, "arbitrum")
	require.NotContains(t, out, "mystery")
	require.Equal(t, "http://127.0.0.1:8545", out["eth"].RpcUrl)
	require.EqualValues(t, 240, out["arbitrum"].BlocksPerMin)

	// Rebuilding with the prior map reuses the dialled client when the RPC is unchanged.
	out2 := dm.buildChains(out, m)
	require.Same(t, out["eth"].EthClient, out2["eth"].EthClient)
}
