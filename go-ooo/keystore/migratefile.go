package keystore

import (
	"errors"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
)

// ErrAlreadyV3 is returned by MigrateKeystoreFile when the target file is already
// in the go-ethereum v3 format, so the caller can report a friendly no-op.
var ErrAlreadyV3 = errors.New("keystore is already in v3 format")

// migratingSuffix is appended to the keystore path while the new v3 file is
// written and verified, before the old file is removed and the new one moved
// into place.
const migratingSuffix = ".migrating"

// MigrateKeystoreFile migrates the legacy keystore at ksFile to the go-ethereum
// v3 (scrypt) format under newPass. It implements the verify-then-delete
// invariant: the new file is written to a sibling temp path and verified from
// disk BEFORE the old (weak-token) file is destroyed. The old file is never
// overwritten in place and no backup is kept — a copy encrypted under the weak
// token is itself the vulnerability being removed.
//
// On any failure before verification the old file is left untouched and the
// partial new file is removed. It returns the migrated wallet address and the
// names of any other accounts in the legacy keystore that were not migrated.
func MigrateKeystoreFile(ksFile, account, oldToken, newPass string) (common.Address, []string, error) {
	data, err := os.ReadFile(ksFile)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("read keystore %s: %w", ksFile, err)
	}

	if IsV3Keystore(data) {
		return common.Address{}, nil, ErrAlreadyV3
	}
	if !IsLegacyKeystore(data) {
		return common.Address{}, nil, fmt.Errorf("%s is not a recognised legacy keystore", ksFile)
	}

	// Decrypt + re-encrypt + in-memory verify. No file has been touched yet, so
	// a failure here (e.g. a wrong old password) leaves the old keystore intact.
	blob, addr, others, err := MigrateLegacyToV3(data, oldToken, newPass, account)
	if err != nil {
		return common.Address{}, nil, err
	}

	// Write the new v3 blob to a sibling temp file. Never overwrite the old file.
	tmpPath := ksFile + migratingSuffix
	if err := os.WriteFile(tmpPath, blob, 0600); err != nil {
		return common.Address{}, nil, fmt.Errorf("write new keystore: %w", err)
	}

	// Authoritative verification: re-read the bytes that actually landed on disk
	// and prove they decrypt to the same working key/address.
	written, err := os.ReadFile(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return common.Address{}, nil, fmt.Errorf("re-read new keystore for verification: %w", err)
	}
	if err := VerifyV3Keystore(written, newPass, addr); err != nil {
		_ = os.Remove(tmpPath)
		return common.Address{}, nil, fmt.Errorf(
			"verification of the new keystore FAILED, old keystore left untouched: %w", err)
	}

	// Verified. Securely remove the weak-token old file (no backup), then move
	// the verified new file into its place.
	if err := secureDeleteFile(ksFile); err != nil {
		return common.Address{}, nil, fmt.Errorf(
			"new keystore verified at %s but the OLD file %s could not be removed "+
				"(remove it manually, then rename the new file): %w", tmpPath, ksFile, err)
	}
	if err := os.Rename(tmpPath, ksFile); err != nil {
		return common.Address{}, nil, fmt.Errorf(
			"new keystore verified at %s but could not be moved to %s "+
				"(rename it manually): %w", tmpPath, ksFile, err)
	}

	return addr, others, nil
}

// secureDeleteFile best-effort overwrites a regular file's contents with zeros
// before removing it, so a sensitive file is not trivially recoverable from free
// space. True secure erase is not guaranteed on modern SSDs or copy-on-write
// filesystems; this reduces casual recovery and is documented as best-effort.
// Overwrite failures are non-fatal — removal is the part that must succeed.
func secureDeleteFile(path string) error {
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		if f, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
			zeros := make([]byte, info.Size())
			_, _ = f.WriteAt(zeros, 0)
			_ = f.Sync()
			_ = f.Close()
		}
	}
	return os.Remove(path)
}
