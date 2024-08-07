package config

import (
	"bytes"
	"github.com/spf13/viper"
	"os"
	"text/template"
)

const DefaultConfigTemplate = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

##########################################
## Chain                                ##
##########################################

[chain]
# Address of the Router smart contract
contract_address = "{{ .Chain.ContractAddress }}"

# Network Id, e.g. 1 for mainnet etc.
network_id = {{ .Chain.NetworkId }}

# RPC nodes - e.g. Infura/Alchemy
eth_http_host = "{{ .Chain.EthHttpHost }}"
eth_ws_host = "{{ .Chain.EthWsHost }}"

# First block to start checking for jobs.
# Generally, the block you registered as a provider.
# Defaults to the block the Router contract was deployed
first_block = {{ .Chain.FirstBlock }}

# Gas limit for fulfilling requests
gas_limit = {{ .Chain.GasLimit }}

# Max gas price you are willing to pay to fulfil a request
max_gas_price = {{ .Chain.MaxGasPrice }}

##########################################
## Database                             ##
##########################################

# Dialect can be either sqlite or postgres.
# For sqlite, storage must be set to the full path of 
# the sqlite database (defaults to the go-ooo home)
# For postgres, the host, port, database, user and password
# are required

[database]
# sqlite or postgres
dialect = "{{ .Database.Dialect }}"

# for sqlite
storage = "{{ .Database.Storage }}"

# for postgres
host = "{{ .Database.Host }}"
port = {{ .Database.Port }}
database = "{{ .Database.Database }}"
user = "{{ .Database.User }}"
password = "{{ .Database.Password }}"

##########################################
## Jobs                                 ##
##########################################

# Configure how often go-ooo checks for requests etc.

[jobs]
# number of seconds between polling the chain for new requests
check_duration = {{ .Jobs.CheckDuration }}

# URL for the Finchains API
ooo_api_url = "{{ .Jobs.OooApiUrl }}"

# Number of blocks to wait before fulfilling a request
wait_confirmations = {{ .Jobs.WaitConfirmations }}

##########################################
## Keystore                             ##
##########################################

# Path to keystore, and account name to use

[keystorage]
account = "{{ .Keystore.Account }}"
file = "{{ .Keystore.File }}"

##########################################
## Logs                                 ##
##########################################

[log]
# info, debug
level = "{{ .Log.Level }}"

##########################################
## Prometheus                           ##
##########################################

[prometheus]

# Port on which to serve Prometheus metrics
port = "{{ .Prometheus.Port }}"

##########################################
## Serve                                ##
##########################################

# Host and port on which to listen for Admin commands
# for example, query the db, update your provider fee on-chain etc.

[serve]
host = "{{ .Serve.Host }}"
port = "{{ .Serve.Port }}"

##########################################
## Subchain                             ##
##########################################

# RPC nodes for each chain the DEX modules are dependent on. This is
# only used by the DEX modules to get simple data for each price request.
# The AdHoc querier will query the DEX's parent chain to get the latest block
# number. This allows the DEX module to query GraphQL to get data based on
# block numbers

[subchain]
eth_http_rpc = "{{ .Subchain.EthHttpRpc }}"
polygon_http_rpc = "{{ .Subchain.PolygonHttpRpc }}"
bsc_http_rpc = "{{ .Subchain.BcsHttpRpc }}"
xdai_http_rpc = "{{ .Subchain.XdaiHttpRpc }}"
fantom_http_rpc = "{{ .Subchain.FantomHttpRpc }}"
shibarium_http_rpc = "{{ .Subchain.ShibariumHttpRpc }}"

##########################################
## API Keys                             ##
##########################################

# some DEX GraphQL APIs require keys

[api_keys]
graph_network_key = "{{ .ApiKeys.GraphNetwork }}"

##########################################
## DEX Limits                           ##
##########################################

# Minimum Liquidity (USD) and Tx thresholds, below which
# a pair is not included in an AdHoc query

[dexs.bsc_pancakeswap_v3]
min_reserve_usd = "{{ .Dexs.BscPancakeswapV3.MinReserveUsd }}"
min_tx_count = "{{ .Dexs.BscPancakeswapV3.MinTxCount }}"

[dexs.eth_shibaswap]
min_reserve_usd = "{{ .Dexs.EthShibaswap.MinReserveUsd }}"
min_tx_count = "{{ .Dexs.EthShibaswap.MinTxCount }}"

[dexs.eth_sushiswap]
min_reserve_usd = "{{ .Dexs.EthSushiswap.MinReserveUsd }}"
min_tx_count = "{{ .Dexs.EthSushiswap.MinTxCount }}"

[dexs.eth_uniswap_v2]
min_reserve_usd = "{{ .Dexs.EthUniswapV2.MinReserveUsd }}"
min_tx_count = "{{ .Dexs.EthUniswapV2.MinTxCount }}"

[dexs.eth_uniswap_v3]
min_reserve_usd = "{{ .Dexs.EthUniswapV3.MinReserveUsd }}"
min_tx_count = "{{ .Dexs.EthUniswapV3.MinTxCount }}"

[dexs.polygon_pos_quickswap_v3]
min_reserve_usd = "{{ .Dexs.PolygonPosQuickswapV3.MinReserveUsd }}"
min_tx_count = "{{ .Dexs.PolygonPosQuickswapV3.MinTxCount }}"

[dexs.xdai_honeyswap]
min_reserve_usd = "{{ .Dexs.XdaiHoneyswap.MinReserveUsd }}"
min_tx_count = "{{ .Dexs.XdaiHoneyswap.MinTxCount }}"

`

var configTemplate *template.Template

func init() {
	var err error

	tmpl := template.New("appConfigFileTemplate")

	if configTemplate, err = tmpl.Parse(DefaultConfigTemplate); err != nil {
		panic(err)
	}
}

// ParseConfig retrieves the default environment configuration for the
// application.
func ParseConfig(v *viper.Viper) (*Config, error) {
	conf := DefaultConfig()
	err := v.Unmarshal(conf)

	return conf, err
}

func WriteConfigFile(configFilePath string, config *Config) {
	var buffer bytes.Buffer

	if err := configTemplate.Execute(&buffer, config); err != nil {
		panic(err)
	}

	err := os.WriteFile(configFilePath, buffer.Bytes(), 0o644)

	if err != nil {
		panic(err)
	}
}
