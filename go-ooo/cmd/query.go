package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

// queryCmd represents the query command
var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Run a sub-command",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run one of the sub-commands. See 'go-ooo admin --help'")
	},
}

func init() {
	// Shares the --chain selector with the admin commands (both go through processAdminTask).
	queryCmd.PersistentFlags().StringVar(&chainFlag, "chain", "", "target chain (name or network id; 'all' for every chain). Optional with a single chain configured")
	rootCmd.AddCommand(queryCmd)
}
