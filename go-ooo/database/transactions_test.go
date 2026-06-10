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

	req, found, err := d.FindByRequestId(reqId)
	require.NoError(t, err)
	require.True(t, found)
	require.EqualValues(t, 2, req.FulfillmentAttempts)
}

// TestFindByRequestId locks the not-found contract: a missing id returns found=false with
// no error (distinct from a real DB failure), and a present id returns found=true.
func TestFindByRequestId(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.Migrate())

	_, found, err := d.FindByRequestId("0xmissing")
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, d.InsertNewRequest("0xp", "0xc", "0xreqX", "WETH.USDC", "WETH.USDC", "0xtx", 0, 0, 1, 1, true))
	row, found, err := d.FindByRequestId("0xreqX")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "0xreqX", row.RequestId)
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
