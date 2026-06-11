// Package report builds operator-facing summaries from the request history: an overall P&L
// summary plus per-consumer, per-pair and failure breakdowns. Build is pure (no DB access), so
// it is trivially unit-testable and reusable by any caller that can supply the rows - the testapp
// report command loads them from a sqlite file or a Postgres dump.
package report

import (
	"sort"

	"go-ooo/database/models"
)

// weiPerEth converts gas costs (wei) to ETH; feeDenom converts the stored fee (xFUND's lowest
// denomination, 9 decimals) to whole xFUND. Costs are accumulated directly in ETH/xFUND as
// float64 to avoid overflowing a uint64 wei sum across a long history - this is a display report,
// not on-chain arithmetic.
const (
	weiPerEth = 1e18
	feeDenom  = 1e9
)

// Overall is the headline summary across every request in the window.
type Overall struct {
	TotalRequests    int
	Successful       int
	FulfilmentFailed int     // requests that gave up (REQUEST_STATUS_FULFILMENT_FAILED)
	Pending          int     // detected but not yet in a terminal state
	SuccessRatePct   float64 // successful / (successful + fulfilmentFailed)
	RevertedAttempts int     // individual reverted/failed fulfilment txs (gas still spent)
	FeesEarnedXfund  float64 // fees from successful fulfilments
	GasCostEth       float64 // winning-tx gas (successes) + every reverted attempt's gas
	FeesEarnedEth    float64 // FeesEarnedXfund * xfundPriceEth (0 if no price given)
	ProfitLossEth    float64 // FeesEarnedEth - GasCostEth (only meaningful with a price)
}

// Group is one row of a per-consumer or per-pair breakdown.
type Group struct {
	Key              string
	Requests         int
	Successful       int
	FulfilmentFailed int
	FeesEarnedXfund  float64
	GasCostEth       float64
}

// Failure is a count of terminal failures sharing a reason.
type Failure struct {
	Reason string
	Count  int
}

// Report is the full operator report.
type Report struct {
	XfundPriceEth float64
	Overall       Overall
	ByConsumer    []Group   // sorted by request count desc, then key
	ByPair        []Group   // sorted by request count desc, then key
	Failures      []Failure // terminal-failure reasons, sorted by count desc, then reason
}

// Build aggregates requests and failed-fulfilment attempts into a Report. xfundPriceEth (ETH per
// xFUND) is optional: pass 0 to skip the ETH-denominated fee/P&L figures. Gas cost counts the
// winning tx of each success plus every recorded reverted attempt whose request is in the set, so
// it reflects real spend without double-counting (the successful tx is never in failed_fulfilments).
func Build(requests []models.DataRequests, failed []models.FailedFulfilment, xfundPriceEth float64) Report {
	r := Report{XfundPriceEth: xfundPriceEth}
	o := &r.Overall

	reqByID := make(map[string]models.DataRequests, len(requests))
	consumers := map[string]*Group{}
	pairs := map[string]*Group{}
	failureCounts := map[string]int{}

	group := func(m map[string]*Group, key string) *Group {
		g := m[key]
		if g == nil {
			g = &Group{Key: key}
			m[key] = g
		}
		return g
	}

	for _, req := range requests {
		reqByID[req.RequestId] = req
		o.TotalRequests++
		cg := group(consumers, req.Consumer)
		pg := group(pairs, req.EndpointDecoded)
		cg.Requests++
		pg.Requests++

		switch req.GetRequestStatus() {
		case models.REQUEST_STATUS_SUCCESS:
			o.Successful++
			cg.Successful++
			pg.Successful++
			feeX := float64(req.Fee) / feeDenom
			gasEth := float64(req.FulfillGasUsed) * float64(req.FulfillGasPrice) / weiPerEth
			o.FeesEarnedXfund += feeX
			cg.FeesEarnedXfund += feeX
			pg.FeesEarnedXfund += feeX
			o.GasCostEth += gasEth
			cg.GasCostEth += gasEth
			pg.GasCostEth += gasEth
		case models.REQUEST_STATUS_FULFILMENT_FAILED:
			o.FulfilmentFailed++
			cg.FulfilmentFailed++
			pg.FulfilmentFailed++
			reason := req.GetStatusReason()
			if reason == "" {
				reason = "(unknown)"
			}
			failureCounts[reason]++
		default:
			o.Pending++
		}
	}

	// Reverted/failed attempts cost gas too. Attribute each to its request's consumer/pair when
	// that request is in this window; skip orphans so Overall stays equal to the group sums.
	for _, f := range failed {
		req, ok := reqByID[f.RequestId]
		if !ok {
			continue
		}
		gasEth := float64(f.GasUsed) * float64(f.GasPrice) / weiPerEth
		o.RevertedAttempts++
		o.GasCostEth += gasEth
		group(consumers, req.Consumer).GasCostEth += gasEth
		group(pairs, req.EndpointDecoded).GasCostEth += gasEth
	}

	if terminal := o.Successful + o.FulfilmentFailed; terminal > 0 {
		o.SuccessRatePct = 100 * float64(o.Successful) / float64(terminal)
	}
	o.FeesEarnedEth = o.FeesEarnedXfund * xfundPriceEth
	o.ProfitLossEth = o.FeesEarnedEth - o.GasCostEth

	r.ByConsumer = sortedGroups(consumers)
	r.ByPair = sortedGroups(pairs)
	r.Failures = sortedFailures(failureCounts)
	return r
}

// sortedGroups flattens the group map into a slice ordered by request count (desc), with the key
// as a stable tie-break so the output is deterministic.
func sortedGroups(m map[string]*Group) []Group {
	out := make([]Group, 0, len(m))
	for _, g := range m {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Requests != out[j].Requests {
			return out[i].Requests > out[j].Requests
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// sortedFailures flattens the reason→count map ordered by count (desc), reason as the tie-break.
func sortedFailures(m map[string]int) []Failure {
	out := make([]Failure, 0, len(m))
	for reason, count := range m {
		out = append(out, Failure{Reason: reason, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}
