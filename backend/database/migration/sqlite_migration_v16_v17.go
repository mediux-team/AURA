package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_16_to_17 points the Movies and Series foreign keys at MediaItems again, as the v5 to v6 migration moved them to MediaItems_old
func migrate_16_to_17(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v16 to v17", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 16).Int("To Version", 17).Msg("Starting database migration")

	backupErr := database.Backup(ctx, 16, 17)
	if backupErr.Message != "" {
		return backupErr
	}

	if Err := database.RebuildMovieAndSeriesTables(ctx); Err.Message != "" {
		return Err
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v16.0 to v17.0 completed successfully")
	return logging.LogErrorInfo{}
}
