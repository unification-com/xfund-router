package dex

import (
	"context"
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
	MinLiquidity() uint64
	MinTxCount() uint64
	GeneratePairsQuery(skip uint64) ([]byte, error)
	ProcessPairsQueryResult(result []byte) ([]types.DexPair, bool, error)
	GenerateDexPricesQuery(pairContractAddress string, minutes, currentBlock, blocksPerMin uint64) ([]byte, error)
	ProcessDexPricesResult(base, target string, minutes uint64, result []byte) ([]float64, error)
}

type Manager struct {
	ctx        context.Context
	db         *database.DB
	httpClient *http.Client

	chains  map[string]*chains.ChainDef
	modules map[string]Module
}

func NewDexManager(ctx context.Context, db *database.DB, modules ...Module) *Manager {
	moduleMap := make(map[string]Module)
	chainMap := make(map[string]*chains.ChainDef)

	supportedChains := []string{types.ChainEth, types.ChainPolygon, types.ChainBsc, types.ChainXdai}

	for _, module := range modules {
		moduleMap[module.Name()] = module
	}

	for _, c := range supportedChains {
		ch, err := chains.GetChain(c)
		if err != nil {
			panic(err)
		}
		chainMap[c] = ch
	}

	return &Manager{
		ctx: ctx,
		db:  db,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},

		chains:  chainMap,
		modules: moduleMap,
	}
}
