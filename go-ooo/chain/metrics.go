package chain

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"go-ooo/database"
	"go-ooo/database/models"
	"go-ooo/logger"
)

// Fulfilment-lifecycle observability (#125). Registered on the default registry at init; the
// service exposes them at /metrics with the rest. Every series carries a `chain` label (the network
// id) so one process running several networks partitions cleanly and a single Grafana board covers the
// whole fleet.
var (
	fulfilmentSentTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ooo_fulfilment_sent_total",
		Help: "Fulfilment transactions broadcast to the chain (initial sends and stuck-tx replacements).",
	}, []string{"chain"})

	fulfilmentResultTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ooo_fulfilment_result_total",
		Help: "Confirmed fulfilment outcomes, by result (success|reverted).",
	}, []string{"chain", "result"})

	fulfilmentErrorTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ooo_fulfilment_error_total",
		Help: "Fulfilment failures before broadcast, by stage (build_opts|sign|send).",
	}, []string{"chain", "stage"})

	fulfilmentGasPriceGwei = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ooo_fulfilment_gas_price_gwei",
		Help:    "Gas price (gwei) of each broadcast fulfilment tx.",
		Buckets: []float64{1, 5, 10, 20, 30, 50, 75, 100, 150, 250},
	}, []string{"chain"})

	fulfilmentGasUsed = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ooo_fulfilment_gas_used",
		Help:    "Gas used by each confirmed fulfilment tx (success or revert).",
		Buckets: []float64{50000, 75000, 100000, 150000, 200000, 300000, 500000},
	}, []string{"chain"})

	fulfilmentBlocks = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ooo_fulfilment_blocks",
		Help:    "Blocks elapsed from request to fulfilment confirmation.",
		Buckets: []float64{1, 2, 3, 5, 8, 13, 21, 50, 100},
	}, []string{"chain"})

	jobQueueRunTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ooo_job_queue_run_total",
		Help: "Pending-job-queue processing runs, by trigger: 'event' is the immediate nudge on a newly-detected request, 'ticker' the periodic sweep.",
	}, []string{"chain", "trigger"})
)

// weiToGwei converts a wei gas price to gwei for the gas-price histogram.
func weiToGwei(wei uint64) float64 {
	return float64(wei) / 1e9
}

// metricChainLabel renders a network id as the `chain` Prometheus label value.
func metricChainLabel(networkId int64) string {
	return strconv.FormatInt(networkId, 10)
}

// chainLabel is this worker's own `chain` metric label value (its network id).
func (o *OoORouterService) chainLabel() string {
	return metricChainLabel(o.networkId)
}

// WarmStartFulfilmentMetrics seeds the cumulative counters from the DB's historical totals at
// startup - call it BEFORE /metrics is served so the all-time counts (and the forward rates
// derived from them) are correct from the first scrape rather than starting at zero. It seeds each
// configured chain's labelled series from THAT chain's rows, so the per-chain counters resume at their
// own history rather than the fleet total. Counters only: the gas/latency histograms stay forward-only,
// since re-observing all history would both distort their live distribution and timestamp every sample
// at startup. Genuine historical analysis lives in the DB / reporting (#126). The sent count is
// approximate - it counts requests that were broadcast at least once, so it slightly under-counts
// re-broadcast retries (rare).
func WarmStartFulfilmentMetrics(db *database.DB, chainIds []int64) {
	for _, chainId := range chainIds {
		label := metricChainLabel(chainId)

		sent, errSent := db.CountFulfilmentsSent(chainId)
		if errSent == nil {
			fulfilmentSentTotal.WithLabelValues(label).Add(float64(sent))
		}
		success, errSuccess := db.CountRequestsByStatus(chainId, models.REQUEST_STATUS_SUCCESS)
		if errSuccess == nil {
			fulfilmentResultTotal.WithLabelValues(label, "success").Add(float64(success))
		}
		reverted, errReverted := db.CountFailedFulfilments(chainId)
		if errReverted == nil {
			fulfilmentResultTotal.WithLabelValues(label, "reverted").Add(float64(reverted))
		}

		logger.InfoWithFields("chain", "WarmStartFulfilmentMetrics", "", "seeded fulfilment counters from DB history", logger.Fields{
			"chain_id": chainId,
			"sent":     sent,
			"success":  success,
			"reverted": reverted,
		})
	}
}
