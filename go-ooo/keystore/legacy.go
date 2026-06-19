package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

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

// ---------------------------------------------------------------------------
// Legacy AES-256-CFB scheme (key = SHA256(token)). Decrypt is used by the
// migration above; Encrypt is retained only to build legacy-format fixtures in
// tests. None of this uses math/rand — the brute-forceable weakness was in how
// the token was generated, not in this codec. Removed with this file once
// migration is universal.
// ---------------------------------------------------------------------------

// Decrypt reverses Encrypt: it base64-decodes cryptoText and AES-256-CFB decrypts
// it with SHA256(keyString) as the key.
func Decrypt(cryptoText string, keyString string) (plainTextString string, err error) {
	encrypted, err := base64.URLEncoding.DecodeString(cryptoText)
	if err != nil {
		return "", err
	}
	if len(encrypted) < aes.BlockSize {
		return "", fmt.Errorf("cipherText too short. It decodes to %v bytes but the minimum length is 16", len(encrypted))
	}

	decrypted, err := decryptAES(hashTo32Bytes(keyString), encrypted)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

func decryptAES(key, data []byte) ([]byte, error) {
	// split the input up in to the IV seed and then the actual encrypted data.
	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCFBDecrypter(block, iv)

	stream.XORKeyStream(data, data)
	return data, nil
}

// Encrypt AES-256-CFB encrypts plainText with SHA256(keyString) as the key and
// returns base64. Retained only for building legacy-format test fixtures.
func Encrypt(plainText string, keyString string) (cipherTextString string, err error) {
	key := hashTo32Bytes(keyString)
	encrypted, err := encryptAES(key, []byte(plainText))
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(encrypted), nil
}

func encryptAES(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// create two 'windows' in to the output slice.
	output := make([]byte, aes.BlockSize+len(data))
	iv := output[:aes.BlockSize]
	encrypted := output[aes.BlockSize:]

	// populate the IV slice with random data.
	if _, err = io.ReadFull(cryptoRand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)

	// note that encrypted is still a window in to the output slice
	stream.XORKeyStream(encrypted, data)
	return output, nil
}

// hashTo32Bytes computes a SHA256 digest of input for use as an AES-256 key.
func hashTo32Bytes(input string) []byte {
	data := sha256.Sum256([]byte(input))
	return data[0:]
}
