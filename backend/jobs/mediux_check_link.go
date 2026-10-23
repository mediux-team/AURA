package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"context"
)

func runCheckMediuxSiteLinkJobBody(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in CheckMediuxSiteLinkJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(ctx, "Cron Job")
	action := ld.AddAction("Check Mediux Site Link Availability", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	mediux.CheckSiteLinkAvailability()
	ld.Log()
}

func StartCheckMediuxSiteLinkJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	removeScheduledJob(JobKeyCheckMediuxSiteLink)

	enabled, spec := config.Current.Jobs.CheckMediuxSiteLink.Resolve(config.JobDefaults.CheckMediuxSiteLink)
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("Check Mediux Site Link Availability Job Stopped")
		return nil
	}

	if err := addScheduledJob(JobKeyCheckMediuxSiteLink, spec, func() {
		runCheckMediuxSiteLinkJobBody(context.Background())
	}); err != nil {
		return err
	}

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Check Mediux Site Link Availability Job Started")
	return nil
}

func RunCheckMediuxSiteLinkJobNow() {
	go func() {
		defer logging.RecoverPanic("Check MediUX Site Link Job")
		recordManualRun(JobKeyCheckMediuxSiteLink)
		runCheckMediuxSiteLinkJobBody(context.Background())
	}()
}
