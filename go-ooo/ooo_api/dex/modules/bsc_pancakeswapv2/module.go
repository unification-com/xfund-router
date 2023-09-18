package bsc_pancakeswapv2

import (
	"context"
	"fmt"
	"go-ooo/config"
	"go-ooo/ooo_api/dex"
	"go-ooo/ooo_api/dex/types"
)

var (
	_ dex.Module = DexModule{}
)

type DexModule struct {
	ctx            context.Context
	nodeRealApiKey string
}

func NewDexModule(ctx context.Context, cfg *config.Config) DexModule {
	return DexModule{
		ctx:            ctx,
		nodeRealApiKey: cfg.ApiKeys.Nodereal,
	}
}

func (d DexModule) Name() string {
	return ModuleName
}

func (d DexModule) SubgraphUrl() string {
	return fmt.Sprintf(`https://open-platform.nodereal.io/%s/pancakeswap-free/graphql`, d.nodeRealApiKey)
}

func (d DexModule) Chain() string {
	return Chain
}

func (d DexModule) MinLiquidity() uint64 {
	return MinLiquidity
}

func (d DexModule) MinTxCount() uint64 {
	return MinTxCount
}

func (d DexModule) GeneratePairsQuery(skip uint64) ([]byte, error) {
	return d.generatePairsListQuery(skip)
}

func (d DexModule) ProcessPairsQueryResult(result []byte) ([]types.DexPair, bool, error) {
	return d.processPairs(result)
}

func (d DexModule) GenerateDexPricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, error) {
	return d.generatePricesQuery(pairContractAddress, minutes, currentBlock, blocksPerMin)
}

func (d DexModule) ProcessDexPricesResult(base, target string, minutes uint64, result []byte) ([]float64, error) {
	return d.processPrices(base, target, minutes, result)
}
