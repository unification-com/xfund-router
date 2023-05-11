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
`,
	PreRunE: func(cmd *cobra.Command, _ []string) error {
		serverCtx := server.GetServerContextFromCmd(cmd)

		// Bind flags to the Context's Viper
		serverCtx.Viper.BindPFlags(cmd.Flags())

		return serverCtx.Config.ValidateBasic()
	},
	Run: func(cmd *cobra.Command, args []string) {
		serverCtx := server.GetServerContextFromCmd(cmd)
		server, err := server.NewServer(serverCtx, keystorePass)
		if err != nil {
			panic(err)
		}
		logger.SetLogLevel(serverCtx.Config.Log.Level)
		server.InitServer()
		server.Run()
	},
}

func init() {
	startCmd.PersistentFlags().StringVar(&keystorePass, "pass", "", "keystore password or password file location")
	rootCmd.AddCommand(startCmd)
}
