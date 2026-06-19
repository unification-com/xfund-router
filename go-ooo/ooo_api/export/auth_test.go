package export

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeSigner stands in for the keystore-backed EIP-191 signer (utils/walletworker.EthSigner).
type fakeSigner struct{ addr string }

func (f fakeSigner) Address() string                 { return f.addr }
func (f fakeSigner) SignText(string) (string, error) { return "0x" + strings.Repeat("ab", 65), nil }

// TestWalletAuthFlow covers the provider challenge-response: Bearer does challenge -> sign -> verify,
// caches the token, re-uses it without re-challenging, and re-auths after Invalidate. It also checks the
// request bodies carry the provider address + chainId + signature.
func TestWalletAuthFlow(t *testing.T) {
	var challengeCalls, verifyCalls int
	var challengeBody, verifyBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/auth/challenge":
			challengeCalls++
			_ = json.Unmarshal(body, &challengeBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "sign me", "nonce": "n1", "expiresAt": time.Now().Add(5 * time.Minute).Unix()})
		case "/auth/verify":
			verifyCalls++
			_ = json.Unmarshal(body, &verifyBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"token": "T" + string(rune('0'+verifyCalls)), "expiresAt": time.Now().Add(time.Hour).Unix()})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	wa := NewWalletAuth(srv.URL, 11155111, fakeSigner{addr: "0xProvider"}, srv.Client())

	tok, err := wa.Bearer(context.Background())
	require.NoError(t, err)
	require.Equal(t, "T1", tok)
	require.Equal(t, 1, challengeCalls)
	require.Equal(t, "0xProvider", challengeBody["address"])
	require.EqualValues(t, 11155111, challengeBody["chainId"])
	require.Equal(t, "0x"+strings.Repeat("ab", 65), verifyBody["signature"])

	// Cached: a second Bearer reuses the token, no new challenge.
	tok2, err := wa.Bearer(context.Background())
	require.NoError(t, err)
	require.Equal(t, "T1", tok2)
	require.Equal(t, 1, challengeCalls)

	// Invalidate forces a fresh challenge-response.
	wa.Invalidate()
	tok3, err := wa.Bearer(context.Background())
	require.NoError(t, err)
	require.Equal(t, "T2", tok3)
	require.Equal(t, 2, challengeCalls)
}

// TestClientReAuthsOn401 proves the export client invalidates a stale token and re-authenticates once
// on a 401: the server only accepts the bearer "good", which the WalletAuth issues on its SECOND verify.
func TestClientReAuthsOn401(t *testing.T) {
	var verifyCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/challenge":
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "m", "nonce": "n", "expiresAt": time.Now().Add(5 * time.Minute).Unix()})
		case "/auth/verify":
			verifyCalls++
			token := "bad"
			if verifyCalls >= 2 {
				token = "good"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"token": token, "expiresAt": time.Now().Add(time.Hour).Unix()})
		case "/export/manifest":
			if r.Header.Get("Authorization") != "Bearer good" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"schemaVersion": 3, "supportedSources": []any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	wa := NewWalletAuth(srv.URL, 1, fakeSigner{addr: "0xP"}, srv.Client())
	c := NewClient(srv.URL, wa, srv.Client())

	m, err := c.FetchManifest(context.Background())
	require.NoError(t, err) // first bearer "bad" -> 401 -> re-auth -> "good" -> 200
	require.Equal(t, 3, m.SchemaVersion)
	require.Equal(t, 2, verifyCalls) // re-authed exactly once
}
