package report

import (
	"testing"

	"go-ooo/database/models"

	"github.com/stretchr/testify/require"
)

const gwei = 1e9

func TestBuild(t *testing.T) {
	requests := []models.DataRequests{
		// consumer A / WETH.USDC: two successes.
		{RequestId: "r1", Consumer: "0xA", EndpointDecoded: "WETH.USDC", Fee: 1_000_000,
			FulfillGasUsed: 100_000, FulfillGasPrice: 20 * gwei, RequestStatus: models.REQUEST_STATUS_SUCCESS},
		{RequestId: "r2", Consumer: "0xA", EndpointDecoded: "WETH.USDC", Fee: 2_000_000,
			FulfillGasUsed: 100_000, FulfillGasPrice: 10 * gwei, RequestStatus: models.REQUEST_STATUS_SUCCESS},
		// consumer B / BONE.WETH: one terminal failure, one still pending.
		{RequestId: "r3", Consumer: "0xB", EndpointDecoded: "BONE.WETH",
			RequestStatus: models.REQUEST_STATUS_FULFILMENT_FAILED, StatusReason: "tx reverted"},
		{RequestId: "r4", Consumer: "0xB", EndpointDecoded: "BONE.WETH",
			RequestStatus: models.REQUEST_STATUS_INITIALISED},
	}
	failed := []models.FailedFulfilment{
		{RequestId: "r1", GasUsed: 100_000, GasPrice: 5 * gwei},  // r1 reverted once before succeeding
		{RequestId: "r3", GasUsed: 100_000, GasPrice: 30 * gwei}, // r3's failed attempt
		{RequestId: "rX", GasUsed: 100_000, GasPrice: 99 * gwei}, // orphan: request not in the set -> ignored
	}

	r := Build(requests, failed, 0.01)

	// Overall counts + rate.
	require.Equal(t, 4, r.Overall.TotalRequests)
	require.Equal(t, 2, r.Overall.Successful)
	require.Equal(t, 1, r.Overall.FulfilmentFailed)
	require.Equal(t, 1, r.Overall.Pending)
	require.InDelta(t, 66.667, r.Overall.SuccessRatePct, 0.01)
	require.Equal(t, 2, r.Overall.RevertedAttempts) // the orphan (rX) is not counted

	// Fees: 0.001 + 0.002 xFUND.
	require.InDelta(t, 0.003, r.Overall.FeesEarnedXfund, 1e-12)
	// Gas: winning 0.002 + 0.001, reverted 0.0005 (r1) + 0.003 (r3) = 0.0065 ETH (orphan excluded).
	require.InDelta(t, 0.0065, r.Overall.GasCostEth, 1e-12)
	// P&L: 0.003 * 0.01 - 0.0065.
	require.InDelta(t, 0.003*0.01, r.Overall.FeesEarnedEth, 1e-15)
	require.InDelta(t, 0.003*0.01-0.0065, r.Overall.ProfitLossEth, 1e-12)

	// Per consumer (tie on 2 reqs -> sorted by key, A before B).
	require.Len(t, r.ByConsumer, 2)
	require.Equal(t, "0xA", r.ByConsumer[0].Key)
	require.Equal(t, 2, r.ByConsumer[0].Successful)
	require.InDelta(t, 0.003, r.ByConsumer[0].FeesEarnedXfund, 1e-12)
	require.InDelta(t, 0.0035, r.ByConsumer[0].GasCostEth, 1e-12) // winning 0.003 + r1 revert 0.0005
	require.Equal(t, "0xB", r.ByConsumer[1].Key)
	require.Equal(t, 1, r.ByConsumer[1].FulfilmentFailed)
	require.InDelta(t, 0.003, r.ByConsumer[1].GasCostEth, 1e-12) // r3 revert only

	// Per pair (tie on 2 reqs -> sorted by key, BONE.WETH before WETH.USDC).
	require.Len(t, r.ByPair, 2)
	require.Equal(t, "BONE.WETH", r.ByPair[0].Key)
	require.Equal(t, "WETH.USDC", r.ByPair[1].Key)
	require.InDelta(t, 0.003, r.ByPair[1].FeesEarnedXfund, 1e-12)

	// Failures.
	require.Len(t, r.Failures, 1)
	require.Equal(t, "tx reverted", r.Failures[0].Reason)
	require.Equal(t, 1, r.Failures[0].Count)
}

func TestBuildEmpty(t *testing.T) {
	r := Build(nil, nil, 0)
	require.Equal(t, 0, r.Overall.TotalRequests)
	require.Equal(t, 0.0, r.Overall.SuccessRatePct) // no divide-by-zero
	require.Empty(t, r.ByConsumer)
	require.Empty(t, r.ByPair)
	require.Empty(t, r.Failures)
}

func TestBuildUnknownFailureReason(t *testing.T) {
	// A terminal failure with no reason recorded should bucket under "(unknown)".
	r := Build([]models.DataRequests{
		{RequestId: "r1", Consumer: "0xA", EndpointDecoded: "WETH.USDC",
			RequestStatus: models.REQUEST_STATUS_FULFILMENT_FAILED},
	}, nil, 0)
	require.Len(t, r.Failures, 1)
	require.Equal(t, "(unknown)", r.Failures[0].Reason)
}
