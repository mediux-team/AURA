package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_14_to_15 adds the statistics tables, and counts the saved sets as downloads so the statistics start with them
func migrate_14_to_15(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v14 to v15", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 14).Int("To Version", 15).Msg("Starting database migration")

	backupErr := database.Backup(ctx, 14, 15)
	if backupErr.Message != "" {
		return backupErr
	}

	if Err := database.CreateStatisticsTables(ctx); Err.Message != "" {
		return Err
	}
	events, Err := database.BackfillDownloadEvents(ctx)
	if Err.Message != "" {
		return Err
	}

	logging.LOGGER.Info().Timestamp().Int("download_events", events).Msg("Database migration v14.0 to v15.0 completed successfully")
	return logging.LogErrorInfo{}
}
