package dex

import (
	"encoding/json"
	"fmt"

	"go-ooo/database/models"
	"go-ooo/logger"
	"go-ooo/ooo_api/dex/chains"
	"go-ooo/ooo_api/export"
)

// sources.go bridges the dex-pair-verify v3 manifest and go-ooo's persisted source catalogue
// (the supported_sources table). The manifest is the source of truth for which DEXs go-ooo can
// price; persisting it means the priceable module set + per-chain config survive a restart and can
// be rebuilt without reaching the API - the resilience floor that replaces the hard-coded seed.

// sourceToModel maps a manifest source to its persisted row, marshalling the endpoint list to JSON.
func sourceToModel(src export.ManifestSource) (models.SupportedSource, error) {
	endpoints, err := json.Marshal(src.Endpoints)
	if err != nil {
		return models.SupportedSource{}, err
	}
	return models.SupportedSource{
		Chain:                src.Chain,
		Dex:                  src.Dex,
		SubgraphSchemaFamily: src.SubgraphSchemaFamily,
		SourceType:           src.SourceType,
		Endpoints:            string(endpoints),
		FactoryAddress:       src.FactoryAddress,
		RpcUrl:               src.RPCURL,
		BlocksPerMin:         src.BlocksPerMin,
		PairCount:            src.PairCount,
		ExportUrl:            src.ExportURL,
		MinLiquidityUsd:      src.MinLiquidityUsd,
		SourceUpdatedAt:      src.LastUpdated,
		SourceVerifiedAt:     src.LastVerifiedAt,
	}, nil
}

// modelToSource maps a persisted row back to a manifest source, unmarshalling the endpoint JSON.
func modelToSource(row models.SupportedSource) (export.ManifestSource, error) {
	var endpoints []export.ManifestEndpoint
	if row.Endpoints != "" {
		if err := json.Unmarshal([]byte(row.Endpoints), &endpoints); err != nil {
			return export.ManifestSource{}, err
		}
	}
	return export.ManifestSource{
		Chain:                row.Chain,
		Dex:                  row.Dex,
		Endpoints:            endpoints,
		SubgraphSchemaFamily: row.SubgraphSchemaFamily,
		SourceType:           row.SourceType,
		FactoryAddress:       row.FactoryAddress,
		RPCURL:               row.RpcUrl,
		BlocksPerMin:         row.BlocksPerMin,
		PairCount:            row.PairCount,
		ExportURL:            row.ExportUrl,
		MinLiquidityUsd:      row.MinLiquidityUsd,
		LastUpdated:          row.SourceUpdatedAt,
		LastVerifiedAt:       row.SourceVerifiedAt,
	}, nil
}

// SyncDexSources refreshes the priceable catalogue: pull the latest manifest + pair feeds from the
// dex-pair-verify API (a no-op when no API base URL is configured, or on failure - the persisted
// catalogue then carries the oracle), then refresh each active pair's live reserve/tx metadata from
// the subgraphs. This is the periodic + startup refresh entry point.
func (dm *Manager) SyncDexSources() {
	// One sync at a time across all callers (startup, the periodic ticker, the admin trigger): a
	// second concurrent caller skips rather than piling up duplicate manifest fetches + subgraph
	// queries.
	if !dm.syncing.CompareAndSwap(false, true) {
		logger.Info("dex", "SyncDexSources", "", "skipping source sync - a previous run is still in progress")
		return
	}
	defer dm.syncing.Store(false)

	if err := dm.syncManifestAndFeeds(); err != nil {
		// A failed sync (e.g. the API is down) is not fatal: keep pricing from the persisted
		// manifest + pairs and just refresh their live metadata below.
		logger.ErrorWithFields("dex", "SyncDexSources", "sync manifest and feeds", err.Error(), logger.Fields{})
	}
	dm.UpdateAllPairsMetaDataFromDexs()
}

