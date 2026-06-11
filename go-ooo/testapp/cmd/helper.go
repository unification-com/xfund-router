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

func createApi() *ooo_api.OOOApi {
	ctx := context.Background()

	cfg := config.DefaultConfig()
	cfg.Log.Level = "debug"
	logger.SetLogLevel(cfg.Log.Level)

	// API keys for testing
	cfg.ApiKeys.GraphNetwork = graphNetApi

	// DB: sqlite by default, or postgres (e.g. a restored production dump) via the --db-* flags.
	// Reuse the application's NewDb so the testapp exercises the exact connection + migration
	// path go-ooo runs in production - including schema migrations against a real dump.
	cfg.Database.Dialect = dbDialect
	cfg.Database.Storage = dbStorage
	cfg.Database.Host = dbHost
	cfg.Database.Port = dbPort
	cfg.Database.User = dbUser
	cfg.Database.Database = dbName
	cfg.Database.Password = dbPass

	dbConn, err := database.NewDb(cfg)
	if err != nil {
		logger.Fatal("cmd", "createApi", "database.NewDb", err.Error())
	}

	if err := dbConn.Migrate(); err != nil {
		logger.Fatal("cmd", "createApi", "db.Migrate", err.Error())
	}

	oooApi, err := ooo_api.NewApi(ctx, cfg, dbConn)
	if err != nil {
		logger.Fatal("cmd", "createApi", "ooo_api.NewApi", err.Error())
	}

	return oooApi
}
