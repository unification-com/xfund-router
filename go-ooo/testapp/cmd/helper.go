package cmd

import (
	"context"
	"go-ooo/config"
	"gorm.io/driver/sqlite"
	"log"
	"os"
	"time"

	"go-ooo/database"
	"go-ooo/ooo_api"
	"gorm.io/gorm"

	gorm_logger "gorm.io/gorm/logger"
)

func createApi() *ooo_api.OOOApi {
	ctx := context.Background()

	cfg := config.DefaultConfig()
	cfg.Log.Level = "debug"

	gormLogger := gorm_logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		gorm_logger.Config{
			SlowThreshold:             time.Second,      // Slow SQL threshold
			LogLevel:                  gorm_logger.Warn, // Log level
			IgnoreRecordNotFoundError: true,             // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,            // Disable color
		},
	)

	db, err := gorm.Open(sqlite.Open("/tmp/go-ooo_testapp.sqlite"), &gorm.Config{
		Logger: gormLogger,
	})

	if err != nil {
		panic(err)
	}
	dbConn := &database.DB{
		DB: db,
	}

	err = dbConn.Migrate()
	if err != nil {
		panic(err)
	}

	oooApi, err := ooo_api.NewApi(ctx, cfg, dbConn)

	if err != nil {
		panic(err)
	}

	return oooApi
}
