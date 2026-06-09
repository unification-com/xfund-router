package keystore

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"

	gethkeystore "github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
)

// This file holds the go-ethereum Web3 Secret Storage (v3) crypto core that
// replaces the bespoke AES-CFB/SHA256/weak-token keystore. The provider key is
// stored as a single standard v3 JSON blob encrypted with scrypt; at unlock the
// raw private key is recovered and fed to the existing chain signing path
// unchanged (no change to how fulfilments are signed).

// EncryptKeyV3 encrypts an ECDSA private key into a standard go-ethereum v3
// JSON keystore blob using scrypt with production parameters
// (StandardScryptN/P). Use this for all non-test callers.
func EncryptKeyV3(privateKey *ecdsa.PrivateKey, passphrase string) ([]byte, error) {
	return encryptKeyV3(privateKey, passphrase, gethkeystore.StandardScryptN, gethkeystore.StandardScryptP)
}

// encryptKeyV3 is the parameterised core. It is unexported so production code
// cannot accidentally pick weak (light) scrypt parameters; tests use the cheaper
// LightScryptN/P via this function directly.
func encryptKeyV3(privateKey *ecdsa.PrivateKey, passphrase string, scryptN, scryptP int) ([]byte, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("cannot encrypt a nil private key")
	}

	// go-ethereum's newKeyFromECDSA is unexported, so build the Key directly.
	// The id is a fresh random v4 uuid (purely an identifier, not key material).
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("could not generate keystore uuid: %w", err)
	}

	key := &gethkeystore.Key{
		Id:         id,
		Address:    crypto.PubkeyToAddress(privateKey.PublicKey),
		PrivateKey: privateKey,
	}

	return gethkeystore.EncryptKey(key, passphrase, scryptN, scryptP)
}

// DecryptKeyV3 decrypts a go-ethereum v3 JSON keystore blob with the given
// passphrase. It returns the recovered private key and the address derived
// from it. The derived address is recomputed from the key rather than trusted
// from the JSON, and is cross-checked against the address declared in the blob
// to detect a tampered or corrupt file.
func DecryptKeyV3(blob []byte, passphrase string) (*ecdsa.PrivateKey, common.Address, error) {
	key, err := gethkeystore.DecryptKey(blob, passphrase)
	if err != nil {
		return nil, common.Address{}, err
	}
	if key.PrivateKey == nil {
		return nil, common.Address{}, fmt.Errorf("decrypted keystore contained no private key")
	}

	derived := crypto.PubkeyToAddress(key.PrivateKey.PublicKey)
	if key.Address != (common.Address{}) && key.Address != derived {
		return nil, common.Address{}, fmt.Errorf(
			"keystore address mismatch: file declares %s but key derives %s",
			key.Address.Hex(), derived.Hex())
	}

	return key.PrivateKey, derived, nil
}

// IsV3Keystore reports whether data looks like a go-ethereum v3 JSON keystore
// blob. A v3 blob carries a "crypto" object (lower- or upper-case); the legacy
// go-ooo format does not.
func IsV3Keystore(data []byte) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return false
	}
	if _, ok := m["crypto"]; ok {
		return true
	}
	_, ok := m["Crypto"]
	return ok
}

// IsLegacyKeystore reports whether data is the bespoke go-ooo keystore format
// ({"keys":[...],"hash":...}) that predates the migration to v3.
func IsLegacyKeystore(data []byte) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return false
	}
	_, ok := m["keys"]
	return ok
}
