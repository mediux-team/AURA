package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// DownloadQueueJob is a single queued/processed download job.
type DownloadQueueJob struct {
	ID             int64              `json:"id"`
	TMDB_ID        string             `json:"tmdb_id"`
	LibraryTitle   string             `json:"library_title"`
	Edition        string             `json:"edition"`
	MediaItemTitle string             `json:"media_item_title"`
	Payload        models.DBSavedItem `json:"item"`
	Status         string             `json:"status"`
	ResultMessage  string             `json:"result_message"`
	ResultErrors   []string           `json:"result_errors"`
	ResultWarnings []string           `json:"result_warnings"`
	CreatedAt      time.Time          `json:"created_at"`
	StartedAt      *time.Time         `json:"started_at,omitempty"`
	FinishedAt     *time.Time         `json:"finished_at,omitempty"`
}

func (s *SQliteDB) EnqueueDownloadQueueJob(ctx context.Context, item models.DBSavedItem) (jobID int64, Err logging.LogErrorInfo) {
	tmdbID, libraryTitle, edition, title := item.QueueEntry()
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Enqueue Download Queue Job for '%s' (%s | %s)", title, tmdbID, libraryTitle),
		logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return 0, *logAction.Error
	}

	// Movie and collection sets only have posters and backdrops
	item = item.WithSetImageTypes()

	payload, err := json.Marshal(item)
	if err != nil {
		logAction.SetError("Failed to marshal Download Queue Job payload", err.Error(), map[string]any{"error": err.Error()})
		return 0, *logAction.Error
	}

	q := `
INSERT INTO DownloadQueueJobs (tmdb_id, library_title, edition, media_item_title, payload)
VALUES (?, ?, ?, ?, ?)
RETURNING id;
`
	err = s.conn.QueryRowContext(ctx, q,
		tmdbID,
		libraryTitle,
		edition,
		title,
		string(payload),
	).Scan(&jobID)
	if err != nil {
		logAction.SetError("DB: INSERT DownloadQueueJobs failed", err.Error(), map[string]any{"error": err.Error()})
		return 0, *logAction.Error
	}

	logAction.AppendResult("job_id", jobID)
	return jobID, logging.LogErrorInfo{}
}

func (s *SQliteDB) ClaimNextDownloadQueueJob(ctx context.Context, workerID string) (job DownloadQueueJob, found bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Claim Next Download Queue Job", logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return DownloadQueueJob{}, false, *logAction.Error
	}

	q := `
UPDATE DownloadQueueJobs
SET status = 'processing', started_at = CURRENT_TIMESTAMP, worker_id = ?
WHERE id = (
    SELECT id FROM DownloadQueueJobs WHERE status = 'pending' ORDER BY created_at ASC LIMIT 1
)
RETURNING id, tmdb_id, library_title, edition, media_item_title, payload, status, created_at, started_at;
`
	var payload string
	err := s.conn.QueryRowContext(ctx, q, workerID).Scan(
		&job.ID, &job.TMDB_ID, &job.LibraryTitle, &job.Edition, &job.MediaItemTitle,
		&payload, &job.Status, &job.CreatedAt, &job.StartedAt,
	)
	if err == sql.ErrNoRows {
		return DownloadQueueJob{}, false, logging.LogErrorInfo{}
	}
	if err != nil {
		logAction.SetError("DB: claim DownloadQueueJobs failed", err.Error(), map[string]any{"error": err.Error()})
		return DownloadQueueJob{}, false, *logAction.Error
	}

	if err := json.Unmarshal([]byte(payload), &job.Payload); err != nil {
		logAction.SetError("Failed to unmarshal Download Queue Job payload", err.Error(), map[string]any{"error": err.Error(), "job_id": job.ID})
		return DownloadQueueJob{}, false, *logAction.Error
	}

	logAction.AppendResult("job_id", job.ID)
	return job, true, logging.LogErrorInfo{}
}

