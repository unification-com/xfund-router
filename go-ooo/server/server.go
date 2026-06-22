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
	srv                  *service.Service
	ctx                  context.Context
	cancel               context.CancelFunc
	srvCtx               *Context
	Vers                 version.Info
	privateKey           string
	db                   *database.DB
	decryptPass          string
	forceFirstBlock      uint64
	forceFirstBlockChain string
}

func NewServer(srcCtx *Context, decryptPass string, forceFirstBlock uint64, forceFirstBlockChain string) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		ctx:                  ctx,
		cancel:               cancel,
		srvCtx:               srcCtx,
		Vers:                 version.NewInfo(),
		decryptPass:          decryptPass,
		forceFirstBlock:      forceFirstBlock,
		forceFirstBlockChain: forceFirstBlockChain,
	}, nil
}

func (s *Server) InitServer() {
	logger.Info("app", "InitServer", "", s.Vers.StringLine())
	s.initServer()
}

func (s *Server) Run() {
	s.srv.Run()
	logger.Info("app", "Run", "", "oracle daemon stopped")
}

func (s *Server) initServer() {
	s.initDatabase()
	s.applyForceFirstBlock()
	s.initKeystore()
	s.initService()
	s.initSignal()
}

// applyForceFirstBlock advances ONE chain's event-scan resume cursor to the --first-block value before
// the service reads it, so that worker starts polling from there instead of grinding a stale gap. Each
// chain has its own chain_id-keyed cursor, so --first-block targets a chain via --chain; with a single
// chain configured the selector is optional. It is advance-only (InsertNewToBlock ignores a lower value)
// and persisted, so the override survives the next restart - drop the flag once it has taken effect.
func (s *Server) applyForceFirstBlock() {
	if s.forceFirstBlock == 0 {
		return
	}
	ch, err := s.srvCtx.Config.ResolveChain(s.forceFirstBlockChain)
	if err != nil {
		panic(fmt.Errorf("could not apply --first-block %d: %w", s.forceFirstBlock, err))
	}
	if err := s.db.InsertNewToBlock(ch.NetworkId, s.forceFirstBlock); err != nil {
		panic(fmt.Errorf("could not apply --first-block %d on network %d: %w", s.forceFirstBlock, ch.NetworkId, err))
	}
	logger.InfoWithFields("app", "applyForceFirstBlock", "",
		"advanced the event-scan cursor (--first-block) - the saved gap below this block is skipped",
		logger.Fields{"first_block": s.forceFirstBlock, "network_id": ch.NetworkId})
}

func (s *Server) initSignal() {
	// Must be buffered (>=1): signal.Notify does not block, so an unbuffered channel
	// can miss a signal that arrives before the receiver goroutine is ready.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		logger.Info("app", "initSignal", "",
			"shutdown signal received - stopping gracefully (interrupt again to force)")
		// Cancel the root context: the service main loop, the event watchers and
		// any in-flight RPC calls observe this and unwind cleanly, after which
		// Run() returns and the process exits.
		s.cancel()

		// A second signal forces an immediate exit if graceful shutdown hangs.
		<-c
		logger.Warn("app", "initSignal", "", "second signal received - forcing exit")
		os.Exit(1)
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
