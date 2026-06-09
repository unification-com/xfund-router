package cmd

import (
	"fmt"
	"os"

	"go-ooo/server"

	"github.com/spf13/cobra"
)

// keystoreSetAdminTokenCmd generates (or rotates) the admin HTTP API bearer.
var keystoreSetAdminTokenCmd = &cobra.Command{
	Use:   "set-admin-token",
	Short: "Generate a new admin HTTP API bearer token",
	Long: `Generates a new admin HTTP API bearer token, stores only its bcrypt hash in a
0600 sidecar next to your keystore, and prints the raw token once.

Use this to rotate the admin token, or to create one if you do not yet have a
token configured. The token is independent of your keystore passphrase, so this
does not touch your private key.

Restart go-ooo for the new token to take effect.

Examples:

  go-ooo keystore set-admin-token
  go-ooo keystore set-admin-token --home=/path/to/go-ooo-home
`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg := server.GetServerContextFromCmd(cmd).Config

		raw, err := issueAdminToken(cfg.Keystore.File)
		if err != nil {
			fmt.Println("Failed to set admin token:", err.Error())
			os.Exit(1)
		}

		fmt.Println("")
		fmt.Println("Your new admin HTTP API token:")
		fmt.Println(" ", raw)
		fmt.Println("")
		fmt.Println("Only its hash is stored. SAVE this token now — it is not shown again.")
		fmt.Println("Restart go-ooo for it to take effect.")
	},
}

func init() {
	keystoreCmd.AddCommand(keystoreSetAdminTokenCmd)
}
