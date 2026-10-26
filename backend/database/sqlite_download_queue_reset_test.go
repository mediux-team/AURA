package database

import (
	"slices"
	"testing"
)

func TestResetInterruptedDownloadQueueJobs(t *testing.T) {
	db := openTestMediaDB(t)
	exec(t, db.conn, `INSERT INTO DownloadQueueJobs (id, tmdb_id, library_title, media_item_title, payload, status, started_at, worker_id) VALUES
		(1, '1', 'Movies', 'Old Run', '{}', 'processing', CURRENT_TIMESTAMP, 'worker-old'),
		(2, '2', 'Movies', 'This Run', '{}', 'processing', CURRENT_TIMESTAMP, 'worker-new'),
		(3, '3', 'Movies', 'Waiting', '{}', 'pending', NULL, ''),
		(4, '4', 'Movies', 'Done', '{}', 'success', CURRENT_TIMESTAMP, 'worker-old')`)

	reset, Err := db.ResetInterruptedDownloadQueueJobs(testContext(), "worker-new")
	if Err.Message != "" {
		t.Fatalf("ResetInterruptedDownloadQueueJobs: %s", Err.Message)
	}
	if reset != 1 {
		t.Errorf("reset %d jobs, want 1", reset)
	}

	want := map[int]string{1: "pending", 2: "processing", 3: "pending", 4: "success"}
	for id, status := range want {
		var got string
		if err := db.conn.QueryRow("SELECT status FROM DownloadQueueJobs WHERE id = ?", id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != status {
			t.Errorf("job %d status = %s, want %s", id, got, status)
		}
	}
	if n := count(t, db.conn, "SELECT COUNT(*) FROM DownloadQueueJobs WHERE id = 1 AND started_at IS NULL AND worker_id = ''"); n != 1 {
		t.Error("the reset job should have no start time or worker")
	}
}

func TestClaimNextDownloadQueueJob(t *testing.T) {
	db := openTestMediaDB(t)
	exec(t, db.conn, `INSERT INTO DownloadQueueJobs (id, tmdb_id, library_title, media_item_title, payload, status, created_at) VALUES
		(1, '1', 'Movies', 'First', '{}', 'pending', '2026-01-01 00:00:00'),
		(2, '2', 'Movies', 'Second', '{}', 'pending', '2026-01-01 00:00:01')`)

	// The claimed job says it's processing, the Download Queue page only shows pending and processing jobs
	job, found, Err := db.ClaimNextDownloadQueueJob(testContext(), "worker")
	if Err.Message != "" || !found {
		t.Fatalf("ClaimNextDownloadQueueJob = %v %s", found, Err.Message)
	}
	if job.ID != 1 || job.Status != "processing" || job.StartedAt == nil {
		t.Errorf("claimed job = %d %q %v, want job 1 processing with a start time", job.ID, job.Status, job.StartedAt)
	}
}

func TestClaimNextDownloadQueueJobSkipsItemsAlreadyRunning(t *testing.T) {
	db := openTestMediaDB(t)
	exec(t, db.conn, `INSERT INTO DownloadQueueJobs (id, tmdb_id, library_title, media_item_title, payload, status, created_at) VALUES
		(1, '1', 'Movies', 'Running', '{}', 'processing', '2026-01-01 00:00:00'),
		(2, '1', 'Movies', 'Same Item', '{}', 'pending', '2026-01-01 00:00:01'),
		(3, '1', '4K Movies', 'Other Library', '{}', 'pending', '2026-01-01 00:00:02')`)

	job, found, Err := db.ClaimNextDownloadQueueJob(testContext(), "worker")
	if Err.Message != "" || !found || job.ID != 3 {
		t.Fatalf("claimed job %d (found %v, %s), want job 3", job.ID, found, Err.Message)
	}
	if _, found, _ := db.ClaimNextDownloadQueueJob(testContext(), "worker"); found {
		t.Error("the item's other job should wait for the running one")
	}
}

func TestMoveDownloadQueueJobToTop(t *testing.T) {
	db := openTestMediaDB(t)
	exec(t, db.conn, `INSERT INTO DownloadQueueJobs (id, tmdb_id, library_title, media_item_title, payload, status, created_at) VALUES
		(1, '1', 'Movies', 'Running', '{}', 'processing', '2026-01-01 00:00:00'),
		(2, '2', 'Movies', 'First', '{}', 'pending', '2026-01-01 00:00:01'),
		(3, '3', 'Movies', 'Second', '{}', 'pending', '2026-01-01 00:00:01'),
		(4, '4', 'Movies', 'Last', '{}', 'pending', '2026-01-01 00:00:05')`)

	moved, Err := db.MoveDownloadQueueJobToTop(testContext(), 4)
	if Err.Message != "" || !moved {
		t.Fatalf("MoveDownloadQueueJobToTop = %v %s", moved, Err.Message)
	}
	jobs, Err := db.GetAllDownloadQueueJobs(testContext())
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}
	order := []int64{}
	for _, job := range jobs {
		order = append(order, job.ID)
	}
	if want := []int64{1, 4, 2, 3}; !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}

	job, found, _ := db.ClaimNextDownloadQueueJob(testContext(), "worker")
	if !found || job.ID != 4 {
		t.Errorf("claimed job %d, want the moved job 4", job.ID)
	}

	// A job that isn't waiting can't be moved
	if moved, _ := db.MoveDownloadQueueJobToTop(testContext(), 1); moved {
		t.Error("a running job shouldn't move")
	}
}
