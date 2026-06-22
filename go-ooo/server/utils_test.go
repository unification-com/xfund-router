package server

import (
	"strings"
	"testing"

	"go-ooo/config"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// decodeWithChainFlag binds a --chain selector flag (as the admin/query/start commands carry) into a
// fresh viper via the collision-aware binder, reads the given TOML, and decodes it into a Config -
// mirroring InterceptConfigsPreRunHandler. It must NOT fail with "'chain' expected a map, got
// 'string'": the string --chain flag must never shadow the [chain]/[[chains]] tables.
func decodeWithChainFlag(t *testing.T, toml string) *config.Config {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("chain", "", "chain selector")
	fs.String("first-block", "", "first block override")

	v := viper.New()
	bindPFlagsExcept(v, fs)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadConfig(strings.NewReader(toml)))

	conf := config.DefaultConfig()
	require.NoError(t, v.Unmarshal(conf))
	return conf
}

// A [[chains]] config decodes cleanly even with the --chain flag bound.
func TestConfigDecodeMultiChainIgnoresChainFlag(t *testing.T) {
	conf := decodeWithChainFlag(t, `
[[chains]]
name = "sepolia"
network_id = 11155111
contract_address = "0xabc"
eth_http_host = "https://s"
`)
	require.Len(t, conf.Chains, 1)
	require.EqualValues(t, 11155111, conf.Chains[0].NetworkId)
}

// The legacy single [chain] table also decodes with the --chain flag bound (the regression case).
func TestConfigDecodeSingleChainIgnoresChainFlag(t *testing.T) {
	conf := decodeWithChainFlag(t, `
[chain]
name = "sepolia"
network_id = 11155111
contract_address = "0xabc"
eth_http_host = "https://s"
`)
	require.EqualValues(t, 11155111, conf.Chain.NetworkId)
	require.Equal(t, "sepolia", conf.Chain.Name)
}
