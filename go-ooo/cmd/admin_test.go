package cmd

import (
	"testing"

	"go-ooo/config"

	"github.com/stretchr/testify/require"
)

// resolveTargetChains maps the --chain selector to the chains a task targets: "all" fans across every
// configured chain; otherwise the single resolved chain (or the sole chain when the flag is empty).
func TestResolveTargetChains(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Chain = config.ChainConfig{}
	cfg.Chains = []config.ChainConfig{
		{Name: "sepolia", NetworkId: 11155111},
		{Name: "polygon", NetworkId: 137},
	}

	// "all" (case-insensitive) returns every chain.
	all, err := resolveTargetChains(cfg, "All")
	require.NoError(t, err)
	require.Len(t, all, 2)

	// A named selector returns just that chain.
	one, err := resolveTargetChains(cfg, "polygon")
	require.NoError(t, err)
	require.Len(t, one, 1)
	require.EqualValues(t, 137, one[0].NetworkId)

	// An empty selector with several chains is ambiguous - the ResolveChain error propagates.
	_, err = resolveTargetChains(cfg, "")
	require.ErrorContains(t, err, "specify --chain")

	// A single configured chain resolves with no selector.
	solo := config.DefaultConfig()
	solo.Chain.Name = "dev"
	solo.Chain.NetworkId = 696969
	got, err := resolveTargetChains(solo, "")
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.EqualValues(t, 696969, got[0].NetworkId)
}

// chainLabel prefers the human name, falling back to "network <id>" for unnamed chains.
func TestChainLabel(t *testing.T) {
	require.Equal(t, "sepolia", chainLabel(config.ChainConfig{Name: "sepolia", NetworkId: 11155111}))
	require.Equal(t, "network 137", chainLabel(config.ChainConfig{NetworkId: 137}))
}
