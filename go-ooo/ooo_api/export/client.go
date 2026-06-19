package export

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// defaultTimeout bounds a single export request. The feeds are small JSON; a slow response is
// almost always a dead/overloaded host we'd rather fail fast on and (later) fall back from.
const defaultTimeout = 30 * time.Second

// Client fetches the dex-pair-verify provider export over HTTP from one base URL. It speaks the
// API-primary contract: a Bearer token + the `?ifModifiedSince=<unix>` 304 short-circuit on the
// per-source pair feeds. An empty apiToken (the unauthenticated GitHub mirror) simply omits the
// Authorization header. The dual-source primary/fallback orchestration lives in the consumer that
// owns two Clients; a single Client talks to a single source.
type Client struct {
	baseURL string
	auth    Authenticator
	http    *http.Client
}

// NewClient builds a Client for baseURL (trailing slash trimmed) authenticating via auth (StaticToken
// for a shared bearer / break-glass, or NewWalletAuth for provider wallet-auth - T8). A nil auth means
// no Authorization header; a nil httpClient gets a sane default.
func NewClient(baseURL string, auth Authenticator, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if auth == nil {
		auth = StaticToken("")
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		auth:    auth,
		http:    httpClient,
	}
}

// FetchManifest GETs the v3 discovery manifest. The manifest endpoint has no 304 short-circuit,
// so this always returns the current manifest (or an error). A manifest whose major schema
// version is newer than this build understands is rejected rather than silently mis-decoded.
func (c *Client) FetchManifest(ctx context.Context) (*Manifest, error) {
	var m Manifest
	status, err := c.get(ctx, c.baseURL+"/export/manifest", &m)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("manifest: unexpected status %d", status)
	}
	if m.SchemaVersion > ManifestSchemaVersion {
		return nil, fmt.Errorf("manifest schema v%d is newer than this build supports (v%d)", m.SchemaVersion, ManifestSchemaVersion)
	}
	return &m, nil
}

// FetchPairFeed GETs one source's verified-pair feed, passing sinceUnix (when > 0) as
// `?ifModifiedSince` for a 304 short-circuit. changed is false when the server replied 304 (the
// feed is unchanged since sinceUnix) - the returned *PairFeed is nil in that case.
func (c *Client) FetchPairFeed(ctx context.Context, chain, dex string, sinceUnix int64) (feed *PairFeed, changed bool, err error) {
	u := fmt.Sprintf("%s/export/%s/%s", c.baseURL, url.PathEscape(chain), url.PathEscape(dex))
	if sinceUnix > 0 {
		u += "?ifModifiedSince=" + url.QueryEscape(fmt.Sprintf("%d", sinceUnix))
	}

	var f PairFeed
	status, err := c.get(ctx, u, &f)
	if err != nil {
		return nil, false, err
	}
	if status == http.StatusNotModified {
		return nil, false, nil
	}
	if f.SchemaVersion > PairSchemaVersion {
		return nil, false, fmt.Errorf("pair feed %s/%s schema v%d is newer than this build supports (v%d)", chain, dex, f.SchemaVersion, PairSchemaVersion)
	}
	return &f, true, nil
}

// get issues an authenticated GET and returns the status code, re-authenticating once on a 401. On 200
// it decodes the body into v; on 304 it returns the status with v untouched; on any other status it
// returns an error carrying a snippet of the body. Transport errors return (0, err).
func (c *Client) get(ctx context.Context, rawURL string, v any) (int, error) {
	return c.doGet(ctx, rawURL, v, false)
}

func (c *Client) doGet(ctx context.Context, rawURL string, v any, retried bool) (int, error) {
	bearer, err := c.auth.Bearer(ctx)
	if err != nil {
		return 0, fmt.Errorf("export auth: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return resp.StatusCode, fmt.Errorf("decode %s: %w", rawURL, err)
		}
		return resp.StatusCode, nil
	case http.StatusNotModified:
		return resp.StatusCode, nil
	case http.StatusUnauthorized:
		if !retried {
			// A stale/expired bearer - drop it and re-authenticate exactly once.
			c.auth.Invalidate()
			return c.doGet(ctx, rawURL, v, true)
		}
		fallthrough
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return resp.StatusCode, fmt.Errorf("GET %s: status %d: %s", rawURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}
