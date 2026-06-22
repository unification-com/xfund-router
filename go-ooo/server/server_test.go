package server

import (
	"path/filepath"
	"testing"

	"go-ooo/config"
	"go-ooo/database"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// multiChainServer wires a Server over a fresh sqlite DB with two chains configured, for the
// --first-block override tests.
func multiChainServer(t *testing.T, firstBlock uint64, chain string) (*Server, *database.DB) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Chain = config.ChainConfig{}
	cfg.Chains = []config.ChainConfig{
		{Name: "sepolia", NetworkId: 11155111},
		{Name: "polygon", NetworkId: 137},
	}
	cfg.Database.Dialect = "sqlite"
	cfg.Database.Storage = filepath.Join(t.TempDir(), "go-ooo.db")

	db, err := database.NewDb(cfg)
	require.NoError(t, err)
	require.NoError(t, db.Migrate())

	return &Server{
		srvCtx:               &Context{Config: cfg},
		db:                   db,
		forceFirstBlock:      firstBlock,
		forceFirstBlockChain: chain,
	}, db
}

// applyForceFirstBlock writes the --first-block override onto ONLY the chain the --chain selector
// resolves to, leaving every other chain's cursor untouched (each chain has its own chain_id cursor).
func TestApplyForceFirstBlockPerChain(t *testing.T) {
	s, db := multiChainServer(t, 15114000, "polygon")
	s.applyForceFirstBlock()

	// The resolved chain's cursor advanced...
	poly, err := db.GetLastBlockNumQueried(137)
	require.NoError(t, err)
	require.EqualValues(t, 15114000, poly.GetBlockNum())

	// ...and the other chain's cursor was left alone.
	_, err = db.GetLastBlockNumQueried(11155111)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// --first-block with several chains and no --chain is ambiguous: ResolveChain errors, so the override
// is rejected (panicked at start-up) rather than guessing a chain.
func TestApplyForceFirstBlockAmbiguous(t *testing.T) {
	s, _ := multiChainServer(t, 15114000, "")
	require.Panics(t, func() { s.applyForceFirstBlock() })
}

// No --first-block is a no-op even with several chains (no --chain needed).
func TestApplyForceFirstBlockNoOp(t *testing.T) {
	s, db := multiChainServer(t, 0, "")
	require.NotPanics(t, func() { s.applyForceFirstBlock() })
	_, err := db.GetLastBlockNumQueried(137)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
