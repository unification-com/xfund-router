package sqs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	uosmo     = "uosmo"
	usdcDenom = "ibc/498A0751C798A0D9A389AA3691123DADA57DAA4FE165D5C75894505B876BA6E4"
)

func TestTokenPrice(t *testing.T) {
	// Captured live SQS shape for GET /tokens/prices?base=uosmo.
	body := `{"uosmo":{"ibc/498A0751C798A0D9A389AA3691123DADA57DAA4FE165D5C75894505B876BA6E4":"0.046692287672290479456622982079540448"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/tokens/prices", r.URL.Path)
		require.Equal(t, "uosmo", r.URL.Query().Get("base"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	price, err := NewClient(srv.URL).TokenPrice(uosmo, usdcDenom)
	require.NoError(t, err)
	require.InDelta(t, 0.046692, price, 1e-6)
}

func TestTokenPriceMissingQuote(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"uosmo":{"ibc/SOMETHINGELSE":"1.23"}}`))
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL).TokenPrice(uosmo, usdcDenom)
	require.Error(t, err)
}

func TestTokenPriceMissingBase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL).TokenPrice(uosmo, usdcDenom)
	require.Error(t, err)
}

func TestTokenPriceNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL).TokenPrice(uosmo, usdcDenom)
	require.Error(t, err)
}
