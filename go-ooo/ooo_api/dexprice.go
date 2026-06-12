package ooo_api

import (
	"errors"
	"fmt"
	"github.com/montanaflynn/stats"
	"go-ooo/logger"
	"go-ooo/utils"
	"math"
	"math/big"
	"time"
)

func (o *OOOApi) QueryDexPrice(parsed ParsedEndpoint, requestId string) (string, error) {
	base, target, minutes := parsed.Base, parsed.Target, parsed.Minutes

	start := time.Now()
	defer func() { priceFetchDuration.Observe(time.Since(start).Seconds()) }()

	logger.Debug("ooo_api", "QueryDexPrice", "ParseEndpoint", "endpoint parsed", logger.Fields{
		"requestId": requestId,
		"base":      base,
		"target":    target,
		"minutes":   minutes,
	})

	samples := o.dexModuleManager.GetPricesFromDexModules(base, target, uint64(minutes))

	// Stage A: reduce each pool's snapshot series to one robust estimate (median), carrying
	// its backing liquidity as the weight and its source venue (chain|dex) for the quality
	// signals. Reducing per-pool first stops a trending series being read as i.i.d. noise and
	// stops a busy multi-pool DEX out-voting a single-pool one.
	var values, weights, liquidities []float64
	var sources []string
	for _, s := range samples {
		fp := finitePrices(s.Prices)
		if len(fp) == 0 {
			continue
		}
		est, err := stats.Median(fp)
		if err != nil || est <= 0 {
			continue
		}
		values = append(values, est)
		// Weight by liquidity scaled by the trust score (XR1); keep the raw liquidity for the
		// quality signals (manipulation resistance is about real depth, not trust-adjusted depth).
		weights = append(weights, confidenceWeight(s.Liquidity, s.Confidence))
		liquidities = append(liquidities, s.Liquidity)
		sources = append(sources, s.Chain+"|"+s.Dex)
	}

	if len(values) == 0 {
		priceNoDataTotal.Inc()
		logger.WarnWithFields("ooo_api", "QueryDexPrice", "", "no prices found on DEXs for pair", logger.Fields{
			"base":   base,
			"target": target,
		})

		return "0", errors.New("no prices found on DEXs for pair")
	}

	// Stage B: robustly reject outlier pool-estimates (median + MAD), then take a
	// liquidity-and-trust-weighted mean of the survivors. MAD has a 50% breakdown point, so a
	// single manipulated/garbage reading cannot inflate the scale and mask itself - unlike the old
	// fixed mean±1σ clip. Deep, trusted pools dominate; a dust or low-trust pool is both outlier-
	// rejected and, if it survives, low-weighted (the weight is liquidity × trust score - XR1).
	median, scale := madCentreAndScale(values)
	var keptVals, keptWeights, keptLiquidities []float64
	var keptSources []string
	for i, v := range values {
		if scale == 0 || math.Abs(v-median)/scale <= madRejectThreshold {
			keptVals = append(keptVals, v)
			keptWeights = append(keptWeights, weights[i])
			keptLiquidities = append(keptLiquidities, liquidities[i])
			keptSources = append(keptSources, sources[i])
		}
	}
	if len(keptVals) == 0 {
		// The median estimate always survives the filter, so this is defensive only.
		keptVals, keptWeights, keptLiquidities, keptSources = values, weights, liquidities, sources
	}

	// S6 manipulation cross-check (opt-in): drop shallow pools that are mild outliers - a likely
	// manipulation the conservative MAD threshold lets through. Deep off-consensus pools (genuine
	// arbitrage gaps) are kept. Disabled unless both knobs are set.
	if o.quality.SuspectDeviation > 0 && o.quality.SuspectMinLiquidityUsd > 0 {
		var dropped int
		keptVals, keptWeights, keptLiquidities, keptSources, dropped = dropSuspectPools(
			keptVals, keptWeights, keptLiquidities, keptSources, median, scale,
			o.quality.SuspectDeviation, float64(o.quality.SuspectMinLiquidityUsd))
		if dropped > 0 {
			logger.WarnWithFields("ooo_api", "QueryDexPrice", "", "dropped shallow-outlier pools as likely manipulation (S6)", logger.Fields{
				"base": base, "target": target, "dropped": dropped,
			})
		}
	}

	priceFloat := weightedMean(keptVals, keptWeights)

	// Quality signals: how many independent venues and how much liquidity back this price, and
	// how dispersed the surviving estimates are (robust coefficient of variation). These gate
	// the price (refuse/flag below) and make a thin, easily-manipulated sample visible rather
	// than returning a confident-looking number from a single source.
	numPools := len(keptVals)
	numVenues, totalLiquidity := venueCountAndLiquidity(keptSources, keptLiquidities)
	dispersion := 0.0
	if median > 0 {
		dispersion = scale / median
	}

	// Refuse backstop (opt-in; every bar disabled at its zero value, all default to 0). A
	// sample too thin/shallow to price safely is declined rather than published - the request
	// then stays unfulfilled (REQUEST_STATUS_API_ERROR) instead of putting a manipulable price
	// on-chain. The existing zero-prices refusal (above) always applies regardless.
	if reason := qualityShortfall(o.quality.RefuseMinPools, o.quality.RefuseMinVenues, o.quality.RefuseMinLiquidityUsd, o.quality.RefuseMaxDispersion,
		numPools, numVenues, totalLiquidity, dispersion); reason != "" {
		priceRefusedTotal.Inc()
		logger.WarnWithFields("ooo_api", "QueryDexPrice", "", "refusing price - sample below quality floor", logger.Fields{
			"base":              base,
			"target":            target,
			"reason":            reason,
			"num_pools":         numPools,
			"num_venues":        numVenues,
			"backing_liquidity": totalLiquidity,
			"dispersion":        dispersion,
		})

		return "0", fmt.Errorf("price sample below quality floor: %s", reason)
	}

	wei := utils.EtherToWei(big.NewFloat(priceFloat))
	if wei.Cmp(big.NewInt(0)) <= 0 {
		// The estimate is positive but rounds below 1 wei (price < 1e-18 of the target); the
		// uint256 ×1e18 callback interface cannot represent it.
		return "", errors.New("calculated price rounds below one wei for pair")
	}

	logger.Debug("ooo_api", "QueryDexPrice", "", "price stats", logger.Fields{
		"base":              base,
		"target":            target,
		"minutes":           minutes,
		"num_pools_raw":     len(samples),
		"num_pools_used":    len(values),
		"num_pools_kept":    numPools,
		"num_pools_removed": len(values) - numPools,
		"num_venues":        numVenues,
		"backing_liquidity": totalLiquidity,
		"dispersion":        dispersion,
		"median":            median,
		"mad_scale":         scale,
		"final_wei_mean":    wei.String(),
	})

	// This price is being served - record its quality signals for the metrics histograms.
	observePriceQuality(numPools, numVenues, totalLiquidity, dispersion)

	// Flag (off-chain): still answers, but warns when below the soft bars so a thin sample is
	// visible to the operator without declining the request.
	if reason := qualityShortfall(o.quality.FlagMinPools, o.quality.FlagMinVenues, o.quality.FlagMinLiquidityUsd, o.quality.FlagMaxDispersion,
		numPools, numVenues, totalLiquidity, dispersion); reason != "" {
		priceFlaggedTotal.Inc()
		logger.WarnWithFields("ooo_api", "QueryDexPrice", "", "thin price sample - low manipulation resistance", logger.Fields{
			"base":              base,
			"target":            target,
			"reason":            reason,
			"num_pools":         numPools,
			"num_venues":        numVenues,
			"backing_liquidity": totalLiquidity,
		})
	}

	return wei.String(), nil
}

