package chain

import (
	"time"

	"go-ooo/database/models"
)

// maxFulfilmentAttempts is the number of tries before a job is given up on.
const maxFulfilmentAttempts uint64 = 3

// blockDiff returns current - stored, saturating to 0 when stored > current. A reorg or
// a stale head can leave stored ahead of current; without this guard the uint64
// subtraction wraps to a huge value and the age/window checks misfire (e.g. a healthy
// job looking "too old").
func blockDiff(current, stored uint64) uint64 {
	if stored > current {
		return 0
	}
	return current - stored
}

// shouldGiveUp reports whether a job has exhausted its retries or exceeded its max age,
// plus the reason, so the stuck/failed handlers can fail it consistently in one place.
func (o *OoORouterService) shouldGiveUp(job models.DataRequests) (bool, string) {
	return decideGiveUp(job.GetFulfillmentAttempts(), time.Since(job.CreatedAt),
		time.Duration(o.cfg.Jobs.MaxJobAge)*time.Second)
}

// decideGiveUp is the pure give-up decision: too many attempts, or older than maxAge
// (wall-clock, chain-agnostic). maxAge <= 0 disables the age check.
func decideGiveUp(attempts uint64, age, maxAge time.Duration) (bool, string) {
	if attempts >= maxFulfilmentAttempts {
		return true, "too many failed attempts"
	}
	if maxAge > 0 && age > maxAge {
		return true, "request too old"
	}
	return false, ""
}
