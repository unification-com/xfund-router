package database

import (
	"errors"
	"fmt"
	"go-ooo/config"
	"go-ooo/database/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
)

type DB struct {
	*gorm.DB
}

func NewDb(cfg *config.Config) (*DB, error) {
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,       // Disable color
		},
	)
	switch cfg.Database.Dialect {
	case "sqlite":
		return NewSqliteDb(cfg, gormLogger)
	case "postgres":
		return NewPostgresDb(cfg, gormLogger)
	default:
		return nil, errors.New("no db dialect in config")
	}
}

func NewSqliteDb(cfg *config.Config, logger logger.Interface) (*DB, error) {
	db, err := gorm.Open(sqlite.Open(cfg.Database.Storage), &gorm.Config{
		Logger: logger,
	})
	if err != nil {
		// Don't hand back a wrapper around a failed/nil handle alongside the error.
		return nil, err
	}

	// SQLite allows only one writer. The oracle fans DB writes out across goroutines
	// (the per-job fetch goroutines + the event watcher), so cap the pool at a single
	// connection and add a busy timeout to avoid "database is locked" errors.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err = db.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func NewPostgresDb(cfg *config.Config, logger logger.Interface) (*DB, error) {
	host := cfg.Database.Host
	port := cfg.Database.Port
	user := cfg.Database.User
	dbName := cfg.Database.Database
	password := cfg.Database.Password
	if host == "" || port == 0 {
		// Fail fast with a clear error rather than returning (nil, nil), which a
		// caller would later dereference as a nil-pointer panic.
		return nil, errors.New("postgres host and port must be set in config")
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s dbname=%s password=%s sslmode=disable",
		host, port, user, dbName, password)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger,
	})

	if err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

func (d *DB) Migrate() error {
	// 1. Sync the schema (additive: tables/columns/indexes). Abort on failure - the
	//    data migrations below assume the schema is already in shape.
	if err := d.AutoMigrate(
		&models.DataRequests{},
		&models.FailedFulfilment{},
		&models.ToBlocks{},
		&models.SupportedPairs{},
		&models.DexPairs{},
		&models.TokenContracts{},
		&models.VersionInfo{},
	); err != nil {
		return fmt.Errorf("auto-migrate schema: %w", err)
	}

	// 2. Apply ordered, transactional, version-guarded data migrations.
	return d.runSchemaMigrations(schemaMigrations)
}
