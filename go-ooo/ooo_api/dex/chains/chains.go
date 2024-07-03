package chains

import (
	"errors"

	"go-ooo/config"

	"github.com/ethereum/go-ethereum/ethclient"
)

func GetChain(name string, cfg config.SubchainConfig) (*ChainDef, error) {
	chainId := ""
	chainName := ""
	blocksPerMin := 5
	rpcUrl := ""

	switch name {
	case "eth":
		chainId = "1"
		chainName = "Ethereum Mainnet"
		blocksPerMin = 5
		rpcUrl = cfg.EthHttpRpc
	case "polygon_pos":
		chainId = "137"
		chainName = "Polygon Mainnet"
		blocksPerMin = 12
		rpcUrl = cfg.PolygonHttpRpc
	case "bsc":
		chainId = "56"
		chainName = "Binance Chain Mainnet"
		blocksPerMin = 20
		rpcUrl = cfg.BcsHttpRpc
	case "xdai":
		chainId = "100"
		chainName = "Gnosis Chain Mainnet"
		blocksPerMin = 12
		rpcUrl = cfg.XdaiHttpRpc
	case "gnosis":
		chainId = "100"
		chainName = "Gnosis Chain Mainnet"
		blocksPerMin = 12
		rpcUrl = cfg.XdaiHttpRpc
	case "fantom":
		chainId = "250"
		chainName = "Fantom Mainnet"
		blocksPerMin = 30
		rpcUrl = cfg.FantomHttpRpc
	case "shibarium":
		chainId = "109"
		chainName = "Shibarium Mainnet"
		blocksPerMin = 12
		rpcUrl = cfg.ShibariumHttpRpc
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
