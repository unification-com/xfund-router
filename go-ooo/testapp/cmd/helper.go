package cmd

import (
	"context"

	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/logger"
	"go-ooo/ooo_api"
)

const (
	FlagGraphNetworkApiKey = "graphnetapi"
)

var (
	graphNetApi string

	dbDialect string
	dbStorage string
	dbHost    string
	dbPort    uint64
	dbUser    string
	dbName    string
	dbPass    string
)

// baseConfig builds a default config with the DB selection from the --db-* flags applied: sqlite
// by default, or postgres (e.g. a restored production dump) when --db-dialect=postgres. The log
// level is the caller's to set.
func baseConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Database.Dialect = dbDialect
	cfg.Database.Storage = dbStorage
	cfg.Database.Host = dbHost
	cfg.Database.Port = dbPort
	cfg.Database.User = dbUser
	cfg.Database.Database = dbName
	cfg.Database.Password = dbPass
	return cfg
}

// openDb opens the configured database without migrating it - so read-only tools (the report)
// can safely point at a live or dumped DB without altering its schema.
func openDb(cfg *config.Config) *database.DB {
	dbConn, err := database.NewDb(cfg)
	if err != nil {
		logger.Fatal("cmd", "openDb", "database.NewDb", err.Error())
	}
	return dbConn
}

func createApi() *ooo_api.OOOApi {
	ctx := context.Background()

	cfg := baseConfig()
	cfg.Log.Level = "debug"
	logger.SetLogLevel(cfg.Log.Level)

	// API keys for testing
	cfg.ApiKeys.GraphNetwork = graphNetApi

	// Reuse the application's NewDb + Migrate so the testapp exercises the exact connection +
	// migration path go-ooo runs in production - including schema migrations against a real dump.
	dbConn := openDb(cfg)
	if err := dbConn.Migrate(); err != nil {
		logger.Fatal("cmd", "createApi", "db.Migrate", err.Error())
	}

	oooApi, err := ooo_api.NewApi(ctx, cfg, dbConn)
	if err != nil {
		logger.Fatal("cmd", "createApi", "ooo_api.NewApi", err.Error())
	}

	return oooApi
}
