package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePreRenameConfig renders the current default config, then rewrites it to the PRE-rename
// shape (a [adhoc_quality] section) with a customised flag_min_pools and a customised
// contract_address, to simulate an operator's existing file from before the rename.
func writePreRenameConfig(t *testing.T, path string) {
	t.Helper()
	WriteConfigFile(path, DefaultConfig())
	cur, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seed config: %v", err)
	}
	s := string(cur)
	s = strings.Replace(s, "[price_quality]", "[adhoc_quality]", 1)
	s = strings.Replace(s, "flag_min_pools = 2", "flag_min_pools = 5", 1)
	s = strings.Replace(s, `contract_address = ""`, `contract_address = "0xSENTINEL"`, 1)
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatalf("write pre-rename config: %v", err)
	}
}

func TestMigrateConfigFile_RenamesAdhocQuality(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writePreRenameConfig(t, path)

	res, err := MigrateConfigFile(path, false)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !res.Changed {
		t.Fatal("expected the file to change")
	}
	if len(res.Applied) == 0 {
		t.Fatal("expected the [adhoc_quality] rename to be reported as applied")
	}

	out, _ := os.ReadFile(path)
	s := string(out)
	if strings.Contains(s, "[adhoc_quality]") {
		t.Error("legacy [adhoc_quality] section should be gone")
	}
	if !strings.Contains(s, "[price_quality]") {
		t.Error("new [price_quality] section should be present")
	}
	if !strings.Contains(s, "flag_min_pools = 5") {
		t.Errorf("the customised flag_min_pools value should carry over:\n%s", s)
	}
	if !strings.Contains(s, `contract_address = "0xSENTINEL"`) {
		t.Error("non-migrated values must be preserved across the regenerate")
	}

	// The previous file is backed up and holds the original layout.
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("backup should exist: %v", err)
	}
	if !strings.Contains(string(bak), "[adhoc_quality]") {
		t.Error("backup should hold the original [adhoc_quality] file")
	}
}

func TestMigrateConfigFile_AddsEip1559Default(t *testing.T) {
	// Simulate a config from before the eip1559 key existed: render the current template, then
	// strip the eip1559 line (and customise a value, to prove preservation).
	path := filepath.Join(t.TempDir(), "config.toml")
	WriteConfigFile(path, DefaultConfig())
	cur, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read seed config: %v", err)
	}
	s := string(cur)
	if !strings.Contains(s, "eip1559 = true") {
		t.Fatalf("precondition: template should emit eip1559 = true:\n%s", s)
	}
	s = strings.Replace(s, "eip1559 = true\n", "", 1)
	s = strings.Replace(s, "max_gas_price = 150", "max_gas_price = 99", 1)
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatalf("write pre-eip1559 config: %v", err)
	}

	res, err := MigrateConfigFile(path, false)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !res.Changed {
		t.Fatal("expected the file to change (the eip1559 key is added)")
	}

	out, _ := os.ReadFile(path)
	got := string(out)
	if !strings.Contains(got, "eip1559 = true") {
		t.Errorf("the new eip1559 key should be added with its default:\n%s", got)
	}
	if !strings.Contains(got, "max_gas_price = 99") {
		t.Error("a customised value must be preserved across the migrate")
	}
}

func TestMigrateConfigFile_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writePreRenameConfig(t, path)

	if _, err := MigrateConfigFile(path, false); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	res, err := MigrateConfigFile(path, false)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if res.Changed {
		t.Error("a second migration of an already-current file should be a no-op")
	}
	if len(res.Applied) != 0 {
		t.Errorf("no migrations should apply the second time, got %v", res.Applied)
	}
}

func TestMigrateConfigFile_CurrentFileUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	WriteConfigFile(path, DefaultConfig()) // already current schema

	res, err := MigrateConfigFile(path, false)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res.Changed {
		t.Error("a current-schema file should not change")
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("no backup should be written when nothing changes")
	}
}

func TestMigrateConfigFile_DryRunWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writePreRenameConfig(t, path)
	before, _ := os.ReadFile(path)

	res, err := MigrateConfigFile(path, true)
	if err != nil {
		t.Fatalf("dry-run migrate: %v", err)
	}
	if !res.Changed {
		t.Error("dry-run should still report that a change is pending")
	}
	if res.BackupPath != "" {
		t.Error("dry-run must not write a backup")
	}

	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("dry-run must not modify the file")
	}
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Error("dry-run must not create a backup file")
	}
}