// syncManifestAndFeeds fetches the v3 manifest from the configured dex-pair-verify API, persists +
// applies it (rebuilding the module set + chain config), then pulls each active source's verified-
// pair feed into dex_pairs. An empty base URL skips the sync entirely (the persisted catalogue is
// authoritative). Per-source feed failures are logged and skipped so one bad source can't abort the
// whole refresh.
func (dm *Manager) syncManifestAndFeeds() error {
	cfg := dm.cfg.Jobs.DexExport
	if cfg.BaseUrl == "" {
		logger.Info("dex", "syncManifestAndFeeds", "",
			"no dex-pair-verify API base URL configured - skipping manifest sync (pricing from the persisted catalogue)")
		return nil
	}

	client := export.NewClient(cfg.BaseUrl, cfg.ApiToken, dm.httpClient)

	manifest, err := client.FetchManifest(dm.ctx)
	if err != nil {
		return fmt.Errorf("fetch manifest: %w", err)
	}

	if _, err := dm.PersistManifest(manifest); err != nil {
		logger.ErrorWithFields("dex", "syncManifestAndFeeds", "persist manifest", err.Error(), logger.Fields{})
	}
	dm.ApplyManifest(manifest)

	// Pull each active (priceable) source's verified-pair feed into dex_pairs. Iterating the built
	// modules - rather than every manifest source - means we only fetch feeds for sources whose
	// schema family go-ooo can actually price.
	floorsUpdated := false
	for _, mod := range dm.snapshotModules() {
		feed, changed, err := client.FetchPairFeed(dm.ctx, mod.Chain(), mod.Dex(), 0)
		if err != nil {
			logger.ErrorWithFields("dex", "syncManifestAndFeeds", "fetch pair feed", err.Error(),
				logger.Fields{"chain": mod.Chain(), "dex": mod.Dex()})
			continue
		}
		if !changed || feed == nil {
			continue
		}
		dm.processPairFeed(*feed)

		// Record the source's curation floor (XR2) from the feed. The fetched manifest doesn't carry
		// it, so this is the only place it becomes known; the re-apply below folds it into the module.
		if feed.MinLiquidityUsd > 0 {
			if err := dm.db.UpdateSourceMinLiquidity(mod.Chain(), mod.Dex(), feed.MinLiquidityUsd); err != nil {
				logger.ErrorWithFields("dex", "syncManifestAndFeeds", "update source floor", err.Error(),
					logger.Fields{"chain": mod.Chain(), "dex": mod.Dex()})
			} else {
				floorsUpdated = true
			}
		}
	}

	// Re-apply from the database so the modules pick up the per-source curation floors just written
	// from the feeds (XR2) - the fetched manifest doesn't carry them. Cheap: the chain clients are
	// reused, only the module floors change.
	if floorsUpdated {
		if restored, err := dm.LoadManifestFromDB(); err != nil {
			logger.ErrorWithFields("dex", "syncManifestAndFeeds", "reload for curation floors", err.Error(), logger.Fields{})
		} else {
			dm.ApplyManifest(restored)
		}
	}

	return nil
}

// PersistManifest writes a freshly-fetched manifest to the DB: upsert every source, then soft-delete
// any source that has dropped out (guarded against an empty/failed manifest, which must not wipe the
// catalogue). Returns the number of sources persisted.
func (dm *Manager) PersistManifest(m *export.Manifest) (int, error) {
	keep := make([]string, 0, len(m.SupportedSources))
	persisted := 0

	for _, src := range m.SupportedSources {
		row, err := sourceToModel(src)
		if err != nil {
			logger.ErrorWithFields("dex", "PersistManifest", "marshal source", err.Error(),
				logger.Fields{"chain": src.Chain, "dex": src.Dex})
			continue
		}
		if _, err := dm.db.UpsertSupportedSource(row); err != nil {
			logger.ErrorWithFields("dex", "PersistManifest", "upsert source", err.Error(),
				logger.Fields{"chain": src.Chain, "dex": src.Dex})
			continue
		}
		keep = append(keep, src.Chain+"|"+src.Dex)
		persisted++
	}

	if removed, err := dm.db.SoftDeleteSupportedSourcesNotIn(keep); err != nil {
		logger.ErrorWithFields("dex", "PersistManifest", "soft-delete dropped sources", err.Error(), logger.Fields{})
	} else if removed > 0 {
		logger.InfoWithFields("dex", "PersistManifest", "", "soft-deleted sources dropped from the manifest",
			logger.Fields{"removed": removed})
	}

	// Persist the asset-class alias payload (S7) too, so the alias index is restored on the next
	// startup without a live fetch - parity with the source catalogue's resilience floor.
	if err := dm.persistAliases(m); err != nil {
		logger.ErrorWithFields("dex", "PersistManifest", "persist aliases", err.Error(), logger.Fields{})
	}

	return persisted, nil
}