func (s *SQliteDB) FinishDownloadQueueJob(ctx context.Context, jobID int64, status string, message string, errs, warnings []string) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Finish Download Queue Job %d (%s)", jobID, status), logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	if errs == nil {
		errs = []string{}
	}
	if warnings == nil {
		warnings = []string{}
	}

	errsJSON, err := json.Marshal(errs)
	if err != nil {
		logAction.SetError("Failed to marshal Download Queue Job errors", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	warningsJSON, err := json.Marshal(warnings)
	if err != nil {
		logAction.SetError("Failed to marshal Download Queue Job warnings", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	q := `
UPDATE DownloadQueueJobs
SET status = ?, result_message = ?, result_errors = ?, result_warnings = ?, finished_at = CURRENT_TIMESTAMP
WHERE id = ?;
`
	_, err = s.conn.ExecContext(ctx, q, status, message, string(errsJSON), string(warningsJSON), jobID)
	if err != nil {
		logAction.SetError("DB: UPDATE DownloadQueueJobs (finish) failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

func (s *SQliteDB) GetAllDownloadQueueJobs(ctx context.Context) (jobs []DownloadQueueJob, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get All Download Queue Jobs", logging.LevelDebug)
	defer logAction.Complete()

	jobs = []DownloadQueueJob{}

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return jobs, *logAction.Error
	}

	q := `
SELECT id, tmdb_id, library_title, edition, media_item_title, payload, status,
       result_message, result_errors, result_warnings, created_at, started_at, finished_at
FROM DownloadQueueJobs
ORDER BY created_at ASC;
`
	rows, err := s.conn.QueryContext(ctx, q)
	if err != nil {
		logAction.SetError("DB: SELECT DownloadQueueJobs failed", err.Error(), map[string]any{"error": err.Error()})
		return jobs, *logAction.Error
	}
	defer rows.Close()

	for rows.Next() {
		var job DownloadQueueJob
		var payload, errsJSON, warningsJSON string
		if err := rows.Scan(
			&job.ID, &job.TMDB_ID, &job.LibraryTitle, &job.Edition, &job.MediaItemTitle,
			&payload, &job.Status, &job.ResultMessage, &errsJSON, &warningsJSON,
			&job.CreatedAt, &job.StartedAt, &job.FinishedAt,
		); err != nil {
			logAction.SetError("DB: scan DownloadQueueJobs failed", err.Error(), map[string]any{"error": err.Error()})
			return jobs, *logAction.Error
		}

		if err := json.Unmarshal([]byte(payload), &job.Payload); err != nil {
			logAction.SetError("Failed to unmarshal Download Queue Job payload", err.Error(), map[string]any{"error": err.Error(), "job_id": job.ID})
			return jobs, *logAction.Error
		}
		_ = json.Unmarshal([]byte(errsJSON), &job.ResultErrors)
		_ = json.Unmarshal([]byte(warningsJSON), &job.ResultWarnings)

		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("DB: iterate DownloadQueueJobs rows failed", err.Error(), map[string]any{"error": err.Error()})
		return jobs, *logAction.Error
	}

	logAction.AppendResult("count", len(jobs))
	return jobs, logging.LogErrorInfo{}
}

func (s *SQliteDB) DeleteDownloadQueueJob(ctx context.Context, jobID int64) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Delete Download Queue Job %d", jobID), logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	_, err := s.conn.ExecContext(ctx, `DELETE FROM DownloadQueueJobs WHERE id = ?;`, jobID)
	if err != nil {
		logAction.SetError("DB: DELETE DownloadQueueJobs failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

func (s *SQliteDB) DeleteAllDownloadQueueJobs(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Delete All Download Queue Jobs", logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	// Skip rows currently being processed so an in-flight worker job isn't orphaned.
	_, err := s.conn.ExecContext(ctx, `DELETE FROM DownloadQueueJobs WHERE status != 'processing';`)
	if err != nil {
		logAction.SetError("DB: DELETE ALL DownloadQueueJobs failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

// ResetInterruptedDownloadQueueJobs puts jobs another worker was processing back in the queue.
// They were stopped part way when aura restarted, so they run again from the start.
func (s *SQliteDB) ResetInterruptedDownloadQueueJobs(ctx context.Context, workerID string) (count int64, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Reset Interrupted Download Queue Jobs", logging.LevelDebug)
	defer logAction.Complete()

	result, err := s.conn.ExecContext(ctx, `
UPDATE DownloadQueueJobs
SET status = 'pending', started_at = NULL, worker_id = ''
WHERE status = 'processing' AND worker_id != ?;
`, workerID)
	if err != nil {
		logAction.SetError("DB: reset interrupted DownloadQueueJobs failed", err.Error(), map[string]any{"error": err.Error()})
		return 0, *logAction.Error
	}
	count, _ = result.RowsAffected()
	logAction.AppendResult("reset_jobs", count)
	return count, logging.LogErrorInfo{}
}
