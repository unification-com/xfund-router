package models

import "testing"

func TestIsTerminalRequestStatus(t *testing.T) {
	terminal := []int{REQUEST_STATUS_SUCCESS, REQUEST_STATUS_FULFILMENT_FAILED}
	for _, s := range terminal {
		if !IsTerminalRequestStatus(s) {
			t.Errorf("status %d should be terminal", s)
		}
	}
	nonTerminal := []int{
		REQUEST_STATUS_INITIALISED, REQUEST_STATUS_FETCHING_DATA,
		REQUEST_STATUS_DATA_READY_TO_SEND, REQUEST_STATUS_TX_SENT,
		REQUEST_STATUS_API_ERROR, REQUEST_STATUS_TX_FAILED,
	}
	for _, s := range nonTerminal {
		if IsTerminalRequestStatus(s) {
			t.Errorf("status %d should not be terminal", s)
		}
	}
}

func TestJobStatusForRequestStatus(t *testing.T) {
	if got := JobStatusForRequestStatus(REQUEST_STATUS_SUCCESS); got != JOB_STATUS_SUCCESS {
		t.Errorf("SUCCESS -> %d, want JOB_STATUS_SUCCESS", got)
	}
	if got := JobStatusForRequestStatus(REQUEST_STATUS_FULFILMENT_FAILED); got != JOB_STATUS_FAIL {
		t.Errorf("FULFILMENT_FAILED -> %d, want JOB_STATUS_FAIL", got)
	}
	for _, s := range []int{REQUEST_STATUS_INITIALISED, REQUEST_STATUS_FETCHING_DATA, REQUEST_STATUS_TX_SENT} {
		if got := JobStatusForRequestStatus(s); got != JOB_STATUS_PENDING {
			t.Errorf("status %d -> %d, want JOB_STATUS_PENDING", s, got)
		}
	}
}

func TestIsExpectedTransition(t *testing.T) {
	legal := [][2]int{
		{REQUEST_STATUS_INITIALISED, REQUEST_STATUS_FETCHING_DATA},
		{REQUEST_STATUS_FETCHING_DATA, REQUEST_STATUS_API_ERROR},
		{REQUEST_STATUS_FETCHING_DATA, REQUEST_STATUS_FETCHING_DATA}, // self / re-fetch
		{REQUEST_STATUS_DATA_READY_TO_SEND, REQUEST_STATUS_TX_SENT},
		{REQUEST_STATUS_TX_SENT, REQUEST_STATUS_TX_SENT}, // self / resend
		{REQUEST_STATUS_TX_SENT, REQUEST_STATUS_FULFILMENT_FAILED},
		{REQUEST_STATUS_API_ERROR, REQUEST_STATUS_FETCHING_DATA},
		{REQUEST_STATUS_TX_FAILED, REQUEST_STATUS_FETCHING_DATA},
	}
	for _, tr := range legal {
		if !IsExpectedTransition(tr[0], tr[1]) {
			t.Errorf("transition %d -> %d should be expected", tr[0], tr[1])
		}
	}

	unexpected := [][2]int{
		{REQUEST_STATUS_INITIALISED, REQUEST_STATUS_TX_SENT},
		{REQUEST_STATUS_FETCHING_DATA, REQUEST_STATUS_TX_SENT},
		{REQUEST_STATUS_SUCCESS, REQUEST_STATUS_TX_SENT},
	}
	for _, tr := range unexpected {
		if IsExpectedTransition(tr[0], tr[1]) {
			t.Errorf("transition %d -> %d should be unexpected", tr[0], tr[1])
		}
	}
}
