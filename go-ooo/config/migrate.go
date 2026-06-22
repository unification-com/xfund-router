package config

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// configMigration carries a renamed or moved setting from an old config layout onto the current
// Config struct. apply inspects the raw (as-read) viper for the legacy key and, if present,
// copies its value into its new home on cfg. It returns true when the legacy key was found (so
// the caller can report it). New sections that were simply added need no migration - they fall
// out of the regenerate step with their defaults.
type configMigration struct {
	name  string
	apply func(v *viper.Viper, cfg *Config) bool
}

// configMigrations is the ordered registry of key renames/moves. Add an entry whenever a config
// key or section is renamed so an operator's existing file can be carried forward automatically.
var configMigrations = []configMigration{
	{
		name: "rename the [adhoc_quality] section to [price_quality]",
		apply: func(v *viper.Viper, cfg *Config) bool {
			if !v.IsSet("adhoc_quality") {
				return false
			}
			// The field names inside the section are unchanged, so the whole sub-tree decodes
			// straight into the new struct.
			if sub := v.Sub("adhoc_quality"); sub != nil {
				var pq PriceQualityConfig
				if err := sub.Unmarshal(&pq); err == nil {
					cfg.PriceQuality = pq
				}
			}
			return true
		},
	},
}

// MigrateResult reports the outcome of a config migration.
type MigrateResult struct {
	Applied    []string // human-readable names of the rename migrations that matched
	Changed    bool     // whether the regenerated file differs from the current one
	BackupPath string   // where the previous file was backed up (empty if unchanged or dry-run)
}

// readConfigFile reads path and decodes it onto a defaults-seeded Config, also returning the raw viper
// (for legacy-key migrations) and the original bytes (for change detection). Shared by LoadConfigFile
// and MigrateConfigFile so the viper read/decode lives in one place.
func readConfigFile(path string) (*Config, *viper.Viper, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read config %s: %w", path, err)
	}
	v := viper.New()
	v.SetConfigType("toml")
	if err := v.ReadConfig(bytes.NewReader(raw)); err != nil {
		return nil, nil, nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	// Start from defaults so removed keys vanish and brand-new sections get their defaults, then
	// overlay the file's current-schema values (merge - absent keys keep their default).
	cfg := DefaultConfig()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, nil, nil, fmt.Errorf("decode config %s: %w", path, err)
	}
	return cfg, v, raw, nil
}

// LoadConfigFile reads an existing config.toml into a Config (defaults overlaid with the file's
// values). Used by maintenance commands that operate on the raw file (e.g. 'init <network> --add').
func LoadConfigFile(path string) (*Config, error) {
	cfg, _, _, err := readConfigFile(path)
	return cfg, err
}

// MigrateConfigFile brings an existing config.toml up to the current schema. It preserves the
// operator's customised values, carries any renamed/moved keys to their new homes (the
// configMigrations registry), adds any new sections with their defaults, and drops settings that
// no longer exist. The file is regenerated from the current template, so hand-added comments are
// not preserved (the values are). When the regenerated content differs from the current file, the
// previous file is copied to <path>.bak before overwriting. dryRun reports what would change
// without touching the file. Running it on an already-current file is a no-op.
func MigrateConfigFile(path string, dryRun bool) (MigrateResult, error) {
	var res MigrateResult

	cfg, v, current, err := readConfigFile(path)
	if err != nil {
		return res, err
	}

	for _, m := range configMigrations {
		if m.apply(v, cfg) {
			res.Applied = append(res.Applied, m.name)
		}
	}

	var buf bytes.Buffer
	if err := configTemplate.Execute(&buf, cfg); err != nil {
		return res, fmt.Errorf("render config: %w", err)
	}

	if bytes.Equal(buf.Bytes(), current) {
		return res, nil // already current - nothing to write
	}
	res.Changed = true

	if dryRun {
		return res, nil
	}

	res.BackupPath = path + ".bak"
	if err := os.WriteFile(res.BackupPath, current, 0o600); err != nil {
		return res, fmt.Errorf("write backup %s: %w", res.BackupPath, err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return res, fmt.Errorf("write config %s: %w", path, err)
	}

	return res, nil
}
