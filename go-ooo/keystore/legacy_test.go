package keystore

import (
	"encoding/json"
	"testing"

	"go-ooo/utils/walletworker"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// buildLegacyFixture constructs an old-format keystore.json encrypted with the
// bespoke AES-CFB path under a known (weak) token, exactly as the pre-migration
// code would have written it.
func buildLegacyFixture(t *testing.T, account, privHex, token string) []byte {
	t.Helper()
	cipher, err := Encrypt(privHex, token)
	require.NoError(t, err)
	hash, err := bcrypt.GenerateFromPassword([]byte(token), 8)
	require.NoError(t, err)

	f := map[string]interface{}{
		"keys": []map[string]string{
			{"account": account, "cipherprivate": cipher},
		},
		"hash": string(hash),
	}
	data, err := json.Marshal(f)
	require.NoError(t, err)
	return data
}

func TestDecryptLegacyKeystore(t *testing.T) {
	const (
		account = "oracle"
		privHex = "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913"
		token   = "weaktokenexample"
	)
	data := buildLegacyFixture(t, account, privHex, token)

	require.True(t, IsLegacyKeystore(data))
	require.False(t, IsV3Keystore(data))

	keys, err := DecryptLegacyKeystore(data, token)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, account, keys[0].Account)
	require.Equal(t, privHex, keys[0].PrivateKeyHex)

	expectedAddr, err := walletworker.AddressFromPrivateKeyString(privHex)
	require.NoError(t, err)
	require.Equal(t, expectedAddr, keys[0].Address)
}

func TestDecryptLegacyKeystoreWrongToken(t *testing.T) {
	data := buildLegacyFixture(t, "oracle",
		"0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913", "right-token")

	require.Error(t, VerifyLegacyToken(data, "wrong-token"))
	_, err := DecryptLegacyKeystore(data, "wrong-token")
	require.Error(t, err)
}
