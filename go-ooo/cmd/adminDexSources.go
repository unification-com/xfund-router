package cmd

import (
	"fmt"

	"go-ooo/server"
	go_ooo_types "go-ooo/types"

	"github.com/spf13/cobra"
)

// dexSourcesCmd is the parent for DEX source-catalogue admin commands.
var dexSourcesCmd = &cobra.Command{
	Use:   "dex-sources",
	Short: "Manage the dex-pair-verify DEX source catalogue",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("run one of the sub-commands. See 'go-ooo admin dex-sources --help'")
	},
}

// dexSourcesSyncCmd triggers an on-demand sync of the source manifest + pair feeds from
// dex-pair-verify - the same refresh the oracle runs periodically - for testing or to force a
// refresh without waiting for the next tick. The sync runs in the background on the oracle.
var dexSourcesSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync the DEX source manifest + pair feeds from dex-pair-verify now",
	Run: func(cmd *cobra.Command, args []string) {
		adminTask := go_ooo_types.AdminTask{Task: "sync_dex_sources"}

		srvCtx := server.GetServerContextFromCmd(cmd)

		processAdminTask(adminTask, srvCtx.Config)
	},
}

func init() {
	adminCmd.AddCommand(dexSourcesCmd)
	dexSourcesCmd.AddCommand(dexSourcesSyncCmd)
}
