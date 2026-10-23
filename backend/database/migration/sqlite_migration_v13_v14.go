package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_13_to_14 adds the Collections tables, so the main media server's collections and their movies are saved like its items
func migrate_13_to_14(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v13 to v14", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 13).Int("To Version", 14).Msg("Starting database migration")

	backupErr := database.Backup(ctx, 13, 14)
	if backupErr.Message != "" {
		return backupErr
	}

	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	if _, err := conn.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS Collections (
    rating_key TEXT PRIMARY KEY,
    library_title TEXT NOT NULL,
    collection_index TEXT NOT NULL DEFAULT '',
    tmdb_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    child_count INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS CollectionItems (
    collection_rating_key TEXT NOT NULL,
    rating_key TEXT NOT NULL,
    tmdb_id TEXT NOT NULL DEFAULT '',
    library_title TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    year INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (collection_rating_key, rating_key)
);
CREATE INDEX IF NOT EXISTS idx_collectionitems_tmdb_id ON CollectionItems(tmdb_id);`); err != nil {
		logAction.SetError("Failed to create the Collections tables", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v13.0 to v14.0 completed successfully")
	return logging.LogErrorInfo{}
}
