package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"go-ooo/version"
)

// startCmd represents the start command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Displays the current app version info",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// prevent getting error due to missing config.toml
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version.NewInfo())
		return
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
