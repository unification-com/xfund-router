package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go-ooo/database"
	"go-ooo/keystore"
	"go-ooo/logger"
	"go-ooo/service"
	"go-ooo/version"
)

type Server struct {
	srv         *service.Service
	ctx         context.Context
	srvCtx      *Context
	Vers        version.Info
	keystore    *keystore.Keystorage
	db          *database.DB
	decryptPass string
}

func NewServer(srcCtx *Context, decryptPass string) (*Server, error) {
	ctx := context.Background()

	return &Server{
		ctx:         ctx,
		srvCtx:      srcCtx,
		Vers:        version.NewInfo(),
		decryptPass: decryptPass,
	}, nil
}

func (s *Server) InitServer() {
	logger.Info("app", "InitServer", "", s.Vers.StringLine())
	s.initServer()
}

func (s *Server) Run() {
	s.srv.Run()
}

func (s *Server) initServer() {
	s.initDatabase()
	s.initKeystore()
	s.initService()
	s.initSignal()
}

func (s *Server) initSignal() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		s.srv.Stop()

		logger.Info("app", "initSignal", "", "exiting oracle daemon...")

		os.Exit(0)
	}()
}

func (s *Server) initKeystore() {

	cfg := s.srvCtx.Config

	logger.Info("app", "initKeystore", "", "initialise keystore")

	logger.Debug("app", "initKeystore", "", "", logger.Fields{
		"keystore": cfg.Keystore.File,
	})

	ks, err := keystore.NewKeyStorage(cfg.Keystore.File)
	if err != nil {
		logger.Warn("app", "initKeystore", "open keystorage",
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

	err = s.keystore.SelectPrivateKey(cfg.Keystore.Account)
	if err != nil {
		panic(err)
	}
}

func (s *Server) initDatabase() {

	logger.Info("app", "initDatabase", "", "initialise database")

	dbConn, err := database.NewDb(s.srvCtx.Config)
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
	logger.Info("app", "initService", "", "initialise service")

	srv, err := service.NewService(s.ctx, s.srvCtx.Config, []byte(s.keystore.GetSelectedPrivateKey()),
		s.db, s.keystore.KeyStore.GetToken())
	if err != nil {
		panic(err)
	}
	s.srv = srv
}
