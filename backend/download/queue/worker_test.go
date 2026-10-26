package downloadqueue

import (
	"aura/config"
	"aura/database"
	"context"
	"sync"
	"testing"
	"time"
)

// fakeQueue hands out jobs, and holds each one running until it's stopped or released
type fakeQueue struct {
	mu      sync.Mutex
	waiting []int64
	started chan int64
	release chan struct{}
	stopped chan int64
}

func useFakeQueue(t *testing.T, jobs int, simultaneous int) *fakeQueue {
	t.Helper()
	q := &fakeQueue{started: make(chan int64, jobs), release: make(chan struct{}), stopped: make(chan int64, jobs)}
	for id := int64(1); id <= int64(jobs); id++ {
		q.waiting = append(q.waiting, id)
	}

	oldClaim, oldProcess, oldLimit := claimJob, processJob, config.Current.Mediux.SimultaneousDownloads
	claimJob = func(context.Context) (database.DownloadQueueJob, bool) {
		q.mu.Lock()
		defer q.mu.Unlock()
		if len(q.waiting) == 0 {
			return database.DownloadQueueJob{}, false
		}
		id := q.waiting[0]
		q.waiting = q.waiting[1:]
		return database.DownloadQueueJob{ID: id}, true
	}
	processJob = func(_ context.Context, stop context.Context, job database.DownloadQueueJob) {
		q.started <- job.ID
		select {
		case <-stop.Done():
			q.stopped <- job.ID
		case <-q.release:
		}
	}
	config.Current.Mediux.SimultaneousDownloads = simultaneous
	t.Cleanup(func() {
		claimJob, processJob, config.Current.Mediux.SimultaneousDownloads = oldClaim, oldProcess, oldLimit
		stopAllJobs()
		workerWG.Wait()
	})
	return q
}

func waitFor(t *testing.T, ch chan int64, what string) int64 {
	t.Helper()
	select {
	case id := <-ch:
		return id
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", what)
		return 0
	}
}

func TestStartJobsRunsUpToTheSimultaneousDownloads(t *testing.T) {
	q := useFakeQueue(t, 3, 2)

	startJobs(context.Background())
	waitFor(t, q.started, "the first job")
	waitFor(t, q.started, "the second job")
	select {
	case id := <-q.started:
		t.Fatalf("job %d started while 2 jobs were running", id)
	case <-time.After(50 * time.Millisecond):
	}

	// A finished job makes room for the next
	q.release <- struct{}{}
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		runningMu.Lock()
		running := len(runningJobs)
		runningMu.Unlock()
		if running < 2 {
			break
		}
	}
	startJobs(context.Background())
	if id := waitFor(t, q.started, "the third job"); id != 3 {
		t.Errorf("started job %d, want job 3", id)
	}
}

func TestStopJobStopsOnlyThatJob(t *testing.T) {
	q := useFakeQueue(t, 2, 2)
	startJobs(context.Background())
	waitFor(t, q.started, "the first job")
	waitFor(t, q.started, "the second job")

	if !stopJob(2) {
		t.Fatal("job 2 should be running")
	}
	if id := waitFor(t, q.stopped, "job 2 to stop"); id != 2 {
		t.Errorf("stopped job %d, want job 2", id)
	}
	if stopJob(3) {
		t.Error("job 3 isn't running")
	}

	stopAllJobs()
	if id := waitFor(t, q.stopped, "job 1 to stop"); id != 1 {
		t.Errorf("stopped job %d, want job 1", id)
	}
}
