package dex

import (
	"fmt"

	"go-ooo/logger"
	"go-ooo/ooo_api/dex/sqs"
	"go-ooo/ooo_api/dex/types"
	"go-ooo/ooo_api/export"
)

// cosmosSourceTypeRestSqs is the manifest sourceType marking an Osmosis SQS source (#128). go-ooo
// builds a CosmosSqsSource for it instead of a subgraph FamilyModule.
const cosmosSourceTypeRestSqs = "rest-sqs"

// CosmosSqsSource is a non-EVM, non-subgraph PriceSource (#128): it prices a dex-pair-verify-curated
// Osmosis pair off the Osmosis Sidecar Query Server REST API (sqs.osmosis.zone). The orchestrator
// (getPrices) and the aggregator (dexprice.go) treat it identically to a subgraph source because the
// transport is hidden behind FetchPoolPrices - it speaks REST/JSON, not GraphQL, and has no subgraph.
//
// Curation + identity now come ENTIRELY from dex-pair-verify (Phase 2): the pairs are persisted to
// dex_pairs from the export feed, and each token's on-chain denom is persisted to token_contracts (the
// feed's per-token contractAddress IS the Cosmos denom). The source resolves a queried symbol to its
// denom via that table, so go-ooo holds no static Osmosis allow-list - the source, its SQS URL, and
// its pairs are all manifest/feed-driven.
type CosmosSqsSource struct {
	chain        string
	dex          string
	minLiquidity uint64
	minTxCount   uint64
	pricer       sqsPricer
	denoms       denomResolver
}

// sqsPricer is the slice of the SQS client the source needs: the spot price of one denom quoted in
// another. *sqs.Client satisfies it; tests inject a fake so FetchPoolPrices is unit-testable without a
// network call.
type sqsPricer interface {
	TokenPrice(baseDenom, quoteDenom string) (float64, error)
}

// denomResolver resolves a token symbol to its on-chain denom on a chain, from the token_contracts
// table that dex-pair-verify's pair feed populates. *database.DB satisfies it; tests inject a fake.
type denomResolver interface {
	FindTokenAddressByChainAndSymbol(chain, symbol string) (string, error)
}

// Compile-time check that CosmosSqsSource satisfies the PriceSource seam (#128).
var _ PriceSource = CosmosSqsSource{}

// NewCosmosSqsSource builds a Cosmos SQS PriceSource for (chain, dex), pointed at sqsBaseURL (empty =
// the public instance), resolving token symbols to denoms via denoms.
func NewCosmosSqsSource(chain, dex, sqsBaseURL string, denoms denomResolver, minLiquidity, minTxCount uint64) CosmosSqsSource {
	return newCosmosSqsSource(chain, dex, sqs.NewClient(sqsBaseURL), denoms, minLiquidity, minTxCount)
}

// newCosmosSqsSource is the internal constructor taking the pricer directly, so tests can inject a
// fake instead of the real SQS client.
func newCosmosSqsSource(chain, dex string, pricer sqsPricer, denoms denomResolver, minLiquidity, minTxCount uint64) CosmosSqsSource {
	return CosmosSqsSource{
		chain:        chain,
		dex:          dex,
		minLiquidity: minLiquidity,
		minTxCount:   minTxCount,
		pricer:       pricer,
		denoms:       denoms,
	}
}

func (s CosmosSqsSource) Name() string         { return fmt.Sprintf("%s_%s", s.chain, s.dex) }
func (s CosmosSqsSource) Chain() string        { return s.chain }
func (s CosmosSqsSource) Dex() string          { return s.dex }
func (s CosmosSqsSource) MinLiquidity() uint64 { return s.minLiquidity }
func (s CosmosSqsSource) MinTxCount() uint64   { return s.minTxCount }

// FetchPoolPrices prices base/target off the Osmosis SQS API. It resolves each symbol to its on-chain
// denom (from token_contracts, populated by the dpv feed) and reads the SQS spot price of base quoted
// in target's denom - a chain-wide spot, not per-pool. The price is echoed under EACH curated pool
// contract in dexInfo (the dpv-curated pools for this pair) so the orchestrator joins each pool's
// liquidity + trust score onto it, exactly as for a subgraph source's per-pool prices. minutes is
// ignored: Cosmos spot prices are current (no historical-block query).
func (s CosmosSqsSource) FetchPoolPrices(base, target string, dexInfo DexInfo, _ uint64) ([]types.PoolPrices, error) {
	baseDenom, err := s.denoms.FindTokenAddressByChainAndSymbol(s.chain, base)
	if err != nil || baseDenom == "" {
		return nil, fmt.Errorf("cosmos-sqs: no denom for base symbol %q on %s: %v", base, s.chain, err)
	}
	targetDenom, err := s.denoms.FindTokenAddressByChainAndSymbol(s.chain, target)
	if err != nil || targetDenom == "" {
		return nil, fmt.Errorf("cosmos-sqs: no denom for target symbol %q on %s: %v", target, s.chain, err)
	}

	price, err := s.pricer.TokenPrice(baseDenom, targetDenom)
	if err != nil {
		return nil, err
	}
	if price <= 0 {
		return nil, fmt.Errorf("cosmos-sqs: non-positive price %v for %s/%s", price, base, target)
	}

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
func (s CosmosSqsSource) FetchPairsMetadata(_ string) ([]types.DexPair, error) {
	return nil, nil
}

// --- manifest-driven construction (#128 Phase 2d) ---

// restURLFor returns the SQS base URL from a rest-sqs source's endpoint list (its first non-empty
// urlTemplate; no {API_KEY} substitution - SQS is keyless).
func restURLFor(endpoints []export.ManifestEndpoint) (string, bool) {
	for _, e := range endpoints {
		if e.URLTemplate != "" {
			return e.URLTemplate, true
		}
	}
	return "", false
}

// buildCosmosSources constructs a CosmosSqsSource for every manifest source whose transport is
// rest-sqs, resolving token denoms via denoms (the token_contracts table). This is the manifest-driven
// replacement for the old static Osmosis allow-list: the source set, its SQS URL, and its pairs all
// come from dex-pair-verify. MinTxCount is 0 (SQS exposes no per-pool tx counts; the dpv curation
// floor + verdict do the filtering); MinLiquidity is the source's curation floor (XR2) when known.
func buildCosmosSources(m *export.Manifest, denoms denomResolver) []CosmosSqsSource {
	var out []CosmosSqsSource
	for _, src := range m.SupportedSources {
		if src.SourceType != cosmosSourceTypeRestSqs {
			continue
		}
		sqsURL, ok := restURLFor(src.Endpoints)
		if !ok {
			logger.WarnWithFields("dex", "buildCosmosSources", "skip source",
				"no SQS endpoint URL on the rest-sqs source - skipping",
				logger.Fields{"chain": src.Chain, "dex": src.Dex})
			continue
		}
		minLiquidity := uint64(types.DefaultMinLiquidity)
		if src.MinLiquidityUsd > 0 {
			minLiquidity = uint64(src.MinLiquidityUsd)
		}
		out = append(out, NewCosmosSqsSource(src.Chain, src.Dex, sqsURL, denoms, minLiquidity, 0))
	}
	return out
}
