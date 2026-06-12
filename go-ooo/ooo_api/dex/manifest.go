package dex

import (
	"strings"

	"go-ooo/config"
	"go-ooo/logger"
	"go-ooo/ooo_api/dex/families/messari"
	"go-ooo/ooo_api/dex/families/uniswap"
	"go-ooo/ooo_api/dex/types"
	"go-ooo/ooo_api/export"
)

// apiKeyPlaceholder is the token a manifest endpoint's urlTemplate carries in place of a literal
// subgraph API key; go-ooo substitutes the key it holds.
const apiKeyPlaceholder = "{API_KEY}"

// schemaFamilyFor maps a manifest subgraphSchemaFamily string to the SchemaFamily that prices it.
// An unrecognised family (dpv's "custom", or a future "univ4"/"infinity") returns false so the
// caller skips the source with a warning rather than crashing an older binary - the forward-compat
// seam the modular dispatch exists to provide.
func schemaFamilyFor(schemaFamily string) (SchemaFamily, bool) {
	switch schemaFamily {
	case "univ2":
		return uniswap.New(uniswap.V2), true
	case "univ3":
		return uniswap.New(uniswap.V3), true
	case "messari":
		return messari.New(), true
	default:
		return nil, false
	}
}

// subgraphURLFor resolves a source's subgraph URL from its ordered endpoint list. It prefers a
// paid (decentralised-network) endpoint when go-ooo holds an API key - substituting {API_KEY} -
// and otherwise the first free-tier endpoint needing no key. Returns false when nothing is usable
// (e.g. the source offers only a paid endpoint but no key is configured).
func subgraphURLFor(endpoints []export.ManifestEndpoint, apiKey string) (string, bool) {
	if apiKey != "" {
		for _, e := range endpoints {
			if strings.Contains(e.URLTemplate, apiKeyPlaceholder) {
				return strings.ReplaceAll(e.URLTemplate, apiKeyPlaceholder, apiKey), true
			}
		}
	}
	for _, e := range endpoints {
		if e.URLTemplate != "" && !strings.Contains(e.URLTemplate, apiKeyPlaceholder) {
			return e.URLTemplate, true
		}
	}
	return "", false
}

// BuildModulesFromManifest constructs one FamilyModule per manifest source whose schema family
// go-ooo recognises AND for which it can resolve a usable subgraph endpoint. Sources with an
// unknown family, or with only a paid endpoint when no API key is held, are skipped with a
// structured warning - so a forward-compat family or a key-less deployment degrades gracefully.
// This is the manifest-driven replacement for the hand-written seed.go constructors (#119 step 5).
//
// The per-source curation floor (the feed's minLiquidityUsd, XR2) is used as the module's
// MinLiquidity when known (carried on the source from the persisted pair feed); go-ooo's configured
// default is the fallback for a source with no floor yet. MinTxCount uses the global default.
func BuildModulesFromManifest(cfg *config.Config, m *export.Manifest) []FamilyModule {
	apiKey := cfg.ApiKeys.GraphNetwork
	modules := make([]FamilyModule, 0, len(m.SupportedSources))

	for _, src := range m.SupportedSources {
		family, ok := schemaFamilyFor(src.SubgraphSchemaFamily)
		if !ok {
			logger.WarnWithFields("dex", "BuildModulesFromManifest", "skip source",
				"unrecognised subgraph schema family - skipping (a newer go-ooo may price it)",
				logger.Fields{"chain": src.Chain, "dex": src.Dex, "family": src.SubgraphSchemaFamily})
			continue
		}

		subgraphUrl, ok := subgraphURLFor(src.Endpoints, apiKey)
		if !ok {
			logger.WarnWithFields("dex", "BuildModulesFromManifest", "skip source",
				"no usable subgraph endpoint (a paid endpoint needs an API key) - skipping",
				logger.Fields{"chain": src.Chain, "dex": src.Dex})
			continue
		}

		minLiquidity := uint64(types.DefaultMinLiquidity)
		if src.MinLiquidityUsd > 0 {
			minLiquidity = uint64(src.MinLiquidityUsd)
		}

		modules = append(modules, NewFamilyModule(FamilyModuleSpec{
			Chain:        src.Chain,
			Dex:          src.Dex,
			SubgraphUrl:  subgraphUrl,
			MinLiquidity: minLiquidity,
			MinTxCount:   types.DefaultMinTxCount,
			Family:       family,
		}))
	}
	return modules
}
