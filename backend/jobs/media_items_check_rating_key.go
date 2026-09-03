package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"context"
)

func StartCheckForMediaItemChangesJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if checkForMediaItemChangesJobID != 0 {
		c.Remove(checkForMediaItemChangesJobID)
		delete(jobSpecs, checkForMediaItemChangesJobID)
		checkForMediaItemChangesJobID = 0
	}

	jobSetting := config.Current.Jobs.CheckForMediaItemChanges
	if !jobSetting.IsEnabled() {
		logging.LOGGER.Info().Timestamp().Msg("Check for Media Item Changes Job is disabled in config")
		return nil
	}

	spec := jobSetting.Cron
	if spec == "" {
		spec = config.DefaultJobsConfig().CheckForMediaItemChanges.Cron
	}

	var err error
	checkForMediaItemChangesJobID, err = c.AddFunc(spec, func() {
		RunCheckForMediaItemChangesJob()
	})
	if err != nil {
		return err
	}
	jobSpecs[checkForMediaItemChangesJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Check for Media Item Changes Job Started")
	return nil
}

func RunCheckForMediaItemChangesJob() {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled CheckForMediaItemChangesJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
	action := ld.AddAction("Check for Media Item Changes", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	Err := mediaserver.CheckForMediaItemChanges(ctx)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Msg("Error running Check for Media Item Changes Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Msg("Check for Media Item Changes Job Completed")
	}
	ld.Log()
}

func RunCheckForMediaItemChangesJobNow() {
	RunCheckForMediaItemChangesJob()
}
