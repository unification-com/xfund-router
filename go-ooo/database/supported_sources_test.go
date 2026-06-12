package database

import (
	"testing"

	"go-ooo/database/models"

	"github.com/stretchr/testify/require"
)

func src(chain, dex, family, endpoints string) models.SupportedSource {
	return models.SupportedSource{
		Chain:                chain,
		Dex:                  dex,
		SubgraphSchemaFamily: family,
		Endpoints:            endpoints,
		BlocksPerMin:         5,
		PairCount:            10,
	}
}

// TestSupportedSourcesRoundTrip locks the persist/restore contract the manifest dispatch relies on:
// upsert is keyed by (chain, dex) and idempotent (a re-sync updates in place, never duplicates), and
// the opaque endpoints JSON round-trips verbatim.
func TestSupportedSourcesRoundTrip(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())

	endpointsJson := `[{"provider":"thegraph","urlTemplate":"https://gateway/{API_KEY}/id/abc","tier":"paid"}]`
	_, err := d.UpsertSupportedSource(src("eth", "uniswap_v3", "univ3", endpointsJson))
	require.NoError(t, err)
	_, err = d.UpsertSupportedSource(src("polygon_pos", "quickswap_v3", "univ3", "[]"))
	require.NoError(t, err)

	sources, err := d.GetSupportedSources()
	require.NoError(t, err)
	require.Len(t, sources, 2)

	// Find the eth source and confirm the endpoints JSON survived untouched.
	var eth models.SupportedSource
	for _, s := range sources {
		if s.Chain == "eth" {
			eth = s
		}
	}
	require.Equal(t, "univ3", eth.SubgraphSchemaFamily)
	require.Equal(t, endpointsJson, eth.Endpoints)
	require.EqualValues(t, 10, eth.PairCount)

	// A re-sync of the same (chain, dex) updates in place rather than inserting a duplicate.
	updated := src("eth", "uniswap_v3", "univ3", `[{"provider":"studio","urlTemplate":"https://studio/abc","tier":"free"}]`)
	updated.PairCount = 99
	persisted, err := d.UpsertSupportedSource(updated)
	require.NoError(t, err)
	require.Equal(t, eth.ID, persisted.ID) // same row
	sources, err = d.GetSupportedSources()
	require.NoError(t, err)
	require.Len(t, sources, 2) // still two, not three
}

// TestSoftDeleteSupportedSourcesNotIn covers the manifest-drop handling + the empty-set data-loss
// guard: a source absent from the keep set is soft-deleted (drops out of GetSupportedSources), but
// an empty keep set is a no-op so a failed/empty manifest can't wipe the catalogue.
func TestSoftDeleteSupportedSourcesNotIn(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())

	_, err := d.UpsertSupportedSource(src("eth", "uniswap_v3", "univ3", "[]"))
	require.NoError(t, err)
	_, err = d.UpsertSupportedSource(src("eth", "sushiswap", "univ2", "[]"))
	require.NoError(t, err)

	// Empty keep set must NOT wipe anything.
	removed, err := d.SoftDeleteSupportedSourcesNotIn(nil)
	require.NoError(t, err)
	require.EqualValues(t, 0, removed)
	sources, err := d.GetSupportedSources()
	require.NoError(t, err)
	require.Len(t, sources, 2)

	// Keep only the uniswap source - sushiswap drops out.
	removed, err = d.SoftDeleteSupportedSourcesNotIn([]string{"eth|uniswap_v3"})
	require.NoError(t, err)
	require.EqualValues(t, 1, removed)
	sources, err = d.GetSupportedSources()
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Equal(t, "uniswap_v3", sources[0].Dex)
}
