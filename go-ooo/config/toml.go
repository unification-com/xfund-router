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

# Single network: the [chain] block below. To run SEVERAL networks from this one process, replace
# [chain] with a [[chains]] array-of-tables - one block per network, each with the same fields shown
# here (contract_address, network_id, RPCs, gas, first_block, event_* ). Network ids must be distinct.
# Example:
#   [[chains]]
#   network_id = 11155111
#   contract_address = "0x..."
#   eth_http_host = "https://..."
#   ... (gas_limit, max_gas_price, etc.)
#   [[chains]]
#   network_id = 137
#   contract_address = "0x..."
#   ...
# One keystore/provider key, DB and pricing engine are shared; each chain runs as an independent worker.

[chain]
# Address of the Router smart contract
contract_address = "{{ .Chain.ContractAddress }}"

# Network Id, e.g. 1 for mainnet etc.
network_id = {{ .Chain.NetworkId }}

# RPC nodes - e.g. Infura/Alchemy/PublicNode.
# eth_http_host (required) carries all calls, getLogs event detection and tx sends.
# eth_ws_host (optional) is used only for low-latency event subscriptions; leave it blank, or when it
# is unavailable, go-ooo detects events by polling eth_getLogs over HTTP instead. Free RPCs often
# throttle or omit WSS, so HTTP-only is a fully supported mode.
eth_http_host = "{{ .Chain.EthHttpHost }}"
eth_ws_host = "{{ .Chain.EthWsHost }}"

# Seconds between eth_getLogs event polls when running in HTTP-poll mode (no/await eth_ws_host).
# Default 6. A few seconds' detection latency is immaterial - a fulfilment waits for
# wait_confirmations and the job sweep before it goes out anyway.
event_poll_interval_sec = {{ .Chain.EventPollIntervalSec }}

# Max block range per eth_getLogs request when scanning for events. Default 2000. Lower it for RPCs
# that reject or throttle wide ranges (e.g. block-explorer eth-rpc proxies).
event_scan_batch_blocks = {{ .Chain.EventScanBatchBlocks }}

# First block to start checking for jobs.
# Generally, the block you registered as a provider.
# Defaults to the block the Router contract was deployed
first_block = {{ .Chain.FirstBlock }}

# Gas limit for fulfilling requests
gas_limit = {{ .Chain.GasLimit }}

# Max gas price you are willing to pay to fulfil a request
max_gas_price = {{ .Chain.MaxGasPrice }}

# Percentage to bump the gas price by when replacing a stuck (still-pending) fulfilment
# tx at the same nonce. Must be >= 10 - the network requires at least a 10% increase to
# replace a transaction; values below 10 fall back to a safe default.
gas_bump_percent = {{ .Chain.GasBumpPercent }}

# Send EIP-1559 (type-2) dynamic-fee transactions, pricing each fulfilment with a priority
# fee (tip) plus a base-fee-derived max fee instead of a single legacy gas price. max_gas_price
# still caps the total. Automatically falls back to legacy pricing on a pre-London chain (one
# whose latest block carries no base fee), so leaving this true is safe everywhere.
eip1559 = {{ .Chain.Eip1559 }}

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

# Number of blocks to wait before fulfilling a request
wait_confirmations = {{ .Jobs.WaitConfirmations }}

# How long (seconds) to keep retrying a request before giving up and marking it failed.
# Wall-clock (chain-agnostic) - the Router has no on-chain expiry, so this is purely
# go-ooo deciding to stop. 0 disables the age-based give-up. Default 3600 (1 hour).
max_job_age = {{ .Jobs.MaxJobAge }}

# dex-pair-verify provider-export consumer: the manifest of priceable DEX sources + their
# verified-pair feeds go-ooo polls to know which DEXs + pairs it can price. The fetched manifest is
# persisted, so the oracle keeps pricing from the last sync if the API is briefly unreachable.
[jobs.dex_export]
# Export API base. Defaults to the Unification dpv (https://ooo-dex.unification.io/api/ooo/v1) for the
# real networks; the provider authenticates with its on-chain-registered wallet. Point at your own dpv,
# or leave EMPTY to skip the sync and price from the catalogue already persisted in the database.
base_url = "{{ .Jobs.DexExport.BaseUrl }}"
# Export bearer token for the API (interim; the wallet challenge-response replaces it).
api_token = "{{ .Jobs.DexExport.ApiToken }}"
# Seconds between export refreshes (default 3600).
poll_interval_sec = {{ .Jobs.DexExport.PollIntervalSec }}

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
# The price querier will query the DEX's parent chain to get the latest block
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
# a pair is not included in a price query

