package dex

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// dexQueryTotal records the outcome of each per-DEX price subquery, labelled by chain, dex and
// result. It surfaces subgraph health: an indexer that starts erroring (as the decentralised
// gateway's shibaswap/pancakeswap indexers have) shows up immediately as a rising "error" rate
// for that dex, while "no_data" flags a curated-but-empty pair. Registered on the default
// registry at init - the service exposes it at /metrics with the rest.
var dexQueryTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "ooo_dex_query_total",
	Help: "Per-DEX price-query outcomes, by chain, dex and result (success|error|no_data).",
}, []string{"chain", "dex", "result"})
