package ooo_api

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPriceCountersIncrement(t *testing.T) {
	cases := []struct {
		name string
		c    prometheus.Counter
	}{
		{"no_data", priceNoDataTotal},
		{"flagged", priceFlaggedTotal},
		{"refused", priceRefusedTotal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := testutil.ToFloat64(c.c)
			c.c.Inc()
			if got := testutil.ToFloat64(c.c); got != before+1 {
				t.Errorf("%s = %v, want %v", c.name, got, before+1)
			}
		})
	}
}

func TestObservePriceQualityRegistersHistograms(t *testing.T) {
	// Should not panic, and each quality/duration histogram must be registered and collectable.
	observePriceQuality(5, 3, 1e8, 0.001)
	for name, h := range map[string]prometheus.Collector{
		"pricePools":               pricePools,
		"priceVenues":              priceVenues,
		"priceBackingLiquidityUsd": priceBackingLiquidityUsd,
		"priceDispersion":          priceDispersion,
		"priceFetchDuration":       priceFetchDuration,
	} {
		if n := testutil.CollectAndCount(h); n != 1 {
			t.Errorf("%s: expected 1 collected metric, got %d", name, n)
		}
	}
}
