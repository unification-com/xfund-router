package dex

import (
	"go-ooo/config"
	"go-ooo/ooo_api/dex/families/uniswap"
	"go-ooo/ooo_api/dex/types"
)

// seed.go holds the interim per-DEX construction of FamilyModules for sources that have
// been migrated off their hand-written modules/<dex>/ package. The literal metadata
// (subgraph id, hosted URL, chain, dex) lives here until the manifest-driven builder lands
// (step 5), at which point this file is replaced by construction from the dex-pair-verify
// v3 manifest. One entry is added here as each DEX is migrated, and its modules/<dex>/ dir
// deleted in the same commit.

const (
	ethUniswapV2HostedSubgraphUrl = "https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v2"
	ethUniswapV2SubgraphId        = "EYCKATKGBKLWvSfwvBjzfCBmGwYNdVkduYXVivCsLRFu"
)

// NewUniswapV2Module builds the eth/uniswap_v2 source as a FamilyModule (UniV2Like family).
func NewUniswapV2Module(cfg *config.Config) FamilyModule {
	return NewFamilyModule(FamilyModuleSpec{
		Chain:        types.ChainEth,
		Dex:          "uniswap_v2",
		SubgraphUrl:  graphGatewayUrl(cfg.ApiKeys.GraphNetwork, ethUniswapV2SubgraphId, ethUniswapV2HostedSubgraphUrl),
		MinLiquidity: cfg.Dexs.EthUniswapV2.MinReserveUsd,
		MinTxCount:   cfg.Dexs.EthUniswapV2.MinTxCount,
		Family:       uniswap.New(uniswap.V2),
	})
}
