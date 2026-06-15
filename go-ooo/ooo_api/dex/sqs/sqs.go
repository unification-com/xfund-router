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

// TokenPricesUsd returns the USD spot price of each denom. SQS GET /tokens/prices?base=<d1,d2,...>
// responds with {"<denom>": {"<usdDenom>": "<price>"}}, where usdDenom is the USD unit SQS quotes in
// (the native USDC). It reads that single quote per denom - DISCOVERING the usdDenom from the response
// rather than passing it, so no denom is hard-coded and any USDC bridge variant a symbol resolved to
// still prices at ~1. A pair price is then base-USD ÷ target-USD (computed by the caller). A denom SQS
// cannot price is simply absent from the result (not an error).
func (c *Client) TokenPricesUsd(denoms []string) (map[string]float64, error) {
	out := make(map[string]float64, len(denoms))
	if len(denoms) == 0 {
		return out, nil
	}
	url := fmt.Sprintf("%s/tokens/prices?base=%s", c.baseURL, strings.Join(denoms, ","))
	body, err := c.get(url)
	if err != nil {
		return nil, err
	}
	var resp map[string]map[string]string
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("sqs: decode prices response: %w", err)
	}
	for denom, quotes := range resp {
		for _, priceStr := range quotes {
			if p, perr := strconv.ParseFloat(priceStr, 64); perr == nil && p > 0 {
				out[denom] = p
			}
			break // a single USD quote per denom
		}
	}
	return out, nil
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
