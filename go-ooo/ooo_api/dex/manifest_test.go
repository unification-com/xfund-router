package dex

import (
	"testing"

	"go-ooo/config"
	"go-ooo/ooo_api/export"

	"github.com/stretchr/testify/require"
)

func paidEp(tmpl string) export.ManifestEndpoint {
	return export.ManifestEndpoint{Provider: "graph-decentralized", URLTemplate: tmpl, Tier: "paid"}
}

func freeEp(tmpl string) export.ManifestEndpoint {
	return export.ManifestEndpoint{Provider: "graph-hosted", URLTemplate: tmpl, Tier: "free"}
}

func mfSource(chain, dex, family string, endpoints ...export.ManifestEndpoint) export.ManifestSource {
	return export.ManifestSource{Chain: chain, Dex: dex, SubgraphSchemaFamily: family, Endpoints: endpoints}
}

func moduleByName(mods []FamilyModule, name string) (FamilyModule, bool) {
	for _, m := range mods {
		if m.Name() == name {
			return m, true
		}
	}
	return FamilyModule{}, false
}

func TestBuildModulesFromManifest(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ApiKeys.GraphNetwork = "KEY"

	m := &export.Manifest{SupportedSources: []export.ManifestSource{
		mfSource("eth", "uniswap_v2", "univ2", paidEp("https://gw/{API_KEY}/v2")),
		mfSource("eth", "uniswap_v3", "univ3", paidEp("https://gw/{API_KEY}/v3")),
		mfSource("arbitrum", "sushiswap", "messari", paidEp("https://gw/{API_KEY}/messari")),
		mfSource("eth", "mystery", "custom", paidEp("https://gw/{API_KEY}/x")), // unknown family -> skip
		mfSource("eth", "paidonly", "univ2"),                                   // no endpoints -> skip
	}}

	mods := BuildModulesFromManifest(cfg, m)
	require.Len(t, mods, 3) // the "custom" + endpoint-less sources are skipped

	v2, ok := moduleByName(mods, "eth_uniswap_v2")
	require.True(t, ok)
	require.Equal(t, "https://gw/KEY/v2", v2.SubgraphUrl()) // {API_KEY} substituted
	q, err := v2.GeneratePairsQuery("0xabc")
	require.NoError(t, err)
	require.Contains(t, string(q), "pairs") // univ2 -> pairs entity

	v3, ok := moduleByName(mods, "eth_uniswap_v3")
	require.True(t, ok)
	q, err = v3.GeneratePairsQuery("0xabc")
	require.NoError(t, err)
	require.Contains(t, string(q), "pools") // univ3 -> pools entity

	_, ok = moduleByName(mods, "arbitrum_sushiswap")
	require.True(t, ok) // messari family recognised + built

	_, ok = moduleByName(mods, "eth_mystery")
	require.False(t, ok) // unknown "custom" family skipped
}

func TestSubgraphURLForEndpointSelection(t *testing.T) {
	// With a key: the paid {API_KEY} endpoint is chosen + substituted.
	url, ok := subgraphURLFor([]export.ManifestEndpoint{paidEp("https://gw/{API_KEY}/id"), freeEp("https://hosted/x")}, "KEY")
	require.True(t, ok)
	require.Equal(t, "https://gw/KEY/id", url)

	// Without a key: skip the paid endpoint, use the free one.
	url, ok = subgraphURLFor([]export.ManifestEndpoint{paidEp("https://gw/{API_KEY}/id"), freeEp("https://hosted/x")}, "")
	require.True(t, ok)
	require.Equal(t, "https://hosted/x", url)

	// Without a key and only a paid endpoint: nothing usable.
	_, ok = subgraphURLFor([]export.ManifestEndpoint{paidEp("https://gw/{API_KEY}/id")}, "")
	require.False(t, ok)
}
