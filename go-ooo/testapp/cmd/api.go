package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// apiCmd represents the query command
var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "api tests",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run one of the sub-commands. See 'testapp api --help'")
	},
}

func init() {
	rootCmd.AddCommand(apiCmd)
}
