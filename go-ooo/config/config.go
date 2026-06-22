package config

import (
	"errors"
	"fmt"
	oooapidextypes "go-ooo/ooo_api/dex/types"
	"os"
)

type JobsConfig struct {
	CheckDuration     uint64          `mapstructure:"check_duration"`
	WaitConfirmations uint64          `mapstructure:"wait_confirmations"`
	MaxJobAge         uint64          `mapstructure:"max_job_age"`
	DexExport         DexExportConfig `mapstructure:"dex_export"`
}

// DexExportConfig drives the dex-pair-verify provider-export consumer (#127): the manifest of
// priceable DEX sources + their verified-pair feeds that go-ooo polls to drive modular DEX dispatch.
// BaseUrl is the dpv API (e.g. https://dex-pair-verify.example.io/api/ooo/v1); when empty the sync is
// skipped and the oracle prices from the catalogue last persisted in the database. ApiToken is the
// interim static export bearer (the XR3 wallet auth replaces it later). PollIntervalSec is how often
// the catalogue is refreshed.
type DexExportConfig struct {
	BaseUrl         string `mapstructure:"base_url"`
	ApiToken        string `mapstructure:"api_token"`
	PollIntervalSec uint64 `mapstructure:"poll_interval_sec"`
}

// DefaultDexExportBaseUrl is the canonical Unification dex-pair-verify export API. init seeds it for
// the real networks (the provider authenticates with its on-chain-registered wallet), so a fresh
// deployment pulls the live pair manifest out of the box. The dev net leaves it blank (local dpv).
const DefaultDexExportBaseUrl = "https://ooo-dex.unification.io/api/ooo/v1"

type ServeConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

type KeystoreConfig struct {
	File    string `mapstructure:"file"`
	Account string `mapstructure:"account"`
}

// DefaultEventPollIntervalSec is the HTTP event-poll cadence used when event_poll_interval_sec is
// unset. A few seconds' detection latency is immaterial - a fulfilment already waits for
// wait_confirmations and the job sweep before going out.
const DefaultEventPollIntervalSec uint64 = 6

// DefaultEventScanBatchBlocks is the eth_getLogs block-range batch size used when
// event_scan_batch_blocks is unset. A wide range can be rejected (or throttled) by limited RPCs, so
// the catch-up is split into batches of at most this many blocks.
const DefaultEventScanBatchBlocks uint64 = 2000

type ChainConfig struct {
	GasLimit        uint64 `mapstructure:"gas_limit"`
	MaxGasPrice     int64  `mapstructure:"max_gas_price"`
	GasBumpPercent  uint64 `mapstructure:"gas_bump_percent"`
	Eip1559         bool   `mapstructure:"eip1559"`
	ContractAddress string `mapstructure:"contract_address"`
	EthHttpHost     string `mapstructure:"eth_http_host"`
	EthWsHost       string `mapstructure:"eth_ws_host"`
	NetworkId       int64  `mapstructure:"network_id"`
	FirstBlock      uint64 `mapstructure:"first_block"`
	// EventPollIntervalSec is how often (seconds) the worker scans eth_getLogs for events when it is
	// running in HTTP-poll mode (no eth_ws_host, or the websocket dropped). 0 → DefaultEventPollIntervalSec.
	EventPollIntervalSec uint64 `mapstructure:"event_poll_interval_sec"`
	// EventScanBatchBlocks caps the eth_getLogs block range per request. Lower it for RPCs that reject
	// or throttle wide ranges (e.g. block-explorer eth-rpc proxies). 0 → DefaultEventScanBatchBlocks.
	EventScanBatchBlocks uint64 `mapstructure:"event_scan_batch_blocks"`
}

