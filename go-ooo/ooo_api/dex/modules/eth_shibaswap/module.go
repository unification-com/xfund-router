package eth_shibaswap

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

// app module Basics object
type DexModule struct {
	ctx                context.Context
	graphNetworkApiKey string
	minLiquidity       uint64
	minTxCount         uint64
}

func NewDexModule(ctx context.Context, cfg *config.Config) DexModule {
	return DexModule{
		ctx:                ctx,
		graphNetworkApiKey: cfg.ApiKeys.GraphNetwork,
		minLiquidity:       cfg.Dexs.EthShibaswap.MinReserveUsd,
		minTxCount:         cfg.Dexs.EthShibaswap.MinTxCount,
	}
}

func (d DexModule) Name() string {
	return ModuleName
}

func (d DexModule) SubgraphUrl() string {
	if d.graphNetworkApiKey == "" {
		return HostedSubgraphUrl
	}
	return fmt.Sprintf(`https://gateway-arbitrum.network.thegraph.com/api/%s/subgraphs/id/%s`, d.graphNetworkApiKey, GraphNetworkSubgraphId)
}

func (d DexModule) Chain() string {
	return Chain
}

func (d DexModule) Dex() string {
	return Dex
}

func (d DexModule) MinLiquidity() uint64 {
	return d.minLiquidity
}

func (d DexModule) MinTxCount() uint64 {
	return d.minTxCount
}

func (d DexModule) GeneratePairsQuery(contractAddresses string) ([]byte, error) {
	return d.generatePairsListQuery(contractAddresses)
}

func (d DexModule) ProcessPairsQueryResult(result []byte) ([]types.DexPair, error) {
	return d.processPairs(result)
}

func (d DexModule) GenerateDexPricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, uint64, error) {
	return d.generatePricesQuery(pairContractAddress, minutes, currentBlock, blocksPerMin)
}

func (d DexModule) ProcessDexPricesResult(base, target string, numQueries uint64, result []byte) ([]float64, error) {
	return d.processPrices(base, target, numQueries, result)
}
