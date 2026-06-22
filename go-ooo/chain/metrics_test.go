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

	// Chain 1: 2 sent (r1, r2), 1 success (r1), 2 reverted attempts.
	require.NoError(t, d.InsertNewRequest(1, "0xp", "0xc", "0xr1", "WETH.USDC", "WETH.USDC", "0xtx1", 0, 0, 1, 100))
	require.NoError(t, d.InsertNewRequest(1, "0xp", "0xc", "0xr2", "WETH.USDC", "WETH.USDC", "0xtx2", 0, 0, 1, 100))
	require.NoError(t, d.UpdateFulfillmentSent(1, "0xr1", "0xfh1", 101, 1, 1_000_000_000, 1_000_000_000))
	require.NoError(t, d.UpdateRequestStatus(1, "0xr1", models.REQUEST_STATUS_SUCCESS, ""))
	require.NoError(t, d.UpdateFulfillmentSent(1, "0xr2", "0xfh2", 102, 2, 2_000_000_000, 1_500_000_000))
	require.NoError(t, d.InsertNewFailedFulfilment(1, "0xr2", "0xfh2a", 100000, 1_000_000_000, "reverted"))
	require.NoError(t, d.InsertNewFailedFulfilment(1, "0xr2", "0xfh2b", 100000, 2_000_000_000, "reverted"))

	// Chain 2: 1 sent (r3), 1 success - seeded under its own label, must not bleed into chain 1.
	require.NoError(t, d.InsertNewRequest(2, "0xp", "0xc", "0xr3", "WETH.USDC", "WETH.USDC", "0xtx3", 0, 0, 1, 100))
	require.NoError(t, d.UpdateFulfillmentSent(2, "0xr3", "0xfh3", 201, 1, 1_000_000_000, 1_000_000_000))
	require.NoError(t, d.UpdateRequestStatus(2, "0xr3", models.REQUEST_STATUS_SUCCESS, ""))

	sentBefore := testutil.ToFloat64(fulfilmentSentTotal.WithLabelValues("1"))
	successBefore := testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues("1", "success"))
	revertedBefore := testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues("1", "reverted"))
	sent2Before := testutil.ToFloat64(fulfilmentSentTotal.WithLabelValues("2"))

	WarmStartFulfilmentMetrics(d, []int64{1, 2})

	require.EqualValues(t, 2, testutil.ToFloat64(fulfilmentSentTotal.WithLabelValues("1"))-sentBefore, "chain 1 sent")
	require.EqualValues(t, 1, testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues("1", "success"))-successBefore, "chain 1 success")
	require.EqualValues(t, 2, testutil.ToFloat64(fulfilmentResultTotal.WithLabelValues("1", "reverted"))-revertedBefore, "chain 1 reverted")
	require.EqualValues(t, 1, testutil.ToFloat64(fulfilmentSentTotal.WithLabelValues("2"))-sent2Before, "chain 2 sent counted under its own label")
}

func TestFulfilmentHistogramsRegistered(t *testing.T) {
	fulfilmentGasPriceGwei.WithLabelValues("1").Observe(weiToGwei(30_000_000_000))
	fulfilmentGasUsed.WithLabelValues("1").Observe(120000)
	fulfilmentBlocks.WithLabelValues("1").Observe(3)
	require.Equal(t, 1, testutil.CollectAndCount(fulfilmentGasPriceGwei))
	require.Equal(t, 1, testutil.CollectAndCount(fulfilmentGasUsed))
	require.Equal(t, 1, testutil.CollectAndCount(fulfilmentBlocks))
}
