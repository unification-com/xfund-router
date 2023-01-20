package utils_test

import (
	"github.com/stretchr/testify/require"
	"go-ooo/utils"
	"testing"
)

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
