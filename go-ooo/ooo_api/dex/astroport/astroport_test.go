package astroport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	usdc  = "ibc/B559A80D62249C8AA07A380E2A2BEA6E5CA9A6F079C912C3A9E9B494105E4F81"
	dAtom = "factory/neutron1k6hr0f83e7un2wjf29cspk7j69jrnskk65k3ek2nj9dztrlzpj6q00rtsa/udatom"
	ntrn  = "untrn"
)

// poolsBody is the real Astroport /api/pools shape (trimmed to the fields we read): assets carry an
// inline priceUSD, the pool a totalLiquidityUSD + isDeregistered. NTRN appears in two pools at
// different prices → the deeper one (0.40) must win; the deregistered pool's price (9.99) is ignored.
const poolsBody = `[
  {"isDeregistered":false,"totalLiquidityUSD":4672140,"assets":[
    {"denom":"` + usdc + `","priceUSD":0.999747},
    {"denom":"` + dAtom + `","priceUSD":2.898235}]},
  {"isDeregistered":false,"totalLiquidityUSD":81000,"assets":[
    {"denom":"` + ntrn + `","priceUSD":0.40},
    {"denom":"` + usdc + `","priceUSD":1.0}]},
  {"isDeregistered":false,"totalLiquidityUSD":3000,"assets":[
    {"denom":"` + ntrn + `","priceUSD":0.39},
    {"denom":"` + usdc + `","priceUSD":1.0}]},
  {"isDeregistered":true,"totalLiquidityUSD":999999,"assets":[
    {"denom":"` + ntrn + `","priceUSD":9.99},
    {"denom":"` + usdc + `","priceUSD":1.0}]}
]`

func newTestServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/pools", r.URL.Path)
		require.Equal(t, "neutron-1", r.URL.Query().Get("chainId"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(poolsBody))
	}))
}

func TestConfigured(t *testing.T) {
	require.True(t, NewClient("", "neutron").Configured())
	require.False(t, NewClient("", "cosmoshub").Configured()) // unmapped chain
}

func TestTokenPricesUsd(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	prices, err := NewClient(srv.URL, "neutron").TokenPricesUsd([]string{ntrn, usdc, dAtom})
	require.NoError(t, err)
	require.InDelta(t, 0.40, prices[ntrn], 1e-9)      // deepest active NTRN pool wins (not 0.39, not the deregistered 9.99)
	require.InDelta(t, 0.999747, prices[usdc], 1e-9)  // taken from the deepest pool it appears in
	require.InDelta(t, 2.898235, prices[dAtom], 1e-9) // inline price read directly
}

func TestTokenPricesUsdMissingDenom(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	prices, err := NewClient(srv.URL, "neutron").TokenPricesUsd([]string{ntrn, "ibc/UNLISTED"})
	require.NoError(t, err)
	require.InDelta(t, 0.40, prices[ntrn], 1e-9)
	_, ok := prices["ibc/UNLISTED"]
	require.False(t, ok) // a denom in no active pool is simply absent
}

func TestTokenPricesUsdUnconfigured(t *testing.T) {
	// An unmapped chain prices nothing and makes no network call (base URL is never reached).
	prices, err := NewClient("http://example.invalid", "cosmoshub").TokenPricesUsd([]string{ntrn})
	require.NoError(t, err)
	require.Empty(t, prices)
}

func TestTokenPricesUsdEmpty(t *testing.T) {
	prices, err := NewClient("http://example.invalid", "neutron").TokenPricesUsd(nil)
	require.NoError(t, err)
	require.Empty(t, prices)
}

func TestTokenPricesUsdNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "neutron").TokenPricesUsd([]string{ntrn})
	require.Error(t, err)
}
