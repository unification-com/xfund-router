package walletworker_test

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"testing"

	"go-ooo/utils/walletworker"
)

func TestGeneratePrivate(t *testing.T) {
	pk, hex, err := walletworker.GeneratePrivate()

	require.NoError(t, err)
	require.NotNil(t, pk)
	require.NotNil(t, hex)
}

func TestStringToPrivate(t *testing.T) {
	pk, err := walletworker.StringToPrivate("646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913")
	pubKey := hexutil.Encode(crypto.FromECDSAPub(pk.Public().(*ecdsa.PublicKey)))
	require.NoError(t, err)
	require.NotNil(t, pk)
	require.Equal(t, "0x0478b7e11a2b1b66504963ba9cfcccc69a37b6fc8138e51b6e82effba9dc6f8d8110892e9aab67621bd770eeccb9bd9397e3c5ca3be9a04423e51950ed679f887a", pubKey)
}

func TestGeneratePublic(t *testing.T) {
	pk, _ := walletworker.StringToPrivate("646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913")
	pubKey, pubKeyStr := walletworker.GeneratePublic(pk)

	pubFrom := hexutil.Encode(crypto.FromECDSAPub(pubKey))

	require.Equal(t, "0x0478b7e11a2b1b66504963ba9cfcccc69a37b6fc8138e51b6e82effba9dc6f8d8110892e9aab67621bd770eeccb9bd9397e3c5ca3be9a04423e51950ed679f887a", pubFrom)
	require.Equal(t, "0x0478b7e11a2b1b66504963ba9cfcccc69a37b6fc8138e51b6e82effba9dc6f8d8110892e9aab67621bd770eeccb9bd9397e3c5ca3be9a04423e51950ed679f887a", pubKeyStr)
}

func TestGenerateAddress(t *testing.T) {
	pk, _ := walletworker.StringToPrivate("646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913")
	pubKey, _ := walletworker.GeneratePublic(pk)
	addr, addrStr := walletworker.GenerateAddress(pubKey)

	require.Equal(t, "0xE11BA2b4D45Eaed5996Cd0823791E0C93114882d", addr.Hex())
	require.Equal(t, "0xE11BA2b4D45Eaed5996Cd0823791E0C93114882d", addrStr)
}

func TestAddressFromPrivateKeyString(t *testing.T) {
	addr, err := walletworker.AddressFromPrivateKeyString("646f1ce2fdad0e6deeeb5c7e8e5543bdde65e86029e2fd9fc169899c440a7913")

	require.NoError(t, err)
	require.Equal(t, "0xE11BA2b4D45Eaed5996Cd0823791E0C93114882d", addr.Hex())
}
