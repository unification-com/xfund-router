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

	ethSushiswapHostedSubgraphUrl = "https://api.thegraph.com/subgraphs/name/sushiswap/exchange"
	ethSushiswapSubgraphId        = "6NUtT5mGjZ1tSshKLf5Q3uEEJtjBZJo1TpL5MXsUBqrT"

	ethShibaswapHostedSubgraphUrl = "https://api.thegraph.com/subgraphs/name/shibaswaparmy/exchange"
	ethShibaswapSubgraphId        = "61LXXvGA1KXkJZbCceYqw9APcwTGefK5MytwnVsdAQpw"

	xdaiHoneyswapHostedSubgraphUrl = "https://api.thegraph.com/subgraphs/name/1hive/honeyswap-xdai"
	xdaiHoneyswapSubgraphId        = "HTxWvPGcZ5oqWLYEVtWnVJDfnai2Ud1WaABiAR72JaSJ"
)

// newUniswapModule builds a FamilyModule for a Uniswap-style source from its metadata and
// the relevant config thresholds. params selects the schema variant (uniswap.V2 / uniswap.V3),
// so this one helper serves every member of both families and is reused by the manifest-driven
// builder at step 5.
func newUniswapModule(chain, dex, subgraphId, hostedUrl, apiKey string, dexCfg config.DexConfig, params uniswap.Params) FamilyModule {
	return NewFamilyModule(FamilyModuleSpec{
		Chain:        chain,
		Dex:          dex,
		SubgraphUrl:  graphGatewayUrl(apiKey, subgraphId, hostedUrl),
		MinLiquidity: dexCfg.MinReserveUsd,
		MinTxCount:   dexCfg.MinTxCount,
		Family:       uniswap.New(params),
	})
}

// NewUniswapV2Module builds the eth/uniswap_v2 source as a FamilyModule (UniV2Like family).
func NewUniswapV2Module(cfg *config.Config) FamilyModule {
	return newUniswapModule(types.ChainEth, "uniswap_v2", ethUniswapV2SubgraphId, ethUniswapV2HostedSubgraphUrl,
		cfg.ApiKeys.GraphNetwork, cfg.Dexs.EthUniswapV2, uniswap.V2)
}

// NewSushiswapModule builds the eth/sushiswap source as a FamilyModule (UniV2Like family).
func NewSushiswapModule(cfg *config.Config) FamilyModule {
	return newUniswapModule(types.ChainEth, "sushiswap", ethSushiswapSubgraphId, ethSushiswapHostedSubgraphUrl,
		cfg.ApiKeys.GraphNetwork, cfg.Dexs.EthSushiswap, uniswap.V2)
}

// NewShibaswapModule builds the eth/shibaswap source as a FamilyModule (UniV2Like family).
func NewShibaswapModule(cfg *config.Config) FamilyModule {
	return newUniswapModule(types.ChainEth, "shibaswap", ethShibaswapSubgraphId, ethShibaswapHostedSubgraphUrl,
		cfg.ApiKeys.GraphNetwork, cfg.Dexs.EthShibaswap, uniswap.V2)
}

// NewHoneyswapModule builds the xdai/honeyswap source as a FamilyModule (UniV2Like family).
func NewHoneyswapModule(cfg *config.Config) FamilyModule {
	return newUniswapModule(types.ChainXdai, "honeyswap", xdaiHoneyswapSubgraphId, xdaiHoneyswapHostedSubgraphUrl,
		cfg.ApiKeys.GraphNetwork, cfg.Dexs.XdaiHoneyswap, uniswap.V2)
}
