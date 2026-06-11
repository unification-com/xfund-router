package chain

import (
	"testing"
)

// A burst of new-request nudges must never block the event watcher (non-blocking send onto a
// cap-1 buffer) and must collapse into a single pending signal - one ProcessPendingJobQueue run
// then picks up every just-inserted request.
func TestNudgeJobQueueCoalesces(t *testing.T) {
	o := &OoORouterService{jobNudge: make(chan struct{}, 1)}

	for i := 0; i < 5; i++ {
		o.nudgeJobQueue() // would deadlock here if the send were blocking
	}

	select {
	case <-o.JobNudge():
	default:
		t.Fatal("expected one buffered nudge after a burst")
	}

	select {
	case <-o.JobNudge():
		t.Fatal("expected the burst to coalesce to a single nudge")
	default:
	}
}

// After the job loop drains a nudge (i.e. runs the queue), a later request must be able to
// re-arm the signal so it is not missed.
func TestNudgeJobQueueReArmsAfterDrain(t *testing.T) {
	o := &OoORouterService{jobNudge: make(chan struct{}, 1)}

	o.nudgeJobQueue()
	<-o.JobNudge() // mimics the select loop consuming the nudge and running the queue

	o.nudgeJobQueue()
	select {
	case <-o.JobNudge():
	default:
		t.Fatal("expected the nudge to re-arm after draining")
	}
}
