package cmd

import (
	"fmt"

	"go-ooo/server"
	go_ooo_types "go-ooo/types"

	"github.com/spf13/cobra"
)

// withdrawCmd represents the withdraw command
var withdrawCmd = &cobra.Command{
	Use:   "withdraw <amount> <recipient>",
	Short: "Withdraw available fees from the Router",
	Long: `Allows you to withdraw your accumulated fees from the Router. You can set the
recipient to an address other than your oracle provider address.

Examples:

  go-ooo admin withdraw 100000000 0x1234556abcd...

`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		adminTask := go_ooo_types.AdminTask{}

		amount, err := parsePositiveUint(args[0], "amount")
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		recipient, err := requireHexAddress(args[1], "recipient address")
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		adminTask.Task = "withdraw"
		adminTask.FeeOrAmount = amount
		adminTask.ToOrConsumer = recipient

		srvCtx := server.GetServerContextFromCmd(cmd)

		processAdminTask(adminTask, srvCtx.Config)
	},
}

func init() {
	adminCmd.AddCommand(withdrawCmd)
}
