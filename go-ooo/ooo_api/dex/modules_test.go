package dex

import (
	"testing"

	"go-ooo/ooo_api/dex/families/messari"
	"go-ooo/ooo_api/dex/families/uniswap"

	"github.com/stretchr/testify/require"
)

func famMod(chain, dex string, fam SchemaFamily) Module {
	return NewFamilyModule(FamilyModuleSpec{Chain: chain, Dex: dex, Family: fam})
}

func TestSetModulesAndSnapshot(t *testing.T) {
	dm := &Manager{modules: map[string]Module{}}
	require.Empty(t, dm.snapshotModules())

	dm.SetModules([]Module{
		famMod("eth", "uniswap_v2", uniswap.New(uniswap.V2)),
		famMod("eth", "uniswap_v3", uniswap.New(uniswap.V3)),
	})
	require.Len(t, dm.snapshotModules(), 2)

	// A later SetModules replaces the set wholesale (not merge).
	dm.SetModules([]Module{famMod("arbitrum", "sushiswap", messari.New())})
	mods := dm.snapshotModules()
	require.Len(t, mods, 1)
	require.Equal(t, "arbitrum_sushiswap", mods[0].Name())
}

// Run with -race: concurrent SetModules + snapshotModules must not data-race.
func TestSetModulesConcurrent(t *testing.T) {
	dm := &Manager{modules: map[string]Module{}}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			dm.SetModules([]Module{famMod("eth", "uniswap_v2", uniswap.New(uniswap.V2))})
		}
		close(done)
	}()
	for i := 0; i < 1000; i++ {
		_ = dm.snapshotModules()
	}
	<-done
	require.Len(t, dm.snapshotModules(), 1)
}
