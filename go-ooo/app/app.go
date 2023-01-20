package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/keystore"
	"go-ooo/logger"
	"go-ooo/service"
	"go-ooo/version"

	"github.com/spf13/viper"
)

type Server struct {
	srv         *service.Service
	logger      *logger.Logger
	ctx         context.Context
	Vers        version.Info
	keystore    *keystore.Keystorage
	db          *database.DB
	decryptPass string
}

func NewServer(decryptPass string) (*Server, error) {
	ctx := context.Background()
	log := logger.NewAppLogger()

	return &Server{
		logger:      log,
		ctx:         ctx,
		Vers:        version.NewInfo(),
		decryptPass: decryptPass,
	}, nil
}

func (s *Server) InitServer() {
	s.logger.Info("main", "InitServer", "", s.Vers.StringLine())
	s.initServer()
}

func (s *Server) Run() {
	s.srv.Run()
}

func (s *Server) initServer() {
	s.initLogger()
	s.initDatabase()
	s.initKeystore()
	s.initService()
	s.initSignal()
}

func (s *Server) initLogger() {
	s.logger.InitLogger()
}

func (s *Server) initSignal() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		s.srv.Stop()

		s.logger.Info("main", "initSignal", "", "exiting oracle daemon...")

		os.Exit(0)
	}()
}

func (s *Server) initKeystore() {

	s.logger.Info("main", "initKeystore", "", "initialise keystore")

	ks, err := keystore.NewKeyStorage(s.logger, viper.GetString(config.KeystorageFile))
	if err != nil {
		s.logger.Warn("main", "initKeystore", "open keystorage",
			"can't read keystorage, creating a new one...")
	}

	s.keystore = ks

	decryptPassword := ""
	if s.decryptPass != "" {
		decryptPassword = getPasswordFromFileOrFlag(s.decryptPass)
	}

	if decryptPassword == "" || (s.keystore.CheckToken(decryptPassword) != nil) {
		err = s.auth()
		if err != nil {
			panic(err)
		}
	}

	err = s.keystore.SelectPrivateKey(viper.GetString(config.KeystorageAccount))
	if err != nil {
		panic(err)
	}
}

func (s *Server) initDatabase() {

	s.logger.Info("main", "initDatabase", "", "initialise database")

	dbConn, err := database.NewDb()
	if err != nil {
		panic(err)
	}
	s.db = dbConn
	err = s.db.Migrate()
	if err != nil {
		panic(err)
	}
}

func (s *Server) initService() {
	s.logger.Info("main", "initService", "", "initialise service")

	srv, err := service.NewService(s.ctx, s.logger, []byte(s.keystore.GetSelectedPrivateKey()),
		s.db, s.keystore.KeyStore.GetToken())
	if err != nil {
		panic(err)
	}
	s.srv = srv
}
