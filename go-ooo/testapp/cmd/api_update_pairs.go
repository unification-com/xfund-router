package cmd

import (
	"github.com/spf13/cobra"
)

// apiCmd represents the query command
var apiUpdatePairsCmd = &cobra.Command{
	Use:   "update-pairs",
	Short: "update tmp db with DEX pairs",
	Run: func(cmd *cobra.Command, args []string) {

		oooApi := createApi()

		oooApi.UpdateDexPairs()
	},
}

func init() {
	apiCmd.AddCommand(apiUpdatePairsCmd)
}
