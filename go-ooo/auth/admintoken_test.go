package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAndVerifyAdminToken(t *testing.T) {
	raw, hash, err := GenerateAdminToken()
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	require.NotEmpty(t, hash)

	require.True(t, VerifyAdminToken(hash, raw))
	require.False(t, VerifyAdminToken(hash, "wrong-token"))
	require.False(t, VerifyAdminToken("", raw))
	require.False(t, VerifyAdminToken(hash, ""))
}

func TestGenerateAdminTokenUnique(t *testing.T) {
	raw1, hash1, err := GenerateAdminToken()
	require.NoError(t, err)
	raw2, hash2, err := GenerateAdminToken()
	require.NoError(t, err)

	require.NotEqual(t, raw1, raw2)
	require.NotEqual(t, hash1, hash2)
	// each hash only validates its own token
	require.False(t, VerifyAdminToken(hash1, raw2))
	require.False(t, VerifyAdminToken(hash2, raw1))
}

func TestHashFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ksFile := filepath.Join(dir, "keystore.json")

	// sidecar is a sibling of the keystore file
	require.Equal(t, filepath.Join(dir, "admin_token.hash"), HashFilePath(ksFile))

	// missing file reads as empty, no error
	got, err := ReadHashFile(ksFile)
	require.NoError(t, err)
	require.Empty(t, got)

	_, hash, err := GenerateAdminToken()
	require.NoError(t, err)
	require.NoError(t, WriteHashFile(ksFile, hash))

	got, err = ReadHashFile(ksFile)
	require.NoError(t, err)
	require.Equal(t, hash, got)

	// written at 0600
	info, err := os.Stat(HashFilePath(ksFile))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
