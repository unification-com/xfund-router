package database

import (
	"errors"
	"fmt"

	"go-ooo/database/models"
	"go-ooo/logger"

	"gorm.io/gorm"
)

/*
  Migration framework.

  Migrate() first runs AutoMigrate (additive: tables/columns/indexes) and aborts on
  any error. It then applies the ordered data migrations below. Each step and its
  version bump run inside ONE transaction, so a failure rolls back atomically and the
  recorded schema version is never advanced past a half-applied step.

  To add a migration: APPEND a step to schemaMigrations with to = previous+1 and bump
  latestSchemaVersion. Never edit or reorder existing steps. A plain additive column,
  table or index needs NO step — AutoMigrate handles it; add a step only for a data
  transformation or a destructive schema change, and make its run func idempotent.
*/

// latestSchemaVersion is the version a fully-migrated database ends on. It MUST equal
// the highest migrationStep.to in the steps built by buildSchemaMigrations.
const latestSchemaVersion uint64 = 6

// migrationStep transforms the database from schema version (to-1) to version `to`.
type migrationStep struct {
	to   uint64
	name string
	run  func(tx *gorm.DB) error
}

var schemaMigrations = []migrationStep{
	{to: 1, name: "clear adhoc token data", run: migrateDeleteAdhocTokenData},
	{to: 2, name: "clear adhoc token data", run: migrateDeleteAdhocTokenData},
	{to: 3, name: "drop legacy dex_tokens table and columns", run: migrateDropLegacyDexTokens},
	{to: 4, name: "drop the Finchains supported_pairs table and the is_adhoc column", run: migrateDropFinchainsRemnants},
}

// buildSchemaMigrations returns the ordered migration steps. The chain_id steps (v5, v6) need the
// configured network id (to backfill legacy single-chain rows), so they are built here as closures over
// the DB rather than living in the static schemaMigrations slice.
func (d *DB) buildSchemaMigrations() []migrationStep {
	steps := make([]migrationStep, len(schemaMigrations), len(schemaMigrations)+2)
	copy(steps, schemaMigrations)
	return append(steps,
		migrationStep{
			to:   5,
			name: "add chain_id to data_requests + to_blocks and backfill",
			run:  func(tx *gorm.DB) error { return migrateAddChainId(tx, d.networkId) },
		},
		migrationStep{
			to:   6,
			name: "add chain_id to failed_fulfilments and backfill",
			run:  func(tx *gorm.DB) error { return migrateAddFailedFulfilmentChainId(tx, d.networkId) },
		},
	)
}

// runSchemaMigrations applies, in order, every step whose target version is ahead of
// the recorded schema version. Each step plus its version bump run in one transaction.
func (d *DB) runSchemaMigrations(steps []migrationStep) error {
	current, err := d.currentSchemaVersion()
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for _, step := range steps {
		if current >= step.to {
			continue // already applied
		}
		if current != step.to-1 {
			// Non-contiguous version: the version table is inconsistent. Refuse to
			// guess rather than apply a step from the wrong starting point.
			return fmt.Errorf("cannot apply migration to v%d: schema is at v%d, expected v%d",
				step.to, current, step.to-1)
		}

		logger.InfoWithFields("database", "runSchemaMigrations", "", "applying migration",
			logger.Fields{"to": step.to, "name": step.name})

		if err := d.Transaction(func(tx *gorm.DB) error {
			if err := step.run(tx); err != nil {
				return err
			}
			return setSchemaVersionTx(tx, step.to)
		}); err != nil {
			return fmt.Errorf("migration to v%d (%s): %w", step.to, step.name, err)
		}
		current = step.to
	}
	return nil
}

// currentSchemaVersion returns the recorded schema version, treating a missing row as
// version 0 (a fresh database).
func (d *DB) currentSchemaVersion() (uint64, error) {
	var v models.VersionInfo
	err := d.Where("version_type = ?", models.VERSION_TYPE_DB_SCHEMA).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return v.CurrentVersion, nil
}

// setSchemaVersionTx upserts the schema version inside the given transaction.
func setSchemaVersionTx(tx *gorm.DB, version uint64) error {
	var v models.VersionInfo
	err := tx.Where("version_type = ?", models.VERSION_TYPE_DB_SCHEMA).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&models.VersionInfo{
			VersionType:    models.VERSION_TYPE_DB_SCHEMA,
			CurrentVersion: version,
		}).Error
	}
	if err != nil {
		return err
	}
	v.CurrentVersion = version
	return tx.Save(&v).Error
}

