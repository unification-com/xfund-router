package chains

import "github.com/ethereum/go-ethereum/ethclient"

type ChainDef struct {
	ChainShort   string
	ChainName    string
	ChainId      string
	BlocksPerMin int
	RpcUrl       string
	EthClient    *ethclient.Client
}
