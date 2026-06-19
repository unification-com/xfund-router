package cmd

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"go-ooo/keystore"
	"go-ooo/utils"
	"go-ooo/utils/walletworker"

	"github.com/ethereum/go-ethereum/crypto"
)

// initOpts carries the optional non-interactive inputs for runKeystoreInit. Any unset
// field falls back to an interactive prompt, so the default behaviour is unchanged.
type initOpts struct {
	account   string // account name; empty -> prompt
	importKey string // private key (file path or 0x hex); empty -> prompt import/generate
	pass      string // keystore passphrase (file path or value); empty -> prompt
}

// runKeystoreInit creates a new v3 keystore at ksFile (scrypt-encrypted under a
// passphrase) and a decoupled admin token, returning the account name to record in
// config. It NEVER prints the private key. Inputs come from opts when supplied
// (non-interactive), otherwise from prompts.
func runKeystoreInit(ksFile string, opts initOpts) (string, error) {
	fmt.Println("")
	fmt.Println("Set up your provider key and account.")

	account := opts.account
	if account == "" {
		account = promptRequired("Account name for this key: ")
	}

	priv, err := resolvePrivateKey(opts.importKey)
	if err != nil {
		return "", err
	}

	passphrase, err := resolveNewPassphrase(opts.pass)
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

// resolvePrivateKey imports the key from importKey (a file path or 0x hex value) when
// given, otherwise prompts to import or generate one.
func resolvePrivateKey(importKey string) (*ecdsa.PrivateKey, error) {
	if importKey == "" {
		return promptForPrivateKey()
	}
	priv, err := crypto.HexToECDSA(utils.RemoveHexPrefix(strings.TrimSpace(readValueOrFile(importKey))))
	if err != nil {
		return nil, fmt.Errorf("invalid --import-key: %w", err)
	}
	return priv, nil
}

// resolveNewPassphrase uses pass (a file path or value) when given, otherwise prompts
// for a confirmed passphrase.
func resolveNewPassphrase(pass string) (string, error) {
	if pass == "" {
		fmt.Println("")
		fmt.Println("Choose a passphrase to encrypt your keystore. You will need it each time you")
		fmt.Println("start the oracle.")
		return readNewPassphrase()
	}
	p := readValueOrFile(pass)
	if len(p) < minPassphraseLen {
		return "", fmt.Errorf("passphrase must be at least %d characters", minPassphraseLen)
	}
	return p, nil
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
