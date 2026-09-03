package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"context"
)

func StartCheckMediuxSiteLinkJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if checkMediuxSiteLinkJobID != 0 {
		c.Remove(checkMediuxSiteLinkJobID)
		delete(jobSpecs, checkMediuxSiteLinkJobID)
		checkMediuxSiteLinkJobID = 0
	}

	jobSetting := config.Current.Jobs.CheckMediuxSiteLink
	if !jobSetting.IsEnabled() {
		logging.LOGGER.Info().Timestamp().Msg("Check Mediux Site Link Availability Job is disabled in config")
		return nil
	}

	spec := jobSetting.Cron
	if spec == "" {
		spec = config.DefaultJobsConfig().CheckMediuxSiteLink.Cron
	}

	var err error
	checkMediuxSiteLinkJobID, err = c.AddFunc(spec, func() {
		RunCheckMediuxSiteLinkJob()
	})
	if err != nil {
		return err
	}
	jobSpecs[checkMediuxSiteLinkJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Check Mediux Site Link Availability Job Started")
	return nil
}

func RunCheckMediuxSiteLinkJob() {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled CheckMediuxSiteLinkJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
	action := ld.AddAction("Check Mediux Site Link Availability", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	mediux.CheckSiteLinkAvailability()
	ld.Log()
}

func RunCheckMediuxSiteLinkJobNow() {
	RunCheckMediuxSiteLinkJob()
}
