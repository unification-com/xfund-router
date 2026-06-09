package keystore

import (
	"crypto/ecdsa"
	"fmt"

	"go-ooo/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// This file holds the pure migration core (no file I/O, no prompts) so it can be
// unit-tested directly. The cmd layer handles reading/writing files and prompting.

// SelectLegacyKey picks the key matching account, or the first key when account
// is empty or not found — mirroring the old SelectPrivateKey fallback. The
// returned others holds the account names that will NOT be migrated, so the
// caller can warn the operator about a multi-key keystore.
func SelectLegacyKey(keys []LegacyKey, account string) (chosen LegacyKey, others []string, err error) {
	if len(keys) == 0 {
		return LegacyKey{}, nil, fmt.Errorf("no keys to migrate")
	}

	idx := 0
	if account != "" {
		idx = -1
		for i, k := range keys {
			if k.Account == account {
				idx = i
				break
			}
		}
		if idx == -1 {
			// account not found — fall back to the first key, as SelectPrivateKey does.
			idx = 0
		}
	}

	chosen = keys[idx]
	for i, k := range keys {
		if i != idx {
			others = append(others, k.Account)
		}
	}
	return chosen, others, nil
}

// VerifyV3Keystore decrypts blob with pass and asserts it yields a working key
// whose derived address equals expected. It goes beyond a plain decrypt: it signs
// a probe digest and ecrecovers the signer, proving the recovered key can produce
// the fulfilment signatures the Router relies on. Returns nil only on a full match.
func VerifyV3Keystore(blob []byte, pass string, expected common.Address) error {
	priv, addr, err := DecryptKeyV3(blob, pass)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}
	if addr != expected {
		return fmt.Errorf("address mismatch: got %s, expected %s", addr.Hex(), expected.Hex())
	}

	digest := crypto.Keccak256([]byte("go-ooo keystore migration verification probe"))
	sig, err := crypto.Sign(digest, priv)
	if err != nil {
		return fmt.Errorf("sign probe: %w", err)
	}
	pub, err := crypto.SigToPub(digest, sig)
	if err != nil {
		return fmt.Errorf("ecrecover probe: %w", err)
	}
	if crypto.PubkeyToAddress(*pub) != expected {
		return fmt.Errorf("ecrecovered signer does not match expected address %s", expected.Hex())
	}
	return nil
}

// EncryptKeyV3Verified encrypts priv under newPass as a v3 blob and verifies the
// blob round-trips to a working key whose address equals expected, before
// returning it. A non-nil error means the blob must not be trusted or written.
func EncryptKeyV3Verified(priv *ecdsa.PrivateKey, newPass string, expected common.Address) ([]byte, error) {
	blob, err := EncryptKeyV3(priv, newPass)
	if err != nil {
		return nil, fmt.Errorf("encrypt v3 keystore: %w", err)
	}
	if err := VerifyV3Keystore(blob, newPass, expected); err != nil {
		return nil, fmt.Errorf("verify freshly-encrypted v3 keystore: %w", err)
	}
	return blob, nil
}

// MigrateLegacyToV3 is the pure migration core: it decrypts the selected key from
// a legacy keystore, re-encrypts it as a verified v3 blob under newPass, and
// returns the blob, the wallet address, and any non-migrated account names. It
// performs no file I/O — the caller writes, re-verifies from disk and deletes.
func MigrateLegacyToV3(legacyData []byte, oldToken, newPass, account string) (v3Blob []byte, addr common.Address, others []string, err error) {
	keys, err := DecryptLegacyKeystore(legacyData, oldToken)
	if err != nil {
		return nil, common.Address{}, nil, err
	}

	chosen, others, err := SelectLegacyKey(keys, account)
	if err != nil {
		return nil, common.Address{}, nil, err
	}

	priv, err := crypto.HexToECDSA(utils.RemoveHexPrefix(chosen.PrivateKeyHex))
	if err != nil {
		return nil, common.Address{}, nil, fmt.Errorf("parse recovered private key: %w", err)
	}

	blob, err := EncryptKeyV3Verified(priv, newPass, chosen.Address)
	if err != nil {
		return nil, common.Address{}, nil, err
	}
	return blob, chosen.Address, others, nil
}
