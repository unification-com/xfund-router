package chain

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go-ooo/database"
	"go-ooo/database/models"
	"go-ooo/logger"
)

// Fulfilment-lifecycle observability (#125). Registered on the default registry at init; the
// service exposes them at /metrics with the rest.
var (
	fulfilmentSentTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ooo_fulfilment_sent_total",
		Help: "Fulfilment transactions broadcast to the chain (initial sends and stuck-tx replacements).",
	})

	fulfilmentResultTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ooo_fulfilment_result_total",
		Help: "Confirmed fulfilment outcomes, by result (success|reverted).",
	}, []string{"result"})

	fulfilmentErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ooo_fulfilment_error_total",
		Help: "Fulfilment failures before broadcast, by stage (build_opts|sign|send).",
	}, []string{"stage"})

	fulfilmentGasPriceGwei = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ooo_fulfilment_gas_price_gwei",
		Help:    "Gas price (gwei) of each broadcast fulfilment tx.",
		Buckets: []float64{1, 5, 10, 20, 30, 50, 75, 100, 150, 250},
	})

	fulfilmentGasUsed = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ooo_fulfilment_gas_used",
		Help:    "Gas used by each confirmed fulfilment tx (success or revert).",
		Buckets: []float64{50000, 75000, 100000, 150000, 200000, 300000, 500000},
	})

	fulfilmentBlocks = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ooo_fulfilment_blocks",
		Help:    "Blocks elapsed from request to fulfilment confirmation.",
		Buckets: []float64{1, 2, 3, 5, 8, 13, 21, 50, 100},
	})
)

// weiToGwei converts a wei gas price to gwei for the gas-price histogram.
func weiToGwei(wei uint64) float64 {
	return float64(wei) / 1e9
}

// WarmStartFulfilmentMetrics seeds the cumulative counters from the DB's historical totals at
// startup - call it BEFORE /metrics is served so the all-time counts (and the forward rates
// derived from them) are correct from the first scrape rather than starting at zero. Counters
// only: the gas/latency histograms stay forward-only, since re-observing all history would both
// distort their live distribution and timestamp every sample at startup. Genuine historical
// analysis lives in the DB / reporting (#126). The sent count is approximate - it counts requests
// that were broadcast at least once, so it slightly under-counts re-broadcast retries (rare).
func WarmStartFulfilmentMetrics(db *database.DB) {
	sent, errSent := db.CountFulfilmentsSent()
	if errSent == nil {
		fulfilmentSentTotal.Add(float64(sent))
	}
	success, errSuccess := db.CountRequestsByStatus(models.REQUEST_STATUS_SUCCESS)
	if errSuccess == nil {
		fulfilmentResultTotal.WithLabelValues("success").Add(float64(success))
	}
	reverted, errReverted := db.CountFailedFulfilments()
	if errReverted == nil {
		fulfilmentResultTotal.WithLabelValues("reverted").Add(float64(reverted))
	}

	logger.InfoWithFields("chain", "WarmStartFulfilmentMetrics", "", "seeded fulfilment counters from DB history", logger.Fields{
		"sent":     sent,
		"success":  success,
		"reverted": reverted,
	})
}
