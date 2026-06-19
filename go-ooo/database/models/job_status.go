package models

// This file is the single source of truth for request-status semantics: which states
// are terminal, the request-status -> job-status mapping used by the pending queue, and
// the expected orchestrator transitions.

// IsTerminalRequestStatus reports whether a request status is final. A terminal job is
// never re-driven by the orchestrator.
func IsTerminalRequestStatus(status int) bool {
	return status == REQUEST_STATUS_SUCCESS || status == REQUEST_STATUS_FULFILMENT_FAILED
}

// JobStatusForRequestStatus maps a request status to the coarse job status that the
// pending-job queue filters on.
func JobStatusForRequestStatus(status int) int {
	switch status {
	case REQUEST_STATUS_SUCCESS:
		return JOB_STATUS_SUCCESS
	case REQUEST_STATUS_FULFILMENT_FAILED:
		return JOB_STATUS_FAIL
	default:
		return JOB_STATUS_PENDING
	}
}

// expectedTransitions documents the legal status moves the orchestrator makes via
// UpdateRequestStatus. (The data-fetched -> DATA_READY_TO_SEND and the on-chain
// success -> SUCCESS moves use their own dedicated setters and are not listed here.)
// Self-transitions (re-fetch / resend) are always allowed. A move outside this map is
// applied but logged - so an unforeseen edge is visible without stalling a job - while
// transitions out of a terminal state are hard-refused in UpdateRequestStatus.
var expectedTransitions = map[int][]int{
	REQUEST_STATUS_INITIALISED:        {REQUEST_STATUS_FETCHING_DATA},
	REQUEST_STATUS_FETCHING_DATA:      {REQUEST_STATUS_API_ERROR, REQUEST_STATUS_FULFILMENT_FAILED},
	REQUEST_STATUS_DATA_READY_TO_SEND: {REQUEST_STATUS_TX_SENT, REQUEST_STATUS_TX_FAILED},
	REQUEST_STATUS_TX_SENT:            {REQUEST_STATUS_TX_FAILED, REQUEST_STATUS_FULFILMENT_FAILED},
	REQUEST_STATUS_API_ERROR:          {REQUEST_STATUS_FETCHING_DATA, REQUEST_STATUS_FULFILMENT_FAILED},
	REQUEST_STATUS_TX_FAILED:          {REQUEST_STATUS_FETCHING_DATA, REQUEST_STATUS_FULFILMENT_FAILED},
}

// IsExpectedTransition reports whether from -> to is an anticipated orchestrator move
// (a self-transition always is).
func IsExpectedTransition(from, to int) bool {
	if from == to {
		return true
	}
	for _, allowed := range expectedTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}
