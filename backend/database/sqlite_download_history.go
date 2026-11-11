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

// DownloadHistoryEntry is one poster-set-per-run download history record.
type DownloadHistoryEntry struct {
	ID              int64                        `json:"id"`
	TMDB_ID         string                       `json:"tmdb_id"`
	LibraryTitle    string                       `json:"library_title"`
	Edition         string                       `json:"edition"`
	MediaItemTitle  string                       `json:"media_item_title"`
	MediaItemYear   int                          `json:"media_item_year"`
	SetID           string                       `json:"set_id"`
	SetTitle        string                       `json:"set_title"`
	SetCreator      string                       `json:"set_creator"`
	SelectedTypes   models.SelectedTypes         `json:"selected_types"`
	WarningCount    int                          `json:"warning_count"`
	ImagesSucceeded int                          `json:"images_succeeded"`
	ImagesFailed    int                          `json:"images_failed"`
	Status          string                       `json:"status"`
	FailedImages    []models.ImageDownloadResult `json:"failed_images"`
	RetryCount      int                          `json:"retry_count"`   // Times the set was retried, automatically or with Retry
	NextRetryAt     *time.Time                   `json:"next_retry_at"` // When it's retried automatically, nil when it isn't
	CreatedAt       time.Time                    `json:"created_at"`
	RatingKey       string                       `json:"rating_key,omitempty"`
	Source          string                       `json:"source"` // What added the download that failed, empty for failures from before it was saved
}

// DownloadRetryDelays is how long a failed download waits before each automatic retry, it's retried this many times
var DownloadRetryDelays = []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour}

// nextRetryModifier is the SQLite datetime modifier for the next automatic retry, nil when there's none left
func nextRetryModifier(setID string, retryCount int) any {
	if setID == "" || retryCount >= len(DownloadRetryDelays) {
		return nil
	}
	return fmt.Sprintf("+%d seconds", int(DownloadRetryDelays[retryCount].Seconds()))
}

