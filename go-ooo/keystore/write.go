package keystore

import (
	"crypto/ecdsa"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// WriteNewV3Keystore encrypts priv under passphrase and writes it as a standard
// go-ethereum v3 keystore file at path with 0600 permissions. It refuses to
// overwrite an existing file, and verifies the encrypted blob round-trips to the
// same key/address before writing. Returns the wallet address. Used by `init`.
func WriteNewV3Keystore(path string, priv *ecdsa.PrivateKey, passphrase string) (common.Address, error) {
	if _, err := os.Stat(path); err == nil {
		return common.Address{}, fmt.Errorf("keystore already exists at %s", path)
	} else if !os.IsNotExist(err) {
		return common.Address{}, err
	}

	addr := crypto.PubkeyToAddress(priv.PublicKey)
	blob, err := EncryptKeyV3Verified(priv, passphrase, addr)
	if err != nil {
		return common.Address{}, err
	}
	if err := os.WriteFile(path, blob, 0600); err != nil {
		return common.Address{}, err
	}
	return addr, nil
}
