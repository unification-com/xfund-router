package cmd

import (
	"context"
	"gorm.io/driver/sqlite"
	"log"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/ooo_api"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func createApi() *ooo_api.OOOApi {
	ctx := context.Background()

	logr := logrus.New()
	logr.SetLevel(5)
	logr.SetOutput(os.Stdout)

	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,       // Disable color
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

	viper.Set(config.SubChainEthHttpRpc, "https://eth.althea.net")
	viper.Set(config.SubChainPolygonHttpRpc, "https://polygon-rpc.com")
	viper.Set(config.SubChainBcsHttpRpc, "https://bsc-dataseed.binance.org")
	viper.Set(config.SubChainXdaiHttpRpc, "https://rpc.gnosischain.com")
	viper.Set(config.SubChainFantomHttpRpc, "https://rpc.ankr.com/fantom")
	viper.Set(config.JobsOooApiUrl, "https://finchains.io/api")

	oooApi, err := ooo_api.NewApi(ctx, dbConn, logr)

	if err != nil {
		panic(err)
	}

	return oooApi
}
