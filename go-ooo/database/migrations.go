package database

import "go-ooo/database/models"

/*
  Migrations
*/

// Schema V0 to V1

func (d *DB) MigrateV0ToV1() {
	dbVers, _ := d.getCurrentDbSchemaVersion()
	if dbVers.CurrentVersion == 0 {
		d.DeleteAdhocTokenData()
		_ = d.setDbSchemaVersion(1)
	}
}

func (d *DB) MigrateV1ToV2() {
	dbVers, _ := d.getCurrentDbSchemaVersion()
	if dbVers.CurrentVersion == 1 {
		d.DeleteAdhocTokenData()
		_ = d.setDbSchemaVersion(2)
	}
}

func (d *DB) MigrateV2ToV3() {
	dbVers, _ := d.getCurrentDbSchemaVersion()
	if dbVers.CurrentVersion == 2 {
		d.DeleteAdhocTokenData()

		if d.Migrator().HasTable("dex_tokens") {
			d.Migrator().DropTable("dex_tokens")
		}

		d.Migrator().DropColumn(&models.DexPairs{}, "t0_dex_token_id")
		d.Migrator().DropColumn(&models.DexPairs{}, "t1_dex_token_id")
		d.Migrator().DropColumn(&models.DexPairs{}, "dex_name")

		_ = d.setDbSchemaVersion(3)
	}
}

// DeleteAdhocTokenData is used when migrating from Db v0 to v1
func (d *DB) DeleteAdhocTokenData() {
	if d.Migrator().HasTable("dex_pairs") {
		d.Exec("DELETE FROM dex_pairs")
	}

	if d.Migrator().HasTable("dex_tokens") {
		d.Exec("DELETE FROM dex_tokens")
	}

	if d.Migrator().HasTable("token_contracts") {
		d.Exec("DELETE FROM token_contracts")
	}
}
