package ooo_api

import (
	"math"
	"testing"
)

// keptByMAD reproduces QueryDexPrice's Stage-B filter so the estimator math can be asserted
// directly: a value survives when there is no usable spread (scale 0) or its modified z-score
// is within the threshold.
func keptByMAD(values []float64) []float64 {
	median, scale := madCentreAndScale(values)
	var kept []float64
	for _, v := range values {
		if scale == 0 || math.Abs(v-median)/scale <= madRejectThreshold {
			kept = append(kept, v)
		}
	}
	return kept
}

func contains(xs []float64, want float64) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func TestMAD_KeepsCleanCluster(t *testing.T) {
	prices := []float64{1635.1, 1636.8, 1637.0, 1638.2, 1636.0, 1639.1}
	if kept := keptByMAD(prices); len(kept) != len(prices) {
		t.Errorf("clean cluster should keep all %d, kept %d (%v)", len(prices), len(kept), kept)
	}
}

func TestMAD_RejectsSingleManipulatedExtreme(t *testing.T) {
	prices := []float64{1636.0, 1637.0, 1636.5, 1638.0, 1637.5, 9000.0}
	kept := keptByMAD(prices)
	if contains(kept, 9000.0) {
		t.Fatalf("the 9000 outlier should have been rejected, kept: %v", kept)
	}
	if len(kept) != 5 {
		t.Errorf("expected 5 survivors, got %d (%v)", len(kept), kept)
	}
}

func TestMAD_DoesNotFalselyRejectBimodal(t *testing.T) {
	// Genuine dispersion (two clusters) is not outliers - the wide MAD must keep both.
	prices := []float64{100, 101, 102, 200, 201, 202}
	if kept := keptByMAD(prices); len(kept) != len(prices) {
		t.Errorf("bimodal spread should not be rejected, kept %d of %d (%v)", len(kept), len(prices), kept)
	}
}

func TestMAD_SmallSamples(t *testing.T) {
	if kept := keptByMAD([]float64{1637.0}); len(kept) != 1 {
		t.Errorf("N=1 should keep the single value, got %v", kept)
	}
	if kept := keptByMAD([]float64{100, 5000}); len(kept) != 2 {
		t.Errorf("N=2 cannot reject either point, got %v", kept)
	}
}

func TestMAD_AllIdenticalHasZeroScale(t *testing.T) {
	median, scale := madCentreAndScale([]float64{5, 5, 5, 5})
	if median != 5 || scale != 0 {
		t.Errorf("all-identical: median=%v scale=%v, want 5/0", median, scale)
	}
	if kept := keptByMAD([]float64{5, 5, 5, 5}); len(kept) != 4 {
		t.Errorf("all-identical should keep all, got %v", kept)
	}
}

func TestMAD_MeanADFallbackCatchesOutlier(t *testing.T) {
	// ≥half identical → MAD is 0; the mean-absolute-deviation fallback must still produce a
	// non-zero scale so the lone extreme is caught.
	_, scale := madCentreAndScale([]float64{1, 1, 1, 1, 5000})
	if scale == 0 {
		t.Fatal("expected the mean-AD fallback to produce a non-zero scale")
	}
	kept := keptByMAD([]float64{1, 1, 1, 1, 5000})
	if contains(kept, 5000) || len(kept) != 4 {
		t.Errorf("expected the four 1s to survive and 5000 rejected, got %v", kept)
	}
}

func TestMAD_Empty(t *testing.T) {
	median, scale := madCentreAndScale(nil)
	if median != 0 || scale != 0 {
		t.Errorf("empty input should return zeros, got median=%v scale=%v", median, scale)
	}
}

func TestWeightedMean_LiquidityWeighting(t *testing.T) {
	// A deep pool at 1640 with 10x the liquidity of a shallow pool at 1600 should pull the
	// result close to 1640, not the midpoint.
	got := weightedMean([]float64{1640, 1600}, []float64{1_000_000, 100_000})
	want := (1640*1_000_000.0 + 1600*100_000.0) / 1_100_000.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("weighted mean = %v, want %v", got, want)
	}
	if got < 1635 {
		t.Errorf("deep pool should dominate, got %v", got)
	}
}

func TestWeightedMean_ZeroWeightsFallBackToPlainMean(t *testing.T) {
	got := weightedMean([]float64{10, 20, 30}, []float64{0, 0, 0})
	if math.Abs(got-20) > 1e-9 {
		t.Errorf("zero weights should fall back to plain mean 20, got %v", got)
	}
}

func TestWeightedMean_NegativeWeightClamped(t *testing.T) {
	// A negative weight must not subtract from the estimate; it is clamped to 0.
	got := weightedMean([]float64{10, 1000}, []float64{5, -3})
	if math.Abs(got-10) > 1e-9 {
		t.Errorf("negative weight should be clamped to 0 (→ 10), got %v", got)
	}
}

func TestQualityShortfall(t *testing.T) {
	// Healthy sample: 5 pools, 3 venues, $1M backing, 0.1% dispersion.
	const (
		nPools = 5
		nVen   = 3
		liq    = 1_000_000.0
		disp   = 0.001
	)

	// All bars disabled (the default) → never a shortfall, however thin the sample.
	if r := qualityShortfall(0, 0, 0, 0, 1, 1, 0, 0.99); r != "" {
		t.Errorf("all-disabled bars should never report a shortfall, got %q", r)
	}

	// Healthy sample passes sensible flag bars.
	if r := qualityShortfall(2, 2, 100_000, 0.02, nPools, nVen, liq, disp); r != "" {
		t.Errorf("healthy sample should pass, got shortfall %q", r)
	}

	cases := []struct {
		name                        string
		minPools, minVenues, minLiq uint64
		maxDisp                     float64
		nPools, nVenues             int
		liq, disp                   float64
		wantHit                     bool
	}{
		{"too few pools", 2, 0, 0, 0, 1, 1, 5_000, 0.5, true},
		{"single venue", 0, 2, 0, 0, 3, 1, 5e6, 0.001, true},
		{"thin liquidity", 0, 0, 25_000, 0, 4, 2, 10_000, 0.001, true},
		{"wild dispersion", 0, 0, 0, 0.2, 4, 2, 5e6, 0.35, true},
		{"liquidity exactly at floor passes", 0, 0, 25_000, 0, 4, 2, 25_000, 0.001, false},
		{"dispersion exactly at bound passes", 0, 0, 0, 0.2, 4, 2, 5e6, 0.2, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := qualityShortfall(c.minPools, c.minVenues, c.minLiq, c.maxDisp, c.nPools, c.nVenues, c.liq, c.disp)
			if (r != "") != c.wantHit {
				t.Errorf("shortfall=%q, wantHit=%v", r, c.wantHit)
			}
		})
	}
}
