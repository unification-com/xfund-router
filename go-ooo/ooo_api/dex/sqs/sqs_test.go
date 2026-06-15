package sqs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	uosmo    = "uosmo"
	axlUsdc  = "ibc/D189335C6E4A68B513C10AB227BF1C1D38C746766278BA3EEB4FB14124F1D858"
	usdDenom = "ibc/498A0751C798A0D9A389AA3691123DADA57DAA4FE165D5C75894505B876BA6E4"
)

func TestTokenPricesUsd(t *testing.T) {
	// Captured live SQS shape: each denom quoted in the native USDC denom (usdDenom). A second USDC
	// bridge (axlUsdc) prices at ~1, which is exactly why base-USD ÷ target-USD is robust to which
	// bridge a symbol resolved to.
	body := `{"uosmo":{"` + usdDenom + `":"0.046692"},"` + axlUsdc + `":{"` + usdDenom + `":"1.0001"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/tokens/prices", r.URL.Path)
		require.Equal(t, "uosmo,"+axlUsdc, r.URL.Query().Get("base"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	prices, err := NewClient(srv.URL).TokenPricesUsd([]string{uosmo, axlUsdc})
	require.NoError(t, err)
	require.InDelta(t, 0.046692, prices[uosmo], 1e-6)
	require.InDelta(t, 1.0001, prices[axlUsdc], 1e-6)
}

func TestTokenPricesUsdMissingDenom(t *testing.T) {
	// A denom SQS doesn't return is absent from the map (not an error).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"uosmo":{"` + usdDenom + `":"0.047"}}`))
	}))
	defer srv.Close()
	prices, err := NewClient(srv.URL).TokenPricesUsd([]string{uosmo, "ibc/UNKNOWN"})
	require.NoError(t, err)
	require.InDelta(t, 0.047, prices[uosmo], 1e-6)
	_, ok := prices["ibc/UNKNOWN"]
	require.False(t, ok)
}

func TestTokenPricesUsdEmpty(t *testing.T) {
	prices, err := NewClient("http://example.invalid").TokenPricesUsd(nil)
	require.NoError(t, err)
	require.Empty(t, prices)
}

func TestTokenPricesUsdNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL).TokenPricesUsd([]string{uosmo})
	require.Error(t, err)
}
