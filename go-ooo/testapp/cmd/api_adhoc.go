package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// apiCmd represents the query command
var apiAdhocCmd = &cobra.Command{
	Use:   "adhoc <endpoint>",
	Short: "adhoc query tests",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		endpoint := args[0]

		oooApi := createApi()

		res, err := oooApi.QueryAdhoc(endpoint, "1234")

		if err != nil {
			fmt.Println(err.Error())
		}

		fmt.Printf("\n\nEndpoint: %s, Res: %s\n\n", endpoint, res)
	},
}

func init() {
	apiCmd.AddCommand(apiAdhocCmd)
}
