package database

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	glog "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.sqlite")), &gorm.Config{
		Logger: glog.Default.LogMode(glog.Silent),
	})
	require.NoError(t, err)
	return &DB{DB: gdb}
}

func TestMigrateFreshDatabase(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())

	v, err := d.currentSchemaVersion()
	require.NoError(t, err)
	require.Equal(t, latestSchemaVersion, v)

	for _, table := range []string{"data_requests", "dex_pairs", "version_info"} {
		require.True(t, d.Migrator().HasTable(table), "expected table %q to exist", table)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())
	require.NoError(t, d.Migrate()) // a second run must be a clean no-op

	v, err := d.currentSchemaVersion()
	require.NoError(t, err)
	require.Equal(t, latestSchemaVersion, v)
}

// TestFailingStepRollsBackVersion proves the transactional guarantee: a step that
// errors must not advance the recorded schema version.
func TestFailingStepRollsBackVersion(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())

	before, err := d.currentSchemaVersion()
	require.NoError(t, err)

	failing := []migrationStep{
		{to: latestSchemaVersion + 1, name: "deliberately failing", run: func(tx *gorm.DB) error {
			return errors.New("boom")
		}},
	}
	require.Error(t, d.runSchemaMigrations(failing))

	after, err := d.currentSchemaVersion()
	require.NoError(t, err)
	require.Equal(t, before, after, "version must not advance past a failed step")
}

// TestNonContiguousVersionIsRejected guards against silently applying a step from the
// wrong starting point when the version table is inconsistent.
func TestNonContiguousVersionIsRejected(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())

	gapped := []migrationStep{
		{to: latestSchemaVersion + 2, name: "skips a version", run: func(tx *gorm.DB) error {
			return nil
		}},
	}
	require.Error(t, d.runSchemaMigrations(gapped))
}
