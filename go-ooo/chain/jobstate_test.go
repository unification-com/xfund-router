package chain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBlockDiff(t *testing.T) {
	require.Equal(t, uint64(5), blockDiff(100, 95))
	require.Equal(t, uint64(0), blockDiff(100, 100))
	// stored ahead of current (reorg / stale head) must saturate to 0, not wrap
	require.Equal(t, uint64(0), blockDiff(100, 101))
	require.Equal(t, uint64(0), blockDiff(0, 1_000_000))
}

func TestDecideGiveUp(t *testing.T) {
	maxAge := time.Hour

	// under both limits -> keep trying
	give, _ := decideGiveUp(2, 30*time.Minute, maxAge)
	require.False(t, give)

	// too many attempts
	give, reason := decideGiveUp(maxFulfilmentAttempts, time.Minute, maxAge)
	require.True(t, give)
	require.Equal(t, "too many failed attempts", reason)

	// too old
	give, reason = decideGiveUp(0, 2*time.Hour, maxAge)
	require.True(t, give)
	require.Equal(t, "request too old", reason)

	// age check disabled (maxAge 0) -> only attempts matter
	give, _ = decideGiveUp(0, 999*time.Hour, 0)
	require.False(t, give)
}
