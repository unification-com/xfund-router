package keystore

import (
	"os"
	"path/filepath"
	"testing"

	gethkeystore "github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestMigrateKeystoreFileSuccess(t *testing.T) {
	const (
		account = "oracle"
		privHex = "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913"
		oldTok  = "weak-token"
		newPass = "strong operator passphrase"
	)
	dir := t.TempDir()
	ksFile := filepath.Join(dir, "keystore.json")
	require.NoError(t, os.WriteFile(ksFile, buildLegacyFixture(t, account, privHex, oldTok), 0600))

	addr, others, err := MigrateKeystoreFile(ksFile, account, oldTok, newPass)
	require.NoError(t, err)
	require.Empty(t, others)

	// The file is now a v3 keystore holding the original key.
	data, err := os.ReadFile(ksFile)
	require.NoError(t, err)
	require.True(t, IsV3Keystore(data))
	require.False(t, IsLegacyKeystore(data))

	priv, gotAddr, err := DecryptKeyV3(data, newPass)
	require.NoError(t, err)
	expected, _ := crypto.HexToECDSA("646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913")
	require.Equal(t, crypto.FromECDSA(expected), crypto.FromECDSA(priv))
	require.Equal(t, crypto.PubkeyToAddress(expected.PublicKey), addr)
	require.Equal(t, addr, gotAddr)

	// No temp file left behind.
	_, statErr := os.Stat(ksFile + migratingSuffix)
	require.True(t, os.IsNotExist(statErr))
}

func TestMigrateKeystoreFileWrongTokenLeavesOldIntact(t *testing.T) {
	const account = "oracle"
	dir := t.TempDir()
	ksFile := filepath.Join(dir, "keystore.json")
	fixture := buildLegacyFixture(t, account,
		"0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913", "right-token")
	require.NoError(t, os.WriteFile(ksFile, fixture, 0600))

	_, _, err := MigrateKeystoreFile(ksFile, account, "WRONG-token", "newpass1234")
	require.Error(t, err)

	// The old file is byte-for-byte unchanged and no temp file exists.
	after, err := os.ReadFile(ksFile)
	require.NoError(t, err)
	require.Equal(t, fixture, after)
	_, statErr := os.Stat(ksFile + migratingSuffix)
	require.True(t, os.IsNotExist(statErr))
}

func TestMigrateKeystoreFileAlreadyV3(t *testing.T) {
	dir := t.TempDir()
	ksFile := filepath.Join(dir, "keystore.json")
	priv, _ := crypto.HexToECDSA(testKeyHex)
	blob, err := encryptKeyV3(priv, "pass", gethkeystore.LightScryptN, gethkeystore.LightScryptP)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(ksFile, blob, 0600))

	_, _, err = MigrateKeystoreFile(ksFile, "oracle", "x", "y")
	require.ErrorIs(t, err, ErrAlreadyV3)

	after, err := os.ReadFile(ksFile)
	require.NoError(t, err)
	require.Equal(t, blob, after)
}

func TestMigrateKeystoreFileNotAKeystore(t *testing.T) {
	dir := t.TempDir()
	ksFile := filepath.Join(dir, "keystore.json")
	require.NoError(t, os.WriteFile(ksFile, []byte(`{"foo":"bar"}`), 0600))

	_, _, err := MigrateKeystoreFile(ksFile, "oracle", "x", "y")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrAlreadyV3)
}
