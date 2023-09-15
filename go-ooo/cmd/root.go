package cmd

import (
	"context"
	"github.com/spf13/cobra"
	"go-ooo/flags"
	"go-ooo/server"
)

var appHomePath string
var keystorePass string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "app",
	Short: "OoO Provider Application",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		return server.InterceptConfigsPreRunHandler(cmd)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(defaultHome string) {
	srvCtx := server.NewDefaultContext()
	ctx := context.Background()
	ctx = context.WithValue(ctx, server.ServerContextKey, srvCtx)

	rootCmd.PersistentFlags().StringVar(&appHomePath, flags.FlagHome, defaultHome, "app home file (default is $HOME/.go-ooo)")
	cobra.CheckErr(rootCmd.ExecuteContext(ctx))
}
