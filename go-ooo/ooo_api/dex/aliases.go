package dex

import (
	"strings"

	"go-ooo/database/models"
	"go-ooo/logger"
	"go-ooo/ooo_api/export"
)

// aliases.go is the consumer half of the asset-class aliases (S7 / #120). dex-pair-verify curates the
// fungible asset classes (USD, ETH, BTC) and ships them in the v3 manifest as AliasGroups (symbol ->
// member CoinGecko coin ids) plus AliasPairs (the active cross-class pairs, e.g. "ETH.USD", each with
// the canonical keys backing it). go-ooo trusts that curation: an alias query like `ETH.USD` is
// expanded to every backing pool across all chains/DEXs and priced as ONE deep, robust estimate, so a
// consumer no longer queries `WETH.USDC` vs `WETH.USDT` vs `WBTC...` separately. The robust aggregator
// (dexprice.go) is unchanged - only the sample-gathering differs (gatherSamples with an alias plan).

// aliasIndex is the query-time view of the manifest aliases, rebuilt wholesale by ApplyManifest and
// swapped in under the Manager lock. It is read-only after construction.
type aliasIndex struct {
	// cgToSymbol maps a CoinGecko coin id to its alias class symbol (e.g. "weth" -> "ETH"). Keys are
	// lower-cased; values are upper-cased class symbols.
	cgToSymbol map[string]string
	// pairKeys maps an active sorted alias-pair key (e.g. "ETH.USD") to the canonical keys that back
	// it. Presence here means dex-pair-verify has verified coverage for that class pair.
	pairKeys map[string][]string
}

// buildAliasIndex constructs the alias index from a manifest's AliasGroups + AliasPairs. A manifest
// reconstructed from the persisted source catalogue carries neither (they live only on the live API
// manifest), so this returns nil for an empty AliasGroups - ApplyManifest then preserves the prior
// index rather than wiping it.
func buildAliasIndex(m *export.Manifest) *aliasIndex {
	if len(m.AliasGroups) == 0 {
		return nil
	}

	idx := &aliasIndex{
		cgToSymbol: make(map[string]string),
		pairKeys:   make(map[string][]string),
	}
	for _, g := range m.AliasGroups {
		sym := strings.ToUpper(strings.TrimSpace(g.Symbol))
		if sym == "" {
			continue
		}
		for _, raw := range g.CgIds {
			cg := strings.ToLower(strings.TrimSpace(raw))
			if cg == "" {
				continue
			}
			idx.cgToSymbol[cg] = sym
		}
	}
	for _, p := range m.AliasPairs {
		key := strings.ToUpper(strings.TrimSpace(p.Pair))
		if key == "" || len(p.CanonicalKeys) == 0 {
			continue
		}
		idx.pairKeys[key] = p.CanonicalKeys
	}
	return idx
}

// aliasSymbol returns the alias class symbol for a token's CoinGecko coin id, or "" if the token is
// not a curated member of any class.
func (idx *aliasIndex) aliasSymbol(cgId string) string {
	return idx.cgToSymbol[strings.ToLower(strings.TrimSpace(cgId))]
}

// aliasPairKey is the order-independent key for a class pair (e.g. ("USD","ETH") -> "ETH.USD"),
// matching the sorted form dex-pair-verify emits in AliasPairs.
func aliasPairKey(a, b string) string {
	a, b = strings.ToUpper(a), strings.ToUpper(b)
	if a <= b {
		return a + "." + b
	}
	return b + "." + a
}

// lookupAliasPair reports whether base/target name a queryable cross-class alias pair that has backing
// coverage, returning the canonical keys that back it. The two sides must be distinct curated classes
// and the pair must be active in the manifest.
func (idx *aliasIndex) lookupAliasPair(base, target string) ([]string, bool) {
	if idx == nil {
		return nil, false
	}
	b := strings.ToUpper(strings.TrimSpace(base))
	t := strings.ToUpper(strings.TrimSpace(target))
	if b == "" || t == "" || b == t {
		return nil, false
	}
	keys, ok := idx.pairKeys[aliasPairKey(b, t)]
	return keys, ok
}

