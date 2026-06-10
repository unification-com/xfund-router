package ooo_api

import (
	"errors"
	"github.com/montanaflynn/stats"
	"go-ooo/logger"
	"go-ooo/utils"
	"math"
	"math/big"
)

func (o *OOOApi) QueryAdhoc(parsed ParsedEndpoint, requestId string) (string, error) {
	base, target, minutes := parsed.Base, parsed.Target, parsed.Minutes

	logger.Debug("ooo_api", "QueryAdhoc", "ParseEndpoint", "AdHoc endpoint parsed", logger.Fields{
		"requestId": requestId,
		"base":      base,
		"target":    target,
		"minutes":   minutes,
	})

	rawPrices := finitePrices(o.dexModuleManager.GetPricesFromDexModules(base, target, uint64(minutes)))

	if len(rawPrices) == 0 {
		logger.WarnWithFields("ooo_api", "QueryAdhoc", "", "no prices found on DEXs for pair", logger.Fields{
			"base":   base,
			"target": target,
		})

		return "0", errors.New("no prices found on DEXs for pair")
	}

	// Robustly reject outliers (median + MAD) then take the mean of the survivors. MAD has a
	// 50% breakdown point, so a single manipulated or garbage reading cannot inflate the scale
	// and mask itself - unlike the previous fixed mean±1σ clip, which both discarded ~32% of a
	// clean sample and let one extreme swamp the test. Liquidity-weighting of the survivors is
	// a later step (needs the per-pool restructure); S1 here is the unweighted mean-of-survivors.
	kept, median, scale := filterOutliersMAD(rawPrices)
	if len(kept) == 0 {
		// The median sample always survives the filter, so this is defensive only.
		kept = rawPrices
	}

	meanPrice, err := stats.Mean(kept)
	if err != nil {
		return "0", errors.New("cannot calculate mean price for pair")
	}

	wei := utils.EtherToWei(big.NewFloat(meanPrice))
	if wei.Cmp(big.NewInt(0)) <= 0 {
		// The mean is positive but rounds below 1 wei (price < 1e-18 of the target); the
		// uint256 ×1e18 callback interface cannot represent it.
		return "", errors.New("calculated price rounds below one wei for pair")
	}

	logger.Debug("ooo_api", "QueryAdhoc", "", "price stats", logger.Fields{
		"base":               base,
		"target":             target,
		"minutes":            minutes,
		"num_prices_raw":     len(rawPrices),
		"num_prices_kept":    len(kept),
		"num_prices_removed": len(rawPrices) - len(kept),
		"median":             median,
		"mad_scale":          scale,
		"final_wei_mean":     wei.String(),
	})

	return wei.String(), nil
}

// finitePrices drops NaN / ±Inf values. strconv.ParseFloat accepts "NaN"/"Inf", so a
// malformed subgraph reply can otherwise reach big.NewFloat (panics on NaN) / EtherToWei
// (nil-panics on Inf) on the fulfilment path and crash an un-recovered goroutine.
func finitePrices(prices []float64) []float64 {
	out := make([]float64, 0, len(prices))
	for _, p := range prices {
		if !math.IsNaN(p) && !math.IsInf(p, 0) {
			out = append(out, p)
		}
	}
	return out
}

// madRejectThreshold is the modified z-score cut-off (Iglewicz & Hoaglin): a sample is an
// outlier when |x - median| / (1.4826·MAD) > 3.5.
const madRejectThreshold = 3.5

// madScaleFactor (1.4826 = 1/0.6745) makes the median absolute deviation a consistent
// estimator of the standard deviation for normally-distributed data.
const madScaleFactor = 1.4826

// meanADScaleFactor (1.253314 = sqrt(π/2)) is the analogous consistency factor for the mean
// absolute deviation, used as a fallback when the MAD is zero (≥half the samples identical).
const meanADScaleFactor = 1.253314

// filterOutliersMAD removes outliers using a median + Median-Absolute-Deviation modified
// z-score. Unlike a fixed mean±σ clip, MAD has a 50% breakdown point, so a single (or up to
// half) manipulated/garbage reading cannot inflate the scale and mask itself. It returns the
// surviving prices plus the median and the scale used (for telemetry). When the sample has no
// usable spread (≥half identical → MAD 0, and the mean-absolute-deviation fallback also 0) it
// rejects nothing. Inputs are assumed finite (see finitePrices) and non-empty.
func filterOutliersMAD(prices []float64) (kept []float64, median, scale float64) {
	if len(prices) == 0 {
		return nil, 0, 0
	}

	median, _ = stats.Median(prices) // only errors on empty input, guarded above

	deviations := make([]float64, len(prices))
	for i, p := range prices {
		deviations[i] = math.Abs(p - median)
	}

	mad, _ := stats.Median(deviations)
	scale = madScaleFactor * mad

	if scale == 0 {
		// ≥half the samples equal the median → MAD is 0. Fall back to the mean absolute
		// deviation so a lone outlier among many identical readings is still caught.
		meanAD, _ := stats.Mean(deviations)
		scale = meanADScaleFactor * meanAD
	}

	if scale == 0 {
		// All samples identical - nothing to reject.
		return append([]float64(nil), prices...), median, 0
	}

	for _, p := range prices {
		if math.Abs(p-median)/scale <= madRejectThreshold {
			kept = append(kept, p)
		}
	}

	return kept, median, scale
}
