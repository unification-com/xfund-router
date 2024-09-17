package config

import (
	"errors"
	"fmt"
	"github.com/spf13/viper"
	oooapidextypes "go-ooo/ooo_api/dex/types"
	"os"
)

type JobsConfig struct {
	OooApiUrl         string `mapstructure:"ooo_api_url"`
	CheckDuration     uint64 `mapstructure:"check_duration"`
	WaitConfirmations uint64 `mapstructure:"wait_confirmations"`
}

type ServeConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
}

type KeystoreConfig struct {
	File    string `mapstructure:"file"`
	Account string `mapstructure:"account"`
}

type ChainConfig struct {
	GasLimit        uint64 `mapstructure:"gas_limit"`
	MaxGasPrice     int64  `mapstructure:"max_gas_price"`
	ContractAddress string `mapstructure:"contract_address"`
	EthHttpHost     string `mapstructure:"eth_http_host"`
	EthWsHost       string `mapstructure:"eth_ws_host"`
	NetworkId       int64  `mapstructure:"network_id"`
	FirstBlock      uint64 `mapstructure:"first_block"`
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

type Config struct {
	Jobs       JobsConfig       `mapstructure:"jobs"`
	Serve      ServeConfig      `mapstructure:"serve"`
	Keystore   KeystoreConfig   `mapstructure:"keystorage"`
	Chain      ChainConfig      `mapstructure:"chain"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
	Log        LogConfig        `mapstructure:"log"`
	Subchain   SubchainConfig   `mapstructure:"subchain"`
	ApiKeys    ApiKeysConfig    `mapstructure:"api_keys"`
	Dexs       DexList          `mapstructure:"dexs"`
}

// DefaultConfig returns server's default configuration.
func DefaultConfig() *Config {
	return &Config{
		Jobs: JobsConfig{
			OooApiUrl:         "https://crypto.finchains.io/api",
			CheckDuration:     5,
			WaitConfirmations: 1,
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
			GasLimit:        500000,
			MaxGasPrice:     150,
			ContractAddress: "",
			EthHttpHost:     "",
			EthWsHost:       "",
			NetworkId:       0,
			FirstBlock:      0,
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
			PolygonHttpRpc:   "https://polygon-rpc.com",
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
	}
}

func (c *Config) InitForNet(network string) {
	switch network {
	case "sepolia":
		c.InitForSepolia()
		break
	case "goerli":
		c.InitForGoerli()
		break
	case "mainnet":
		c.InitForMainnet()
		break
	case "polygon":
		c.InitForPolygon()
		break
	case "dev":
		c.InitForDevNet()
		break
	case "shibarium":
		c.InitForShibarium()
		break
	case "puppynet":
		c.InitForShibariumPuppynet()
		break
	default:
		c.InitForDevNet()
		break
	}
}

func (c *Config) InitForDevNet() {
	c.Chain.ContractAddress = "0x5b1869D9A4C187F2EAa108f3062412ecf0526b24"
	c.Chain.EthHttpHost = "http://127.0.0.1:8545"
	c.Chain.EthWsHost = "ws://127.0.0.1:8545"
	c.Chain.NetworkId = 696969
	c.Chain.FirstBlock = 1
}

func (c *Config) InitForSepolia() {
	c.Chain.ContractAddress = "0xf6b5d6eafE402d22609e685DE3394c8b359CaD31"
	c.Chain.EthHttpHost = ""
	c.Chain.EthWsHost = ""
	c.Chain.NetworkId = 11155111
	c.Chain.FirstBlock = 3647468
}

// InitForGoerli will be deprecated and removed soon
func (c *Config) InitForGoerli() {
	c.Chain.ContractAddress = "0xf6b5d6eafE402d22609e685DE3394c8b359CaD31"
	c.Chain.EthHttpHost = ""
	c.Chain.EthWsHost = ""
	c.Chain.NetworkId = 5
	c.Chain.FirstBlock = 7345730
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

// GetConfig returns a fully parsed Config object.
func GetConfig(v *viper.Viper) Config {

	return Config{}
}

func (c Config) ValidateBasic() error {

	if c.Chain.ContractAddress == "" {
		return errors.New("chain.contract_address not set in config.toml")
	}

	if c.Chain.EthWsHost == "" {
		return errors.New("chain.eth_ws_host not set in config.toml")
	}

	if c.Chain.EthHttpHost == "" {
		return errors.New("chain.eth_http_host not set in config.toml")
	}

	if c.Chain.NetworkId == 0 {
		return errors.New("chain.network_id not set in config.toml")
	}

	if c.Chain.GasLimit == 0 {
		return errors.New("chain.gas_limit not set in config.toml")
	}

	if c.Chain.MaxGasPrice == 0 {
		return errors.New("chain.max_gas_price not set in config.toml")
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
	if c.Jobs.OooApiUrl == "" {
		return errors.New("jobs.ooo_api_url not set in config.toml")
	}

	if c.Keystore.Account == "" {
		return errors.New("keystorage.account not set in config.toml")
	}
	if c.Keystore.File == "" {
		return errors.New("keystorage.file not set in config.toml")
	}

	if _, err := os.Stat(c.Keystore.File); errors.Is(err, os.ErrNotExist) {
		return errors.New(fmt.Sprintf(`cannot find %s - check keystorage.file in config.toml`, c.Keystore.File))
	}

	return nil
}
