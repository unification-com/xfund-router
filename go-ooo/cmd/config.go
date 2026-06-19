package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"go-ooo/config"

	"github.com/spf13/cobra"
)

// configCmd groups commands that maintain the config.toml file.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect and maintain the go-ooo config file",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// Operate on the raw config.toml directly; skip the standard config loader so a file
		// that the current schema can't fully load (e.g. holding a renamed legacy section) can
		// still be migrated.
		return nil
	},
}

var configMigrateDryRun bool

// configMigrateCmd brings an existing config.toml up to the current schema.
var configMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Bring config.toml up to the current schema (renames moved keys, adds new sections)",
	Long: `Updates an existing config.toml to the current go-ooo schema.

It preserves your customised values, carries any renamed or moved settings to their new
homes (for example the [adhoc_quality] section is renamed to [price_quality]), adds any
new sections with their defaults, and drops settings that no longer exist.

The file is regenerated from the current template, so hand-added comments are not kept
(your values are). Before overwriting, the previous file is copied to config.toml.bak.
Running it on an already-current file makes no changes.

Examples:

  go-ooo config migrate
  go-ooo config migrate --dry-run
  go-ooo config migrate --home=/path/to/go-ooo-home
`,
	Run: func(cmd *cobra.Command, args []string) {
		cfgFile := filepath.Join(appHomePath, "config.toml")
		if _, err := os.Stat(cfgFile); err != nil {
			fmt.Println(cfgFile, "not found - run 'go-ooo init <network>' first.")
			os.Exit(1)
		}

		res, err := config.MigrateConfigFile(cfgFile, configMigrateDryRun)
		if err != nil {
			fmt.Println("config migration failed:", err.Error())
			os.Exit(1)
		}

		for _, m := range res.Applied {
			fmt.Println("  -", m)
		}

		switch {
		case configMigrateDryRun && res.Changed:
			fmt.Println("config.toml would be updated to the current schema (dry run - nothing written).")
		case configMigrateDryRun:
			fmt.Println("config.toml is already up to date (dry run).")
		case res.Changed:
			fmt.Println("config.toml updated to the current schema.")
			fmt.Println("  previous file backed up to:", res.BackupPath)
		default:
			fmt.Println("config.toml is already up to date - no changes.")
		}
	},
}

func init() {
	configMigrateCmd.Flags().BoolVar(&configMigrateDryRun, "dry-run", false, "show what would change without writing")
	configCmd.AddCommand(configMigrateCmd)
	rootCmd.AddCommand(configCmd)
}
