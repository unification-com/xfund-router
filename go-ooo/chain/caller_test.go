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
