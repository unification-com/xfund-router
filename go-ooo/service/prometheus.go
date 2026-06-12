package service

// add:
// _ "net/http/pprof"
// to import statement for profiling
// curl http://localhost:9000/debug/pprof/heap > heap.out
// curl http://localhost:9000/debug/pprof/goroutine > goroutine.out
// go tool pprof -http=:8080 heap.out
// go tool pprof heap.out

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go-ooo/logger"
	"net/http"
)

func (s *Service) initPrometheus() {

	// just a simple up gauge that can be used in, for example, Grafana
	// for monitoring up status
	srvUp := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "up",
		Help: "Service is up",
	})

	srvUp.Set(1)

	// Serve metrics on a DEDICATED mux + server, not the global http.DefaultServeMux,
	// so nothing else can be co-mounted onto the metrics listener by accident — in
	// particular the net/http/pprof handlers in the comment above, which register on
	// DefaultServeMux and would otherwise expose process internals on this port.
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// NB: this binds to all interfaces (:port). Operators should firewall the metrics
	// port or restrict it to the scraper's network; it is unauthenticated by design
	// (low-sensitivity gauges/counters).
	promListen := fmt.Sprintf(":%s", s.cfg.Prometheus.Port)

	logger.InfoWithFields("service", "initPrometheus", "", "initialise prometheus", logger.Fields{
		"listen": promListen,
	})

	srv := &http.Server{Addr: promListen, Handler: mux}
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("service", "initPrometheus", "prometheus server stopped", err.Error())
	}
}
