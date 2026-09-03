package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"context"
)

func StartRefreshMediuxUsersJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if refreshMediuxUsersJobID != 0 {
		c.Remove(refreshMediuxUsersJobID)
		delete(jobSpecs, refreshMediuxUsersJobID)
		refreshMediuxUsersJobID = 0
	}

	jobSetting := config.Current.Jobs.RefreshMediuxUsers
	if !jobSetting.IsEnabled() {
		logging.LOGGER.Info().Timestamp().Msg("Refresh Mediux Users Job is disabled in config")
		return nil
	}

	spec := jobSetting.Cron
	if spec == "" {
		spec = config.DefaultJobsConfig().RefreshMediuxUsers.Cron
	}

	var err error
	refreshMediuxUsersJobID, err = c.AddFunc(spec, func() {
		RunRefreshMediuxUsersJob()
	})
	if err != nil {
		return err
	}
	jobSpecs[refreshMediuxUsersJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Refresh Mediux Users Job Started")
	return nil
}

func RunRefreshMediuxUsersJob() {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled RefreshMediuxUsersJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
	action := ld.AddAction("Refresh Mediux Users", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	_, Err := mediux.GetAllUsers(ctx)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Msg("Error running Refresh Mediux Users Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Msg("Refresh Mediux Users Job Completed")
	}
	ld.Log()
}

func RunRefreshMediuxUsersJobNow() {
	RunRefreshMediuxUsersJob()
}
