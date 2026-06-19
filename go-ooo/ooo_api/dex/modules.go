package dex

import (
	"context"
	"go-ooo/config"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go-ooo/database"
	"go-ooo/ooo_api/dex/chains"
	"go-ooo/ooo_api/dex/types"
	"go-ooo/ooo_api/export"
)

// PriceSource is the price-source seam (#128): the Manager treats every priceable source uniformly
// through it, regardless of transport. A source OWNS its own transport - FamilyModule (the EVM
// implementation) speaks GraphQL to a decentralised-network subgraph; a future REST source (e.g.
// Osmosis SQS) speaks HTTP/JSON - so the orchestrator (GetPricesFromDexModules / getPrices) and the
// aggregator stay transport-blind. FetchPoolPrices collapses the old generate-query -> run-query ->
// parse triplet into a single transport-owning call; FetchPairsMetadata does the same for the
// metadata refresh. The transport-specific building blocks (subgraph URL, GraphQL query/parse) are
// no longer on the interface - they are an internal detail of the subgraph implementation, so a
// non-subgraph source need not provide them.
type PriceSource interface {
	Name() string
	Chain() string
	Dex() string
	MinLiquidity() uint64
	MinTxCount() uint64
	// FetchPoolPrices returns this source's per-pool prices for base/target. dexInfo carries the
	// per-chain block context the query needs; the source owns query + fetch + parse internally.
	FetchPoolPrices(base, target string, dexInfo DexInfo, minutes uint64) ([]types.PoolPrices, error)
	// FetchPairsMetadata returns refreshed reserve/tx metadata for the given pool contract addresses.
	FetchPairsMetadata(contractAddresses string) ([]types.DexPair, error)
}

// Module is retained as an alias for PriceSource so the Manager's existing "module" vocabulary
// (modules map, SetModules, snapshotModules, BuildModulesFromManifest) keeps reading naturally; new
// code (a Cosmos REST source, the architecture doc) uses the transport-neutral PriceSource name.
type Module = PriceSource

type Manager struct {
	ctx        context.Context
	cfg        *config.Config
	db         *database.DB
	httpClient *http.Client

	// syncing guards SyncDexSources so the startup refresh, the periodic refresh and an admin-
	// triggered sync can't overlap - a second caller skips. Distinct from mu, which guards only the
	// module/chain pointer swap.
	syncing atomic.Bool

	chains map[string]*chains.ChainDef

	// modules is the priceable source set. It is replaced wholesale by SetModules when the
	// dex-pair-verify manifest refreshes (#127), so mu guards it against the concurrent price +
	// pair-refresh loops that read it.
	mu      sync.RWMutex
	modules map[string]Module
	// aliases is the asset-class alias index (S7), rebuilt from the manifest's AliasGroups/AliasPairs
	// by ApplyManifest and read by the price loop to recognise + expand an alias query. Guarded by mu
	// alongside modules/chains so the loop always sees a consistent (modules, chains, aliases) set.
	aliases *aliasIndex
	// exportAuth authenticates the dex-pair-verify export pulls (T8). Defaults to the static
	// EXPORT_API_TOKEN from config (break-glass / simple setups); the service swaps in a wallet
	// authenticator (SetExportAuth) when no static token is configured + a provider key is available.
	// Set once at startup before any sync, so it needs no lock.
	exportAuth export.Authenticator
}

// NewDexManager builds an empty manager. The priceable module set + per-chain config are populated
// from the dex-pair-verify manifest - restored from the database at startup, then refreshed from the
// API - via ApplyManifest, rather than a hard-coded seed list. A fresh database with no persisted
// manifest starts with nothing to price until the first successful sync.
func NewDexManager(ctx context.Context, cfg *config.Config, db *database.DB) *Manager {
	return &Manager{
		ctx: ctx,
		cfg: cfg,
		db:  db,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},

		chains:  make(map[string]*chains.ChainDef),
		modules: make(map[string]Module),
		// Default to the static token from config; the service may swap in wallet-auth before the first sync.
		exportAuth: export.StaticToken(cfg.Jobs.DexExport.ApiToken),
	}
}

// SetExportAuth swaps the authenticator used for dex-pair-verify export pulls (T8). Called once at
// startup by the service to install provider wallet-auth in place of the static-token default, before
// any sync runs.
func (dm *Manager) SetExportAuth(a export.Authenticator) {
	if a != nil {
		dm.exportAuth = a
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
