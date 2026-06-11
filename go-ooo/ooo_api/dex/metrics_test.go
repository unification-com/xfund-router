package dex

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestDexQueryTotalIncrements(t *testing.T) {
	for _, result := range []string{"success", "error", "no_data"} {
		c := dexQueryTotal.WithLabelValues("eth", "uniswap_v2", result)
		before := testutil.ToFloat64(c)
		c.Inc()
		if got := testutil.ToFloat64(c); got != before+1 {
			t.Errorf("ooo_dex_query_total{result=%q} = %v, want %v", result, got, before+1)
		}
	}
}