// orient resolves which of a member pool's two tokens is the base side and which is the target, by
// their CoinGecko classes. Returns the pool's real token symbols in (base, target) order so the
// subgraph/REST price query orients correctly, or ok=false if the pool's tokens don't map to exactly
// this base/target class pair (a defensive guard - the canonical-key selection should already ensure
// they do).
func (idx *aliasIndex) orient(row models.DexPairs, baseSym, targetSym string) (base, target string, ok bool) {
	c0 := idx.aliasSymbol(row.T0Cg)
	c1 := idx.aliasSymbol(row.T1Cg)
	switch {
	case c0 == baseSym && c1 == targetSym:
		return row.T0Symbol, row.T1Symbol, true
	case c1 == baseSym && c0 == targetSym:
		return row.T1Symbol, row.T0Symbol, true
	default:
		return "", "", false
	}
}

// IsAliasPair reports whether base/target is a queryable asset-class alias pair (e.g. ETH.USD) with
// backing coverage. QueryDexPrice routes such a query to GetAliasPricesFromDexModules; everything else
// (an exact symbol pair such as WETH.USDC) takes the direct path.
func (dm *Manager) IsAliasPair(base, target string) bool {
	dm.mu.RLock()
	idx := dm.aliases
	dm.mu.RUnlock()
	_, ok := idx.lookupAliasPair(base, target)
	return ok
}

// GetAliasPricesFromDexModules collects per-pool price samples for an asset-class alias query (S7),
// e.g. ETH.USD. It resolves the alias pair to its backing canonical keys, pulls every verified pool on
// those keys across all chains/DEXs, orients each to the queried base/target classes (via the
// per-token cg ids), and feeds them to the shared gatherSamples driver grouped per module by oriented
// symbol pair. The result is one combined []PoolSample the robust aggregator prices exactly as for a
// direct query.
func (dm *Manager) GetAliasPricesFromDexModules(base, target string, minutes uint64) []PoolSample {
	dm.mu.RLock()
	idx := dm.aliases
	dm.mu.RUnlock()

	keys, ok := idx.lookupAliasPair(base, target)
	if !ok {
		return nil
	}

	baseSym := strings.ToUpper(strings.TrimSpace(base))
	targetSym := strings.ToUpper(strings.TrimSpace(target))

	rows, err := dm.db.FindByCanonicalKeys(keys)
	if err != nil {
		logger.ErrorWithFields("dex", "GetAliasPricesFromDexModules", "find by canonical keys", err.Error(),
			logger.Fields{"base": baseSym, "target": targetSym})
		return nil
	}

	// Group the member pools per module (chain|dex), and within a module per oriented symbol pair, so
	// each distinct (baseSym, targetSym) becomes one subgraph/REST query the price source can orient.
	byModule := make(map[string]map[string]*poolQuery) // chain|dex -> "BASE|TARGET" -> query
	for _, row := range rows {
		oBase, oTarget, ok := idx.orient(row, baseSym, targetSym)
		if !ok {
			continue
		}
		modKey := row.Chain + "|" + row.Dex
		groups := byModule[modKey]
		if groups == nil {
			groups = make(map[string]*poolQuery)
			byModule[modKey] = groups
		}
		groupKey := oBase + "|" + oTarget
		q := groups[groupKey]
		if q == nil {
			q = &poolQuery{base: oBase, target: oTarget}
			groups[groupKey] = q
		}
		q.pools = append(q.pools, row)
	}

	return dm.gatherSamples(baseSym, targetSym, minutes, func(module Module) []poolQuery {
		groups := byModule[module.Chain()+"|"+module.Dex()]
		if len(groups) == 0 {
			return nil
		}
		queries := make([]poolQuery, 0, len(groups))
		for _, q := range groups {
			queries = append(queries, *q)
		}
		return queries
	})
}
