package dex

import (
	"encoding/json"

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
		Endpoints:            string(endpoints),
		FactoryAddress:       src.FactoryAddress,
		RpcUrl:               src.RPCURL,
		BlocksPerMin:         src.BlocksPerMin,
		PairCount:            src.PairCount,
		ExportUrl:            src.ExportURL,
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
		FactoryAddress:       row.FactoryAddress,
		RPCURL:               row.RpcUrl,
		BlocksPerMin:         row.BlocksPerMin,
		PairCount:            row.PairCount,
		ExportURL:            row.ExportUrl,
		LastUpdated:          row.SourceUpdatedAt,
		LastVerifiedAt:       row.SourceVerifiedAt,
	}, nil
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

	return persisted, nil
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

	dm.mu.Lock()
	dm.modules = moduleMap
	dm.chains = newChains
	dm.mu.Unlock()

	logger.InfoWithFields("dex", "ApplyManifest", "", "applied manifest", logger.Fields{
		"sources": len(m.SupportedSources),
		"modules": len(moduleMap),
		"chains":  len(newChains),
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
