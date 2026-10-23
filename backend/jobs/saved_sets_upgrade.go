package jobs

import (
	"aura/config"
	autodownload "aura/download/auto"
	"aura/logging"
	"context"
)

func runUpgradeSavedSetsJobBody(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in UpgradeSavedSetsJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(ctx, "Cron Job")
	action := ld.AddAction("Upgrade Saved Sets", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	_, Err := autodownload.UpgradeSavedSets(ctx)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Str("next_run", nextRunString(JobKeyUpgradeSavedSets)).
			Msg("Error running Upgrade Saved Sets Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Str("next_run", nextRunString(JobKeyUpgradeSavedSets)).
			Msg("Upgrade Saved Sets Job Completed")
	}
	ld.Log()
}

func StartUpgradeSavedSetsJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	removeScheduledJob(JobKeyUpgradeSavedSets)

	enabled, spec := config.Current.Jobs.UpgradeSavedSets.Resolve(config.JobDefaults.UpgradeSavedSets)
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("Upgrade Saved Sets Job Stopped")
		return nil
	}

	if err := addScheduledJob(JobKeyUpgradeSavedSets, spec, func() {
		runUpgradeSavedSetsJobBody(context.Background())
	}); err != nil {
		return err
	}

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Upgrade Saved Sets Job Started")
	return nil
}

func RunUpgradeSavedSetsJobNow() {
	go func() {
		defer logging.RecoverPanic("Upgrade Saved Sets Job")
		recordManualRun(JobKeyUpgradeSavedSets)
		runUpgradeSavedSetsJobBody(context.Background())
	}()
}
