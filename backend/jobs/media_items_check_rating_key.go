package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"context"
)

func runCheckForMediaItemChangesJobBody(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in CheckForMediaItemChangesJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(ctx, "Cron Job")
	action := ld.AddAction("Check for Media Item Changes", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	Err := mediaserver.CheckForMediaItemChanges(ctx)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Str("next_run", nextRunString(JobKeyCheckForMediaItemChanges)).
			Msg("Error running Check for Media Item Changes Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Str("next_run", nextRunString(JobKeyCheckForMediaItemChanges)).
			Msg("Check for Media Item Changes Job Completed")
	}
	ld.Log()
}

func StartCheckForMediaItemChangesJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	removeScheduledJob(JobKeyCheckForMediaItemChanges)

	enabled, spec := config.Current.Jobs.CheckForMediaItemChanges.Resolve(config.JobDefaults.CheckForMediaItemChanges)
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("Check for Media Item Changes Job Stopped")
		return nil
	}

	if err := addScheduledJob(JobKeyCheckForMediaItemChanges, spec, func() {
		runCheckForMediaItemChangesJobBody(context.Background())
	}); err != nil {
		return err
	}

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Check for Media Item Changes Job Started")
	return nil
}

func RunCheckForMediaItemChangesJobNow() {
	go func() {
		defer logging.RecoverPanic("Check for Media Item Changes Job")
		recordManualRun(JobKeyCheckForMediaItemChanges)
		runCheckForMediaItemChangesJobBody(context.Background())
	}()
}
