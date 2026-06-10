package database

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIncrementFulfillmentAttempts locks the atomic increment: two increments leave
// the counter at exactly 2 (the old read-modify-write could lose an update).
func TestIncrementFulfillmentAttempts(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())

	const reqId = "0xreq1"
	require.NoError(t, d.InsertNewRequest("0xprov", "0xcons", reqId, "WETH.USDC", "WETH.USDC", "0xtx", 0, 0, 1, 100, true))

	require.NoError(t, d.IncrementFulfillmentAttempts(reqId))
	require.NoError(t, d.IncrementFulfillmentAttempts(reqId))

	req, err := d.FindByRequestId(reqId)
	require.NoError(t, err)
	require.EqualValues(t, 2, req.FulfillmentAttempts)
}

// TestInsertNewToBlock checks the resume-point guard: the first insert on an empty
// table works (record-not-found is tolerated), a lower/equal block is a no-op, and a
// higher block advances the head.
func TestInsertNewToBlock(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())

	require.NoError(t, d.InsertNewToBlock(100)) // first insert on an empty table
	last, err := d.GetLastBlockNumQueried()
	require.NoError(t, err)
	require.EqualValues(t, 100, last.GetBlockNum())

	require.NoError(t, d.InsertNewToBlock(50)) // lower → no-op
	last, err = d.GetLastBlockNumQueried()
	require.NoError(t, err)
	require.EqualValues(t, 100, last.GetBlockNum())

	require.NoError(t, d.InsertNewToBlock(200)) // higher → advances
	last, err = d.GetLastBlockNumQueried()
	require.NoError(t, err)
	require.EqualValues(t, 200, last.GetBlockNum())
}
