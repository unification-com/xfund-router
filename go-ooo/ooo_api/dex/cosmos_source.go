package dex

import (
	"fmt"
	"strings"

	"go-ooo/ooo_api/dex/sqs"
	"go-ooo/ooo_api/dex/types"
)

// CosmosSqsSource is the Osmosis SQS PriceSource (#128 Phase 0b) - the first non-EVM, non-subgraph
// transport. It implements PriceSource by pricing an allow-list of blue-chip Osmosis pairs off the
// Osmosis Sidecar Query Server REST API (sqs.osmosis.zone), proving go-ooo's price path is
// transport-agnostic: the orchestrator (getPrices) and the aggregator (dexprice.go) treat it
// identically to a subgraph source, because the transport is hidden behind FetchPoolPrices. It
// speaks REST/JSON, not GraphQL, and has no subgraph URL.
//
// Phase 0b deliberately curates a small STATIC allow-list inside go-ooo (bypassing dex-pair-verify)
// to prove the non-EVM price path with minimal moving parts; Phase 2 moves Cosmos curation to dpv,
// the same way EVM pairs are curated.
type CosmosSqsSource struct {
	chain        string
	dex          string
	minLiquidity uint64
	minTxCount   uint64
	pricer       sqsPricer
	tokens       map[string]cosmosToken
}

// sqsPricer is the slice of the SQS client the source needs: the spot price of one denom quoted in
// another. *sqs.Client satisfies it; tests inject a fake so FetchPoolPrices is unit-testable without
// a network call.
type sqsPricer interface {
	TokenPrice(baseDenom, quoteDenom string) (float64, error)
}

// cosmosToken is one Osmosis token in the static allow-list: its symbol, on-chain denom (native
// "uosmo" or an "ibc/..." hash) and decimals.
type cosmosToken struct {
	symbol   string
	denom    string
	decimals int
}

const (
	// CosmosChainOsmosis is the chain id the Osmosis SQS source registers under. It is a NON-EVM chain
	// (its ChainDef has no ethclient), so the price orchestrator skips the EVM block-number lookup for
	// it - Cosmos spot prices are current, with no historical-block query.
	CosmosChainOsmosis = "osmosis"
	// DexOsmosisSqs is the dex id for the Osmosis SQS source.
	DexOsmosisSqs = "osmosis_sqs"

	// osmosisUsdcDenom / osmosisAtomDenom are the IBC denoms SQS uses for USDC (Noble) and ATOM on
	// Osmosis, confirmed live against /tokens/metadata (coingeckoId usd-coin / cosmos).
	osmosisUsdcDenom = "ibc/498A0751C798A0D9A389AA3691123DADA57DAA4FE165D5C75894505B876BA6E4"
	osmosisAtomDenom = "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2"
)

// osmosisAllowList is the blue-chip Osmosis token set go-ooo prices in Phase 0b. Pairs are priced
// vs USDC (the SQS default quote). Symbols are the canonical oracle symbols a consumer queries.
var osmosisAllowList = map[string]cosmosToken{
	"OSMO": {symbol: "OSMO", denom: "uosmo", decimals: 6},
	"ATOM": {symbol: "ATOM", denom: osmosisAtomDenom, decimals: 6},
	"USDC": {symbol: "USDC", denom: osmosisUsdcDenom, decimals: 6},
}

// Compile-time check that CosmosSqsSource satisfies the PriceSource seam (#128).
var _ PriceSource = CosmosSqsSource{}

// NewOsmosisSqsSource builds the Osmosis SQS PriceSource pointed at sqsBaseURL (empty = the public
// instance). It prices the blue-chip Osmosis allow-list (OSMO/ATOM vs USDC) off SQS.
func NewOsmosisSqsSource(sqsBaseURL string, minLiquidity, minTxCount uint64) CosmosSqsSource {
	return newCosmosSqsSource(sqs.NewClient(sqsBaseURL), minLiquidity, minTxCount)
}

// newCosmosSqsSource is the internal constructor taking the pricer directly, so tests can inject a
// fake instead of the real SQS client.
func newCosmosSqsSource(pricer sqsPricer, minLiquidity, minTxCount uint64) CosmosSqsSource {
	return CosmosSqsSource{
		chain:        CosmosChainOsmosis,
		dex:          DexOsmosisSqs,
		minLiquidity: minLiquidity,
		minTxCount:   minTxCount,
		pricer:       pricer,
		tokens:       osmosisAllowList,
	}
}

func (s CosmosSqsSource) Name() string         { return fmt.Sprintf("%s_%s", s.chain, s.dex) }
func (s CosmosSqsSource) Chain() string        { return s.chain }
func (s CosmosSqsSource) Dex() string          { return s.dex }
func (s CosmosSqsSource) MinLiquidity() uint64 { return s.minLiquidity }
func (s CosmosSqsSource) MinTxCount() uint64   { return s.minTxCount }

// FetchPoolPrices prices base/target off the Osmosis SQS API. Both symbols must be in the allow-list;
// the price is the SQS spot price of base quoted in target's denom. dexInfo and minutes are
// intentionally ignored: Cosmos spot prices are current (no historical-block query - time series is
// Phase 1's Numia work), and the priceable set is the allow-list, not dexInfo's EVM contract
// addresses. It returns ONE PoolPrices sample keyed by a synthetic "<base>-<target>" contract that
// matches the seeded dex_pairs row, so the orchestrator joins liquidity/confidence onto it.
func (s CosmosSqsSource) FetchPoolPrices(base, target string, _ DexInfo, _ uint64) ([]types.PoolPrices, error) {
	b, ok := s.tokens[base]
	if !ok {
		return nil, fmt.Errorf("cosmos-sqs: base symbol %q not in the Osmosis allow-list", base)
	}
	t, ok := s.tokens[target]
	if !ok {
		return nil, fmt.Errorf("cosmos-sqs: target symbol %q not in the Osmosis allow-list", target)
	}
	price, err := s.pricer.TokenPrice(b.denom, t.denom)
	if err != nil {
		return nil, err
	}
	if price <= 0 {
		return nil, fmt.Errorf("cosmos-sqs: non-positive price %v for %s/%s", price, base, target)
	}
	return []types.PoolPrices{{
		Contract: cosmosPairContract(base, target),
		Prices:   []float64{price},
	}}, nil
}

// FetchPairsMetadata is a no-op for the allow-list source: there is no external pair discovery to
// refresh in Phase 0b (the allow-list is static and its synthetic pairs carry no live reserves to
// update). Returning nil leaves the seeded rows untouched.
func (s CosmosSqsSource) FetchPairsMetadata(_ string) ([]types.DexPair, error) {
	return nil, nil
}

// cosmosPairContract is the synthetic pool key for an allow-list pair ("osmo-usdc"). Lower-cased to
// match the orchestrator's case-insensitive metadata join (it lower-cases contract addresses), and
// used as the ContractAddress when seeding the pair into dex_pairs.
func cosmosPairContract(base, target string) string {
	return strings.ToLower(base + "-" + target)
}
