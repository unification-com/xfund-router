package walletworker

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"go-ooo/utils"
)

func GeneratePrivate() (*ecdsa.PrivateKey, string, error) {
	privateKey, err := crypto.GenerateKey()
	return privateKey, hexutil.Encode(crypto.FromECDSA(privateKey)), err
}

func StringToPrivate(bytePrivateKey string) (*ecdsa.PrivateKey, error) {
	privateKey, err := crypto.HexToECDSA(bytePrivateKey)
	return privateKey, err
}

func GeneratePublic(privateKey *ecdsa.PrivateKey) (*ecdsa.PublicKey, string) {
	publicKey := privateKey.Public()
	publicKeyECDSA, _ := publicKey.(*ecdsa.PublicKey)
	return publicKeyECDSA, hexutil.Encode(crypto.FromECDSAPub(publicKey.(*ecdsa.PublicKey)))
}

func GenerateAddress(publicKeyECDSA *ecdsa.PublicKey) (common.Address, string) {
	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	return address, address.Hex()
}

func AddressFromPrivateKeyString(strPrivateKey string) (common.Address, error) {
	if utils.HasHexPrefix(strPrivateKey) {
		strPrivateKey = utils.RemoveHexPrefix(strPrivateKey)
	}
	pkey, err := StringToPrivate(strPrivateKey)
	if err != nil {
		return common.Address{}, err
	}
	pubKey, _ := GeneratePublic(pkey)

	addr, _ := GenerateAddress(pubKey)

	return addr, nil
}

// EthSigner is an EIP-191 personal_sign signer over a single keystore key. It satisfies the
// export.Signer interface (structurally) so the dex-pair-verify export client can authenticate as the
// registered OoO provider (T8) using the same oracle key that signs fulfilment transactions.
type EthSigner struct {
	priv *ecdsa.PrivateKey
	addr string
}

// NewEthSigner builds a signer from a private key, caching its checksummed 0x address.
func NewEthSigner(priv *ecdsa.PrivateKey) *EthSigner {
	return &EthSigner{priv: priv, addr: crypto.PubkeyToAddress(priv.PublicKey).Hex()}
}

// Address returns the provider's checksummed 0x address (the registered OoO provider wallet).
func (s *EthSigner) Address() string { return s.addr }

// SignText returns an EIP-191 personal_sign over message as a 0x-prefixed 65-byte hex signature. The
// recovery id is shifted 0/1 -> 27/28 to match the standard personal_sign V convention (what web3 /
// ecrecover on the verifier expects).
func (s *EthSigner) SignText(message string) (string, error) {
	hash := accounts.TextHash([]byte(message))
	sig, err := crypto.Sign(hash, s.priv)
	if err != nil {
		return "", err
	}
	sig[64] += 27
	return hexutil.Encode(sig), nil
}
