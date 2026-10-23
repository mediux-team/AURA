package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"context"
)

func runRefreshMediaItemsAndCollectionsJobBody(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in RefreshMediaItemsAndCollectionsJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(ctx, "Cron Job")
	action := ld.AddAction("Refresh Media Items and Collections", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	mediaserver.GetAllLibrarySectionsAndItems(ctx, true)
	ld.Log()
}

func StartRefreshMediaItemsAndCollectionsJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	removeScheduledJob(JobKeyRefreshMediaItemsAndCollections)

	enabled, spec := config.Current.Jobs.RefreshMediaItemsAndCollections.Resolve(config.JobDefaults.RefreshMediaItemsAndCollections)
	if !enabled {
		logging.LOGGER.Info().Timestamp().Msg("Refresh Media Items and Collections Job Stopped")
		return nil
	}

	if err := addScheduledJob(JobKeyRefreshMediaItemsAndCollections, spec, func() {
		runRefreshMediaItemsAndCollectionsJobBody(context.Background())
	}); err != nil {
		return err
	}

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Refresh Media Items and Collections Job Started")
	return nil
}

func RunRefreshMediaItemsAndCollectionsJobNow() {
	go func() {
		defer logging.RecoverPanic("Refresh Media Items and Collections Job")
		recordManualRun(JobKeyRefreshMediaItemsAndCollections)
		runRefreshMediaItemsAndCollectionsJobBody(context.Background())
	}()
}
