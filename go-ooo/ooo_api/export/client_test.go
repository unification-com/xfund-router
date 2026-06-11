package export

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchManifest(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		require.Equal(t, "/export/manifest", r.URL.Path)
		fmt.Fprint(w, `{"schemaVersion":3,"generatedAt":100,"supportedSources":[
			{"chain":"eth","dex":"uniswap_v3","subgraphSchemaFamily":"univ3","factoryAddress":"0xf",
			 "rpcUrl":"https://rpc","blocksPerMin":5,"pairCount":2,"exportUrl":"/export/eth/uniswap_v3",
			 "endpoints":[{"provider":"graph-decentralized","urlTemplate":"https://g/{API_KEY}/id","tier":"paid"}]}]}`)
	}))
	defer srv.Close()

	m, err := NewClient(srv.URL, "tok123", srv.Client()).FetchManifest(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Bearer tok123", gotAuth)
	require.Equal(t, 3, m.SchemaVersion)
	require.Len(t, m.SupportedSources, 1)
	s := m.SupportedSources[0]
	require.Equal(t, "eth", s.Chain)
	require.Equal(t, "univ3", s.SubgraphSchemaFamily)
	require.Equal(t, "graph-decentralized", s.Endpoints[0].Provider)
	require.Equal(t, "https://g/{API_KEY}/id", s.Endpoints[0].URLTemplate)
}

func TestFetchManifestRejectsNewerSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"schemaVersion":99,"supportedSources":[]}`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "", srv.Client()).FetchManifest(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "newer than this build")
}

func TestFetchPairFeedAndIfModifiedSince(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/export/eth/uniswap_v3", r.URL.Path)
		if r.URL.Query().Get("ifModifiedSince") == "500" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		fmt.Fprint(w, `{"schemaVersion":3,"chain":"eth","dex":"uniswap_v3","minLiquidityUsd":1000,
			"pairs":[{"contractAddress":"0xabc","pair":"WETH.USDC","reserveUsd":5e6,"confidence":0.9,"canonicalKey":null}]}`)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "tok", srv.Client())

	feed, changed, err := c.FetchPairFeed(context.Background(), "eth", "uniswap_v3", 0)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, 1000.0, feed.MinLiquidityUsd)
	require.Len(t, feed.Pairs, 1)
	require.Equal(t, 0.9, feed.Pairs[0].Confidence)
	require.Equal(t, "", feed.Pairs[0].CanonicalKey) // JSON null decodes to ""

	// ifModifiedSince=500 short-circuits to 304 -> not changed, nil feed.
	feed, changed, err = c.FetchPairFeed(context.Background(), "eth", "uniswap_v3", 500)
	require.NoError(t, err)
	require.False(t, changed)
	require.Nil(t, feed)
}

func TestFetchUnauthorised(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"unauthorised"}`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "bad", srv.Client()).FetchManifest(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestNoAuthHeaderWhenTokenEmpty(t *testing.T) {
	var hadAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadAuth = r.Header["Authorization"]
		fmt.Fprint(w, `{"schemaVersion":3,"supportedSources":[]}`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "", srv.Client()).FetchManifest(context.Background())
	require.NoError(t, err)
	require.False(t, hadAuth, "no Authorization header should be sent for an empty token")
}
