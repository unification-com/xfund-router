package utils_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go-ooo/utils"
)

func TestEtherToWei(t *testing.T) {
	require.Equal(t, "1500000000000000000", utils.EtherToWei(big.NewFloat(1.5)).String())
	require.Equal(t, "2000000000000000000", utils.EtherToWei(big.NewFloat(2)).String())
	require.Equal(t, "0", utils.EtherToWei(big.NewFloat(0)).String())

	// A non-finite value must return zero, not panic.
	require.NotPanics(t, func() {
		require.Equal(t, "0", utils.EtherToWei(new(big.Float).SetInf(false)).String())
	})
}

func TestRemoveHexPrefix(t *testing.T) {
	r := utils.RemoveHexPrefix("0x1234")
	require.Equal(t, "1234", r)
}

func TestHasHexPrefix(t *testing.T) {
	has := utils.HasHexPrefix("0x1234")
	notHas := utils.HasHexPrefix("1234")

	require.True(t, has)
	require.False(t, notHas)
}

func TestAddHexPrefix(t *testing.T) {
	prf := utils.AddHexPrefix("1234")
	require.Equal(t, "0x1234", prf)
}
