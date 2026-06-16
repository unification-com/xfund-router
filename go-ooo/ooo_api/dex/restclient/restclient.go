// Package restclient is the shared JSON-over-REST transport for go-ooo's non-subgraph price clients
// (the Cosmos sources: Osmosis SQS, Astroport). Those speak REST/JSON rather than the subgraph GraphQL
// transport, but each only needs "GET this URL, return the body, error on non-200" - so that lives here
// once rather than being copied into every client. It is a leaf package (imported by the per-DEX REST
// client packages, not by the dex package) so it introduces no import cycle.
package restclient

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// client is shared across calls; *http.Client is safe for concurrent use.
var client = &http.Client{Timeout: 30 * time.Second}

// Get performs an HTTP GET for url and returns the response body, erroring on any non-200 status. It
// sets Accept: application/json (the Cosmos REST APIs serve JSON).
func Get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("restclient: non-200 status %s for %s", resp.Status, url)
	}
	return io.ReadAll(resp.Body)
}
