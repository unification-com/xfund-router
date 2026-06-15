package dex

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakePricer is a stub sqsPricer for unit-testing CosmosSqsSource without a network call.
type fakePricer struct {
	price             float64
	err               error
	gotBase, gotQuote string
}

func (f *fakePricer) TokenPrice(baseDenom, quoteDenom string) (float64, error) {
	f.gotBase, f.gotQuote = baseDenom, quoteDenom
	return f.price, f.err
}

func TestCosmosSqsSourceMetadata(t *testing.T) {
	s := newCosmosSqsSource(&fakePricer{}, 1000, 0)
	require.Equal(t, "osmosis_osmosis_sqs", s.Name())
	require.Equal(t, "osmosis", s.Chain())
	require.Equal(t, "osmosis_sqs", s.Dex())
	require.Equal(t, uint64(1000), s.MinLiquidity())

	// Satisfies the PriceSource seam (so it slots into the Manager like a subgraph source).
	var _ PriceSource = s
}

func TestCosmosSqsSourceFetchPoolPrices(t *testing.T) {
	fp := &fakePricer{price: 0.046692}
	s := newCosmosSqsSource(fp, 0, 0)

	pools, err := s.FetchPoolPrices("OSMO", "USDC", DexInfo{}, 0)
	require.NoError(t, err)
	require.Len(t, pools, 1)
	require.Equal(t, "osmo-usdc", pools[0].Contract)
	require.Equal(t, []float64{0.046692}, pools[0].Prices)

	// Resolved the right denoms from the allow-list (OSMO -> uosmo, USDC -> its IBC denom).
	require.Equal(t, "uosmo", fp.gotBase)
	require.Equal(t, osmosisUsdcDenom, fp.gotQuote)
}

func TestCosmosSqsSourceUnknownSymbol(t *testing.T) {
	s := newCosmosSqsSource(&fakePricer{price: 1}, 0, 0)
	_, err := s.FetchPoolPrices("OSMO", "DOGE", DexInfo{}, 0)
	require.Error(t, err)
}

func TestCosmosSqsSourcePricerError(t *testing.T) {
	s := newCosmosSqsSource(&fakePricer{err: errors.New("boom")}, 0, 0)
	_, err := s.FetchPoolPrices("ATOM", "USDC", DexInfo{}, 0)
	require.Error(t, err)
}

func TestCosmosSqsSourceNonPositivePrice(t *testing.T) {
	s := newCosmosSqsSource(&fakePricer{price: 0}, 0, 0)
	_, err := s.FetchPoolPrices("OSMO", "USDC", DexInfo{}, 0)
	require.Error(t, err)
}

func TestCosmosSqsSourceFetchPairsMetadataNoop(t *testing.T) {
	s := newCosmosSqsSource(&fakePricer{}, 0, 0)
	pairs, err := s.FetchPairsMetadata("anything")
	require.NoError(t, err)
	require.Nil(t, pairs)
}
