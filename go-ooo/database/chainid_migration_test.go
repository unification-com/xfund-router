package database

import (
	"os"
	"testing"

	"go-ooo/database/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glog "gorm.io/gorm/logger"
)

// runChainIdMigrationAssertions exercises the v5 chain_id migration end-to-end against the given DB,
// so the same checks run on both sqlite and Postgres (the #100 both-dialects discipline). It builds a
// legacy-shaped schema (the composite index from the current model PLUS the obsolete single-column
// request_id unique index a pre-v5 DB carried, and rows lacking chain_id), runs migrateAddChainId, and
// asserts the backfill, the index swap, cross-chain non-collision, intra-chain dedup, and idempotency.
func runChainIdMigrationAssertions(t *testing.T, d *DB) {
	t.Helper()
	const netId int64 = 137

	require.NoError(t, d.AutoMigrate(&models.DataRequests{}, &models.ToBlocks{}))
	// The pre-v5 standalone unique index on request_id (the old single-chain schema). Name verified
	// against a real legacy go-ooo DB; identical on both dialects (GORM's namer is dialect-independent).
	require.NoError(t, d.Exec("CREATE UNIQUE INDEX idx_data_requests_request_id ON data_requests(request_id)").Error)
	require.True(t, d.Migrator().HasIndex(&models.DataRequests{}, "idx_data_requests_request_id"))
	require.NoError(t, d.Create(&models.DataRequests{RequestId: "0xabc", JobStatus: models.JOB_STATUS_PENDING}).Error)
	require.NoError(t, d.Create(&models.ToBlocks{BlockNum: 500}).Error)

	require.NoError(t, migrateAddChainId(d.DB, netId))

	// 1. backfill — the legacy rows now carry the configured network id.
	var dr models.DataRequests
	require.NoError(t, d.Where("request_id = ?", "0xabc").First(&dr).Error)
	require.EqualValues(t, netId, dr.ChainId)
	var tb models.ToBlocks
	require.NoError(t, d.First(&tb).Error)
	require.EqualValues(t, netId, tb.ChainId)

	// 2. the obsolete single-column unique index is gone.
	require.False(t, d.Migrator().HasIndex(&models.DataRequests{}, "idx_data_requests_request_id"))

	// 3. a DIFFERENT chain may now hold the SAME bare request_id (composite, not global, uniqueness).
	require.NoError(t, d.Create(&models.DataRequests{ChainId: 999, RequestId: "0xabc"}).Error)

	// 4. dedup still holds WITHIN a chain — re-inserting (netId, 0xabc) violates the composite index.
	require.Error(t, d.Create(&models.DataRequests{ChainId: netId, RequestId: "0xabc"}).Error)

	// 5. idempotent — re-running is a no-op (chain_id already set, index already dropped).
	require.NoError(t, migrateAddChainId(d.DB, netId))
	require.NoError(t, d.Where("chain_id = ? AND request_id = ?", netId, "0xabc").First(&dr).Error)
	require.EqualValues(t, netId, dr.ChainId)
}

// TestMigrateAddChainId runs the chain_id migration assertions on sqlite (always).
func TestMigrateAddChainId(t *testing.T) {
	runChainIdMigrationAssertions(t, newTestDB(t))
}

// TestMigrateAddChainIdPostgres runs the same assertions on Postgres (the production dialect).
// Skipped unless GOOOO_TEST_PG_DSN is set; it drops the touched tables first for a clean slate.
func TestMigrateAddChainIdPostgres(t *testing.T) {
	dsn := os.Getenv("GOOOO_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set GOOOO_TEST_PG_DSN to run the Postgres chain_id migration test")
	}
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: glog.Default.LogMode(glog.Silent)})
	require.NoError(t, err)
	d := &DB{DB: gdb}
	require.NoError(t, d.Migrator().DropTable(&models.DataRequests{}, &models.ToBlocks{}))
	runChainIdMigrationAssertions(t, d)
}

// TestPerChainCursorIsolation confirms the resume cursor is independent per chain after the migration:
// advancing chain A's cursor never moves chain B's, and each reads back its own block.
func TestPerChainCursorIsolation(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())

	require.NoError(t, d.InsertNewToBlock(1, 1000))
	require.NoError(t, d.InsertNewToBlock(137, 2000))
	require.NoError(t, d.InsertNewToBlock(1, 1500)) // advance chain 1 only

	c1, err := d.GetLastBlockNumQueried(1)
	require.NoError(t, err)
	require.EqualValues(t, 1500, c1.GetBlockNum())

	c137, err := d.GetLastBlockNumQueried(137)
	require.NoError(t, err)
	require.EqualValues(t, 2000, c137.GetBlockNum(), "chain 137 cursor is untouched by chain 1 writes")
}
