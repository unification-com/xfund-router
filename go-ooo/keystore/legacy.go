package keystore

import (
	"encoding/json"
	"fmt"

	"go-ooo/utils/walletworker"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/bcrypt"
)

// This file holds the decrypt-only path for the bespoke (pre-v3) go-ooo
// keystore. It exists ONLY so `keystore migrate` can read an old keystore.json,
// recover the private key and re-encrypt it in the standard v3 format. It is
// removed once migration is universal. It deliberately reuses the existing
// AES-CFB Decrypt helper rather than duplicating any crypto.

// LegacyKey is one decrypted entry recovered from a legacy keystore.
type LegacyKey struct {
	Account       string
	PrivateKeyHex string
	Address       common.Address
}

// legacyKeystoreFile mirrors the on-disk shape of the bespoke keystore. It is a
// local, minimal struct (account + cipherprivate + hash) so the migration does
// not depend on the live KeyStorageModel, whose bespoke fields are being removed.
type legacyKeystoreFile struct {
	Keys []struct {
		Account       string `json:"account"`
		CipherPrivate string `json:"cipherprivate"`
	} `json:"keys"`
	Hash string `json:"hash"`
}

// VerifyLegacyToken checks a candidate token against the bcrypt hash stored in a
// legacy keystore file. Returns nil only on a match.
func VerifyLegacyToken(data []byte, token string) error {
	var f legacyKeystoreFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse legacy keystore: %w", err)
	}
	if f.Hash == "" {
		return fmt.Errorf("legacy keystore has no token hash")
	}
	return bcrypt.CompareHashAndPassword([]byte(f.Hash), []byte(token))
}

// DecryptLegacyKeystore parses a legacy go-ooo keystore file and decrypts every
// stored private key with the supplied (weak) token, returning the recovered
// keys. The token is bcrypt-verified against the stored hash first so a wrong
// token fails cleanly rather than yielding garbage plaintext.
func DecryptLegacyKeystore(data []byte, token string) ([]LegacyKey, error) {
	if err := VerifyLegacyToken(data, token); err != nil {
		return nil, fmt.Errorf("token does not match legacy keystore: %w", err)
	}

	var f legacyKeystoreFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse legacy keystore: %w", err)
	}
	if len(f.Keys) == 0 {
		return nil, fmt.Errorf("legacy keystore contains no keys")
	}

	out := make([]LegacyKey, 0, len(f.Keys))
	for _, k := range f.Keys {
		// Decrypt is the existing legacy AES-CFB/SHA256 helper (keystore.go).
		privHex, err := Decrypt(k.CipherPrivate, token)
		if err != nil {
			return nil, fmt.Errorf("decrypt key for account %q: %w", k.Account, err)
		}
		addr, err := walletworker.AddressFromPrivateKeyString(privHex)
		if err != nil {
			return nil, fmt.Errorf("derive address for account %q: %w", k.Account, err)
		}
		out = append(out, LegacyKey{
			Account:       k.Account,
			PrivateKeyHex: privHex,
			Address:       addr,
		})
	}
	return out, nil
}
