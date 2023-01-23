package cmd

import (
	"fmt"
	"time"

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

		start := time.Now()

		res, err := oooApi.QueryAdhoc(endpoint, "1234")

		elapsed := time.Since(start)

		if err != nil {
			fmt.Println(err.Error())
		}

		fmt.Printf("\n\nEndpoint: %s, Res: %s\n\n", endpoint, res)
		fmt.Println("Query took:", elapsed)
	},
}

func init() {
	apiCmd.AddCommand(apiAdhocCmd)
}
