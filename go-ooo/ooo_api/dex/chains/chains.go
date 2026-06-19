package chains

import (
	"errors"

	"go-ooo/config"

	"github.com/ethereum/go-ethereum/ethclient"
)

// DefaultBlocksPerMin is the assumed block cadence for a chain with no specific value configured.
const DefaultBlocksPerMin = 5

// ChainMeta is the static, non-dialled description of a chain: its ids, block cadence and RPC URL.
// Splitting this out from the dialled ChainDef lets the manifest-driven builder apply per-source RPC
// / block overrides and dial exactly once (rather than dialling the configured RPC then re-dialling
// a manifest-supplied one).
type ChainMeta struct {
	ChainShort   string
	ChainName    string
	ChainId      string
	BlocksPerMin int
	RpcUrl       string
}

// LookupChain returns the static metadata for a known chain, taking its RPC URL from cfg. ok is
// false for an unknown chain - the manifest-driven builder can still construct one from a
// manifest-supplied RPC, so a new chain needs no code change here.
func LookupChain(name string, cfg config.SubchainConfig) (ChainMeta, bool) {
	chainId := ""
	chainName := ""
	blocksPerMin := DefaultBlocksPerMin
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
		return ChainMeta{}, false
	}

	return ChainMeta{
		ChainShort:   name,
		ChainName:    chainName,
		ChainId:      chainId,
		BlocksPerMin: blocksPerMin,
		RpcUrl:       rpcUrl,
	}, true
}

// DialChain dials meta.RpcUrl and returns a ready ChainDef carrying the dialled client.
func DialChain(meta ChainMeta) (*ChainDef, error) {
	ethClient, err := ethclient.Dial(meta.RpcUrl)
	if err != nil {
		return &ChainDef{}, err
	}

	return &ChainDef{
		ChainShort:   meta.ChainShort,
		ChainId:      meta.ChainId,
		ChainName:    meta.ChainName,
		BlocksPerMin: meta.BlocksPerMin,
		RpcUrl:       meta.RpcUrl,
		EthClient:    ethClient,
	}, nil
}

// GetChain returns a dialled ChainDef for a known chain - the pre-manifest construction path.
func GetChain(name string, cfg config.SubchainConfig) (*ChainDef, error) {
	meta, ok := LookupChain(name, cfg)
	if !ok {
		return &ChainDef{}, errors.New("not supported")
	}
	return DialChain(meta)
}
