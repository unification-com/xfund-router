package dex

import (
	"fmt"

	"go-ooo/logger"
	"go-ooo/ooo_api/dex/astroport"
	"go-ooo/ooo_api/dex/sqs"
	"go-ooo/ooo_api/dex/types"
	"go-ooo/ooo_api/export"
)

// The manifest sourceTypes marking a Cosmos REST source (#128). go-ooo builds a CosmosSource for each
// instead of a subgraph FamilyModule, picking the matching pricer (cosmosPricerFor).
const (
	cosmosSourceTypeRestSqs       = "rest-sqs"       // Osmosis Sidecar Query Server
	cosmosSourceTypeRestAstroport = "rest-astroport" // Astroport on Neutron
)

// CosmosSource is a non-EVM, non-subgraph PriceSource (#128): it prices a dex-pair-verify-curated
// Cosmos pair off a chain's REST price API (Osmosis SQS, Astroport on Neutron, ...). The orchestrator
// (getPrices) and the aggregator (dexprice.go) treat it identically to a subgraph source because the
// transport is hidden behind FetchPoolPrices - it speaks REST/JSON, not GraphQL, and has no subgraph.
// The only thing that varies per Cosmos API is the pricer (the USD-price client); everything else -
// denom resolution, the USD-pivot cross price, the per-pool echo - is shared.
//
// Curation + identity come ENTIRELY from dex-pair-verify (Phase 2): the pairs are persisted to
// dex_pairs from the export feed, and each token's on-chain denom is persisted to token_contracts (the
// feed's per-token contractAddress IS the Cosmos denom). The source resolves a queried symbol to its
// denom via that table, so go-ooo holds no static Cosmos allow-list - the sources, their REST URLs, and
// their pairs are all manifest/feed-driven.
type CosmosSource struct {
	chain        string
	dex          string
	minLiquidity uint64
	minTxCount   uint64
	pricer       cosmosPricer
	denoms       denomResolver
}

// cosmosPricer is the slice of a Cosmos REST client the source needs: the USD price of each denom.
// *sqs.Client and *astroport.Client both satisfy it; tests inject a fake so FetchPoolPrices is
// unit-testable without a network call.
type cosmosPricer interface {
	TokenPricesUsd(denoms []string) (map[string]float64, error)
}

// denomResolver resolves a token symbol to its on-chain denom on a chain, from the token_contracts
// table that dex-pair-verify's pair feed populates. *database.DB satisfies it; tests inject a fake.
type denomResolver interface {
	FindTokenAddressByChainAndSymbol(chain, symbol string) (string, error)
}

// Compile-time check that CosmosSource satisfies the PriceSource seam (#128).
var _ PriceSource = CosmosSource{}

// newCosmosSource is the constructor taking the pricer directly, so build picks the per-transport
// client (SQS / Astroport) and tests inject a fake instead of a real REST client.
func newCosmosSource(chain, dex string, pricer cosmosPricer, denoms denomResolver, minLiquidity, minTxCount uint64) CosmosSource {
	return CosmosSource{
		chain:        chain,
		dex:          dex,
		minLiquidity: minLiquidity,
		minTxCount:   minTxCount,
		pricer:       pricer,
		denoms:       denoms,
	}
}

func (s CosmosSource) Name() string         { return fmt.Sprintf("%s_%s", s.chain, s.dex) }
func (s CosmosSource) Chain() string        { return s.chain }
func (s CosmosSource) Dex() string          { return s.dex }
func (s CosmosSource) MinLiquidity() uint64 { return s.minLiquidity }
func (s CosmosSource) MinTxCount() uint64   { return s.minTxCount }

// FetchPoolPrices prices base/target off the source's Cosmos REST API. It resolves each symbol to its
// on-chain denom (from token_contracts, populated by the dpv feed), reads each side's USD spot price
// from the pricer, and divides (base-USD ÷ target-USD) for the cross price - a chain-wide spot, not
// per-pool. Pricing via USD makes it robust to which bridge variant a symbol resolved to (any USDC
// bridge prices at ~1) and works for any base/target, not just vs USDC. The price is echoed under EACH
// curated pool contract in dexInfo so the orchestrator joins each pool's liquidity + trust score onto
// it, exactly as for a subgraph source's per-pool prices. minutes is ignored: Cosmos spot prices are
// current.
func (s CosmosSource) FetchPoolPrices(base, target string, dexInfo DexInfo, _ uint64) ([]types.PoolPrices, error) {
	baseDenom, err := s.denoms.FindTokenAddressByChainAndSymbol(s.chain, base)
	if err != nil || baseDenom == "" {
		return nil, fmt.Errorf("cosmos: no denom for base symbol %q on %s: %v", base, s.chain, err)
	}
	targetDenom, err := s.denoms.FindTokenAddressByChainAndSymbol(s.chain, target)
	if err != nil || targetDenom == "" {
		return nil, fmt.Errorf("cosmos: no denom for target symbol %q on %s: %v", target, s.chain, err)
	}

	usd, err := s.pricer.TokenPricesUsd([]string{baseDenom, targetDenom})
	if err != nil {
		return nil, err
	}
	basePrice, targetPrice := usd[baseDenom], usd[targetDenom]
	if basePrice <= 0 || targetPrice <= 0 {
		return nil, fmt.Errorf("cosmos: missing USD price for %s/%s on %s (base=%v target=%v)", base, target, s.chain, basePrice, targetPrice)
	}
	price := basePrice / targetPrice

	// Echo the chain-wide spot under each curated pool contract so the aggregator weights it by that
	// pool's liquidity + confidence (the metadata the orchestrator joins by contract address).
	out := make([]types.PoolPrices, 0, len(dexInfo.ContractAddressList))
	for _, contract := range dexInfo.ContractAddressList {
		out = append(out, types.PoolPrices{Contract: contract, Prices: []float64{price}})
	}
	return out, nil
}

