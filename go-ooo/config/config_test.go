package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestInitForNet(t *testing.T) {
	c := DefaultConfig()
	require.NoError(t, c.InitForNet("mainnet"))
	require.EqualValues(t, 1, c.Chain.NetworkId)
	// Post-London chains default to EIP-1559 pricing.
	require.True(t, c.Chain.Eip1559)

	// The dev env runs anvil (London+), so it also uses EIP-1559 pricing.
	dev := DefaultConfig()
	require.NoError(t, dev.InitForNet("dev"))
	require.True(t, dev.Chain.Eip1559)

	// A typo must error, not silently configure DevNet (127.0.0.1).
	typo := DefaultConfig()
	require.Error(t, typo.InitForNet("mainet"))
}

func TestWriteConfigDexThresholdsUnquoted(t *testing.T) {
	def := DefaultConfig()
	want := def.Dexs.EthUniswapV2.MinReserveUsd
	require.NotZero(t, want, "precondition: default min_reserve_usd should be non-zero")

	path := filepath.Join(t.TempDir(), "config.toml")
	WriteConfigFile(path, def)

	out, err := os.ReadFile(path)
	require.NoError(t, err)
	// uint64 thresholds must render unquoted so they re-parse as integers, not strings.
	require.Contains(t, string(out), "min_reserve_usd = ")
	require.NotContains(t, string(out), `min_reserve_usd = "`)
	require.NotContains(t, string(out), `min_tx_count = "`)

	// And they must survive the write → read round-trip as their uint64 value.
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())
	got, err := ParseConfig(v)
	require.NoError(t, err)
	require.EqualValues(t, want, got.Dexs.EthUniswapV2.MinReserveUsd)
}
