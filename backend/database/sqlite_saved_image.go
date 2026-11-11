package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
	"time"
)

// SaveDownloadedImage saves an image that downloaded to the item's saved set, and when the set was last downloaded.
// saved is false when the set isn't saved for the item yet, it's saved when the job finishes.
func (s *SQliteDB) SaveDownloadedImage(ctx context.Context, item models.MediaItem, setID string, image models.ImageFile) (saved bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Save the downloaded %s image of set %s", image.Type, setID), logging.LevelTrace)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return false, *logAction.Error
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("DB: TX BEGIN failed", err.Error(), map[string]any{"error": err.Error()})
		return false, *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	var posterSetRowID int64
	err = tx.QueryRowContext(ctx, `
SELECT si.poster_set_id
FROM SavedItems si
JOIN PosterSets ps ON ps.id = si.poster_set_id
WHERE si.tmdb_id = ? AND si.library_title = ? AND si.edition = ? AND ps.set_id = ?;
`, item.TMDB_ID, item.LibraryTitle, item.Edition, setID).Scan(&posterSetRowID)
	if err != nil {
		// The set isn't saved for the item yet
		return false, logging.LogErrorInfo{}
	}

	if _, err = tx.ExecContext(ctx, `
UPDATE SavedItems SET last_downloaded = ?
WHERE tmdb_id = ? AND library_title = ? AND edition = ? AND poster_set_id = ?;
`, time.Now(), item.TMDB_ID, item.LibraryTitle, item.Edition, posterSetRowID); err != nil {
		logAction.SetError("DB: UPDATE SavedItems last_downloaded failed", err.Error(), map[string]any{"error": err.Error()})
		return false, *logAction.Error
	}

	if Err := upsertImageFiles(ctx, tx, models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: setID}, Images: []models.ImageFile{image}}}, posterSetRowID, item.TMDB_ID); Err.Message != "" {
		logAction.SetError(Err.Message, "", Err.Detail)
		return false, *logAction.Error
	}

	if err := tx.Commit(); err != nil {
		logAction.SetError("DB: TX COMMIT failed", err.Error(), map[string]any{"error": err.Error()})
		return false, *logAction.Error
	}
	return true, logging.LogErrorInfo{}
}
