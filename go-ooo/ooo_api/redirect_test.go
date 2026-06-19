package ooo_api

import (
	"context"
	"path/filepath"
	"testing"

	"go-ooo/config"
	"go-ooo/database"
	"go-ooo/ooo_api/dex"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newRedirectTestApi builds an OOOApi with an EMPTY dex manager: no modules means no subgraph
// network calls and no pairs, so every price lookup misses - exactly what the redirect's
// "pair not on any DEX" path needs.
func newRedirectTestApi(t *testing.T) *OOOApi {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.sqlite")), &gorm.Config{})
	require.NoError(t, err)
	db := &database.DB{DB: gdb}
	require.NoError(t, db.Migrate())

	ctx := context.Background()
	cfg := config.DefaultConfig()
	return &OOOApi{
		db:               db,
		ctx:              ctx,
		dexModuleManager: dex.NewDexManager(ctx, cfg, db),
		quality:          cfg.PriceQuality,
	}
}

func TestRouteQueryRedirectsFinchainsShapeToDex(t *testing.T) {
	api := newRedirectTestApi(t)

	// A legacy Finchains-shape (.PR) request for a pair on no DEX returns the single, clear
	// ErrFinchainsRemoved - the lossy redirect tried the DEX path and missed.
	_, err := api.RouteQuery("BTC.GBP.PR.AVC.1H", "req-pr")
	require.ErrorIs(t, err, ErrFinchainsRemoved)

	// The canonical AdHoc arm for a pair on no DEX errors too, but NOT as ErrFinchainsRemoved -
	// only the Finchains-redirect arm maps a miss to that message.
	_, err = api.RouteQuery("WETH.USDC", "req-ad")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrFinchainsRemoved)
}
