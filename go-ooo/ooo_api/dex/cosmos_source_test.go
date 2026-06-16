package dex

import (
	"errors"
	"testing"

	"go-ooo/ooo_api/export"

	"github.com/stretchr/testify/require"
)

// fakePricer is a stub cosmosPricer for unit-testing CosmosSource without a network call.
type fakePricer struct {
	usd       map[string]float64
	err       error
	gotDenoms []string
}

func (f *fakePricer) TokenPricesUsd(denoms []string) (map[string]float64, error) {
	f.gotDenoms = denoms
	return f.usd, f.err
}

// fakeDenoms is a stub denomResolver: a fixed symbol -> denom map (the token_contracts the dpv feed
// would have populated).
type fakeDenoms struct {
	denoms map[string]string
}

func (f fakeDenoms) FindTokenAddressByChainAndSymbol(_ string, symbol string) (string, error) {
	d, ok := f.denoms[symbol]
	if !ok {
		return "", errors.New("not found")
	}
	return d, nil
}

func osmoDenoms() fakeDenoms {
	return fakeDenoms{denoms: map[string]string{
		"OSMO": "uosmo",
		"USDC": "ibc/USDCDENOM",
		"ATOM": "ibc/ATOMDENOM",
	}}
}

func TestCosmosSourceMetadata(t *testing.T) {
	s := newCosmosSource("osmosis", "osmosis_sqs", &fakePricer{}, osmoDenoms(), 1000, 0)
	require.Equal(t, "osmosis_osmosis_sqs", s.Name())
	require.Equal(t, "osmosis", s.Chain())
	require.Equal(t, "osmosis_sqs", s.Dex())
	require.Equal(t, uint64(1000), s.MinLiquidity())

	// Satisfies the PriceSource seam (so it slots into the Manager like a subgraph source).
	var _ PriceSource = s
}

func TestCosmosSourceFetchPoolPrices(t *testing.T) {
	// USD prices: OSMO ≈ $0.046692, USDC ≈ $1 → cross price 0.046692 ÷ 1.
	fp := &fakePricer{usd: map[string]float64{"uosmo": 0.046692, "ibc/USDCDENOM": 1.0}}
	s := newCosmosSource("osmosis", "osmosis_sqs", fp, osmoDenoms(), 0, 0)

	// Two curated pools for this pair → the chain-wide spot is echoed under each pool contract so the
	// orchestrator joins each pool's liquidity/confidence onto it.
	dexInfo := DexInfo{ContractAddressList: []string{"0xpool1", "0xpool2"}}
	pools, err := s.FetchPoolPrices("OSMO", "USDC", dexInfo, 0)
	require.NoError(t, err)
	require.Len(t, pools, 2)
	require.Equal(t, "0xpool1", pools[0].Contract)
	require.Equal(t, "0xpool2", pools[1].Contract)
	require.InDelta(t, 0.046692, pools[0].Prices[0], 1e-9)

	// Priced the denoms resolved from token_contracts (OSMO → uosmo, USDC → its IBC denom).
	require.ElementsMatch(t, []string{"uosmo", "ibc/USDCDENOM"}, fp.gotDenoms)
}

func TestCosmosSourceUnknownSymbol(t *testing.T) {
	s := newCosmosSource("osmosis", "osmosis_sqs", &fakePricer{}, osmoDenoms(), 0, 0)
	_, err := s.FetchPoolPrices("OSMO", "DOGE", DexInfo{ContractAddressList: []string{"0xp"}}, 0)
	require.Error(t, err) // DOGE has no denom in token_contracts
}

func TestCosmosSourcePricerError(t *testing.T) {
	s := newCosmosSource("osmosis", "osmosis_sqs", &fakePricer{err: errors.New("boom")}, osmoDenoms(), 0, 0)
	_, err := s.FetchPoolPrices("ATOM", "USDC", DexInfo{ContractAddressList: []string{"0xp"}}, 0)
	require.Error(t, err)
}

func TestCosmosSourceMissingPrice(t *testing.T) {
	// SQS returned no USD price for the target denom → can't form a cross price.
	s := newCosmosSource("osmosis", "osmosis_sqs", &fakePricer{usd: map[string]float64{"uosmo": 0.047}}, osmoDenoms(), 0, 0)
	_, err := s.FetchPoolPrices("OSMO", "USDC", DexInfo{ContractAddressList: []string{"0xp"}}, 0)
	require.Error(t, err)
}

func TestCosmosSourceFetchPairsMetadataNoop(t *testing.T) {
	s := newCosmosSource("osmosis", "osmosis_sqs", &fakePricer{}, osmoDenoms(), 0, 0)
	pairs, err := s.FetchPairsMetadata("anything")
	require.NoError(t, err)
	require.Nil(t, pairs)
}

func TestBuildCosmosSources(t *testing.T) {
	m := &export.Manifest{SupportedSources: []export.ManifestSource{
		{Chain: "eth", Dex: "uniswap_v3", SubgraphSchemaFamily: "univ3", SourceType: "subgraph"}, // not cosmos → skipped
		{Chain: "osmosis", Dex: "osmosis_sqs", SourceType: "rest-sqs", MinLiquidityUsd: 25000,
			Endpoints: []export.ManifestEndpoint{{Provider: "self-hosted", URLTemplate: "https://sqs.osmosis.zone", Tier: "free"}}},
		{Chain: "cosmoshub", Dex: "x", SourceType: "rest-sqs"}, // rest-sqs but no endpoint → skipped
	}}

	mods := buildCosmosSources(m, osmoDenoms())
	require.Len(t, mods, 1) // only the osmosis source with a usable endpoint
	require.Equal(t, "osmosis_osmosis_sqs", mods[0].Name())
	require.Equal(t, uint64(25000), mods[0].MinLiquidity()) // curation floor honoured
}
