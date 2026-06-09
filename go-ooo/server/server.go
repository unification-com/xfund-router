package server

import (
	"context"
	"fmt"
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
	privateKey  string
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

	data, err := os.ReadFile(cfg.Keystore.File)
	if err != nil {
		panic(fmt.Errorf("cannot read keystore %s: %w", cfg.Keystore.File, err))
	}

	// A legacy keystore must be migrated before it can be used.
	if keystore.IsLegacyKeystore(data) {
		panic(fmt.Errorf("keystore %s is in the old format - run 'go-ooo keystore migrate' first",
			cfg.Keystore.File))
	}
	if !keystore.IsV3Keystore(data) {
		panic(fmt.Errorf("keystore %s is not a recognised v3 keystore", cfg.Keystore.File))
	}

	// Try the supplied --pass (file or value) first, then prompt and retry.
	pass := ""
	if s.decryptPass != "" {
		pass = getPasswordFromFileOrFlag(s.decryptPass)
	}

	for {
		if pass != "" {
			privHex, address, derr := keystore.DecryptKeyV3Hex(data, pass)
			if derr == nil {
				s.privateKey = privHex
				logger.InfoWithFields("app", "initKeystore", "", "keystore unlocked",
					logger.Fields{"address": address})
				return
			}
			fmt.Println("Could not decrypt the keystore with that passphrase.")
		}

		entered, perr := promptPassphrase()
		if perr != nil {
			panic(fmt.Errorf("failed to read keystore passphrase: %w", perr))
		}
		pass = entered
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

	srv, err := service.NewService(s.ctx, cfg, []byte(s.privateKey),
		s.db, adminTokenHash)
	if err != nil {
		panic(err)
	}
	s.srv = srv
}
