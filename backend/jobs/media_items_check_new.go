package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"context"
)

func runCheckForNewMediaItemsJobBody(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in CheckForNewMediaItemsJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(ctx, "Cron Job")
	action := ld.AddAction("Check for New Media Items", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	_, Err := mediaserver.CheckForNewMediaItems(ctx)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Str("next_run", nextRunString(JobKeyCheckForNewMediaItems)).
			Msg("Error running Check for New Media Items Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Str("next_run", nextRunString(JobKeyCheckForNewMediaItems)).
			Msg("Check for New Media Items Job Completed")
	}
	ld.Log()
}

func StartCheckForNewMediaItemsJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	removeScheduledJob(JobKeyCheckForNewMediaItems)

	enabled, spec := config.Current.Jobs.CheckForNewMediaItems.Resolve(config.JobDefaults.CheckForNewMediaItems)
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("Check for New Media Items Job Stopped")
		return nil
	}

	if err := addScheduledJob(JobKeyCheckForNewMediaItems, spec, func() {
		runCheckForNewMediaItemsJobBody(context.Background())
	}); err != nil {
		return err
	}

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Check for New Media Items Job Started")
	return nil
}

func RunCheckForNewMediaItemsJobNow() {
	go func() {
		defer logging.RecoverPanic("Check for New Media Items Job")
		recordManualRun(JobKeyCheckForNewMediaItems)
		runCheckForNewMediaItemsJobBody(context.Background())
	}()
}
