package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"go-ooo/config"

	"github.com/stretchr/testify/require"
)

// writeSingleChainConfig writes a starter config.toml with a single [chain] (sepolia), as a first-time
// 'init' would, and returns its path.
func writeSingleChainConfig(t *testing.T) string {
	t.Helper()
	c := config.DefaultConfig()
	c.InitForSepolia()
	path := filepath.Join(t.TempDir(), "config.toml")
	config.WriteConfigFile(path, c)
	return path
}

// addNetworkToConfig folds the existing single [chain] into [[chains]], appends the new network, and
// backs up the previous file.
func TestAddNetworkToConfig(t *testing.T) {
	path := writeSingleChainConfig(t)

	require.NoError(t, addNetworkToConfig(path, "polygon"))

	got, err := config.LoadConfigFile(path)
	require.NoError(t, err)
	require.Len(t, got.Chains, 2)
	require.Equal(t, "sepolia", got.Chains[0].Name)
	require.Equal(t, "polygon", got.Chains[1].Name)
	require.EqualValues(t, 137, got.Chains[1].NetworkId)

	// The previous file was backed up.
	_, err = os.Stat(path + ".bak")
	require.NoError(t, err)
}

// Adding the same network twice is rejected (duplicate network id).
func TestAddNetworkToConfigDuplicate(t *testing.T) {
	path := writeSingleChainConfig(t)
	require.NoError(t, addNetworkToConfig(path, "polygon"))
	require.Error(t, addNetworkToConfig(path, "polygon"))
}

// --add against a missing config is an error, not a silent fresh init.
func TestAddNetworkToConfigMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.toml")
	require.ErrorContains(t, addNetworkToConfig(path, "polygon"), "does not exist")
}

// An unknown network name is rejected.
func TestAddNetworkToConfigUnknownNetwork(t *testing.T) {
	path := writeSingleChainConfig(t)
	require.Error(t, addNetworkToConfig(path, "not-a-network"))
}
