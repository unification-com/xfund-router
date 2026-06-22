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

// baseValidConfig returns a config that passes ValidateBasic, with a real temp keystore file (the
// validator stats it). Tests tweak individual fields from here.
func baseValidConfig(t *testing.T) Config {
	t.Helper()
	ksf, err := os.CreateTemp(t.TempDir(), "keystore-*.json")
	require.NoError(t, err)
	require.NoError(t, ksf.Close())

	c := *DefaultConfig()
	c.Chain.ContractAddress = "0x0000000000000000000000000000000000000001"
	c.Chain.EthHttpHost = "https://rpc.example"
	c.Chain.EthWsHost = "wss://rpc.example"
	c.Chain.NetworkId = 1
	c.Database.Dialect = "sqlite"
	c.Database.Storage = filepath.Join(t.TempDir(), "go-ooo.db")
	c.Keystore.Account = "oracle"
	c.Keystore.File = ksf.Name()
	return c
}

// A blank eth_ws_host is valid: the worker detects events by polling eth_getLogs over HTTP instead.
func TestValidateBasicWebsocketOptional(t *testing.T) {
	c := baseValidConfig(t)
	c.Chain.EthWsHost = ""
	require.NoError(t, c.ValidateBasic(), "blank eth_ws_host should be valid (HTTP-poll mode)")
}

// eth_http_host stays required - it is the foundation transport (calls, getLogs, tx sends).
func TestValidateBasicHttpRequired(t *testing.T) {
	c := baseValidConfig(t)
	c.Chain.EthHttpHost = ""
	err := c.ValidateBasic()
	require.Error(t, err)
	require.Contains(t, err.Error(), "eth_http_host")
}

// The new event_poll_interval_sec renders with its default and survives the round-trip.
func TestEventPollIntervalRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	WriteConfigFile(path, DefaultConfig())

	out, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(out), "event_poll_interval_sec = ")
	require.NotContains(t, string(out), `event_poll_interval_sec = "`)

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())
	got, err := ParseConfig(v)
	require.NoError(t, err)
	require.EqualValues(t, DefaultEventPollIntervalSec, got.Chain.EventPollIntervalSec)
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
