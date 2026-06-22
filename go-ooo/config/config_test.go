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
	// Real networks seed the canonical dpv export base so a fresh deployment pulls the live manifest.
	require.Equal(t, DefaultDexExportBaseUrl, c.Jobs.DexExport.BaseUrl)

	// The dev env runs anvil (London+), so it also uses EIP-1559 pricing.
	dev := DefaultConfig()
	require.NoError(t, dev.InitForNet("dev"))
	require.True(t, dev.Chain.Eip1559)
	// Dev uses a local dpv (or none), so it leaves the export base blank.
	require.Empty(t, dev.Jobs.DexExport.BaseUrl)

	// QL1 (QoM) inits with its network id + a working public RPC default (QL1 has no Infura), and
	// accepts the "ql1" alias.
	qom := DefaultConfig()
	require.NoError(t, qom.InitForNet("qom"))
	require.EqualValues(t, 766, qom.Chain.NetworkId)
	require.NotEmpty(t, qom.Chain.EthHttpHost, "QL1 ships a default RPC since it has no Infura")
	require.NoError(t, DefaultConfig().InitForNet("ql1"), "the ql1 alias resolves to qom")

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

// ChainList returns the legacy single [chain] as a one-element list, or the [[chains]] list when set.
func TestChainList(t *testing.T) {
	c := DefaultConfig()
	c.Chain.NetworkId = 1
	require.Len(t, c.ChainList(), 1)
	require.EqualValues(t, 1, c.ChainList()[0].NetworkId)

	c.Chains = []ChainConfig{{NetworkId: 11155111}, {NetworkId: 137}}
	require.Len(t, c.ChainList(), 2)
	require.EqualValues(t, 11155111, c.ChainList()[0].NetworkId, "[[chains]] takes precedence over [chain]")
}

// BackfillNetworkId resolves the id used to stamp legacy rows: the legacy [chain], a single [[chains]]
// entry, or 0 for several chains.
func TestBackfillNetworkId(t *testing.T) {
	c := DefaultConfig()
	c.Chain.NetworkId = 11155111
	require.EqualValues(t, 11155111, c.BackfillNetworkId())

	c.Chain.NetworkId = 0
	c.Chains = []ChainConfig{{NetworkId: 137}}
	require.EqualValues(t, 137, c.BackfillNetworkId())

	c.Chains = []ChainConfig{{NetworkId: 137}, {NetworkId: 1}}
	require.EqualValues(t, 0, c.BackfillNetworkId())
}

// ValidateBasic validates every [[chains]] entry and rejects duplicate network ids.
func TestValidateBasicMultiChain(t *testing.T) {
	c := baseValidConfig(t) // a valid single [chain]
	mk := func(id int64) ChainConfig { ch := c.Chain; ch.NetworkId = id; return ch }

	c.Chains = []ChainConfig{mk(11155111), mk(137)}
	require.NoError(t, c.ValidateBasic(), "two distinct valid chains should pass")

	c.Chains = []ChainConfig{mk(137), mk(137)}
	require.ErrorContains(t, c.ValidateBasic(), "duplicate network_id")

	bad := mk(137)
	bad.ContractAddress = ""
	c.Chains = []ChainConfig{mk(11155111), bad}
	require.ErrorContains(t, c.ValidateBasic(), "chains[1].contract_address")
}

// A [[chains]] config parses into Chains via the viper round-trip.
func TestParseMultiChainToml(t *testing.T) {
	toml := `
[[chains]]
network_id = 11155111
contract_address = "0xaaa"
eth_http_host = "https://sepolia"
gas_limit = 500000
max_gas_price = 150

[[chains]]
network_id = 137
contract_address = "0xbbb"
eth_http_host = "https://polygon"
gas_limit = 500000
max_gas_price = 200
`
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(toml), 0o600))

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())
	got, err := ParseConfig(v)
	require.NoError(t, err)
	require.Len(t, got.Chains, 2)
	require.EqualValues(t, 11155111, got.Chains[0].NetworkId)
	require.Equal(t, "0xbbb", got.Chains[1].ContractAddress)
	require.EqualValues(t, 200, got.Chains[1].MaxGasPrice)
	require.Len(t, got.ChainList(), 2)
}

