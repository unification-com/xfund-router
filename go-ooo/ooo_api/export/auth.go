package export

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Authenticator supplies the Bearer token for an export request and lets the client drop a stale token
// (on a 401) so the next call re-authenticates. StaticToken is the operator break-glass / simple-setup
// path (the shared EXPORT_API_TOKEN); WalletAuth is the going-forward provider wallet challenge-response
// (T8) that proves control of a wallet registered on the OoO Router.
type Authenticator interface {
	// Bearer returns the token to send (empty = no Authorization header), refreshing it when needed.
	Bearer(ctx context.Context) (string, error)
	// Invalidate drops any cached token so the next Bearer re-authenticates (called on a 401).
	Invalidate()
}

// staticToken authenticates with a fixed bearer. An empty token sends no Authorization header (an
// unauthenticated source).
type staticToken struct{ token string }

// StaticToken builds an Authenticator that always presents the same bearer.
func StaticToken(token string) Authenticator { return staticToken{token: token} }

func (s staticToken) Bearer(context.Context) (string, error) { return s.token, nil }
func (s staticToken) Invalidate()                            {}

// Signer is the slice of the provider wallet that WalletAuth needs: the provider address and an EIP-191
// personal_sign over a message. Implemented in utils/walletworker over the oracle keystore key, so this
// package stays free of crypto/go-ethereum imports.
type Signer interface {
	Address() string                         // 0x provider address (the registered OoO provider)
	SignText(message string) (string, error) // EIP-191 personal_sign -> 0x hex (65-byte) signature
}

// tokenRefreshSkew re-authenticates this long before a token's stated expiry, to avoid racing it.
const tokenRefreshSkew = 60 * time.Second

// WalletAuth authenticates as a registered OoO provider (T8): POST /auth/challenge -> sign the returned
// message with the provider key -> POST /auth/verify -> cache the bearer until shortly before it
// expires. Thread-safe; re-auths on Invalidate (a 401) or expiry. The token is held in memory only - a
// restart simply re-auths (two cheap calls), which is fine for the export poll cadence.
type WalletAuth struct {
	baseURL string
	chainID int64
	signer  Signer
	http    *http.Client

	mu     sync.Mutex
	token  string
	expiry time.Time
}

// NewWalletAuth builds a wallet authenticator against the dpv API baseURL (the same base the export is
// served from; /auth/challenge + /auth/verify hang off it), for the given chainId + signer.
func NewWalletAuth(baseURL string, chainID int64, signer Signer, httpClient *http.Client) *WalletAuth {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &WalletAuth{baseURL: strings.TrimRight(baseURL, "/"), chainID: chainID, signer: signer, http: httpClient}
}

type challengeResp struct {
	Message   string `json:"message"`
	Nonce     string `json:"nonce"`
	ExpiresAt int64  `json:"expiresAt"`
}

type verifyResp struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
}

func (w *WalletAuth) Bearer(ctx context.Context) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.token != "" && time.Now().Before(w.expiry.Add(-tokenRefreshSkew)) {
		return w.token, nil
	}

	var ch challengeResp
	if err := w.postJSON(ctx, "/auth/challenge", map[string]any{"address": w.signer.Address(), "chainId": w.chainID}, &ch); err != nil {
		return "", fmt.Errorf("provider auth challenge: %w", err)
	}

	sig, err := w.signer.SignText(ch.Message)
	if err != nil {
		return "", fmt.Errorf("provider auth sign: %w", err)
	}

	var vr verifyResp
	if err := w.postJSON(ctx, "/auth/verify", map[string]any{"address": w.signer.Address(), "chainId": w.chainID, "signature": sig}, &vr); err != nil {
		return "", fmt.Errorf("provider auth verify: %w", err)
	}

	w.token = vr.Token
	w.expiry = time.Unix(vr.ExpiresAt, 0)
	return w.token, nil
}

func (w *WalletAuth) Invalidate() {
	w.mu.Lock()
	w.token = ""
	w.mu.Unlock()
}

// postJSON POSTs body as JSON to baseURL+path and decodes a 200 body into out; any other status is an
// error carrying a body snippet.
func (w *WalletAuth) postJSON(ctx context.Context, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snip, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(snip)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
