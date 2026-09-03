package jobs

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"context"
)

func StartRefreshMediaItemsAndCollectionsJob() error {
	mu.Lock()
	defer mu.Unlock()

	if c == nil {
		logging.LOGGER.Error().Timestamp().Msg("Cron Jobs Scheduler is not initialized")
		return nil
	}

	if refreshMediaItemsAndCollectionsJobID != 0 {
		c.Remove(refreshMediaItemsAndCollectionsJobID)
		delete(jobSpecs, refreshMediaItemsAndCollectionsJobID)
		refreshMediaItemsAndCollectionsJobID = 0
	}

	jobSetting := config.Current.Jobs.RefreshMediaItemsAndCollections
	if !jobSetting.IsEnabled() {
		logging.LOGGER.Info().Timestamp().Msg("Refresh Media Items and Collections Job is disabled in config")
		return nil
	}

	spec := jobSetting.Cron
	if spec == "" {
		spec = config.DefaultJobsConfig().RefreshMediaItemsAndCollections.Cron
	}

	var err error
	refreshMediaItemsAndCollectionsJobID, err = c.AddFunc(spec, func() {
		RunRefreshMediaItemsAndCollectionsJob()
	})
	if err != nil {
		return err
	}
	jobSpecs[refreshMediaItemsAndCollectionsJobID] = spec

	logging.LOGGER.Info().Timestamp().
		Str("cron", spec).
		Msg("Refresh Media Items and Collections Job Started")
	return nil
}

func RunRefreshMediaItemsAndCollectionsJob() {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Msg("PANIC: in scheduled RefreshMediaItemsAndCollectionsJob")
		}
	}()
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Cron Job")
	action := ld.AddAction("Refresh Media Items and Collections", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, action)
	mediaserver.GetAllLibrarySectionsAndItems(ctx, true)
	ld.Log()
}