type DatabaseConfig struct {
	Dialect  string `mapstructure:"dialect"`
	Storage  string `mapstructure:"storage"`
	Host     string `mapstructure:"host"`
	Port     uint64 `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
}

type PrometheusConfig struct {
	Port string `mapstructure:"port"`
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type SubchainConfig struct {
	EthHttpRpc       string `mapstructure:"eth_http_rpc"`
	PolygonHttpRpc   string `mapstructure:"polygon_http_rpc"`
	BcsHttpRpc       string `mapstructure:"bsc_http_rpc"`
	XdaiHttpRpc      string `mapstructure:"xdai_http_rpc"`
	FantomHttpRpc    string `mapstructure:"fantom_http_rpc"`
	ShibariumHttpRpc string `mapstructure:"shibarium_http_rpc"`
}

type ApiKeysConfig struct {
	GraphNetwork string `mapstructure:"graph_network_key"`
}

type DexConfig struct {
	MinReserveUsd uint64 `mapstructure:"min_reserve_usd"`
	MinTxCount    uint64 `mapstructure:"min_tx_count"`
}

type DexList struct {
	BscPancakeswapV3      DexConfig `mapstructure:"bsc_pancakeswap_v3"`
	EthShibaswap          DexConfig `mapstructure:"eth_shibaswap"`
	EthSushiswap          DexConfig `mapstructure:"eth_sushiswap"`
	EthUniswapV2          DexConfig `mapstructure:"eth_uniswap_v2"`
	EthUniswapV3          DexConfig `mapstructure:"eth_uniswap_v3"`
	PolygonPosQuickswapV3 DexConfig `mapstructure:"polygon_pos_quickswap_v3"`
	XdaiHoneyswap         DexConfig `mapstructure:"xdai_honeyswap"`
}

// PriceQualityConfig gates DEX prices on the quality of the live sample backing them.
// flag_* are soft bars that log a warning but still fulfil (sensible defaults). refuse_* are a
// conservative backstop that declines to publish (the request stays unfulfilled); each refuse
// check is disabled at its zero value, and all default to 0 so refusal is opt-in - only the
// existing zero-prices refusal applies until an operator sets them.
type PriceQualityConfig struct {
	FlagMinPools        uint64  `mapstructure:"flag_min_pools"`
	FlagMinVenues       uint64  `mapstructure:"flag_min_venues"`
	FlagMinLiquidityUsd uint64  `mapstructure:"flag_min_liquidity_usd"`
	FlagMaxDispersion   float64 `mapstructure:"flag_max_dispersion"`

	RefuseMinPools        uint64  `mapstructure:"refuse_min_pools"`
	RefuseMinVenues       uint64  `mapstructure:"refuse_min_venues"`
	RefuseMinLiquidityUsd uint64  `mapstructure:"refuse_min_liquidity_usd"`
	RefuseMaxDispersion   float64 `mapstructure:"refuse_max_dispersion"`

	// S6 manipulation cross-check (opt-in, default-off): drop a surviving pool whose estimate
	// deviates beyond SuspectDeviation (modified z-score) AND whose backing liquidity is below
	// SuspectMinLiquidityUsd. A shallow pool notably off the consensus is a likely manipulation; a
	// deep one is a genuine arbitrage gap (kept). Disabled unless BOTH are set > 0.
	SuspectDeviation       float64 `mapstructure:"suspect_deviation"`
	SuspectMinLiquidityUsd uint64  `mapstructure:"suspect_min_liquidity_usd"`
}

type Config struct {
	Jobs     JobsConfig     `mapstructure:"jobs"`
	Serve    ServeConfig    `mapstructure:"serve"`
	Keystore KeystoreConfig `mapstructure:"keystorage"`
	// Chain is the legacy single-network block ([chain]). Chains is the multi-network form
	// ([[chains]]); when set it takes precedence (see ChainList). One go-ooo process runs every chain
	// in ChainList, each as an independent worker sharing the keystore, DB and pricing engine.
	Chain        ChainConfig        `mapstructure:"chain"`
	Chains       []ChainConfig      `mapstructure:"chains"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Prometheus   PrometheusConfig   `mapstructure:"prometheus"`
	Log          LogConfig          `mapstructure:"log"`
	Subchain     SubchainConfig     `mapstructure:"subchain"`
	ApiKeys      ApiKeysConfig      `mapstructure:"api_keys"`
	Dexs         DexList            `mapstructure:"dexs"`
	PriceQuality PriceQualityConfig `mapstructure:"price_quality"`
}

