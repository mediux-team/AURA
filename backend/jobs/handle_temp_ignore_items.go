package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"context"
)

func StartHandleTempIgnoredItemsJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if handleTempIgnoredItemsJobID != 0 {
		c.Remove(handleTempIgnoredItemsJobID)
		delete(jobSpecs, handleTempIgnoredItemsJobID)
		handleTempIgnoredItemsJobID = 0
	}

	jobSetting := config.Current.Jobs.HandleTempIgnoredItems
	if !jobSetting.IsEnabled() {
		logging.LOGGER.Info().Timestamp().Msg("Handle Temp Ignored Items Job is disabled in config")
		return nil
	}

	spec := jobSetting.Cron
	if spec == "" {
		spec = config.DefaultJobsConfig().HandleTempIgnoredItems.Cron
	}

	var err error
	handleTempIgnoredItemsJobID, err = c.AddFunc(spec, func() {
		RunHandleTempIgnoredItemsJob()
	})
	if err != nil {
		return err
	}
	jobSpecs[handleTempIgnoredItemsJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Handle Temp Ignored Items Job Started")
	return nil
}

func RunHandleTempIgnoredItemsJob() {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled HandleTempIgnoredItemsJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
	action := ld.AddAction("Handle Temp Ignored Items", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	Err := mediaserver.HandleTempIgnoredItems(ctx)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Msg("Error running Handle Temp Ignored Items Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Msg("Handle Temp Ignored Items Job Completed")
	}
	ld.Log()
}

func RunHandleTempIgnoredItemsJobNow() {
	RunHandleTempIgnoredItemsJob()
}
