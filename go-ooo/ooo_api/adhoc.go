package ooo_api

import (
	"errors"
	"github.com/montanaflynn/stats"
	"go-ooo/logger"
	"go-ooo/utils"
	"math"
	"math/big"
	"strconv"
)

func (o *OOOApi) QueryAdhoc(endpoint string, requestId string) (string, error) {
	base, target, _, mins, _, _, _, err := ParseEndpoint(endpoint)

	minutes, _ := strconv.ParseInt(mins, 10, 64)

	if minutes < 0 {
		minutes = 0
	}

	// no more than 60 minutes
	if minutes > 60 {
		minutes = 60
	}

	if err != nil {
		return "", err
	}

	logger.Debug("ooo_api", "QueryAdhoc", "ParseEndpoint", "AdHoc endpoint parsed", logger.Fields{
		"requestId": requestId,
		"endpoint":  endpoint,
		"base":      base,
		"target":    target,
		"minutes":   minutes,
	})

	priceCount := 0
	total := big.NewInt(0)

	rawPrices := o.dexModuleManager.GetPricesFromDexModules(base, target, uint64(minutes))

	if len(rawPrices) == 0 {
		logger.WarnWithFields("ooo_api", "QueryAdhoc", "", "no prices found on DEXs for pair", logger.Fields{
			"base":   base,
			"target": target,
		})

		return "0", errors.New("no prices found on DEXs for pair")
	}

	dMax := float64(1)

	outliersRemoved, mean, stdDev, chauvenetUsed := removeOutliersFromData(rawPrices, dMax)

	if len(outliersRemoved) == 0 {
		for len(outliersRemoved) == 0 {
			dMax += 1
			if dMax >= 4 {
				break
			}
			outliersRemoved, mean, stdDev, chauvenetUsed = removeOutliersFromData(rawPrices, dMax)
		}
	}

	if len(outliersRemoved) == 0 {
		outliersRemoved = rawPrices
	}

	// calculate mean from data set with outliers removed
	for _, oR := range outliersRemoved {
		p := big.NewFloat(oR)
		wei := utils.EtherToWei(p)
		if wei.Cmp(big.NewInt(0)) > 0 {
			total = new(big.Int).Add(total, wei)
			priceCount++
		}
	}

	if total.Cmp(big.NewInt(0)) <= 0 {
		return "", errors.New("cannot calculate mean, price is zero")
	}

	meanPrice := new(big.Int).Div(total, big.NewInt(int64(priceCount)))

	logger.Debug("ooo_api", "QueryAdhoc", "", "price stats", logger.Fields{
		"base":               base,
		"target":             target,
		"minutes":            minutes,
		"num_prices_raw":     len(rawPrices),
		"num_prices_chauv":   len(outliersRemoved),
		"num_prices_removed": len(rawPrices) - len(outliersRemoved),
		"raw_prices_mean":    mean,
		"raw_std_dev":        stdDev,
		"final_wei_mean":     meanPrice.String(),
		"chauvenet_used":     chauvenetUsed,
		"d_max":              dMax,
	})

	return meanPrice.String(), nil
}

func removeOutliersFromData(rawPrices []float64, dMax float64) ([]float64, float64, float64, bool) {
	var outliersRemoved []float64

	mean, err := stats.Mean(rawPrices)

	if err != nil {
		return rawPrices, 0, 0, false
	}

	stdDev, err := stats.StandardDeviation(rawPrices)

	if err != nil {
		return rawPrices, 0, 0, false
	}

	chauvenetUsed := false

	// remove outliers with Chauvenet Criterion, but only if stdDev > 0
	// as some pair prices are too small to calculate stdDev
	for _, p := range rawPrices {
		if stdDev > 0 {
			chauvenetUsed = true
			d := math.Abs(p-mean) / stdDev
			if dMax > d {
				outliersRemoved = append(outliersRemoved, p)
			}
		} else {
			// prices are too small to use Chauvenet Criterion
			outliersRemoved = append(outliersRemoved, p)
		}
	}

	return outliersRemoved, mean, stdDev, chauvenetUsed
}
