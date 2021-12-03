package walletworker

import (
	"crypto/ecdsa"
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