[dexs.bsc_pancakeswap_v3]
min_reserve_usd = {{ .Dexs.BscPancakeswapV3.MinReserveUsd }}
min_tx_count = {{ .Dexs.BscPancakeswapV3.MinTxCount }}

[dexs.eth_shibaswap]
min_reserve_usd = {{ .Dexs.EthShibaswap.MinReserveUsd }}
min_tx_count = {{ .Dexs.EthShibaswap.MinTxCount }}

[dexs.eth_sushiswap]
min_reserve_usd = {{ .Dexs.EthSushiswap.MinReserveUsd }}
min_tx_count = {{ .Dexs.EthSushiswap.MinTxCount }}

[dexs.eth_uniswap_v2]
min_reserve_usd = {{ .Dexs.EthUniswapV2.MinReserveUsd }}
min_tx_count = {{ .Dexs.EthUniswapV2.MinTxCount }}

[dexs.eth_uniswap_v3]
min_reserve_usd = {{ .Dexs.EthUniswapV3.MinReserveUsd }}
min_tx_count = {{ .Dexs.EthUniswapV3.MinTxCount }}

[dexs.polygon_pos_quickswap_v3]
min_reserve_usd = {{ .Dexs.PolygonPosQuickswapV3.MinReserveUsd }}
min_tx_count = {{ .Dexs.PolygonPosQuickswapV3.MinTxCount }}

[dexs.xdai_honeyswap]
min_reserve_usd = {{ .Dexs.XdaiHoneyswap.MinReserveUsd }}
min_tx_count = {{ .Dexs.XdaiHoneyswap.MinTxCount }}

##########################################
## Price-quality gate                   ##
##########################################

# Each DEX price is scored on its live sample: number of surviving pools, distinct venues
# (chain+dex), total backing liquidity (whole USD) and dispersion (robust coefficient of
# variation = MAD scale / median).
#
# flag_*   - log a "thin price sample" warning but STILL fulfil. Soft bars; defaults calibrated
#            against a live mainnet sweep (legit pairs: >=2 venues, dispersion <=0.7%).
# refuse_* - refuse to publish (the request stays unfulfilled) when a sample is degenerate. A
#            conservative backstop: each check is disabled at 0, and all default to 0 so the
#            refuse path is opt-in. Calibrated starting set if you enable it:
#              refuse_min_liquidity_usd = 25000   (sub-$25k is dust; the thinnest legit pair was $30k)
#              refuse_max_dispersion    = 0.20    (>20% post-MAD disagreement is clearly broken)
#            Do NOT refuse on venues - real single-venue pairs exist. Note a liquidity floor will
#            also refuse pairs whose ReserveUsd metadata is stale/zero.

[price_quality]
flag_min_pools = {{ .PriceQuality.FlagMinPools }}
flag_min_venues = {{ .PriceQuality.FlagMinVenues }}
flag_min_liquidity_usd = {{ .PriceQuality.FlagMinLiquidityUsd }}
flag_max_dispersion = {{ .PriceQuality.FlagMaxDispersion }}
refuse_min_pools = {{ .PriceQuality.RefuseMinPools }}
refuse_min_venues = {{ .PriceQuality.RefuseMinVenues }}
refuse_min_liquidity_usd = {{ .PriceQuality.RefuseMinLiquidityUsd }}
refuse_max_dispersion = {{ .PriceQuality.RefuseMaxDispersion }}
# S6 manipulation cross-check (opt-in): drop a surviving pool that is a mild outlier
# (deviation > suspect_deviation, a modified z-score) AND shallow (liquidity <
# suspect_min_liquidity_usd) - a likely manipulation. Deep off-consensus pools are kept.
# Disabled unless both are > 0. Suggested: suspect_deviation = 2.0, suspect_min_liquidity_usd = 10000.
suspect_deviation = {{ .PriceQuality.SuspectDeviation }}
suspect_min_liquidity_usd = {{ .PriceQuality.SuspectMinLiquidityUsd }}

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

	// 0600 - config.toml can hold secrets (e.g. the database password).
	err := os.WriteFile(configFilePath, buffer.Bytes(), 0o600)

	if err != nil {
		panic(err)
	}
}