// persistAliases stores the manifest's alias payload (S7) as the singleton alias_configs row. A
// manifest with no AliasGroups is skipped - a DB-reconstructed manifest carries none and must not
// overwrite the persisted payload with nothing (mirrors the empty-feed/empty-manifest data guards).
func (dm *Manager) persistAliases(m *export.Manifest) error {
	if len(m.AliasGroups) == 0 {
		return nil
	}
	groups, err := json.Marshal(m.AliasGroups)
	if err != nil {
		return err
	}
	pairs, err := json.Marshal(m.AliasPairs)
	if err != nil {
		return err
	}
	return dm.db.UpsertAliasConfig(string(groups), string(pairs))
}

// loadAliasesInto populates a manifest's alias payload from the persisted singleton, so a DB-restored
// manifest carries the aliases just like the live API one - buildAliasIndex then has a single input
// path regardless of where the manifest came from. A missing row leaves the payload empty.
func (dm *Manager) loadAliasesInto(m *export.Manifest) error {
	groupsJSON, pairsJSON, found, err := dm.db.GetAliasConfig()
	if err != nil || !found {
		return err
	}
	if groupsJSON != "" {
		if err := json.Unmarshal([]byte(groupsJSON), &m.AliasGroups); err != nil {
			return err
		}
	}
	if pairsJSON != "" {
		if err := json.Unmarshal([]byte(pairsJSON), &m.AliasPairs); err != nil {
			return err
		}
	}
	return nil
}

// LoadManifestFromDB reconstructs the last-synced manifest from the persisted sources, so the module
// set + chain config can be rebuilt at startup (or whenever the API is unreachable) without a fetch.
func (dm *Manager) LoadManifestFromDB() (*export.Manifest, error) {
	rows, err := dm.db.GetSupportedSources()
	if err != nil {
		return nil, err
	}
	m := &export.Manifest{
		SchemaVersion:    export.ManifestSchemaVersion,
		SupportedSources: make([]export.ManifestSource, 0, len(rows)),
	}
	for _, row := range rows {
		src, err := modelToSource(row)
		if err != nil {
			logger.ErrorWithFields("dex", "LoadManifestFromDB", "unmarshal source", err.Error(),
				logger.Fields{"chain": row.Chain, "dex": row.Dex})
			continue
		}
		m.SupportedSources = append(m.SupportedSources, src)
	}

	// Restore the alias payload (S7) onto the reconstructed manifest so alias pricing is available
	// immediately on startup, before any live sync.
	if err := dm.loadAliasesInto(m); err != nil {
		logger.ErrorWithFields("dex", "LoadManifestFromDB", "load aliases", err.Error(), logger.Fields{})
	}

	return m, nil
}

