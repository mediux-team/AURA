package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_15_to_16 adds the season and episode counts of Silo shows, as Silo's library listing doesn't have them
func migrate_15_to_16(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v15 to v16", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 15).Int("To Version", 16).Msg("Starting database migration")

	backupErr := database.Backup(ctx, 15, 16)
	if backupErr.Message != "" {
		return backupErr
	}

	if Err := database.AddSiloItemCountColumns(ctx); Err.Message != "" {
		return Err
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v15.0 to v16.0 completed successfully")
	return logging.LogErrorInfo{}
}
