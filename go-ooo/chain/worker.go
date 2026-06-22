package chain

import (
	"time"

	"go-ooo/logger"
	go_ooo_types "go-ooo/types"
)

// defaultJobPollSeconds is the job-queue sweep interval used when jobs.check_duration is unset.
const defaultJobPollSeconds uint64 = 30

// NetworkId returns the EVM chain id this worker is bound to.
func (o *OoORouterService) NetworkId() int64 {
	return o.networkId
}

// jobPollInterval is how often this worker sweeps its pending job queue, from jobs.check_duration
// (a global setting; the same cadence applies to every chain), falling back to the default.
func (o *OoORouterService) jobPollInterval() time.Duration {
	secs := o.cfg.Jobs.CheckDuration
	if secs == 0 {
		secs = defaultJobPollSeconds
	}
	return time.Second * time.Duration(secs)
}

// Run drives this chain's worker until the context is cancelled. It first replays any events
// missed while the oracle was down, then starts the live event subscriptions, then serialises
// this chain's work - fulfilment sweeps (periodic + event-nudged) and admin transactions - on a
// single loop. Keeping every transaction-sending operation for a chain on one goroutine is what
// guarantees the provider account's nonce on that chain is never raced. One worker runs per chain;
// because each has its own nonce space and RPC client, the workers run concurrently and never
// block one another.
//
// Run is intended to be started in its own goroutine (one per chain). Teardown of the event
// subscriptions (and saving the resume block) is owned by Shutdown(), which the supervisor calls
// on the same cancellation.
func (o *OoORouterService) Run() {
	logger.InfoWithFields("chain", "Run", "", "starting chain worker", logger.Fields{
		"network_id": o.networkId,
		"provider":   o.oracleAddress.Hex(),
	})

	// Pick up from the last block we know about to process any historical events missed while the
	// oracle was down. This completes before the live subscriptions start, so a request that was
	// both raised and (potentially) fulfilled during the downtime is reconciled first.
	o.GetHistoricalEvents()

	go o.RunEventWatchers()

	jobTicker := time.NewTicker(o.jobPollInterval())
	defer jobTicker.Stop()

	// A nudge buffered during the historical catch-up above is drained on the first iteration, so
	// the startup backlog is processed at once too, rather than waiting for the first ticker tick.
	for {
		select {
		case <-o.context.Done():
			logger.InfoWithFields("chain", "Run", "", "context cancelled - stopping chain worker", logger.Fields{
				"network_id": o.networkId,
			})
			return
		case <-jobTicker.C:
			o.ProcessPendingJobQueue("ticker")
		case <-o.jobNudge:
			o.ProcessPendingJobQueue("event")
		case t := <-o.adminTasks:
			// Serialised here with this chain's fulfilment txs (same goroutine), so an admin
			// send and a fulfilment send never race the account nonce.
			t.Resp <- o.ProcessAdminTask(t)
		}
	}
}

// SubmitAdminTask hands an admin task to this worker's run loop and returns once the loop has
// accepted it (the loop replies on task.Resp). If the worker is already shutting down, it replies
// with a failure rather than blocking forever on a loop that will never receive. The supervisor
// calls this (off its own goroutine) to route an admin request to the correct chain.
func (o *OoORouterService) SubmitAdminTask(task go_ooo_types.AdminTask) {
	select {
	case o.adminTasks <- task:
	case <-o.context.Done():
		task.Resp <- go_ooo_types.AdminTaskResponse{
			AdminTask: task,
			Success:   false,
			Error:     "service is shutting down",
		}
	}
}
