package downloadqueue

import (
	"aura/database"
	"aura/logging"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const workerSafetyTickInterval = 30 * time.Second

var (
	wakeCh   = make(chan struct{}, 1)
	draining atomic.Bool
	workerID = fmt.Sprintf("worker-%d", time.Now().UnixNano())

	// Closed on shutdown, so the worker stops claiming jobs
	stopCh   = make(chan struct{})
	stopOnce sync.Once
	workerWG sync.WaitGroup
)

// SignalNewJob wakes the worker loop to check for new pending jobs. Safe to
// call from any goroutine; bursts of calls coalesce into a single wake.
func SignalNewJob() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
}

// StartWorker starts the background goroutine that claims and processes
// Download Queue Jobs one at a time. It is woken by SignalNewJob (fired on
// enqueue) and by a periodic safety-net tick, so a job can never be picked up
// twice: claiming is an atomic DB operation regardless of what triggered it.
func StartWorker(ctx context.Context) {
	workerWG.Add(1)
	go func() {
		defer workerWG.Done()
		ticker := time.NewTicker(workerSafetyTickInterval)
		defer ticker.Stop()

		// Jobs that were processing when aura last stopped run again, then the queue starts
		resetInterruptedJobs(ctx)
		retryDueDownloads(ctx)
		drainQueue(ctx)

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
			drainQueue(ctx)
		}
	}()
}

// StopWorker stops the worker from claiming more jobs and waits for the current job to finish, or for ctx to end
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
		logging.LOGGER.Warn().Timestamp().Msg("Stopped waiting for the current download queue job to finish")
	}
}

func drainQueue(ctx context.Context) {
	if !draining.CompareAndSwap(false, true) {
		// Another drain is already in progress; it will pick up any jobs this signal was for.
		return
	}
	defer draining.Store(false)

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		default:
		}

		processedOne := processNextJob(ctx)
		if !processedOne {
			return
		}
	}
}

func processNextJob(ctx context.Context) (processed bool) {
	defer logging.RecoverPanic("Download Queue worker")

	return ProcessNextQueueJob(ctx)
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
