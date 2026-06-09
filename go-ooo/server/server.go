package server

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go-ooo/auth"
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
	// Must be buffered (>=1): signal.Notify does not block, so an unbuffered channel
	// can miss a signal that arrives before the receiver goroutine is ready.
	c := make(chan os.Signal, 1)
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

	cfg := s.srvCtx.Config

	// The admin HTTP bearer is decoupled from the keystore: its bcrypt hash lives
	// in a sidecar next to the keystore (written by `keystore migrate` / `init`).
	// A missing sidecar leaves the admin API disabled rather than failing start.
	adminTokenHash, err := auth.ReadHashFile(cfg.Keystore.File)
	if err != nil {
		panic(err)
	}
	if adminTokenHash == "" {
		logger.Warn("app", "initService", "",
			"no admin token configured - the admin HTTP API is disabled; run "+
				"'go-ooo keystore set-admin-token' to create one")
	}

	srv, err := service.NewService(s.ctx, cfg, []byte(s.keystore.GetSelectedPrivateKey()),
		s.db, adminTokenHash)
	if err != nil {
		panic(err)
	}
	s.srv = srv
}
