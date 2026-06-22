package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// routeWorkerIndex is the admin-task routing decision: an unspecified target (0) goes to the sole
// worker, a set target matches by network id, and anything ambiguous or unmatched is -1 (the caller
// turns that into an "ask for --chain" error).
func TestRouteWorkerIndex(t *testing.T) {
	tests := []struct {
		name       string
		networkIds []int64
		target     int64
		want       int
	}{
		{"unspecified routes to the sole chain", []int64{137}, 0, 0},
		{"unspecified is ambiguous with several chains", []int64{11155111, 137}, 0, -1},
		{"unspecified with no chains is -1", nil, 0, -1},
		{"match by network id", []int64{11155111, 137}, 137, 1},
		{"match the first chain by id", []int64{11155111, 137}, 11155111, 0},
		{"no chain matches the target", []int64{11155111, 137}, 1, -1},
		{"set target with no chains is -1", nil, 137, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, routeWorkerIndex(tt.networkIds, tt.target))
		})
	}
}
