package dex

import (
	"context"
	"go-ooo/config"
	"go-ooo/logger"
	"net/http"
	"time"

	"go-ooo/database"
	"go-ooo/ooo_api/dex/chains"
	"go-ooo/ooo_api/dex/types"
)

type Module interface {
	Name() string
	SubgraphUrl() string
	Chain() string
	Dex() string
	MinLiquidity() uint64
	MinTxCount() uint64
	GeneratePairsQuery(contractAddresses string) ([]byte, error)
	ProcessPairsQueryResult(result []byte) ([]types.DexPair, error)
	GenerateDexPricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, uint64, error)
	ProcessDexPricesResult(base, target string, numQueries uint64, result []byte) ([]float64, error)
}

type Manager struct {
	ctx        context.Context
	cfg        *config.Config
	db         *database.DB
	httpClient *http.Client

	chains  map[string]*chains.ChainDef
	modules map[string]Module
}

func NewDexManager(ctx context.Context, cfg *config.Config, db *database.DB, modules ...Module) *Manager {
	moduleMap := make(map[string]Module)
	chainMap := make(map[string]*chains.ChainDef)

	supportedChains := []string{types.ChainEth, types.ChainPolygon, types.ChainBsc, types.ChainXdai, types.ChainShibarium}

	for _, module := range modules {
		moduleMap[module.Name()] = module
	}

	for _, c := range supportedChains {
		ch, err := chains.GetChain(c, cfg.Subchain)
		if err != nil {
			panic(err)
		}
		logger.Debug("dex", "NewDexManager", "GetChain", "got config for chain block number queries", logger.Fields{
			"chain_name":     ch.ChainName,
			"chain_id":       ch.ChainId,
			"chain_short":    ch.ChainShort,
			"blocks_per_min": ch.BlocksPerMin,
			"rpc":            ch.RpcUrl,
		})
		chainMap[c] = ch
	}

	return &Manager{
		ctx: ctx,
		cfg: cfg,
		db:  db,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},

		chains:  chainMap,
		modules: moduleMap,
	}
}
