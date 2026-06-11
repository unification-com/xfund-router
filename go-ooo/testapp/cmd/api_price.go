package cmd

import (
	"fmt"
	"go-ooo/ooo_api"
	"go-ooo/utils"
	"math/big"
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
		parsed, err := ooo_api.ParseEndpoint(endpoint)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		oooApi := createApi()

		start := time.Now()

		res, err := oooApi.QueryAdhoc(parsed, "1234")

		elapsed := time.Since(start)

		if err != nil {
			fmt.Println(err.Error())
		}

		resBi := big.NewInt(0)
		resBi.SetString(res, 10)
		resBf := utils.WeiToEther(resBi)

		fmt.Printf("\n\nEndpoint: %s\nRes: %s\n\n", endpoint, res)
		fmt.Printf("1 %s = %s %s\n\n", parsed.Base, resBf.String(), parsed.Target)
		fmt.Println("Query took:", elapsed)
	},
}

func init() {
	apiCmd.AddCommand(apiAdhocCmd)
}