func (s *SQliteDB) InsertDownloadHistoryEntry(ctx context.Context, entry DownloadHistoryEntry) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Insert Download History Entry for '%s' (Set: %s)", entry.MediaItemTitle, entry.SetTitle),
		logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	if entry.FailedImages == nil {
		entry.FailedImages = []models.ImageDownloadResult{}
	}
	failedImagesJSON, err := json.Marshal(entry.FailedImages)
	if err != nil {
		logAction.SetError("Failed to marshal Download History failed images", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	selectedTypesJSON, err := json.Marshal(entry.SelectedTypes)
	if err != nil {
		logAction.SetError("Failed to marshal Download History selected types", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	status := entry.Status
	if status == "" {
		switch {
		case entry.ImagesFailed == 0:
			status = "success"
		case entry.ImagesSucceeded == 0:
			status = "error"
		default:
			status = "warning"
		}
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("DB: TX BEGIN failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	// Same media item + edition + set overwrites its existing entry instead of piling up duplicates.
	var existingID int64
	var retryCount int
	findErr := tx.QueryRowContext(ctx, `
SELECT id, retry_count FROM DownloadHistory
WHERE tmdb_id = ? AND library_title = ? AND edition = ? AND set_id = ?;
`, entry.TMDB_ID, entry.LibraryTitle, entry.Edition, entry.SetID).Scan(&existingID, &retryCount)
	// The failure is retried automatically after a wait, while it has retries left
	nextRetry := nextRetryModifier(entry.SetID, retryCount)

	if findErr != nil && findErr != sql.ErrNoRows {
		logAction.SetError("DB: SELECT existing DownloadHistory entry failed", findErr.Error(), map[string]any{"error": findErr.Error()})
		return *logAction.Error
	}

	if findErr == nil {
		_, err = tx.ExecContext(ctx, `
UPDATE DownloadHistory
SET media_item_title = ?, media_item_year = ?, set_title = ?, set_creator = ?, selected_types = ?, warning_count = ?,
    images_succeeded = ?, images_failed = ?, status = ?, failed_images = ?, created_at = CURRENT_TIMESTAMP,
    next_retry_at = CASE WHEN ? IS NULL THEN NULL ELSE datetime('now', ?) END,
    -- A retry that fails again keeps where the download came from
    source = CASE WHEN ? = 'retry' AND source != '' THEN source ELSE ? END
WHERE id = ?;
`,
			entry.MediaItemTitle, entry.MediaItemYear, entry.SetTitle, entry.SetCreator, string(selectedTypesJSON), entry.WarningCount,
			entry.ImagesSucceeded, entry.ImagesFailed, status, string(failedImagesJSON), nextRetry, nextRetry, entry.Source, entry.Source, existingID,
		)
		if err != nil {
			logAction.SetError("DB: UPDATE DownloadHistory failed", err.Error(), map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	} else {
		_, err = tx.ExecContext(ctx, `
INSERT INTO DownloadHistory (
  tmdb_id, library_title, edition, media_item_title, media_item_year,
  set_id, set_title, set_creator, selected_types, warning_count,
  images_succeeded, images_failed, status, failed_images, next_retry_at, source
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CASE WHEN ? IS NULL THEN NULL ELSE datetime('now', ?) END, ?);
`,
			entry.TMDB_ID, entry.LibraryTitle, entry.Edition, entry.MediaItemTitle, entry.MediaItemYear,
			entry.SetID, entry.SetTitle, entry.SetCreator, string(selectedTypesJSON), entry.WarningCount,
			entry.ImagesSucceeded, entry.ImagesFailed, status, string(failedImagesJSON), nextRetry, nextRetry, entry.Source,
		)
		if err != nil {
			logAction.SetError("DB: INSERT DownloadHistory failed", err.Error(), map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	}

	if err := tx.Commit(); err != nil {
		logAction.SetError("DB: TX COMMIT failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

func (s *SQliteDB) GetDownloadHistory(ctx context.Context, limit, offset int) (entries []DownloadHistoryEntry, total int, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get Download History", logging.LevelDebug)
	defer logAction.Complete()

	entries = []DownloadHistoryEntry{}

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return entries, 0, *logAction.Error
	}

	if err := s.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM DownloadHistory;`).Scan(&total); err != nil {
		logAction.SetError("DB: COUNT DownloadHistory failed", err.Error(), map[string]any{"error": err.Error()})
		return entries, 0, *logAction.Error
	}

	q := `SELECT ` + downloadHistoryColumns + `
FROM DownloadHistory
ORDER BY created_at DESC
LIMIT ? OFFSET ?;
`
	rows, err := s.conn.QueryContext(ctx, q, limit, offset)
	if err != nil {
		logAction.SetError("DB: SELECT DownloadHistory failed", err.Error(), map[string]any{"error": err.Error()})
		return entries, 0, *logAction.Error
	}
	defer rows.Close()

	for rows.Next() {
		entry, err := scanDownloadHistoryEntry(rows)
		if err != nil {
			logAction.SetError("DB: scan DownloadHistory failed", err.Error(), map[string]any{"error": err.Error()})
			return entries, 0, *logAction.Error
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("DB: iterate DownloadHistory rows failed", err.Error(), map[string]any{"error": err.Error()})
		return entries, 0, *logAction.Error
	}

	logAction.AppendResult("count", len(entries))
	return entries, total, logging.LogErrorInfo{}
}

// GetDownloadHistoryEntry returns one Download History entry by ID
func (s *SQliteDB) GetDownloadHistoryEntry(ctx context.Context, id int64) (entry DownloadHistoryEntry, found bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Get Download History Entry %d", id), logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return entry, false, *logAction.Error
	}

	row := s.conn.QueryRowContext(ctx, `SELECT `+downloadHistoryColumns+` FROM DownloadHistory WHERE id = ?;`, id)
	entry, err := scanDownloadHistoryEntry(row)
	if err == sql.ErrNoRows {
		return entry, false, logging.LogErrorInfo{}
	}
	if err != nil {
		logAction.SetError("DB: SELECT DownloadHistory entry failed", err.Error(), map[string]any{"error": err.Error()})
		return entry, false, *logAction.Error
	}
	return entry, true, logging.LogErrorInfo{}
}

const downloadHistoryColumns = `id, tmdb_id, library_title, edition, media_item_title, media_item_year,
       set_id, set_title, set_creator, selected_types, warning_count,
       images_succeeded, images_failed, status, failed_images, retry_count, next_retry_at, created_at, source`

// scanDownloadHistoryEntry reads one row of downloadHistoryColumns
func scanDownloadHistoryEntry(row interface{ Scan(dest ...any) error }) (entry DownloadHistoryEntry, err error) {
	var failedImagesJSON, selectedTypesJSON string
	var nextRetryAt sql.NullTime
	if err := row.Scan(
		&entry.ID, &entry.TMDB_ID, &entry.LibraryTitle, &entry.Edition, &entry.MediaItemTitle, &entry.MediaItemYear,
		&entry.SetID, &entry.SetTitle, &entry.SetCreator, &selectedTypesJSON, &entry.WarningCount,
		&entry.ImagesSucceeded, &entry.ImagesFailed, &entry.Status, &failedImagesJSON, &entry.RetryCount, &nextRetryAt, &entry.CreatedAt, &entry.Source,
	); err != nil {
		return entry, err
	}
	if nextRetryAt.Valid {
		entry.NextRetryAt = &nextRetryAt.Time
	}
	_ = json.Unmarshal([]byte(failedImagesJSON), &entry.FailedImages)
	_ = json.Unmarshal([]byte(selectedTypesJSON), &entry.SelectedTypes)
	return entry, nil
}

// GetDueDownloadRetries returns the failed downloads due to be retried automatically
func (s *SQliteDB) GetDueDownloadRetries(ctx context.Context) (entries []DownloadHistoryEntry, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get Due Download Retries", logging.LevelTrace)
	defer logAction.Complete()

	entries = []DownloadHistoryEntry{}
	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return entries, *logAction.Error
	}

	rows, err := s.conn.QueryContext(ctx, `SELECT `+downloadHistoryColumns+`
FROM DownloadHistory
WHERE set_id != '' AND next_retry_at IS NOT NULL AND next_retry_at <= datetime('now')
ORDER BY next_retry_at ASC;`)
	if err != nil {
		logAction.SetError("DB: SELECT due DownloadHistory retries failed", err.Error(), map[string]any{"error": err.Error()})
		return entries, *logAction.Error
	}
	defer rows.Close()
	for rows.Next() {
		entry, err := scanDownloadHistoryEntry(rows)
		if err != nil {
			logAction.SetError("DB: scan DownloadHistory failed", err.Error(), map[string]any{"error": err.Error()})
			return entries, *logAction.Error
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("DB: iterate DownloadHistory rows failed", err.Error(), map[string]any{"error": err.Error()})
		return entries, *logAction.Error
	}
	return entries, logging.LogErrorInfo{}
}

// MarkDownloadHistoryRetried counts a retry of a failed download, it isn't retried automatically until it fails again
func (s *SQliteDB) MarkDownloadHistoryRetried(ctx context.Context, id int64) (Err logging.LogErrorInfo) {
	return s.execDownloadHistory(ctx, fmt.Sprintf("Mark Download History Entry %d Retried", id),
		`UPDATE DownloadHistory SET retry_count = retry_count + 1, next_retry_at = NULL WHERE id = ?;`, id)
}

// StopDownloadRetries stops the automatic retries of a failed download that can't be retried
func (s *SQliteDB) StopDownloadRetries(ctx context.Context, id int64) (Err logging.LogErrorInfo) {
	return s.execDownloadHistory(ctx, fmt.Sprintf("Stop Download History Entry %d Retries", id),
		`UPDATE DownloadHistory SET next_retry_at = NULL WHERE id = ?;`, id)
}

func (s *SQliteDB) execDownloadHistory(ctx context.Context, name, query string, args ...any) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, name, logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}
	if _, err := s.conn.ExecContext(ctx, query, args...); err != nil {
		logAction.SetError("DB: UPDATE DownloadHistory failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}

func (s *SQliteDB) DeleteDownloadHistoryEntry(ctx context.Context, id int64) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Delete Download History Entry %d", id), logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	_, err := s.conn.ExecContext(ctx, `DELETE FROM DownloadHistory WHERE id = ?;`, id)
	if err != nil {
		logAction.SetError("DB: DELETE DownloadHistory failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

func (s *SQliteDB) DeleteAllDownloadHistory(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Delete All Download History", logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	_, err := s.conn.ExecContext(ctx, `DELETE FROM DownloadHistory;`)
	if err != nil {
		logAction.SetError("DB: DELETE ALL DownloadHistory failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

// DeleteDownloadHistoryForSet removes an item's failures for a set that downloaded without any, and its failures before any set was downloaded
func (s *SQliteDB) DeleteDownloadHistoryForSet(ctx context.Context, tmdbID, libraryTitle, edition, setID string) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Delete Download History for Set %s", setID), logging.LevelDebug)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	result, err := s.conn.ExecContext(ctx, `
DELETE FROM DownloadHistory
WHERE tmdb_id = ? AND library_title = ? AND edition = ? AND (set_id = ? OR set_id = '');
`, tmdbID, libraryTitle, edition, setID)
	if err != nil {
		logAction.SetError("DB: DELETE DownloadHistory for a set failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	if deleted, err := result.RowsAffected(); err == nil {
		logAction.AppendResult("deleted", deleted)
	}
	return logging.LogErrorInfo{}
}