// migrateDeleteAdhocTokenData clears the adhoc DEX/token cache tables. Idempotent:
// skips any table that does not exist.
func migrateDeleteAdhocTokenData(tx *gorm.DB) error {
	for _, table := range []string{"dex_pairs", "dex_tokens", "token_contracts"} {
		if tx.Migrator().HasTable(table) {
			if err := tx.Exec("DELETE FROM " + table).Error; err != nil {
				return fmt.Errorf("clear %s: %w", table, err)
			}
		}
	}
	return nil
}

// migrateDropLegacyDexTokens removes the retired dex_tokens table and the legacy
// DexPairs columns. Idempotent: every drop is guarded by an existence check, so it is
// a no-op on a fresh database (where AutoMigrate never created them).
func migrateDropLegacyDexTokens(tx *gorm.DB) error {
	if err := migrateDeleteAdhocTokenData(tx); err != nil {
		return err
	}

	m := tx.Migrator()
	if m.HasTable("dex_tokens") {
		if err := m.DropTable("dex_tokens"); err != nil {
			return fmt.Errorf("drop dex_tokens: %w", err)
		}
	}
	for _, col := range []string{"t0_dex_token_id", "t1_dex_token_id", "dex_name"} {
		if m.HasColumn(&models.DexPairs{}, col) {
			if err := m.DropColumn(&models.DexPairs{}, col); err != nil {
				return fmt.Errorf("drop dex_pairs.%s: %w", col, err)
			}
		}
	}
	return nil
}

// migrateAddChainId backfills the new chain_id column on data_requests + to_blocks with the
// deployment's configured network id (legacy single-chain rows predate the column, so AutoMigrate
// defaulted them to 0), and drops the obsolete single-column unique index on request_id - superseded
// by the composite (chain_id, request_id) index AutoMigrate created. Idempotent: the backfill only
// touches the chain_id = 0 sentinel (a valid network_id is never 0, enforced by ValidateBasic), and
// the index drop is existence-guarded (a no-op on a fresh DB, where the old index never existed).
func migrateAddChainId(tx *gorm.DB, networkId int64) error {
	if err := tx.Exec("UPDATE data_requests SET chain_id = ? WHERE chain_id = 0", networkId).Error; err != nil {
		return fmt.Errorf("backfill data_requests.chain_id: %w", err)
	}
	if err := tx.Exec("UPDATE to_blocks SET chain_id = ? WHERE chain_id = 0", networkId).Error; err != nil {
		return fmt.Errorf("backfill to_blocks.chain_id: %w", err)
	}
	// The old standalone unique index on request_id would wrongly reject a second chain's identical
	// request_id once N chains share the table - drop it in favour of the composite index.
	const oldRequestIdIndex = "idx_data_requests_request_id"
	m := tx.Migrator()
	if m.HasIndex(&models.DataRequests{}, oldRequestIdIndex) {
		if err := m.DropIndex(&models.DataRequests{}, oldRequestIdIndex); err != nil {
			return fmt.Errorf("drop old request_id unique index: %w", err)
		}
	}
	return nil
}

// migrateAddFailedFulfilmentChainId backfills the new chain_id column on failed_fulfilments with the
// deployment's configured network id (legacy single-chain rows predate the column, so AutoMigrate
// defaulted them to 0). It mirrors migrateAddChainId for the fulfilment-history table, so the per-chain
// "reverted" warm-start counter seeds correctly after a restart. Idempotent: only the chain_id = 0
// sentinel is touched (a valid network_id is never 0), so a fresh multi-chain DB (no legacy rows) is a
// no-op.
func migrateAddFailedFulfilmentChainId(tx *gorm.DB, networkId int64) error {
	if err := tx.Exec("UPDATE failed_fulfilments SET chain_id = ? WHERE chain_id = 0", networkId).Error; err != nil {
		return fmt.Errorf("backfill failed_fulfilments.chain_id: %w", err)
	}
	return nil
}

// migrateDropFinchainsRemnants removes the Finchains-only supported_pairs table and the now-dead
// is_adhoc column on data_requests (Finchains is gone; every request is a DEX query). Idempotent:
// each drop is existence-guarded, so it is a no-op on a fresh database (where AutoMigrate never
// created them) and safe to re-run on a populated one.
func migrateDropFinchainsRemnants(tx *gorm.DB) error {
	m := tx.Migrator()
	if m.HasTable("supported_pairs") {
		if err := m.DropTable("supported_pairs"); err != nil {
			return fmt.Errorf("drop supported_pairs: %w", err)
		}
	}
	if m.HasColumn(&models.DataRequests{}, "is_adhoc") {
		if err := m.DropColumn(&models.DataRequests{}, "is_adhoc"); err != nil {
			return fmt.Errorf("drop data_requests.is_adhoc: %w", err)
		}
	}
	return nil
}
