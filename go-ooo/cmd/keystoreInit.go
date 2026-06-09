package cmd

import (
	"crypto/ecdsa"
	"fmt"

	"go-ooo/keystore"
	"go-ooo/utils"
	"go-ooo/utils/walletworker"

	"github.com/ethereum/go-ethereum/crypto"
)

// runKeystoreInit interactively creates a new v3 keystore at ksFile (encrypted with
// scrypt under an operator-chosen passphrase) and a decoupled admin token. It returns
// the chosen account name to record in config. It NEVER prints the private key.
func runKeystoreInit(ksFile string) (string, error) {
	fmt.Println("")
	fmt.Println("Set up your provider key and account.")

	account := promptRequired("Account name for this key: ")

	priv, err := promptForPrivateKey()
	if err != nil {
		return "", err
	}

	fmt.Println("")
	fmt.Println("Choose a passphrase to encrypt your keystore. You will need it each time you")
	fmt.Println("start the oracle.")
	passphrase, err := readNewPassphrase()
	if err != nil {
		return "", err
	}

	addr, err := keystore.WriteNewV3Keystore(ksFile, priv, passphrase)
	if err != nil {
		return "", err
	}

	adminToken, err := issueAdminToken(ksFile)
	if err != nil {
		return "", fmt.Errorf("keystore created, but generating the admin token failed - run "+
			"'go-ooo keystore set-admin-token' to create one: %w", err)
	}

	fmt.Println("")
	fmt.Println("Keystore created.")
	fmt.Println("  Account:       ", account)
	fmt.Println("  Wallet address:", addr.Hex())
	fmt.Println("  Keystore file: ", ksFile)
	fmt.Println("")
	fmt.Println("Your admin HTTP API token (store it safely — it is not shown again):")
	fmt.Println(" ", adminToken)
	fmt.Println("")
	fmt.Println("KEEP your passphrase and admin token safe, and BACK UP the keystore file —")
	fmt.Println("without them you cannot run the oracle or recover the key.")
	return account, nil
}

// promptForPrivateKey asks whether to import an existing key or generate a new one,
// returning the resulting private key. An imported key is read without echo.
func promptForPrivateKey() (*ecdsa.PrivateKey, error) {
	for {
		fmt.Println("")
		fmt.Println("Add an existing private key or generate a new one?")
		choice := promptRequired("[1 = import existing, 2 = generate new]: ")
		switch choice {
		case "1":
			hexKey, err := readSecret("Enter your private key: ")
			if err != nil {
				return nil, err
			}
			priv, err := crypto.HexToECDSA(utils.RemoveHexPrefix(hexKey))
			if err != nil {
				fmt.Println("That does not look like a valid private key. Try again.")
				continue
			}
			return priv, nil
		case "2":
			priv, _, err := walletworker.GeneratePrivate()
			if err != nil {
				return nil, err
			}
			return priv, nil
		default:
			fmt.Println("Please enter 1 or 2.")
		}
	}
}
