package dex

import (
	"context"
	"go-ooo/config"
	"go-ooo/logger"
	"net/http"
	"sync"
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
	ProcessDexPricesResult(base, target string, numQueries uint64, result []byte) ([]types.PoolPrices, error)
}

type Manager struct {
	ctx        context.Context
	cfg        *config.Config
	db         *database.DB
	httpClient *http.Client

	chains map[string]*chains.ChainDef

	// modules is the priceable source set. It is replaced wholesale by SetModules when the
	// dex-pair-verify manifest refreshes (#127), so mu guards it against the concurrent price +
	// pair-refresh loops that read it.
	mu      sync.RWMutex
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

// SetModules replaces the priceable module set wholesale. The manifest-driven export refresh
// (#127) calls it after rebuilding modules from the v3 manifest. Concurrent price/pair-refresh
// loops snapshot the set under the read lock, so they always see the complete old or complete new
// set, never a partial one.
func (dm *Manager) SetModules(modules []Module) {
	moduleMap := make(map[string]Module, len(modules))
	for _, m := range modules {
		moduleMap[m.Name()] = m
	}
	dm.mu.Lock()
	dm.modules = moduleMap
	dm.mu.Unlock()
}

// snapshotModules returns the current modules under a read lock. Callers iterate the returned
// slice (doing slow, networked per-module work) without holding the lock, so a SetModules can
// swap the set in without blocking on in-flight queries.
func (dm *Manager) snapshotModules() []Module {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	mods := make([]Module, 0, len(dm.modules))
	for _, m := range dm.modules {
		mods = append(mods, m)
	}
	return mods
}
