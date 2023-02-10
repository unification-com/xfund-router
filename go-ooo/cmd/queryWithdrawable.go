package cmd

import (
	"github.com/spf13/cobra"
	"go-ooo/server"
	go_ooo_types "go-ooo/types"
)

// setFeeCmd represents the setFee command
var queryWithdrawableCmd = &cobra.Command{
	Use:   "withdrawable",
	Short: "Query amount available to withdraw",
	Run: func(cmd *cobra.Command, args []string) {
		adminTask := go_ooo_types.AdminTask{}

		adminTask.Task = "query_withdrawable"

		srvCtx := server.GetServerContextFromCmd(cmd)

		processAdminTask(adminTask, srvCtx.Config)
	},
}

func init() {
	queryCmd.AddCommand(queryWithdrawableCmd)
}
