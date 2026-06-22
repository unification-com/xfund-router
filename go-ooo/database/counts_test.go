package database

import (
	"testing"

	"go-ooo/database/models"

	"github.com/stretchr/testify/require"
)

func TestFulfilmentCounts(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())

	// Three requests; r1 sent + success, r2 sent only, r3 never sent.
	require.NoError(t, d.InsertNewRequest(1, "0xp", "0xc", "0xr1", "WETH.USDC", "WETH.USDC", "0xtx1", 0, 0, 1, 100))
	require.NoError(t, d.InsertNewRequest(1, "0xp", "0xc", "0xr2", "WETH.USDC", "WETH.USDC", "0xtx2", 0, 0, 1, 100))
	require.NoError(t, d.InsertNewRequest(1, "0xp", "0xc", "0xr3", "WETH.USDC", "WETH.USDC", "0xtx3", 0, 0, 1, 100))

	require.NoError(t, d.UpdateFulfillmentSent(1, "0xr1", "0xfh1", 101, 1, 1_000_000_000, 1_000_000_000))
	require.NoError(t, d.UpdateRequestStatus(1, "0xr1", models.REQUEST_STATUS_SUCCESS, ""))
	require.NoError(t, d.UpdateFulfillmentSent(1, "0xr2", "0xfh2", 102, 2, 2_000_000_000, 1_500_000_000))

	// Two reverted attempts recorded for r2.
	require.NoError(t, d.InsertNewFailedFulfilment(1, "0xr2", "0xfh2a", 100000, 1_000_000_000, "reverted"))
	require.NoError(t, d.InsertNewFailedFulfilment(1, "0xr2", "0xfh2b", 100000, 2_000_000_000, "reverted"))

	// A second chain's rows must not leak into chain 1's counts.
	require.NoError(t, d.InsertNewRequest(2, "0xp", "0xc", "0xr9", "WETH.USDC", "WETH.USDC", "0xtx9", 0, 0, 1, 100))
	require.NoError(t, d.UpdateFulfillmentSent(2, "0xr9", "0xfh9", 201, 1, 1_000_000_000, 1_000_000_000))
	require.NoError(t, d.InsertNewFailedFulfilment(2, "0xr9", "0xfh9a", 100000, 1_000_000_000, "reverted"))

	sent, err := d.CountFulfilmentsSent(1)
	require.NoError(t, err)
	require.EqualValues(t, 2, sent, "r1 and r2 have a fulfil tx hash")

	success, err := d.CountRequestsByStatus(1, models.REQUEST_STATUS_SUCCESS)
	require.NoError(t, err)
	require.EqualValues(t, 1, success)

	reverted, err := d.CountFailedFulfilments(1)
	require.NoError(t, err)
	require.EqualValues(t, 2, reverted)

	// Chain 2 is counted independently.
	sent2, err := d.CountFulfilmentsSent(2)
	require.NoError(t, err)
	require.EqualValues(t, 1, sent2)
	reverted2, err := d.CountFailedFulfilments(2)
	require.NoError(t, err)
	require.EqualValues(t, 1, reverted2)
}