// DefaultConfig returns server's default configuration.
func DefaultConfig() *Config {
	return &Config{
		Jobs: JobsConfig{
			CheckDuration:     5,
			WaitConfirmations: 1,
			MaxJobAge:         3600,
			DexExport: DexExportConfig{
				BaseUrl:         DefaultDexExportBaseUrl,
				ApiToken:        "",
				PollIntervalSec: 3600,
			},
		},
		Serve: ServeConfig{
			Host: "127.0.0.1",
			Port: "8445",
		},
		Keystore: KeystoreConfig{
			File:    "",
			Account: "",
		},
		Chain: ChainConfig{
			GasLimit:       500000,
			MaxGasPrice:    150,
			GasBumpPercent: 13,
			// Modern default: send EIP-1559 dynamic-fee txs. Auto-falls-back to legacy on a
			// pre-London chain (one with no base fee), so this is safe even on a migrated config.
			Eip1559:              true,
			ContractAddress:      "",
			EthHttpHost:          "",
			EthWsHost:            "",
			NetworkId:            0,
			FirstBlock:           0,
			EventPollIntervalSec: DefaultEventPollIntervalSec,
			EventScanBatchBlocks: DefaultEventScanBatchBlocks,
		},
		Database: DatabaseConfig{
			Dialect:  "sqlite",
			Storage:  "",
			Host:     "",
			Port:     0,
			User:     "",
			Password: "",
			Database: "",
		},
		Prometheus: PrometheusConfig{
			Port: "9000",
		},
		Log: LogConfig{
			Level: "info",
		},
		Subchain: SubchainConfig{
			EthHttpRpc:       "https://rpc.mevblocker.io",
			PolygonHttpRpc:   "https://polygon-bor-rpc.publicnode.com",
			BcsHttpRpc:       "https://bsc-dataseed.binance.org",
			XdaiHttpRpc:      "https://rpc.gnosischain.com",
			FantomHttpRpc:    "https://finchains.io/api",
			ShibariumHttpRpc: "https://rpc.shibrpc.com",
		},
		ApiKeys: ApiKeysConfig{
			GraphNetwork: "",
		},
		Dexs: DexList{
			BscPancakeswapV3: DexConfig{
				MinReserveUsd: oooapidextypes.DefaultMinLiquidity,
				MinTxCount:    oooapidextypes.DefaultMinTxCount,
			},
			EthShibaswap: DexConfig{
				MinReserveUsd: oooapidextypes.DefaultMinLiquidity,
				MinTxCount:    oooapidextypes.DefaultMinTxCount,
			},
			EthSushiswap: DexConfig{
				MinReserveUsd: oooapidextypes.DefaultMinLiquidity,
				MinTxCount:    oooapidextypes.DefaultMinTxCount,
			},
			EthUniswapV2: DexConfig{
				MinReserveUsd: oooapidextypes.DefaultMinLiquidity,
				MinTxCount:    oooapidextypes.DefaultMinTxCount,
			},
			EthUniswapV3: DexConfig{
				MinReserveUsd: oooapidextypes.DefaultMinLiquidity,
				MinTxCount:    oooapidextypes.DefaultMinTxCount,
			},
			PolygonPosQuickswapV3: DexConfig{
				MinReserveUsd: oooapidextypes.DefaultMinLiquidity,
				MinTxCount:    oooapidextypes.DefaultMinTxCount,
			},
			XdaiHoneyswap: DexConfig{
				MinReserveUsd: oooapidextypes.DefaultMinLiquidity,
				MinTxCount:    oooapidextypes.DefaultMinTxCount,
			},
		},
		PriceQuality: PriceQualityConfig{
			// Flag defaults calibrated against a live mainnet sweep (2026-06-11): every legit
			// pair had >=2 venues and legit dispersion stayed <=0.7%, while a thin long-tail pair
			// reached ~2% - so 2 venues / $100k / 1% dispersion warn on the genuinely thin/dispersed
			// without false-flagging blue-chips. Warn (but still answer) below these.
			FlagMinPools:        2,
			FlagMinVenues:       2,
			FlagMinLiquidityUsd: 100000,
			FlagMaxDispersion:   0.01,
			// Refuse backstop disabled by default (opt-in). The same sweep suggests a conservative
			// starting set if you enable it: refuse_min_liquidity_usd = 25000 (sub-$25k is dust;
			// the thinnest legit pair was $30k) and refuse_max_dispersion = 0.20 (>20% post-MAD is
			// clearly broken). Do NOT refuse on venues - real single-venue pairs exist.
			RefuseMinPools:        0,
			RefuseMinVenues:       0,
			RefuseMinLiquidityUsd: 0,
			RefuseMaxDispersion:   0,
			// S6 suspect-pool drop disabled by default (opt-in). Suggested if enabling:
			// suspect_deviation = 2.0 (modified-z; catches mild outliers within MAD's 3.5) +
			// suspect_min_liquidity_usd = 10000 (drop sub-$10k mild outliers as likely manipulation;
			// deep pools off-consensus are kept as real arbitrage gaps).
			SuspectDeviation:       0,
			SuspectMinLiquidityUsd: 0,
		},
	}
}

