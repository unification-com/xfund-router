// Package sqs is a minimal client for the Osmosis Sidecar Query Server (SQS) public REST API
// (https://sqs.osmosis.zone) - the Cosmos analogue of the subgraph GraphQL transport. It returns
// spot token prices (one token quoted in another token's denom), so a CosmosSqsSource can price
// Osmosis pairs without a subgraph. It is pure HTTP/JSON and deliberately does NOT import the dex
// package, so it stays transport-only and unit-testable (the dex package adapts it to PriceSource).
package sqs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the public Osmosis SQS instance.
const DefaultBaseURL = "https://sqs.osmosis.zone"

// Client talks to an Osmosis SQS REST endpoint. Construct with NewClient; a custom baseURL lets a
// test point it at an httptest server.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a Client for baseURL (empty = the public instance, DefaultBaseURL).
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// TokenPrice returns the spot price of baseDenom quoted in quoteDenom - e.g. "uosmo" quoted in the
// USDC IBC denom gives OSMO's price in USDC. SQS GET /tokens/prices?base=<denom> responds with
// {"<baseDenom>": {"<quoteDenom>": "<price>"}}; this reads out the requested quote.
func (c *Client) TokenPrice(baseDenom, quoteDenom string) (float64, error) {
	url := fmt.Sprintf("%s/tokens/prices?base=%s", c.baseURL, baseDenom)
	body, err := c.get(url)
	if err != nil {
		return 0, err
	}
	var resp map[string]map[string]string
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("sqs: decode prices response: %w", err)
	}
	quotes, ok := resp[baseDenom]
	if !ok {
		return 0, fmt.Errorf("sqs: no prices returned for base denom %s", baseDenom)
	}
	priceStr, ok := quotes[quoteDenom]
	if !ok {
		return 0, fmt.Errorf("sqs: no %s quote for base denom %s", quoteDenom, baseDenom)
	}
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return 0, fmt.Errorf("sqs: unparseable price %q for base denom %s: %w", priceStr, baseDenom, err)
	}
	return price, nil
}

func (c *Client) get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sqs: non-200 status %s for %s", resp.Status, url)
	}
	return io.ReadAll(resp.Body)
}
