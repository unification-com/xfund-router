// Package astroport is a minimal client for the Astroport public REST API (api.astroport.fi) - the
// Neutron analogue of the Osmosis SQS transport. One GET /api/pools?chainId=<id> returns every pool
// with its assets inline (each carrying denom + priceUSD) plus the pool's totalLiquidityUSD, so a
// CosmosSource can read a token's USD spot without a subgraph or a separate prices call. It is pure
// HTTP/JSON and deliberately does NOT import the dex package, so it stays transport-only and
// unit-testable (the dex package adapts it to PriceSource). The raw GET lives in the shared restclient
// package (both Cosmos clients use it).
package astroport

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"go-ooo/ooo_api/dex/restclient"
)

// DefaultBaseURL is the public Astroport API instance.
const DefaultBaseURL = "https://api.astroport.fi"

// chainIDs maps our internal chain key to the chainId the Astroport API is queried by. Only chains we
// curate are listed; an unmapped chain yields an unconfigured client that prices nothing.
var chainIDs = map[string]string{
	"neutron": "neutron-1",
}

// asset is the slice of an Astroport pool asset we consume: a denom and its USD spot price.
type asset struct {
	Denom    string  `json:"denom"`
	PriceUSD float64 `json:"priceUSD"`
}

// pool is the slice of an Astroport pool we consume (the endpoint returns more per pool).
type pool struct {
	Assets            []asset `json:"assets"`
	TotalLiquidityUSD float64 `json:"totalLiquidityUSD"`
	IsDeregistered    bool    `json:"isDeregistered"`
}

// Client talks to the Astroport REST API for a single chain. Construct with NewClient; a custom
// baseURL lets a test point it at an httptest server.
type Client struct {
	baseURL string
	chainID string
}

// NewClient returns a Client for baseURL (empty = the public instance) querying chain's Astroport
// pools. If chain is not one we map to an Astroport chainId the client is unconfigured (Configured
// reports false) and prices nothing - the caller skips it.
func NewClient(baseURL, chain string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		chainID: chainIDs[chain],
	}
}

// Configured reports whether the client's chain maps to a known Astroport chainId.
func (c *Client) Configured() bool { return c.chainID != "" }

// TokenPricesUsd returns the USD spot price of each requested denom. It fetches the whole pool set once
// (GET /api/pools?chainId=<id>) and, for each denom, takes the priceUSD from the DEEPEST active pool
// holding it - robust whether priceUSD is a global token price (identical everywhere) or a per-pool
// figure (deepest pool = most reliable). A denom absent from every active pool is simply absent from
// the result (not an error), mirroring the SQS client. A pair price is then base-USD ÷ target-USD
// (computed by the caller).
func (c *Client) TokenPricesUsd(denoms []string) (map[string]float64, error) {
	out := make(map[string]float64, len(denoms))
	if len(denoms) == 0 || c.chainID == "" {
		return out, nil
	}

	pools, err := c.fetchPools()
	if err != nil {
		return nil, err
	}

	want := make(map[string]bool, len(denoms))
	for _, d := range denoms {
		want[d] = true
	}
	// denom -> liquidity of the pool we took its price from, so a deeper pool's price wins.
	deepest := make(map[string]float64, len(denoms))
	for _, p := range pools {
		if p.IsDeregistered {
			continue
		}
		for _, a := range p.Assets {
			if !want[a.Denom] || a.PriceUSD <= 0 {
				continue
			}
			if _, seen := out[a.Denom]; !seen || p.TotalLiquidityUSD > deepest[a.Denom] {
				out[a.Denom] = a.PriceUSD
				deepest[a.Denom] = p.TotalLiquidityUSD
			}
		}
	}
	return out, nil
}

func (c *Client) fetchPools() ([]pool, error) {
	u := fmt.Sprintf("%s/api/pools?chainId=%s", c.baseURL, url.QueryEscape(c.chainID))
	body, err := restclient.Get(u)
	if err != nil {
		return nil, err
	}
	var pools []pool
	if err := json.Unmarshal(body, &pools); err != nil {
		return nil, fmt.Errorf("astroport: decode pools response: %w", err)
	}
	return pools, nil
}
