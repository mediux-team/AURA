package jobs

import (
	"aura/config"
	autodownload "aura/download/auto"
	"aura/logging"
	"context"
	"runtime/debug"
)

func runAutoDownloadJobBody(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().
				Timestamp().
				Interface("recover", r).
				Str("stack", string(debug.Stack())).
				Msg("PANIC: in AutoDownload Job")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(ctx, "Cron Job")
	action := ld.AddAction("AutoDownload Check", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	Err := autodownload.CheckAllItems(ctx)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Str("next_run", nextRunString(JobKeyAutoDownload)).
			Msg("Error running AutoDownload Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Str("next_run", nextRunString(JobKeyAutoDownload)).
			Msg("AutoDownload Job Completed")
	}
	ld.Log()
}

func StartAutoDownloadJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	removeScheduledJob(JobKeyAutoDownload)

	enabled, spec := config.Current.Jobs.AutoDownload.Resolve(config.JobDefaults.AutoDownload)
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("AutoDownload Job Stopped")
		return nil
	}

	if err := addScheduledJob(JobKeyAutoDownload, spec, func() {
		runAutoDownloadJobBody(context.Background())
	}); err != nil {
		return err
	}

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("AutoDownload Job Started")
	return nil
}

func RunAutoDownloadJobNow() {
	go func() {
		defer logging.RecoverPanic("Auto Download Job")
		recordManualRun(JobKeyAutoDownload)
		runAutoDownloadJobBody(context.Background())
	}()
}
