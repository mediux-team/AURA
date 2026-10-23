package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"context"
)

func runHandleTempIgnoredItemsJobBody(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in HandleTempIgnoredItemsJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(ctx, "Cron Job")
	action := ld.AddAction("Handle Temp Ignored Items", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	Err := mediaserver.HandleTempIgnoredItems(ctx)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Str("next_run", nextRunString(JobKeyHandleTempIgnoredItems)).
			Msg("Error running Handle Temp Ignored Items Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Str("next_run", nextRunString(JobKeyHandleTempIgnoredItems)).
			Msg("Handle Temp Ignored Items Job Completed")
	}
	ld.Log()
}

func StartHandleTempIgnoredItemsJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	removeScheduledJob(JobKeyHandleTempIgnoredItems)

	enabled, spec := config.Current.Jobs.HandleTempIgnoredItems.Resolve(config.JobDefaults.HandleTempIgnoredItems)
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("Handle Temp Ignored Items Job Stopped")
		return nil
	}

	if err := addScheduledJob(JobKeyHandleTempIgnoredItems, spec, func() {
		runHandleTempIgnoredItemsJobBody(context.Background())
	}); err != nil {
		return err
	}

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Handle Temp Ignored Items Job Started")
	return nil
}

func RunHandleTempIgnoredItemsJobNow() {
	go func() {
		defer logging.RecoverPanic("Handle Temp Ignored Items Job")
		recordManualRun(JobKeyHandleTempIgnoredItems)
		runHandleTempIgnoredItemsJobBody(context.Background())
	}()
}