// ApplyManifest rebuilds the priceable module set + per-chain config from a manifest and swaps both
// in atomically, so the concurrent price / pair-refresh loops always observe a consistent
// (modules, chains) pair - never new modules against the old chain set or vice versa. The dialling
// happens outside the lock; only the pointer swap is held under it.
func (dm *Manager) ApplyManifest(m *export.Manifest) {
	modules := BuildModulesFromManifest(dm.cfg, m)
	moduleMap := make(map[string]Module, len(modules))
	for _, mod := range modules {
		moduleMap[mod.Name()] = mod
	}

	dm.mu.RLock()
	existing := dm.chains
	dm.mu.RUnlock()
	newChains := dm.buildChains(existing, m)

	// Cosmos (rest-sqs) sources are built from the manifest too, but as REST PriceSources rather than
	// subgraph FamilyModules; their pairs + token denoms come from the dex-pair-verify feed (dex_pairs
	// / token_contracts), so go-ooo no longer curates a static Osmosis allow-list. buildChains has
	// already registered their non-EVM chain (nil EthClient) so the orchestrator skips the block lookup.
	for _, cs := range buildCosmosSources(m, dm.db) {
		moduleMap[cs.Name()] = cs
	}

	// Rebuild the asset-class alias index (S7) from the manifest. A manifest reconstructed from the
	// persisted source catalogue (LoadManifestFromDB - the startup restore + the post-feed floor
	// re-apply) carries no AliasGroups, so buildAliasIndex returns nil; in that case keep the prior
	// in-memory index rather than wiping it, since only the live API manifest carries the aliases.
	newAliases := buildAliasIndex(m)

	dm.mu.Lock()
	dm.modules = moduleMap
	dm.chains = newChains
	if newAliases != nil {
		dm.aliases = newAliases
	}
	dm.mu.Unlock()

	logger.InfoWithFields("dex", "ApplyManifest", "", "applied manifest", logger.Fields{
		"sources":      len(m.SupportedSources),
		"modules":      len(moduleMap),
		"chains":       len(newChains),
		"alias_groups": len(m.AliasGroups),
		"alias_pairs":  len(m.AliasPairs),
	})
}

// buildChains constructs the per-chain block/RPC config from the manifest's distinct chains. The
// manifest is authoritative where it supplies an rpcUrl / blocksPerMin; for a chain it does not map,
// known-chain defaults + the configured RPC fill the gap (so the existing chains keep working). A
// chain with no RPC from either source is skipped - it cannot be queried. An existing dialled client
// is reused when the resolved RPC is unchanged, to avoid re-dialling on every refresh.
func (dm *Manager) buildChains(existing map[string]*chains.ChainDef, m *export.Manifest) map[string]*chains.ChainDef {
	out := make(map[string]*chains.ChainDef)

	for _, src := range m.SupportedSources {
		if _, done := out[src.Chain]; done {
			continue
		}

		// A non-EVM Cosmos REST source (Osmosis SQS, Astroport, #128) has no RPC; register a chain def
		// with no eth client so the price orchestrator skips the EVM block-number lookup for it (Cosmos
		// spot prices are current, with no historical-block query).
		if isCosmosSourceType(src.SourceType) {
			out[src.Chain] = &chains.ChainDef{ChainShort: src.Chain, ChainName: src.Chain}
			continue
		}

		// Known-chain defaults (incl. the configured RPC) as the base; the manifest overrides where
		// it provides values. LookupChain returns a zero ChainMeta for an unknown chain, so a new
		// chain is driven entirely by the manifest's rpcUrl / blocksPerMin.
		meta, _ := chains.LookupChain(src.Chain, dm.cfg.Subchain)
		meta.ChainShort = src.Chain
		if src.RPCURL != "" {
			meta.RpcUrl = src.RPCURL
		}
		if src.BlocksPerMin > 0 {
			meta.BlocksPerMin = src.BlocksPerMin
		}
		if meta.BlocksPerMin == 0 {
			meta.BlocksPerMin = chains.DefaultBlocksPerMin
		}

		if meta.RpcUrl == "" {
			logger.WarnWithFields("dex", "buildChains", "skip chain",
				"no RPC for chain from the manifest or config - cannot query it; skipping",
				logger.Fields{"chain": src.Chain})
			continue
		}

		if cur := existing[src.Chain]; cur != nil && cur.RpcUrl == meta.RpcUrl && cur.EthClient != nil {
			out[src.Chain] = cur
			continue
		}

		cd, err := chains.DialChain(meta)
		if err != nil {
			logger.ErrorWithFields("dex", "buildChains", "dial chain", err.Error(),
				logger.Fields{"chain": src.Chain, "rpc": meta.RpcUrl})
			continue
		}
		out[src.Chain] = cd
	}

	return out
}

// chainDef returns the per-chain config under the read lock, so a concurrent ApplyManifest swapping
// the chain set in does not race the price loop's lookups. Returns nil for an unconfigured chain.
func (dm *Manager) chainDef(chain string) *chains.ChainDef {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.chains[chain]
}
