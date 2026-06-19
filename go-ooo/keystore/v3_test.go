package keystore

import (
	"testing"

	gethkeystore "github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// testKeyHex is the dev-env oracle key (account #3) — used only as known,
// deterministic test material.
const testKeyHex = "646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913"

func TestEncryptDecryptKeyV3RoundTrip(t *testing.T) {
	priv, err := crypto.HexToECDSA(testKeyHex)
	require.NoError(t, err)

	const pass = "correct horse battery staple"
	blob, err := encryptKeyV3(priv, pass, gethkeystore.LightScryptN, gethkeystore.LightScryptP)
	require.NoError(t, err)
	require.True(t, IsV3Keystore(blob))
	require.False(t, IsLegacyKeystore(blob))

	got, addr, err := DecryptKeyV3(blob, pass)
	require.NoError(t, err)
	require.Equal(t, crypto.FromECDSA(priv), crypto.FromECDSA(got))
	require.Equal(t, crypto.PubkeyToAddress(priv.PublicKey), addr)
}

func TestDecryptKeyV3WrongPassphrase(t *testing.T) {
	priv, err := crypto.HexToECDSA(testKeyHex)
	require.NoError(t, err)
	blob, err := encryptKeyV3(priv, "right", gethkeystore.LightScryptN, gethkeystore.LightScryptP)
	require.NoError(t, err)

	_, _, err = DecryptKeyV3(blob, "wrong")
	require.Error(t, err)
}

// TestKeyV3SignatureUnchanged is the proof behind the Option-1 design choice:
// a key recovered from the v3 blob signs byte-for-byte identically to the
// original key, so moving storage to v3 cannot change the fulfilment signatures
// the Router verifies.
func TestKeyV3SignatureUnchanged(t *testing.T) {
	priv, err := crypto.HexToECDSA(testKeyHex)
	require.NoError(t, err)

	const pass = "passphrase"
	blob, err := encryptKeyV3(priv, pass, gethkeystore.LightScryptN, gethkeystore.LightScryptP)
	require.NoError(t, err)
	recovered, _, err := DecryptKeyV3(blob, pass)
	require.NoError(t, err)

	// The same shape the fulfilment path signs: a 32-byte digest.
	digest := crypto.Keccak256([]byte("fulfilment digest"))

	sigOrig, err := crypto.Sign(digest, priv)
	require.NoError(t, err)
	sigRecovered, err := crypto.Sign(digest, recovered)
	require.NoError(t, err)
	require.Equal(t, sigOrig, sigRecovered)

	// And it ecrecovers to the oracle address.
	pub, err := crypto.SigToPub(digest, sigRecovered)
	require.NoError(t, err)
	require.Equal(t, crypto.PubkeyToAddress(priv.PublicKey), crypto.PubkeyToAddress(*pub))
}

func TestFormatDetection(t *testing.T) {
	priv, err := crypto.HexToECDSA(testKeyHex)
	require.NoError(t, err)
	v3, err := encryptKeyV3(priv, "p", gethkeystore.LightScryptN, gethkeystore.LightScryptP)
	require.NoError(t, err)

	legacy := []byte(`{"keys":[{"account":"a","cipherprivate":"x"}],"hash":"y"}`)

	require.True(t, IsV3Keystore(v3))
	require.False(t, IsLegacyKeystore(v3))
	require.True(t, IsLegacyKeystore(legacy))
	require.False(t, IsV3Keystore(legacy))

	require.False(t, IsV3Keystore([]byte("not json")))
	require.False(t, IsLegacyKeystore([]byte("not json")))
}
