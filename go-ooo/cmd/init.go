package cmd

import (
	"errors"
	"fmt"
	"go-ooo/config"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// startCmd represents the start command
var initCmd = &cobra.Command{
	Use:   "init <network>",
	Short: "Initialise your OoO service",
	Long: `Initialise your OoO service with some default configuration values.

The command will initialise the config.toml file according to the given network option, and
also your keystore. You can generate a new key, or import an existing private key hex string,
and will be prompted to do so during the process.

By default, the environment will use $HOME/.go-ooo to store the app's configuration. You can
specify where to store these files with the --home flag.

Once initialised, you must edit the config.toml file with the appropriate values for your
database configuration, Eth RPC URLs etc.

Current network options are:

  dev
  sepolia
  mainnet
  polygon

Examples:

  go-ooo init sepolia
  go-ooo init dev --home=/path/to/go-ooo-home

`,
	Args: cobra.ExactArgs(1),
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// prevent getting error due to missing config.toml
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		network := args[0]
		fmt.Println("init called")
		fmt.Println("appHomePath", appHomePath)

		cfgFile := filepath.Join(appHomePath, "config.toml")
		dbFile := filepath.Join(appHomePath, "ooo.sqlite")
		ksFile := filepath.Join(appHomePath, "keystore.json")
		if _, err := os.Stat(cfgFile); errors.Is(err, os.ErrNotExist) {
			fmt.Println(cfgFile, "does not exist. Creating with defaults")

			if _, err := os.Stat(appHomePath); errors.Is(err, os.ErrNotExist) {
				// 0700 - the home directory holds the keystore and admin-token hash.
				err := os.MkdirAll(appHomePath, 0700)
				if err != nil {
					panic(err)
				}
			}

			conf := config.DefaultConfig()
			conf.InitForNet(network)

			account, err := runKeystoreInit(ksFile)
			if err != nil {
				fmt.Println("keystore initialisation failed:", err.Error())
				return
			}

			conf.SetKeystore(ksFile, account)

			conf.SetSqliteDb(dbFile)

			config.WriteConfigFile(cfgFile, conf)

			fmt.Println("config saved to:")
			fmt.Println(cfgFile)

		} else {
			fmt.Println(cfgFile, "already exists. Exiting")
			return
		}

	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
