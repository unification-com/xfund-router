package cmd

import (
	"github.com/spf13/cobra"
)

// keystoreCmd is the parent for keystore-management subcommands.
var keystoreCmd = &cobra.Command{
	Use:   "keystore",
	Short: "Keystore management commands",
	Long:  `Commands for managing your provider keystore, such as migrating it to a newer format.`,
}

func init() {
	rootCmd.AddCommand(keystoreCmd)
}