func (c *Config) InitForNet(network string) error {
	switch network {
	case "sepolia":
		c.InitForSepolia()
	case "mainnet":
		c.InitForMainnet()
	case "polygon":
		c.InitForPolygon()
	case "dev":
		c.InitForDevNet()
	case "shibarium":
		c.InitForShibarium()
	case "puppynet":
		c.InitForShibariumPuppynet()
	default:
		// Don't silently configure DevNet (127.0.0.1) for a typo'd network - the
		// operator would get a config quietly pointing at localhost.
		return fmt.Errorf("unknown network %q (expected one of: dev, sepolia, mainnet, polygon, shibarium, puppynet)", network)
	}
	return nil
}

func (c *Config) InitForDevNet() {
	c.Chain.ContractAddress = "0x5b1869D9A4C187F2EAa108f3062412ecf0526b24"
	c.Chain.EthHttpHost = "http://127.0.0.1:8545"
	c.Chain.EthWsHost = "ws://127.0.0.1:8545"
	c.Chain.NetworkId = 696969
	c.Chain.FirstBlock = 1
	// The dev env runs anvil, a London+ chain, so it exercises the EIP-1559 path like the prod
	// networks. (Kept explicit rather than relying on the default, to document the dev intent.)
	c.Chain.Eip1559 = true
	// Dev points at a local dpv (or none), not the production export API - leave base_url blank so the
	// dev provider (unregistered on any real chain) doesn't fail wallet-auth against prod dpv.
	c.Jobs.DexExport.BaseUrl = ""
}

func (c *Config) InitForSepolia() {
	c.Chain.ContractAddress = "0xf6b5d6eafE402d22609e685DE3394c8b359CaD31"
	c.Chain.EthHttpHost = ""
	c.Chain.EthWsHost = ""
	c.Chain.NetworkId = 11155111
	c.Chain.FirstBlock = 3647468
}

func (c *Config) InitForMainnet() {
	c.Chain.ContractAddress = "0x9ac9AE20a17779c17b069b48A8788e3455fC6121"
	c.Chain.EthHttpHost = ""
	c.Chain.EthWsHost = ""
	c.Chain.NetworkId = 1
	c.Chain.FirstBlock = 12728316
}

func (c *Config) InitForPolygon() {
	c.Chain.ContractAddress = "0x5E9405888255C142207Ab692C72A8cd6fc85C3A2"
	c.Chain.EthHttpHost = ""
	c.Chain.EthWsHost = ""
	c.Chain.NetworkId = 137
	c.Chain.FirstBlock = 24460663
}

func (c *Config) InitForShibarium() {
	c.Chain.ContractAddress = "0x2E9ade949900e19735689686E61BF6338a65B881"
	c.Chain.EthHttpHost = ""
	c.Chain.EthWsHost = ""
	c.Chain.NetworkId = 109
	c.Chain.FirstBlock = 591096
}

func (c *Config) InitForShibariumPuppynet() {
	c.Chain.ContractAddress = "0x7a99f98EfC7C1313E3a8FA4Be36aE2b100a1622F"
	c.Chain.EthHttpHost = ""
	c.Chain.EthWsHost = ""
	c.Chain.NetworkId = 157
	c.Chain.FirstBlock = 4764990
}

func (c *Config) SetKeystore(path, account string) {
	c.Keystore.File = path
	c.Keystore.Account = account
}

