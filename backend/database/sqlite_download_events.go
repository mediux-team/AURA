package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// downloadEventsTablesSQL creates the tables for the statistics. Used by new installs and the v14 to v15 migration.
const downloadEventsTablesSQL = `
CREATE TABLE IF NOT EXISTS DownloadEvents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tmdb_id TEXT NOT NULL,
    library_title TEXT NOT NULL,
    edition TEXT NOT NULL DEFAULT '',
    media_item_title TEXT NOT NULL DEFAULT '',
    media_item_type TEXT NOT NULL DEFAULT '',
    set_id TEXT NOT NULL DEFAULT '',
    set_title TEXT NOT NULL DEFAULT '',
    set_type TEXT NOT NULL DEFAULT '',
    set_creator TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('download','update','reapply','saved')),
    images_succeeded INTEGER NOT NULL DEFAULT 0,
    images_failed INTEGER NOT NULL DEFAULT 0,
    image_types TEXT NOT NULL DEFAULT '{}',
    bytes INTEGER NOT NULL DEFAULT 0,
    estimated INTEGER NOT NULL DEFAULT 0 CHECK (estimated IN (0,1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_downloadevents_created ON DownloadEvents(created_at);
CREATE INDEX IF NOT EXISTS idx_downloadevents_item_set ON DownloadEvents(tmdb_id, library_title, edition, set_id);

-- One row a day of numbers that can't be worked out again later, as JSON so new numbers don't need a migration
CREATE TABLE IF NOT EXISTS StatisticsSnapshots (
    snapshot_date TEXT PRIMARY KEY,
    data TEXT NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

func v15_CreateStatisticsTables(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating Statistics Tables", logging.LevelTrace)
	defer logAction.Complete()

	if _, err := conn.ExecContext(ctx, downloadEventsTablesSQL); err != nil {
		logAction.SetError("Failed to create the statistics tables", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}

// CreateStatisticsTables creates the statistics tables, for the v14 to v15 migration
func CreateStatisticsTables(ctx context.Context) (Err logging.LogErrorInfo) {
	conn, _, Err := GetDBConnection(ctx)
	if Err.Message != "" {
		return Err
	}
	return v15_CreateStatisticsTables(ctx, conn)
}

func (s *SQliteDB) InsertDownloadEvent(ctx context.Context, event models.DownloadEvent) (Err logging.LogErrorInfo) {
	if s == nil || s.conn == nil {
		return logging.LogErrorInfo{Message: "DB: connection is nil"}
	}
	return insertDownloadEvent(ctx, s.conn, event)
}

// dbExec is a connection or a transaction
type dbExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertDownloadEvent(ctx context.Context, conn dbExec, event models.DownloadEvent) logging.LogErrorInfo {
	if event.ImageTypes == nil {
		event.ImageTypes = map[string]int{}
	}
	imageTypes, err := json.Marshal(event.ImageTypes)
	if err != nil {
		return logging.LogErrorInfo{Message: "Failed to marshal the download event image types", Detail: map[string]any{"error": err.Error()}}
	}
	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	if _, err := conn.ExecContext(ctx, `
INSERT INTO DownloadEvents (
    tmdb_id, library_title, edition, media_item_title, media_item_type,
    set_id, set_title, set_type, set_creator, source, kind,
    images_succeeded, images_failed, image_types, bytes, estimated, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		event.TMDB_ID, event.LibraryTitle, event.Edition, event.MediaItemTitle, event.MediaItemType,
		event.SetID, event.SetTitle, event.SetType, event.SetCreator, event.Source, event.Kind,
		event.ImagesSucceeded, event.ImagesFailed, string(imageTypes), event.Bytes, boolToInt(event.Estimated),
		createdAt.UTC().Format("2006-01-02 15:04:05")); err != nil {
		return logging.LogErrorInfo{Message: "Failed to save the download event", Detail: map[string]any{"error": err.Error()}}
	}
	return logging.LogErrorInfo{}
}

// BackfillDownloadEvents counts each saved set as one download, with the images of its selected types the item has.
// They're dated by when the set was last downloaded, and marked as estimated.
func BackfillDownloadEvents(ctx context.Context) (events int, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Counting Downloads from the Saved Sets", logging.LevelInfo)
	defer logAction.Complete()

	conn, _, Err := GetDBConnection(ctx)
	if Err.Message != "" {
		return 0, Err
	}
	saved, Err := GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
	if Err.Message != "" {
		return 0, Err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to start the transaction", err.Error(), nil)
		return 0, *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	for _, item := range saved.Items {
		for _, set := range item.PosterSets {
			event := models.NewDownloadEvent(item.MediaItem, set.PosterSet, models.SourceBackfill, models.DownloadKindDownload)
			event.Estimated = true
			event.CreatedAt = set.LastDownloaded
			if event.CreatedAt.IsZero() {
				event.CreatedAt = set.DateUpdated
			}
			for _, image := range set.Images {
				if models.ImageIsSelected(item.MediaItem, set.SelectedTypes, image) {
					event.AddImage(image, true)
				}
			}
			if !event.Attempted() {
				event.Kind = models.DownloadKindSaved
			}
			if Err := insertDownloadEvent(ctx, tx, event); Err.Message != "" {
				logAction.SetErrorFromInfo(Err)
				return 0, *logAction.Error
			}
			events++
		}
	}
	if err := tx.Commit(); err != nil {
		logAction.SetError("Failed to save the counted downloads", err.Error(), nil)
		return 0, *logAction.Error
	}
	logAction.AppendResult("events", events)
	return events, logging.LogErrorInfo{}
}

// RecordDownloadEvent saves a download event. A failure is only logged, as the download itself worked.
func RecordDownloadEvent(ctx context.Context, event models.DownloadEvent) {
	if !event.Attempted() && event.Kind != models.DownloadKindSaved {
		return
	}
	if Err := InsertDownloadEvent(ctx, event); Err.Message != "" {
		logging.LOGGER.Warn().Timestamp().Str("item", event.MediaItemTitle).Str("set_id", event.SetID).Str("error", Err.Message).
			Msgf("Failed to record the download of %s for the statistics", event.SetTitle)
	}
}
