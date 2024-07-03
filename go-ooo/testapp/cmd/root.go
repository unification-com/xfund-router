package cmd

import (
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "testapp",
	Short: "App for testing OoO API interface",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.PersistentFlags().StringVar(&graphNetApi, FlagGraphNetworkApiKey, "", "Graph Network API Key")
	cobra.CheckErr(rootCmd.Execute())
}