// venueCountAndLiquidity reports how many distinct venues (chain|dex) and how much total
// backing liquidity stand behind the surviving pool estimates.
func venueCountAndLiquidity(sources []string, weights []float64) (int, float64) {
	venues := make(map[string]struct{})
	var total float64
	for i, s := range sources {
		venues[s] = struct{}{}
		if weights[i] > 0 {
			total += weights[i]
		}
	}
	return len(venues), total
}

// qualityShortfall returns a non-empty reason when a sample falls below any enabled bar (each
// bar is disabled at its zero value). It drives both the refuse and the flag checks - the only
// difference between them is which set of bars (RefuseMin* vs FlagMin*) is passed in.
func qualityShortfall(minPools, minVenues, minLiquidityUsd uint64, maxDispersion float64, numPools, numVenues int, liquidity, dispersion float64) string {
	if minPools > 0 && uint64(numPools) < minPools {
		return fmt.Sprintf("pools %d < %d", numPools, minPools)
	}
	if minVenues > 0 && uint64(numVenues) < minVenues {
		return fmt.Sprintf("venues %d < %d", numVenues, minVenues)
	}
	if minLiquidityUsd > 0 && liquidity < float64(minLiquidityUsd) {
		return fmt.Sprintf("backing liquidity %.0f < %d", liquidity, minLiquidityUsd)
	}
	if maxDispersion > 0 && dispersion > maxDispersion {
		return fmt.Sprintf("dispersion %.4f > %.4f", dispersion, maxDispersion)
	}
	return ""
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
// outlier when |x - median| / scale > 3.5.
const madRejectThreshold = 3.5

// madScaleFactor (1.4826 = 1/0.6745) makes the median absolute deviation a consistent
// estimator of the standard deviation for normally-distributed data.
const madScaleFactor = 1.4826

// meanADScaleFactor (1.253314 = sqrt(π/2)) is the analogous consistency factor for the mean
// absolute deviation, used as a fallback when the MAD is zero (≥half the samples identical).
const meanADScaleFactor = 1.253314

// madCentreAndScale returns the median of the sample and a robust scale estimate: the median
// absolute deviation, made σ-consistent via 1.4826. When ≥half the samples equal the median
// the MAD is 0 and it falls back to the mean absolute deviation (σ-consistent via 1.253314).
// A returned scale of 0 means the sample has no usable spread (e.g. all identical), in which
// case callers should reject nothing. Inputs are assumed finite (see finitePrices) and
// non-empty; an empty input returns zeros.
func madCentreAndScale(prices []float64) (median, scale float64) {
	if len(prices) == 0 {
		return 0, 0
	}

	median, _ = stats.Median(prices) // only errors on empty input, guarded above

	deviations := make([]float64, len(prices))
	for i, p := range prices {
		deviations[i] = math.Abs(p - median)
	}

	mad, _ := stats.Median(deviations)
	scale = madScaleFactor * mad

	if scale == 0 {
		meanAD, _ := stats.Mean(deviations)
		scale = meanADScaleFactor * meanAD
	}

	return median, scale
}

// dropSuspectPools is the S6 manipulation cross-check: it drops a pool whose estimate deviates beyond
// suspectDeviation (modified z-score relative to median/scale) AND whose backing liquidity is below
// suspectMinLiquidity - a shallow pool notably off the consensus, a likely manipulation. A deep
// outlier (a real arbitrage gap) is kept. It is a no-op when scale is 0 (no usable spread) and never
// drops every pool (a degenerate all-suspect set is returned unchanged). Returns the filtered slices
// + the number dropped.
func dropSuspectPools(vals, weights, liquidities []float64, sources []string, median, scale, suspectDeviation, suspectMinLiquidity float64) ([]float64, []float64, []float64, []string, int) {
	if scale == 0 {
		return vals, weights, liquidities, sources, 0
	}

	var fVals, fWeights, fLiq []float64
	var fSources []string
	dropped := 0
	for i, v := range vals {
		deviation := math.Abs(v-median) / scale
		if deviation > suspectDeviation && liquidities[i] < suspectMinLiquidity {
			dropped++
			continue
		}
		fVals = append(fVals, v)
		fWeights = append(fWeights, weights[i])
		fLiq = append(fLiq, liquidities[i])
		fSources = append(fSources, sources[i])
	}

	if dropped == 0 || len(fVals) == 0 {
		// Nothing suspect, or everything is - keep the original set rather than wipe it.
		return vals, weights, liquidities, sources, 0
	}
	return fVals, fWeights, fLiq, fSources, dropped
}

// confidenceWeight blends a pool's backing liquidity with its dex-pair-verify trust score (XR1):
// weight = liquidity × confidence, so a lower-trust pool counts proportionally less in the weighted
// mean (a ManualVerified pool scores 1 and keeps full weight). A confidence of 0 means unscored
// (legacy rows, or a source predating the trust export) and any out-of-range value is treated as
// neutral (factor 1) - the pool keeps full liquidity weight rather than being silently dropped.
func confidenceWeight(liquidity, confidence float64) float64 {
	factor := confidence
	if factor <= 0 || factor > 1 {
		factor = 1
	}
	return liquidity * factor
}

// weightedMean returns sum(v·w)/sum(w). If the total weight is zero (no liquidity data for
// any survivor) it falls back to a plain arithmetic mean so a price is still produced.
func weightedMean(values, weights []float64) float64 {
	var sumVW, sumW float64
	for i, v := range values {
		w := weights[i]
		if w < 0 {
			w = 0
		}
		sumVW += v * w
		sumW += w
	}

	if sumW > 0 {
		return sumVW / sumW
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
