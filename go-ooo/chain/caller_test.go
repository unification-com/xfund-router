package chain

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func gwei(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), big.NewInt(params.GWei))
}

func TestComputeReplacementGasPrice(t *testing.T) {
	// +13% bump, suggested below, uncapped -> 100 gwei * 1.13 = 113 gwei
	require.Equal(t, gwei(113),
		computeReplacementGasPrice(gwei(100).Uint64(), gwei(50), 13, 0))

	// market rose above the bump -> the node suggestion wins
	require.Equal(t, gwei(200),
		computeReplacementGasPrice(gwei(100).Uint64(), gwei(200), 13, 0))

	// cap below the bumped price -> capped at MaxGasPrice
	require.Equal(t, gwei(105),
		computeReplacementGasPrice(gwei(100).Uint64(), gwei(50), 13, 105))

	// uncapped, larger bump -> full bump (100 gwei * 1.5)
	require.Equal(t, gwei(150),
		computeReplacementGasPrice(gwei(100).Uint64(), gwei(50), 50, 0))
}

func TestCapToMaxGwei(t *testing.T) {
	require.Equal(t, gwei(100), capToMaxGwei(gwei(100), 0))   // 0 = uncapped
	require.Equal(t, gwei(100), capToMaxGwei(gwei(100), 150)) // under the cap
	require.Equal(t, gwei(150), capToMaxGwei(gwei(200), 150)) // over the cap -> capped
}

func TestCappedDynamicFees(t *testing.T) {
	// tip under fee cap, both under the gwei cap -> unchanged
	f := cappedDynamicFees(gwei(2), gwei(40), 150)
	require.Equal(t, gwei(2), f.gasTipCap)
	require.Equal(t, gwei(40), f.gasFeeCap)
	require.Nil(t, f.gasPrice) // dynamic-fee form leaves the legacy field unset

	// fee cap over the gwei cap -> fee cap capped, tip untouched (still below it)
	f = cappedDynamicFees(gwei(2), gwei(200), 150)
	require.Equal(t, gwei(150), f.gasFeeCap)
	require.Equal(t, gwei(2), f.gasTipCap)

	// tip above the (capped) fee cap -> tip clamped down to the fee cap
	f = cappedDynamicFees(gwei(300), gwei(200), 150)
	require.Equal(t, gwei(150), f.gasFeeCap)
	require.Equal(t, gwei(150), f.gasTipCap)
}

func TestComputeReplacementDynamicFees(t *testing.T) {
	// market below the bump, uncapped: both caps get the +13% bump.
	suggested := gasFees{gasTipCap: gwei(1), gasFeeCap: gwei(20)}
	f := computeReplacementDynamicFees(gwei(100).Uint64(), gwei(2).Uint64(), suggested, 13, 0)
	require.Equal(t, gwei(113), f.gasFeeCap) // 100 * 1.13
	// 2 gwei * 1.13 = 2.26 gwei
	require.Equal(t, new(big.Int).Div(new(big.Int).Mul(gwei(2), big.NewInt(113)), big.NewInt(100)), f.gasTipCap)

	// market rose above the bump -> the current suggestion floors both legs.
	suggested = gasFees{gasTipCap: gwei(5), gasFeeCap: gwei(250)}
	f = computeReplacementDynamicFees(gwei(100).Uint64(), gwei(2).Uint64(), suggested, 13, 0)
	require.Equal(t, gwei(250), f.gasFeeCap)
	require.Equal(t, gwei(5), f.gasTipCap)

	// bumped fee cap above MaxGasPrice -> fee cap capped, and the tip clamped to it.
	suggested = gasFees{gasTipCap: gwei(1), gasFeeCap: gwei(20)}
	f = computeReplacementDynamicFees(gwei(100).Uint64(), gwei(120).Uint64(), suggested, 13, 105)
	require.Equal(t, gwei(105), f.gasFeeCap) // 100*1.13=113, capped to 105
	require.Equal(t, gwei(105), f.gasTipCap) // 120*1.13=135.6, clamped to the 105 fee cap
}
