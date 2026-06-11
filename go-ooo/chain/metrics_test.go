package chain

import (
	"path/filepath"
	"testing"

	"go-ooo/database"
	"go-ooo/database/models"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newChainTestDB(t *testing.T) *database.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.sqlite")), &gorm.Config{})
	require.NoError(t, err)
	d := &database.DB{DB: gdb}
	require.NoError(t, d.Migrate())
	return d
}

func TestWarmStartFulfilmentMetrics(t *testing.T) {
	d := newChainTestDB(t)

	// 2 sent (r1, r2), 1 success (r1), 2 reverted attempts.
	require.NoError(t, d.InsertNewRequest("0xp", "0xc", "0xr1", "WETH.USDC", "WETH.USDC", "0xtx1", 0, 0, 1, 100, true))
	require.NoError(t, d.InsertNewRequest("0xp", "0xc", "0xr2", "WETH.USDC", "WETH.USDC", "0xtx2", 0, 0, 1, 100, true))
	require.NoError(t, d.UpdateFulfillmentSent("0xr1", "0xfh1", 101, 1, 1_000_000_000, 1_000_000_000))
	require.NoError(t, d.UpdateRequestStatus("0xr1", models.REQUEST_STATUS_SUCCESS, ""))
	require.NoError(t, d.UpdateFulfillmentSent("0xr2", "0xfh2", 102, 2, 2_000_000_000, 1_500_000_000))
	require.NoError(t, d.InsertNewFailedFulfilment("0xr2", "0xfh2a", 100000, 1_000_000_000, "reverted"))
	require.NoError(t, d.InsertNewFailedFulfilment("0xr2", "0xfh2b", 100000, 2_000_000_000, "reverted"))

	sentBefore := testutil.ToFloat64(fulfilmentSentTotal)
	successBefore := testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues("success"))
	revertedBefore := testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues("reverted"))

	WarmStartFulfilmentMetrics(d)

	require.EqualValues(t, 2, testutil.ToFloat64(fulfilmentSentTotal)-sentBefore, "sent")
	require.EqualValues(t, 1, testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues("success"))-successBefore, "success")
	require.EqualValues(t, 2, testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues("reverted"))-revertedBefore, "reverted")
}

func TestFulfilmentHistogramsRegistered(t *testing.T) {
	fulfilmentGasPriceGwei.Observe(weiToGwei(30_000_000_000))
	fulfilmentGasUsed.Observe(120000)
	fulfilmentBlocks.Observe(3)
	require.Equal(t, 1, testutil.CollectAndCount(fulfilmentGasPriceGwei))
	require.Equal(t, 1, testutil.CollectAndCount(fulfilmentGasUsed))
	require.Equal(t, 1, testutil.CollectAndCount(fulfilmentBlocks))
}