// ResolveChain maps the --chain selector to one configured chain: empty → the sole chain, a numeric
// network id, or a case-insensitive name. It errors when the selector is empty with several chains, or
// matches nothing.
func TestResolveChain(t *testing.T) {
	// Single legacy [chain]: an empty selector resolves to it.
	single := DefaultConfig()
	single.Chain.Name = "sepolia"
	single.Chain.NetworkId = 11155111
	ch, err := single.ResolveChain("")
	require.NoError(t, err)
	require.EqualValues(t, 11155111, ch.NetworkId)

	// Several [[chains]]: resolve by name (case-insensitive) and by network id.
	multi := DefaultConfig()
	multi.Chain = ChainConfig{}
	multi.Chains = []ChainConfig{
		{Name: "sepolia", NetworkId: 11155111},
		{Name: "polygon", NetworkId: 137},
	}

	byName, err := multi.ResolveChain("Polygon")
	require.NoError(t, err)
	require.EqualValues(t, 137, byName.NetworkId)

	byId, err := multi.ResolveChain("11155111")
	require.NoError(t, err)
	require.Equal(t, "sepolia", byId.Name)

	// Empty selector with several chains is ambiguous - the error lists the choices.
	_, err = multi.ResolveChain("")
	require.ErrorContains(t, err, "specify --chain")
	require.ErrorContains(t, err, "sepolia(11155111)")

	// An unknown selector errors and lists the configured chains.
	_, err = multi.ResolveChain("optimism")
	require.ErrorContains(t, err, `--chain "optimism"`)
	require.ErrorContains(t, err, "polygon(137)")
}

// The chain name renders into the written config and survives the round-trip, so --chain works after init.
func TestChainNameRoundTrip(t *testing.T) {
	c := DefaultConfig()
	c.InitForSepolia()
	require.Equal(t, "sepolia", c.Chain.Name)

	path := filepath.Join(t.TempDir(), "config.toml")
	WriteConfigFile(path, c)

	out, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(out), `name = "sepolia"`)

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())
	got, err := ParseConfig(v)
	require.NoError(t, err)
	require.Equal(t, "sepolia", got.Chain.Name)
}

// chainNames renders "name(id)" for named chains and the bare id for unnamed ones.
func TestChainNames(t *testing.T) {
	require.Equal(t, "sepolia(11155111), 137", chainNames([]ChainConfig{
		{Name: "sepolia", NetworkId: 11155111},
		{NetworkId: 137},
	}))
}

// AddChain converts a legacy single [chain] into [[chains]] and appends, rejecting a duplicate id.
func TestAddChain(t *testing.T) {
	c := DefaultConfig()
	c.Chain.Name = "sepolia"
	c.Chain.NetworkId = 11155111

	require.NoError(t, c.AddChain(ChainConfig{Name: "polygon", NetworkId: 137}))
	require.Len(t, c.Chains, 2, "the legacy [chain] was folded in and the new chain appended")
	require.EqualValues(t, 11155111, c.Chains[0].NetworkId)
	require.EqualValues(t, 137, c.Chains[1].NetworkId)
	require.Empty(t, c.Chain.NetworkId, "the legacy single block is cleared once [[chains]] is authoritative")

	// A third distinct chain appends to the existing list.
	require.NoError(t, c.AddChain(ChainConfig{Name: "mainnet", NetworkId: 1}))
	require.Len(t, c.Chains, 3)

	// A duplicate network id is rejected.
	require.ErrorContains(t, c.AddChain(ChainConfig{Name: "polygon-again", NetworkId: 137}), "already configured")
	require.Len(t, c.Chains, 3, "the rejected chain was not added")
}

// A [[chains]] config survives the full write -> read round-trip through the template (the multi-chain
// rendering path), so 'init --add' produces a file go-ooo can read back.
func TestWriteReadMultiChainRoundTrip(t *testing.T) {
	c := DefaultConfig()
	c.Chain = ChainConfig{}
	c.Chains = []ChainConfig{
		{Name: "sepolia", NetworkId: 11155111, ContractAddress: "0xaaa", EthHttpHost: "https://s", GasLimit: 500000, MaxGasPrice: 150, EventPollIntervalSec: 6, EventScanBatchBlocks: 2000},
		{Name: "polygon", NetworkId: 137, ContractAddress: "0xbbb", EthHttpHost: "https://p", GasLimit: 500000, MaxGasPrice: 200, EventPollIntervalSec: 4, EventScanBatchBlocks: 2000},
	}

	path := filepath.Join(t.TempDir(), "config.toml")
	WriteConfigFile(path, c)

	out, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(out), "[[chains]]")
	require.NotContains(t, string(out), "[chain]\n", "the single [chain] block must not also render")

	got, err := LoadConfigFile(path)
	require.NoError(t, err)
	require.Len(t, got.Chains, 2)
	require.Equal(t, "polygon", got.Chains[1].Name)
	require.EqualValues(t, 137, got.Chains[1].NetworkId)
	require.EqualValues(t, 4, got.Chains[1].EventPollIntervalSec)
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
