package keystore

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"go-ooo/utils"
	"go-ooo/utils/walletworker"
)

func (ks *Keystorage) InitNewKeystore(keyStorePath string) (err error, addusername string) {
	var walletAddress common.Address
	var addgenerate string
	var privateKey string

	token, err := ks.GenerateToken()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println("")
	fmt.Println("Import a private key or create a new account")
	fmt.Print("Username: ")

	fmt.Scanf("%s\n", &addusername)

	if ks.ExistsByUsername(addusername) {
		fmt.Println("This account name is already used")
		_, _ = ks.InitNewKeystore(keyStorePath)
	} else if addusername == "" {
		fmt.Println("Please enter account username.")
		_, _ = ks.InitNewKeystore(keyStorePath)
	}
	fmt.Println("")
	fmt.Println("Do you want to add an existing private key or generate a new one?")
	fmt.Print("[ 1-import private key; 2-generate new key ]:	")

	fmt.Scanf("%s\n", &addgenerate)

	switch addgenerate {
	case "1":
		fmt.Println("")
		fmt.Print("Input your private key: ")

		fmt.Scanf("%s\n", &privateKey)

		if !utils.HasHexPrefix(privateKey) {
			privateKey = utils.AddHexPrefix(privateKey)
		}

		err = ks.AddExisting(addusername, privateKey)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Println("\nSuccessfully imported private key!")
	case "2":
		privateKey, err = ks.GeneratePrivate(addusername)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		fmt.Println("\nSuccessfully generated a new private key:")

		if !utils.HasHexPrefix(privateKey) {
			privateKey = utils.AddHexPrefix(privateKey)
		}

		fmt.Println(privateKey)
	default:
		fmt.Println("eh?")
		err, _ = ks.InitNewKeystore(keyStorePath)
	}

	fmt.Print("Your keystore decryption & admin password:")
	fmt.Println("")
	fmt.Println(token)
	fmt.Println("")
	fmt.Println("KEEP THIS KEY SAFE! You will need it to run the application and admin tasks!")
	fmt.Println("")
	fmt.Println("Your oracle wallet address:")
	walletAddress, err = walletworker.AddressFromPrivateKeyString(privateKey)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(walletAddress.Hex())
	fmt.Println("Keystore saved to:")
	fmt.Println(keyStorePath)

	return
}
