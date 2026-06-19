package keystore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestWriteNewV3Keystore(t *testing.T) {
	dir := t.TempDir()
	ksFile := filepath.Join(dir, "keystore.json")

	priv, err := crypto.HexToECDSA(testKeyHex)
	require.NoError(t, err)

	const pass = "operator passphrase"
	addr, err := WriteNewV3Keystore(ksFile, priv, pass)
	require.NoError(t, err)
	require.Equal(t, crypto.PubkeyToAddress(priv.PublicKey), addr)

	info, err := os.Stat(ksFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())

	data, err := os.ReadFile(ksFile)
	require.NoError(t, err)
	require.True(t, IsV3Keystore(data))

	got, gotAddr, err := DecryptKeyV3(data, pass)
	require.NoError(t, err)
	require.Equal(t, crypto.FromECDSA(priv), crypto.FromECDSA(got))
	require.Equal(t, addr, gotAddr)
}

func TestWriteNewV3KeystoreRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	ksFile := filepath.Join(dir, "keystore.json")

	priv, err := crypto.HexToECDSA(testKeyHex)
	require.NoError(t, err)

	_, err = WriteNewV3Keystore(ksFile, priv, "passphrase")
	require.NoError(t, err)

	_, err = WriteNewV3Keystore(ksFile, priv, "passphrase")
	require.Error(t, err)
}
