package downloadqueue

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"context"
	"fmt"
	"sync"
	"time"
)

const workerSafetyTickInterval = 30 * time.Second

var (
	wakeCh   = make(chan struct{}, 1)
	workerID = fmt.Sprintf("worker-%d", time.Now().UnixNano())

	// Closed on shutdown, so the worker stops claiming jobs
	stopCh   = make(chan struct{})
	stopOnce sync.Once
	// The worker loop and the running jobs
	workerWG sync.WaitGroup

	// Stops each running job, by job ID
	runningMu   sync.Mutex
	runningJobs = map[int64]context.CancelFunc{}

	// Set in tests
	claimJob   = claimNextJob
	processJob = processQueueJob
)

// SignalNewJob wakes the worker loop to check for new pending jobs. Safe to
// call from any goroutine; bursts of calls coalesce into a single wake.
func SignalNewJob() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
}

// StartWorker starts the background goroutine that claims Download Queue Jobs,
// running up to the simultaneous downloads at once. It is woken by SignalNewJob
// (fired on enqueue and when a job finishes) and by a periodic safety-net tick,
// so a job can never be picked up twice: claiming is an atomic DB operation
// regardless of what triggered it.
func StartWorker(ctx context.Context) {
	workerWG.Add(1)
	go func() {
		defer workerWG.Done()
		ticker := time.NewTicker(workerSafetyTickInterval)
		defer ticker.Stop()

		// Jobs that were processing when aura last stopped run again, then the queue starts
		resetInterruptedJobs(ctx)
		retryDueDownloads(ctx)
		startJobs(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-stopCh:
				return
			case <-wakeCh:
			case <-ticker.C:
				// Failed downloads are retried automatically after a wait
				retryDueDownloads(ctx)
			}
			startJobs(ctx)
		}
	}()
}

// StopWorker stops the worker from claiming more jobs and waits for the running jobs to finish, or for ctx to end
func StopWorker(ctx context.Context) {
	stopOnce.Do(func() { close(stopCh) })
	done := make(chan struct{})
	go func() {
		workerWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		logging.LOGGER.Warn().Timestamp().Msg("Stopped waiting for the running download queue jobs to finish")
	}
}

// startJobs starts waiting jobs until the simultaneous downloads are running.
// Only the worker loop calls it.
func startJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		default:
		}

		// Claimed and added while locked, so clearing the queue stops a job that was just claimed
		runningMu.Lock()
		if len(runningJobs) >= config.SimultaneousDownloads() {
			runningMu.Unlock()
			return
		}
		job, found := claimJob(ctx)
		if !found {
			runningMu.Unlock()
			return
		}
		stopCtx, stop := context.WithCancel(ctx)
		runningJobs[job.ID] = stop
		runningMu.Unlock()

		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			defer func() {
				runningMu.Lock()
				delete(runningJobs, job.ID)
				runningMu.Unlock()
				stop()
				// A job can start in its place
				SignalNewJob()
			}()
			defer logging.RecoverPanic("Download Queue worker")
			processJob(ctx, stopCtx, job)
		}()
	}
}

// stopJob stops a running job, and returns whether it was running
func stopJob(jobID int64) bool {
	runningMu.Lock()
	defer runningMu.Unlock()
	stop, ok := runningJobs[jobID]
	if ok {
		stop()
	}
	return ok
}

// stopAllJobs stops every running job
func stopAllJobs() {
	runningMu.Lock()
	defer runningMu.Unlock()
	for _, stop := range runningJobs {
		stop()
	}
}

func resetInterruptedJobs(ctx context.Context) {
	ctx, ld := logging.CreateLoggingContext(ctx, "Download Queue Worker")
	logAction := ld.AddAction("Reset Interrupted Download Queue Jobs", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	count, Err := database.ResetInterruptedDownloadQueueJobs(ctx, workerID)
	logAction.Complete()
	if Err.Message != "" {
		ld.Log()
		return
	}
	if count > 0 {
		logging.LOGGER.Info().Timestamp().Int64("jobs", count).Msg("Download queue jobs stopped by the last restart will run again")
	}
}
