package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"context"
)

func runRefreshMediuxUsersJobBody(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in RefreshMediuxUsersJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(ctx, "Cron Job")
	action := ld.AddAction("Refresh Mediux Users", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	_, Err := mediux.GetAllUsers(ctx)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Str("next_run", nextRunString(JobKeyRefreshMediuxUsers)).
			Msg("Error running Refresh Mediux Users Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Str("next_run", nextRunString(JobKeyRefreshMediuxUsers)).
			Msg("Refresh Mediux Users Job Completed")
	}
	ld.Log()
}

func StartRefreshMediuxUsersJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	removeScheduledJob(JobKeyRefreshMediuxUsers)

	enabled, spec := config.Current.Jobs.RefreshMediuxUsers.Resolve(config.JobDefaults.RefreshMediuxUsers)
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("Refresh Mediux Users Job Stopped")
		return nil
	}

	if err := addScheduledJob(JobKeyRefreshMediuxUsers, spec, func() {
		runRefreshMediuxUsersJobBody(context.Background())
	}); err != nil {
		return err
	}

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Refresh Mediux Users Job Started")
	return nil
}

func RunRefreshMediuxUsersJobNow() {
	go func() {
		defer logging.RecoverPanic("Refresh MediUX Users Job")
		recordManualRun(JobKeyRefreshMediuxUsers)
		runRefreshMediuxUsersJobBody(context.Background())
	}()
}
