package cmd

import (
	"github.com/spf13/cobra"
	"go-ooo/logger"
	"go-ooo/server"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the OoO service",
	Long: `Start the OoO service, which will listen for requests and process them.
Service will start with the default home path $HOME/.go-ooo and expects the configuration
files and keystore to exist. 

If you have not run this before, begin by running the 'go-ooo init' command to initialise
the configuration.

The --home path can be specified.
The --pass flag can also be used to pass the location of the file containing your
keystore password, or the password itself.

Examples:

  go-ooo start
  go-ooo start --home=/home/user/some-other-go-ooo
  go-ooo start --home=/home/user/some-other-go-ooo --pass=/path/to/pass.txt
  go-ooo start --first-block=15114000
  go-ooo start --first-block=15114000 --chain=puppynet
`,
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		// The config is already loaded + flags bound by the root PersistentPreRunE
		// (InterceptConfigsPreRunHandler); just validate it before starting.
		return server.GetServerContextFromCmd(cmd).Config.ValidateBasic()
	},
	Run: func(cmd *cobra.Command, args []string) {
		serverCtx := server.GetServerContextFromCmd(cmd)
		server, err := server.NewServer(serverCtx, keystorePass, startFirstBlock, startFirstBlockChain)
		if err != nil {
			panic(err)
		}
		logger.SetLogLevel(serverCtx.Config.Log.Level)
		server.InitServer()
		server.Run()
	},
}

var (
	startFirstBlock      uint64
	startFirstBlockChain string
)

func init() {
	startCmd.PersistentFlags().StringVar(&keystorePass, "pass", "", "keystore password or password file location")
	// One-shot resume-point override: advance the event-scan cursor to this block before starting, to
	// skip a stale gap (e.g. when far behind on a rate-limited RPC). Advance-only; persisted, so drop
	// the flag on the next start. 0 = use the saved cursor / first_block as normal.
	startCmd.PersistentFlags().Uint64Var(&startFirstBlock, "first-block", 0, "advance the event-scan cursor to this block before starting (skip a stale gap)")
	// Each chain has its own cursor, so --first-block targets one chain. Optional with a single chain
	// configured; required (name or network id) once several [[chains]] run.
	startCmd.PersistentFlags().StringVar(&startFirstBlockChain, "chain", "", "chain (name or network id) the --first-block override applies to; optional with a single chain")
	rootCmd.AddCommand(startCmd)
}
