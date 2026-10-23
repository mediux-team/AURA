package jobs

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"context"
)

func runBackupDatabaseJobBody(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in BackupDatabaseJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(ctx, "Cron Job")
	action := ld.AddAction("Backup Database", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	backup, Err := database.CreateBackup(ctx, config.Current.Backups.Keep)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Str("next_run", nextRunString(JobKeyBackupDatabase)).
			Msg("Error running Backup Database Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Str("backup", backup.Name).
			Str("next_run", nextRunString(JobKeyBackupDatabase)).
			Msg("Backup Database Job Completed")
	}
	ld.Log()
}

func StartBackupDatabaseJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	removeScheduledJob(JobKeyBackupDatabase)

	enabled, spec := config.Current.Jobs.BackupDatabase.Resolve(config.JobDefaults.BackupDatabase)
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("Backup Database Job Stopped")
		return nil
	}

	if err := addScheduledJob(JobKeyBackupDatabase, spec, func() {
		runBackupDatabaseJobBody(context.Background())
	}); err != nil {
		return err
	}

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Backup Database Job Started")
	return nil
}

func RunBackupDatabaseJobNow() {
	go func() {
		defer logging.RecoverPanic("Backup Database Job")
		recordManualRun(JobKeyBackupDatabase)
		runBackupDatabaseJobBody(context.Background())
	}()
}