func (c *Config) SetSqliteDb(path string) {
	c.Database.Storage = path
}

// ChainList returns the effective set of chains to run: the multi-network [[chains]] when present,
// otherwise the single legacy [chain] wrapped as a one-element list. Every consumer iterates this, so
// single- and multi-network configs share one path.
func (c Config) ChainList() []ChainConfig {
	if len(c.Chains) > 0 {
		return c.Chains
	}
	return []ChainConfig{c.Chain}
}

// BackfillNetworkId is the chain id used to stamp legacy (pre-chain_id) rows during the DB migration.
// It is the legacy [chain].network_id, or - for a single [[chains]] entry - that chain's id. With
// several [[chains]] it is 0 (an already-v5 DB has nothing to backfill; a populated pre-v5 DB must be
// migrated single-chain first, or keep [chain] set to the legacy network).
func (c Config) BackfillNetworkId() int64 {
	if c.Chain.NetworkId != 0 {
		return c.Chain.NetworkId
	}
	if len(c.Chains) == 1 {
		return c.Chains[0].NetworkId
	}
	return 0
}

// validateChain checks one chain block; label scopes the error message (e.g. "chain", "chains[1]").
func validateChain(ch ChainConfig, label string) error {
	if ch.ContractAddress == "" {
		return fmt.Errorf("%s.contract_address not set in config.toml", label)
	}
	// eth_ws_host is OPTIONAL: when blank (or the websocket later drops), the worker detects events by
	// polling eth_getLogs over the HTTP endpoint. eth_http_host is the foundation and stays required.
	if ch.EthHttpHost == "" {
		return fmt.Errorf("%s.eth_http_host not set in config.toml", label)
	}
	if ch.NetworkId == 0 {
		return fmt.Errorf("%s.network_id not set in config.toml", label)
	}
	if ch.GasLimit == 0 {
		return fmt.Errorf("%s.gas_limit not set in config.toml", label)
	}
	if ch.MaxGasPrice == 0 {
		return fmt.Errorf("%s.max_gas_price not set in config.toml", label)
	}
	return nil
}

func (c Config) ValidateBasic() error {

	// Validate every configured chain and reject duplicate network ids (each worker is keyed by its id).
	usingList := len(c.Chains) > 0
	seen := map[int64]bool{}
	for i, ch := range c.ChainList() {
		label := "chain"
		if usingList {
			label = fmt.Sprintf("chains[%d]", i)
		}
		if err := validateChain(ch, label); err != nil {
			return err
		}
		if seen[ch.NetworkId] {
			return fmt.Errorf("duplicate network_id %d in config.toml - each chain must be distinct", ch.NetworkId)
		}
		seen[ch.NetworkId] = true
	}

	if c.Database.Dialect == "sqlite" {
		if c.Database.Storage == "" {
			return errors.New("sqlite selected as dialect but database.storage not set in config.toml")
		}
	}

	if c.Database.Dialect == "postgres" {
		if c.Database.Host == "" {
			return errors.New("postgres selected as dialect but database.host not set in config.toml")
		}
		if c.Database.Port == 0 {
			return errors.New("postgres selected as dialect but database.port not set in config.toml")
		}
		if c.Database.Database == "" {
			return errors.New("postgres selected as dialect but database.database not set in config.toml")
		}
		if c.Database.User == "" {
			return errors.New("postgres selected as dialect but database.user not set in config.toml")
		}
		if c.Database.Password == "" {
			return errors.New("postgres selected as dialect but database.password not set in config.toml")
		}
	}

	if c.Jobs.CheckDuration == 0 {
		return errors.New("jobs.check_duration not set in config.toml")
	}
	if c.Jobs.WaitConfirmations == 0 {
		return errors.New("jobs.wait_confirmations not set in config.toml")
	}

	if c.Keystore.Account == "" {
		return errors.New("keystorage.account not set in config.toml")
	}
	if c.Keystore.File == "" {
		return errors.New("keystorage.file not set in config.toml")
	}

	if _, err := os.Stat(c.Keystore.File); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf(`cannot find %s - check keystorage.file in config.toml`, c.Keystore.File)
	}

	return nil
}
