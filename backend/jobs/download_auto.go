package jobs

import (
	"aura/config"
	autodownload "aura/download/auto"
	"aura/logging"
	"context"
	"runtime/debug"
)

func StartAutoDownloadJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if autodownloadJobID != 0 {
		c.Remove(autodownloadJobID)
		delete(jobSpecs, autodownloadJobID)
		autodownloadJobID = 0
	}

	enabled := config.Current.AutoDownload.Enabled
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("AutoDownload Job Stopped")
		return nil
	}

	spec := config.Current.AutoDownload.Cron
	if spec == "" {
		spec = "0 0 * * *" // Default to daily at midnight
	}

	var err error

	autodownloadJobID, err = c.AddFunc(spec, func() {
		RunAutoDownloadJob()
	})
	if err != nil {
		return err
	}
	jobSpecs[autodownloadJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("AutoDownload Job Started")
	return nil
}

func RunAutoDownloadJob() {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().
				Timestamp().
				Interface("recover", r).
				Str("stack", string(debug.Stack())).
				Msg("PANIC: in AutoDownload Job")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
	action := ld.AddAction("AutoDownload Check", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	Err := autodownload.CheckAllItems(ctx)
	if Err.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", Err.Message).
			Msg("Error running AutoDownload Job")
	} else {
		logging.LOGGER.Info().Timestamp().
			Msg("AutoDownload Job Completed")
	}
	ld.Log()
}

func RunAutoDownloadJobNow() {
	mu.Lock()
	defer mu.Unlock()

	go func() {
		RunAutoDownloadJob()
	}()
}
