package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_17_to_18 adds where each failed download came from to the Download History
func migrate_17_to_18(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v17 to v18", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 17).Int("To Version", 18).Msg("Starting database migration")

	backupErr := database.Backup(ctx, 17, 18)
	if backupErr.Message != "" {
		return backupErr
	}

	exists, Err := checkColumnExists(ctx, "DownloadHistory", "source")
	if Err.Message != "" {
		return Err
	}
	if !exists {
		conn, _, Err := database.GetDBConnection(ctx)
		if Err.Message != "" {
			return Err
		}
		if _, err := conn.ExecContext(ctx, `ALTER TABLE DownloadHistory ADD COLUMN source TEXT NOT NULL DEFAULT '';`); err != nil {
			logAction.SetError("Failed to add the source to the Download History", err.Error(), nil)
			return *logAction.Error
		}
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v17.0 to v18.0 completed successfully")
	return logging.LogErrorInfo{}
}
