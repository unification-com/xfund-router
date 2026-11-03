package chain

import (
	"errors"
	"time"

	"go-ooo/database/models"
	"go-ooo/logger"

	"github.com/ethereum/go-ethereum"
)

// maxFulfilmentAttempts is the number of tries before a job is given up on.
const maxFulfilmentAttempts uint64 = 3

// droppedTxBlocks is how many blocks after sending a fulfilment tx that is neither mined nor in
// the mempool counts as dropped (evicted, or never propagated). Generous, so a tx that a
// load-balanced RPC node has simply not seen yet is not mistaken for a dropped one.
const droppedTxBlocks uint64 = 50

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

// giveUpIfDue fails the job (FULFILMENT_FAILED) when shouldGiveUp says so, logging why under
// fn, and reports whether it did - the one give-up path for every stuck/failed handler.
func (o *OoORouterService) giveUpIfDue(job models.DataRequests, fn string) bool {
	giveUp, reason := o.shouldGiveUp(job)
	if !giveUp {
		return false
	}
	o.log.WarnWithFields("chain", fn, "give up", reason,
		logger.Fields{
			"request_id":   job.GetRequestId(),
			"num_attempts": job.GetFulfillmentAttempts(),
		})
	_ = o.db.UpdateRequestStatus(o.networkId, job.GetRequestId(), models.REQUEST_STATUS_FULFILMENT_FAILED, reason)
	return true
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

// isDroppedTx reports whether a failed tx lookup means the tx was dropped: the node does not
// know it (neither mined nor pending) and it is at least droppedTxBlocks old. Any other error,
// or a younger tx, may be transient, so the caller should simply look again next cycle.
func isDroppedTx(lookupErr error, blocksSinceSent uint64) bool {
	return errors.Is(lookupErr, ethereum.NotFound) && blocksSinceSent >= droppedTxBlocks
}
