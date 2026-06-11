package ooo_api

import (
	"go-ooo/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// These counters register on the default Prometheus registry (the same one service exposes at
// /metrics) at package init, so they need no wiring beyond importing ooo_api.
var (
	// legacyEndpointTotal counts routed requests that used a deprecated explicit-qualifier
	// endpoint form (base.target.AD / base.target.PR...). The going-forward form is the
	// suffix-less base.target[.minutes]. This is the signal that gates removal of the legacy
	// parsing: once it stays at zero across a trailing window, the legacy forms can be dropped
	// (queries are one-shot, so no replay can resurrect an old form). Labelled by qualifier.
	legacyEndpointTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ooo_legacy_endpoint_total",
		Help: "Routed requests using a deprecated explicit-qualifier endpoint form, by qualifier (AD/PR).",
	}, []string{"qualifier"})

	// droppedEndpointFieldTotal counts AdHoc requests that carried trailing fields the AdHoc
	// path cannot honour (e.g. leftover Finchains subtype/exchange/window params). Per the
	// silent-drop policy the request is still served from the DEX mean; this surfaces who is
	// still sending the redundant params.
	droppedEndpointFieldTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ooo_dropped_endpoint_field_total",
		Help: "AdHoc requests carrying trailing fields the AdHoc path ignored.",
	})

	// Price-path observability (#125). One served price = one observation across the histograms
	// below; the counters track the outcomes the histograms can't (no data / flagged / refused).

	priceFetchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ooo_price_fetch_seconds",
		Help:    "Wall-clock time to fetch and aggregate a DEX price for a request.",
		Buckets: prometheus.DefBuckets,
	})

	pricePools = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ooo_price_pools",
		Help:    "Surviving pool count behind each served price.",
		Buckets: []float64{1, 2, 3, 5, 8, 13, 21},
	})

	priceVenues = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ooo_price_venues",
		Help:    "Distinct venues (chain|dex) behind each served price.",
		Buckets: []float64{1, 2, 3, 4, 5, 7, 10},
	})

	priceBackingLiquidityUsd = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ooo_price_backing_liquidity_usd",
		Help:    "Total backing liquidity (USD) behind each served price.",
		Buckets: []float64{1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9},
	})

	priceDispersion = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "ooo_price_dispersion",
		Help:    "Robust dispersion (MAD scale / median) of each served price's surviving pool estimates.",
		Buckets: []float64{0.0005, 0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.25},
	})

	priceNoDataTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ooo_price_no_data_total",
		Help: "Price queries that found no usable DEX prices for the pair.",
	})

	priceFlaggedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ooo_price_flagged_total",
		Help: "Served prices that tripped a quality flag bar (thin sample; still fulfilled).",
	})

	priceRefusedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ooo_price_refused_total",
		Help: "Prices refused by the quality gate (request left unfulfilled).",
	})
)

// observePriceQuality records the quality signals behind a single served price.
func observePriceQuality(numPools, numVenues int, liquidityUsd, dispersion float64) {
	pricePools.Observe(float64(numPools))
	priceVenues.Observe(float64(numVenues))
	priceBackingLiquidityUsd.Observe(liquidityUsd)
	priceDispersion.Observe(dispersion)
}

// observeEndpoint records deprecation + silent-drop telemetry for a routed endpoint. It is
// called once per request from RouteQuery so the legacy counter reflects distinct served
// requests rather than the multiple internal ParseEndpoint calls.
func observeEndpoint(parsed ParsedEndpoint) {
	if parsed.Legacy {
		legacyEndpointTotal.WithLabelValues(parsed.QType).Inc()
		logger.InfoWithFields("ooo_api", "observeEndpoint", "deprecated endpoint form",
			"request used a deprecated explicit-qualifier endpoint; the going-forward form is base.target[.minutes]",
			logger.Fields{
				"qualifier": parsed.QType,
				"base":      parsed.Base,
				"target":    parsed.Target,
			})
	}

	if parsed.IsAdHoc() && len(parsed.IgnoredFields) > 0 {
		droppedEndpointFieldTotal.Inc()
		logger.WarnWithFields("ooo_api", "observeEndpoint", "ignored endpoint fields",
			"AdHoc query carried fields it cannot honour; they were ignored and the price served from the DEX mean",
			logger.Fields{
				"base":           parsed.Base,
				"target":         parsed.Target,
				"ignored_fields": parsed.IgnoredFields,
			})
	}
}
