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
	return &DB{
		db,
	}, err
}

func NewPostgresDb(cfg *config.Config, logger logger.Interface) (*DB, error) {
	host := cfg.Database.Host
	port := cfg.Database.Port
	user := cfg.Database.User
	dbName := cfg.Database.Database
	password := cfg.Database.Password
	if host == "" || port == 0 {
		return nil, nil
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

func (d *DB) Migrate() (err error) {
	// migrate models
	err = d.AutoMigrate(
		&models.DataRequests{},
		&models.FailedFulfilment{},
		&models.ToBlocks{},
		&models.SupportedPairs{},
		&models.DexTokens{},
		&models.DexPairs{},
		&models.TokenContracts{},
		&models.VersionInfo{},
	)

	// post-model data migration
	d.MigrateV0ToV1()

	d.MigrateV1ToV2()

	return
}
