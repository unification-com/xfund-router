package keystore

import (
	"testing"

	gethkeystore "github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestMigrateLegacyToV3(t *testing.T) {
	const (
		account = "oracle"
		privHex = "0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913"
		oldTok  = "weak-legacy-token"
		newPass = "a much stronger operator passphrase"
	)
	data := buildLegacyFixture(t, account, privHex, oldTok)

	blob, addr, others, err := MigrateLegacyToV3(data, oldTok, newPass, account)
	require.NoError(t, err)
	require.Empty(t, others)
	require.True(t, IsV3Keystore(blob))

	// The migrated blob must hold exactly the original key.
	priv, gotAddr, err := DecryptKeyV3(blob, newPass)
	require.NoError(t, err)
	expected, err := crypto.HexToECDSA("646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913")
	require.NoError(t, err)
	require.Equal(t, crypto.FromECDSA(expected), crypto.FromECDSA(priv))
	require.Equal(t, crypto.PubkeyToAddress(expected.PublicKey), addr)
	require.Equal(t, addr, gotAddr)
}

func TestMigrateLegacyToV3WrongToken(t *testing.T) {
	data := buildLegacyFixture(t, "oracle",
		"0x646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913", "right")
	_, _, _, err := MigrateLegacyToV3(data, "wrong", "newpass", "oracle")
	require.Error(t, err)
}

func TestSelectLegacyKey(t *testing.T) {
	keys := []LegacyKey{
		{Account: "a", PrivateKeyHex: "0x01", Address: common.HexToAddress("0xaa")},
		{Account: "b", PrivateKeyHex: "0x02", Address: common.HexToAddress("0xbb")},
	}

	chosen, others, err := SelectLegacyKey(keys, "b")
	require.NoError(t, err)
	require.Equal(t, "b", chosen.Account)
	require.Equal(t, []string{"a"}, others)

	// missing account falls back to the first key
	chosen, others, err = SelectLegacyKey(keys, "zzz")
	require.NoError(t, err)
	require.Equal(t, "a", chosen.Account)
	require.Equal(t, []string{"b"}, others)

	// empty account falls back to the first key
	chosen, _, err = SelectLegacyKey(keys, "")
	require.NoError(t, err)
	require.Equal(t, "a", chosen.Account)

	// empty list is an error
	_, _, err = SelectLegacyKey(nil, "a")
	require.Error(t, err)
}

func TestVerifyV3KeystoreAddressMismatch(t *testing.T) {
	priv, err := crypto.HexToECDSA(testKeyHex)
	require.NoError(t, err)
	blob, err := encryptKeyV3(priv, "p", gethkeystore.LightScryptN, gethkeystore.LightScryptP)
	require.NoError(t, err)

	// Verifying against a different address must fail.
	err = VerifyV3Keystore(blob, "p", common.HexToAddress("0x000000000000000000000000000000000000dEaD"))
	require.Error(t, err)

	// Verifying with the right address succeeds.
	err = VerifyV3Keystore(blob, "p", crypto.PubkeyToAddress(priv.PublicKey))
	require.NoError(t, err)
}
