package ooo_api

import (
	"math"
	"sort"
	"testing"

	"github.com/montanaflynn/stats"
)

func sortedCopy(in []float64) []float64 {
	out := append([]float64(nil), in...)
	sort.Float64s(out)
	return out
}

func TestFilterOutliersMAD_KeepsCleanCluster(t *testing.T) {
	prices := []float64{1635.1, 1636.8, 1637.0, 1638.2, 1636.0, 1639.1}
	kept, _, _ := filterOutliersMAD(prices)
	if len(kept) != len(prices) {
		t.Errorf("clean cluster should keep all %d, kept %d (%v)", len(prices), len(kept), kept)
	}
}

func TestFilterOutliersMAD_RejectsSingleManipulatedExtreme(t *testing.T) {
	// One flash-loan-style extreme among a tight cluster. Median+MAD must reject it; a plain
	// mean would be dragged well above the cluster.
	prices := []float64{1636.0, 1637.0, 1636.5, 1638.0, 1637.5, 9000.0}

	kept, _, _ := filterOutliersMAD(prices)
	for _, p := range kept {
		if p == 9000.0 {
			t.Fatalf("the 9000 outlier should have been rejected, kept: %v", kept)
		}
	}
	if len(kept) != 5 {
		t.Errorf("expected 5 survivors, got %d (%v)", len(kept), kept)
	}

	// Demonstrate the win: the plain mean is dragged ~25%% high; the survivor mean is not.
	rawMean, _ := stats.Mean(prices)
	keptMean, _ := stats.Mean(kept)
	if rawMean < 2700 {
		t.Errorf("sanity: the raw mean should be dragged high, got %v", rawMean)
	}
	if math.Abs(keptMean-1637.0) > 2 {
		t.Errorf("survivor mean should sit on the cluster ~1637, got %v", keptMean)
	}
}

func TestFilterOutliersMAD_DoesNotFalselyRejectBimodal(t *testing.T) {
	// A genuinely spread (bimodal) sample is legitimate dispersion, not outliers - the wide
	// MAD must keep both clusters rather than discarding the smaller one.
	prices := []float64{100, 101, 102, 200, 201, 202}
	kept, _, _ := filterOutliersMAD(prices)
	if len(kept) != len(prices) {
		t.Errorf("bimodal spread should not be rejected, kept %d of %d (%v)", len(kept), len(prices), kept)
	}
}

func TestFilterOutliersMAD_SmallSamples(t *testing.T) {
	// N=1 keeps the single value; N=2 cannot identify which point is the outlier, so both stay.
	if kept, _, _ := filterOutliersMAD([]float64{1637.0}); len(kept) != 1 {
		t.Errorf("N=1 should keep the single value, got %v", kept)
	}
	if kept, _, _ := filterOutliersMAD([]float64{100, 5000}); len(kept) != 2 {
		t.Errorf("N=2 cannot reject either point, got %v", kept)
	}
}

func TestFilterOutliersMAD_AllIdentical(t *testing.T) {
	prices := []float64{5, 5, 5, 5}
	kept, median, scale := filterOutliersMAD(prices)
	if len(kept) != 4 || median != 5 || scale != 0 {
		t.Errorf("all-identical should keep all with scale 0, got kept=%v median=%v scale=%v", kept, median, scale)
	}
}

func TestFilterOutliersMAD_MeanADFallbackCatchesOutlier(t *testing.T) {
	// ≥half identical → MAD is 0; the mean-absolute-deviation fallback must still catch the
	// lone extreme rather than silently keeping it.
	prices := []float64{1, 1, 1, 1, 5000}
	kept, _, scale := filterOutliersMAD(prices)
	if scale == 0 {
		t.Fatal("expected the mean-AD fallback to produce a non-zero scale")
	}
	if want := sortedCopy([]float64{1, 1, 1, 1}); !floatsEqual(sortedCopy(kept), want) {
		t.Errorf("expected the four 1s to survive and 5000 rejected, got %v", kept)
	}
}

func TestFilterOutliersMAD_Empty(t *testing.T) {
	kept, median, scale := filterOutliersMAD(nil)
	if len(kept) != 0 || median != 0 || scale != 0 {
		t.Errorf("empty input should return zero values, got kept=%v median=%v scale=%v", kept, median, scale)
	}
}

func floatsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
