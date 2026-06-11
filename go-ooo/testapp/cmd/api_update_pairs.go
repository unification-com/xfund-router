package cmd

import (
	"github.com/spf13/cobra"
)

// apiCmd represents the query command
var apiUpdateAdhocCmd = &cobra.Command{
	Use:   "adhoc-update",
	Short: "update tmp db with adhoc pairs",
	Run: func(cmd *cobra.Command, args []string) {

		oooApi := createApi()

		oooApi.UpdateDexPairs()
	},
}

func init() {
	apiCmd.AddCommand(apiUpdateAdhocCmd)
}
