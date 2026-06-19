package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// This package owns the admin HTTP API bearer token. It is deliberately decoupled
// from the provider keystore: the bearer is its own crypto/rand secret, and only
// its bcrypt hash is persisted (in a 0600 sidecar next to the keystore). A leaked
// bearer therefore no longer implies private-key compromise.

// adminTokenHashFile is the sidecar filename holding the bcrypt hash of the admin
// bearer. It lives alongside the keystore in the app home directory.
const adminTokenHashFile = "admin_token.hash"

// GenerateAdminToken creates a new admin HTTP bearer using crypto/rand and returns
// the raw token (shown to the operator once) together with its bcrypt hash, which
// is the only form persisted.
func GenerateAdminToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate admin token: %w", err)
	}
	raw = hex.EncodeToString(b)

	h, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("hash admin token: %w", err)
	}
	return raw, string(h), nil
}

// VerifyAdminToken reports whether candidate matches the stored bcrypt hash. An
// empty hash or candidate never matches. bcrypt's comparison is inherently
// resistant to timing attacks (the KDF cost dominates the runtime).
func VerifyAdminToken(hash, candidate string) bool {
	if hash == "" || candidate == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(candidate)) == nil
}

// HashFilePath returns the admin-token hash sidecar path, derived as a sibling of
// the keystore file so both secrets live together in the home directory.
func HashFilePath(keystoreFile string) string {
	return filepath.Join(filepath.Dir(keystoreFile), adminTokenHashFile)
}

// WriteHashFile writes the bcrypt hash to the sidecar at 0600.
func WriteHashFile(keystoreFile, hash string) error {
	return os.WriteFile(HashFilePath(keystoreFile), []byte(hash), 0600)
}

// ReadHashFile reads the bcrypt hash from the sidecar. A missing file returns an
// empty hash and no error, so callers can treat "no admin token configured" as a
// disabled admin API rather than a hard failure.
func ReadHashFile(keystoreFile string) (string, error) {
	data, err := os.ReadFile(HashFilePath(keystoreFile))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