// FetchPairsMetadata is a no-op: a Cosmos source's pairs + reserves come from the dex-pair-verify feed
// (persisted to dex_pairs), not from a live metadata query here.
func (s CosmosSource) FetchPairsMetadata(_ string) ([]types.DexPair, error) {
	return nil, nil
}

// --- manifest-driven construction (#128 Phase 2d) ---

// isCosmosSourceType reports whether a manifest sourceType is one of the Cosmos REST transports that
// build a CosmosSource rather than a subgraph FamilyModule (#128). buildChains uses it too, to register
// the source's non-EVM chain def.
func isCosmosSourceType(sourceType string) bool {
	return sourceType == cosmosSourceTypeRestSqs || sourceType == cosmosSourceTypeRestAstroport
}

// cosmosPricerFor returns the USD-price client for a Cosmos source's transport (restURL = its API base)
// and whether the transport is supported. *sqs.Client / *astroport.Client both satisfy cosmosPricer,
// so CosmosSource is identical across them - only the pricer differs. An Astroport source on a chain we
// do not map to an Astroport chainId is treated as unsupported (skipped) rather than pricing nothing.
func cosmosPricerFor(src export.ManifestSource, restURL string) (cosmosPricer, bool) {
	switch src.SourceType {
	case cosmosSourceTypeRestSqs:
		return sqs.NewClient(restURL), true
	case cosmosSourceTypeRestAstroport:
		c := astroport.NewClient(restURL, src.Chain)
		return c, c.Configured()
	default:
		return nil, false
	}
}

// restURLFor returns the REST base URL from a Cosmos source's endpoint list (its first non-empty
// urlTemplate; no {API_KEY} substitution - the Cosmos REST APIs are keyless).
func restURLFor(endpoints []export.ManifestEndpoint) (string, bool) {
	for _, e := range endpoints {
		if e.URLTemplate != "" {
			return e.URLTemplate, true
		}
	}
	return "", false
}

// buildCosmosSources constructs a CosmosSource for every manifest source whose transport is a Cosmos
// REST one, resolving token denoms via denoms (the token_contracts table). This is the manifest-driven
// replacement for the old static Cosmos allow-list: the source set, their REST URLs, and their pairs
// all come from dex-pair-verify. MinTxCount is 0 (the Cosmos REST APIs expose no per-pool tx counts;
// the dpv curation floor + verdict do the filtering); MinLiquidity is the source's curation floor (XR2)
// when known.
func buildCosmosSources(m *export.Manifest, denoms denomResolver) []CosmosSource {
	var out []CosmosSource
	for _, src := range m.SupportedSources {
		if !isCosmosSourceType(src.SourceType) {
			continue
		}
		restURL, ok := restURLFor(src.Endpoints)
		if !ok {
			logger.WarnWithFields("dex", "buildCosmosSources", "skip source",
				"no REST endpoint URL on the Cosmos source - skipping",
				logger.Fields{"chain": src.Chain, "dex": src.Dex})
			continue
		}
		pricer, ok := cosmosPricerFor(src, restURL)
		if !ok {
			logger.WarnWithFields("dex", "buildCosmosSources", "skip source",
				"unsupported Cosmos source transport - skipping",
				logger.Fields{"chain": src.Chain, "dex": src.Dex, "sourceType": src.SourceType})
			continue
		}
		minLiquidity := uint64(types.DefaultMinLiquidity)
		if src.MinLiquidityUsd > 0 {
			minLiquidity = uint64(src.MinLiquidityUsd)
		}
		out = append(out, newCosmosSource(src.Chain, src.Dex, pricer, denoms, minLiquidity, 0))
	}
	return out
}
