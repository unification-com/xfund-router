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
)

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
