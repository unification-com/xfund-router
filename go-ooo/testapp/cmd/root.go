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

	// DB selection: defaults to a throwaway sqlite file, or point at a Postgres instance
	// (e.g. a restored production dump) with --db-dialect=postgres + the connection flags.
	rootCmd.PersistentFlags().StringVar(&dbDialect, "db-dialect", "sqlite", "database dialect: sqlite or postgres")
	rootCmd.PersistentFlags().StringVar(&dbStorage, "db-storage", "/tmp/go-ooo_testapp.sqlite", "sqlite file path (sqlite only)")
	rootCmd.PersistentFlags().StringVar(&dbHost, "db-host", "", "postgres host (or unix socket dir, e.g. /tmp)")
	rootCmd.PersistentFlags().Uint64Var(&dbPort, "db-port", 0, "postgres port")
	rootCmd.PersistentFlags().StringVar(&dbUser, "db-user", "", "postgres user")
	rootCmd.PersistentFlags().StringVar(&dbName, "db-name", "", "postgres database name")
	rootCmd.PersistentFlags().StringVar(&dbPass, "db-pass", "", "postgres password")

	cobra.CheckErr(rootCmd.Execute())
}
