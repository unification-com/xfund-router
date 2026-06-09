package cmd

import (
	"errors"
	"fmt"
	"os"

	"go-ooo/keystore"
	"go-ooo/server"

	"github.com/spf13/cobra"
)

// minPassphraseLen is the lower bound for a new keystore passphrase. scrypt makes
// a human passphrase acceptable, but a floor still rules out trivially weak input.
const minPassphraseLen = 8

// keystoreMigrateCmd migrates a legacy keystore to the go-ethereum v3 format.
var keystoreMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate a legacy keystore to the go-ethereum v3 (scrypt) format",
	Long: `Re-encrypts your provider private key from the legacy go-ooo keystore format
into the standard go-ethereum v3 (scrypt) format under a new passphrase.

The legacy format encrypted the key with a weak, predictable token, so the old
file must not survive. This command writes the new keystore, verifies it decrypts
to exactly the same key and address, and only then removes the old file. No backup
of the old file is kept — a copy encrypted under the weak token would preserve the
very weakness being removed.

You will need your CURRENT keystore password, and you will choose a NEW passphrase.

Examples:

  go-ooo keystore migrate
  go-ooo keystore migrate --home=/path/to/go-ooo-home
`,
	Run: func(cmd *cobra.Command, args []string) {
		srvCtx := server.GetServerContextFromCmd(cmd)
		cfg := srvCtx.Config
		if err := runKeystoreMigrate(cfg.Keystore.File, cfg.Keystore.Account); err != nil {
			fmt.Println("")
			fmt.Println("Keystore migration failed:", err.Error())
			os.Exit(1)
		}
	},
}

func init() {
	keystoreCmd.AddCommand(keystoreMigrateCmd)
}

// runKeystoreMigrate drives the interactive migration: it gates on the on-disk
// format so the operator is not prompted for secrets when there is nothing to do,
// gathers the old password and a confirmed new passphrase, then delegates the
// verify-then-delete file work to the keystore package.
func runKeystoreMigrate(ksFile, account string) error {
	data, err := os.ReadFile(ksFile)
	if err != nil {
		return fmt.Errorf("read keystore %s: %w", ksFile, err)
	}
	if keystore.IsV3Keystore(data) {
		fmt.Println("Keystore is already in the v3 format — nothing to migrate.")
		return nil
	}
	if !keystore.IsLegacyKeystore(data) {
		return fmt.Errorf("%s is not a recognised legacy keystore", ksFile)
	}

	fmt.Println("")
	fmt.Println("Legacy keystore detected at:")
	fmt.Println(" ", ksFile)
	fmt.Println("")
	fmt.Println("This will re-encrypt your provider key in the go-ethereum v3 (scrypt) format")
	fmt.Println("under a NEW passphrase, then permanently remove the old file. Have your")
	fmt.Println("CURRENT keystore password ready.")
	fmt.Println("")

	oldToken, err := readSecret("Current keystore password: ")
	if err != nil {
		return err
	}
	if oldToken == "" {
		return errors.New("current keystore password must not be empty")
	}

	newPass, err := readNewPassphrase()
	if err != nil {
		return err
	}

	addr, others, err := keystore.MigrateKeystoreFile(ksFile, account, oldToken, newPass)
	if errors.Is(err, keystore.ErrAlreadyV3) {
		fmt.Println("Keystore is already in the v3 format — nothing to migrate.")
		return nil
	}
	if err != nil {
		return err
	}

	for _, o := range others {
		fmt.Printf("WARNING: account %q is also in the legacy keystore but was NOT migrated "+
			"(go-ooo uses a single key).\n", o)
	}

	fmt.Println("")
	fmt.Println("Migration complete.")
	fmt.Println("  Wallet address:", addr.Hex())
	fmt.Println("  Keystore file: ", ksFile)
	fmt.Println("")
	fmt.Println("The old keystore file has been removed. BACK UP the new keystore file and")
	fmt.Println("REMEMBER your new passphrase — without them you cannot run the oracle or")
	fmt.Println("recover the key.")
	return nil
}

// readNewPassphrase prompts for a new passphrase twice and returns it once the two
// entries match and meet the minimum length.
func readNewPassphrase() (string, error) {
	for {
		p1, err := readSecret(fmt.Sprintf("New keystore passphrase (min %d chars): ", minPassphraseLen))
		if err != nil {
			return "", err
		}
		if len(p1) < minPassphraseLen {
			fmt.Printf("Passphrase must be at least %d characters. Try again.\n", minPassphraseLen)
			continue
		}
		p2, err := readSecret("Confirm new passphrase: ")
		if err != nil {
			return "", err
		}
		if p1 != p2 {
			fmt.Println("Passphrases do not match. Try again.")
			continue
		}
		return p1, nil
	}
}
