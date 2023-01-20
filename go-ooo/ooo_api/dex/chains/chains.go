package chains

import (
	"errors"
	"go-ooo/config"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/viper"
)

func GetChain(name string) (*ChainDef, error) {
	chainId := ""
	chainName := ""
	blocksPerMin := 5
	rpcUrl := ""

	switch name {
	case "eth":
		chainId = "1"
		chainName = "Ethereum Mainnet"
		blocksPerMin = 5
		rpcUrl = viper.GetString(config.SubChainEthHttpRpc)
	case "polygon":
		chainId = "137"
		chainName = "Polygon Mainnet"
		blocksPerMin = 20
		rpcUrl = viper.GetString(config.SubChainPolygonHttpRpc)
	case "bsc":
		chainId = "56"
		chainName = "Binance Chain Mainnet"
		blocksPerMin = 20
		rpcUrl = viper.GetString(config.SubChainBcsHttpRpc)
	case "xdai":
		chainId = "100"
		chainName = "Gnosis Chain Mainnet"
		blocksPerMin = 12
		rpcUrl = viper.GetString(config.SubChainXdaiHttpRpc)
	case "gnosis":
		chainId = "100"
		chainName = "Gnosis Chain Mainnet"
		blocksPerMin = 12
		rpcUrl = viper.GetString(config.SubChainXdaiHttpRpc)
	case "fantom":
		chainId = "250"
		chainName = "Fantom Mainnet"
		blocksPerMin = 30
		rpcUrl = viper.GetString(config.SubChainFantomHttpRpc)
	default:
		return &ChainDef{}, errors.New("not supported")
	}

	ethClient, err := ethclient.Dial(rpcUrl)

	if err != nil {
		return &ChainDef{}, err
	}

	return &ChainDef{
		ChainShort:   name,
		ChainId:      chainId,
		ChainName:    chainName,
		BlocksPerMin: blocksPerMin,
		RpcUrl:       rpcUrl,
		EthClient:    ethClient,
	}, nil
}
