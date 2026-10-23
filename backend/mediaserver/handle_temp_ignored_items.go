package mediaserver

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/notification"
	"aura/utils"
	"context"
	"fmt"
)

func HandleTempIgnoredItems(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Handling Temp Ignored Items", logging.LevelInfo)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Get all temp ignored items from the database
	tempIgnoredItems, dbErr := database.GetTempIgnoredItems(ctx)
	if dbErr.Message != "" {
		return dbErr
	}

	for _, mediaItem := range tempIgnoredItems {
		numOfSets := 0
		var mainImage models.ImageFile
		switch mediaItem.Type {
		case "movie":
			setItems := map[string]models.IncludedItem{}
			// For Movies, we get Movie Sets and Movie Collection Sets
			movieSets, _ := mediux.GetMovieItemSets(ctx, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition, &setItems)
			collectionSets, _ := mediux.GetMovieItemCollectionSets(ctx, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition, &setItems)
			if len(movieSets) > 0 || len(collectionSets) > 0 {
				numOfSets += len(movieSets) + len(collectionSets)
				if len(movieSets) > 0 {
					mainImage = getMainImage(movieSets[0].Images)
				} else if len(collectionSets) > 0 {
					mainImage = getMainImage(collectionSets[0].Images)
				}
			}
		case "show":
			showSets, _, _ := mediux.GetShowItemSets(ctx, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition)
			if len(showSets) > 0 {
				numOfSets += len(showSets)
				mainImage = getMainImage(showSets[0].Images)
			}
		}

		// If the item has sets, we can remove it from the temp ignored items
		if numOfSets > 0 && mediaItem.IgnoredMode == "until-set-available" {
			dbErr = database.StopIgnoringMediaItem(ctx, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition)
			if dbErr.Message != "" {
				logging.LOGGER.Error().Timestamp().Str("tmdb_id", mediaItem.TMDB_ID).Str("library_title", mediaItem.LibraryTitle).Str("error", dbErr.Message).Msg("Failed to stop ignoring media item")
				continue
			}
			logging.LOGGER.Info().Timestamp().Str("tmdb_id", mediaItem.TMDB_ID).Str("library_title", mediaItem.LibraryTitle).Msgf("Stopped ignoring media item %s because %d are now available", mediaItem.Title, numOfSets)
			logging.Go("Temp Ignored Item Notification", func() { sendNotification(mediaItem, numOfSets, mainImage) })
		} else if numOfSets > 0 && mediaItem.IgnoredMode == "until-new-set-available" {
			// For "until-new-set-available" mode, we need to check if there are new sets available compared to the current sets when the item was ignored
			if numOfSets > len(mediaItem.IgnoredSets) {
				dbErr = database.StopIgnoringMediaItem(ctx, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition)
				if dbErr.Message != "" {
					logging.LOGGER.Error().Timestamp().Str("tmdb_id", mediaItem.TMDB_ID).Str("library_title", mediaItem.LibraryTitle).Str("error", dbErr.Message).Msg("Failed to stop ignoring media item")
					continue
				}
				logging.LOGGER.Info().Timestamp().Str("tmdb_id", mediaItem.TMDB_ID).Str("library_title", mediaItem.LibraryTitle).Int("current_sets", len(mediaItem.IgnoredSets)).Int("new_sets", numOfSets).Msgf("New sets are available for media item %s, there are now %d sets available compared to %d sets when the item was ignored", mediaItem.Title, numOfSets, len(mediaItem.IgnoredSets))
				logging.Go("Temp Ignored Item Notification", func() { sendNotification(mediaItem, numOfSets, mainImage) })
			}
		}
	}

	return Err
}

func sendNotification(mediaItem models.MediaItem, setCount int, mainImage models.ImageFile) {
	// If notifications are disabled, skip
	if !config.Current.Notifications.Enabled {
		logging.LOGGER.Debug().Timestamp().Msg("Notifications are disabled, skipping app start notification")
		return
	}

	// If notification providers are not configured, skip
	if len(config.Current.Notifications.Providers) == 0 {
		logging.LOGGER.Debug().Timestamp().Msg("No notification providers configured, skipping app start notification")
		return
	}

	// If new sets available for ignored items notification is disabled, skip
	if !config.Current.Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Enabled {
		logging.LOGGER.Debug().Timestamp().Msg("New sets available for ignored items notification is disabled, skipping notification")
		return
	}

	vars := utils.TemplateVars_NewSetsAvailableForIgnoredItems(mediaItem, setCount)
	title := utils.RenderTemplate(config.Current.Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Title, vars)
	message := utils.RenderTemplate(config.Current.Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Message, vars)
	imageURL := ""
	if config.Current.Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.IncludeImage {
		mediuxInfo, Err := mediux.GetBaseItemInfoByTMDB_ID(mediaItem.TMDB_ID, mediaItem.Type)
		if Err.Message != "" {
			imageURL = fmt.Sprintf("%s/%s?v=%s&key=jpg",
				"https://images.mediux.io/assets",
				mainImage.ID,
				mainImage.Modified.Format("20060102150405"),
			)
		} else {
			if mediuxInfo.TMDB_PosterPath == "" && mediuxInfo.TMDB_BackdropPath == "" {
				imageURL = fmt.Sprintf("%s/%s?v=%s&key=jpg",
					"https://images.mediux.io/assets",
					mainImage.ID,
					mainImage.Modified.Format("20060102150405"),
				)
			} else if mediuxInfo.TMDB_PosterPath != "" {
				imageURL = mediuxInfo.TMDB_PosterPath
			} else if mediuxInfo.TMDB_BackdropPath != "" {
				imageURL = mediuxInfo.TMDB_BackdropPath
			}
			if imageURL != "" {
				imageURL = fmt.Sprintf("https://image.tmdb.org/t/p/original%s", imageURL)
			}

		}
	}

	ctx, ld := logging.CreateLoggingContext(context.Background(), "Notification - Send Temp Ignored Item Update")
	logAction := ld.AddAction("Sending Temp Ignored Item Update Notification", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer ld.Log()
	defer logAction.Complete()

	// Send a notification to all configured providers
	for _, provider := range config.Current.Notifications.Providers {
		if provider.Enabled {
			switch provider.Provider {
			case "Discord":
				notification.SendDiscordMessage(
					ctx,
					provider.Discord,
					message,
					imageURL,
					title,
				)
			case "Pushover":
				notification.SendPushoverMessage(
					ctx,
					provider.Pushover,
					message,
					imageURL,
					title,
				)
			case "Gotify":
				notification.SendGotifyMessage(
					ctx,
					provider.Gotify,
					message,
					imageURL,
					title,
				)
			case "Webhook":
				notification.SendWebhookMessage(
					ctx,
					provider.Webhook,
					message,
					imageURL,
					title,
				)
			}
		}
	}
}

func getMainImage(images []models.ImageFile) models.ImageFile {
	hasPosterImage := false
	hasBackdropImage := false
	var posterImage models.ImageFile
	var backdropImage models.ImageFile
	for _, image := range images {
		switch image.Type {
		case "poster":
			hasPosterImage = true
			posterImage = image
		case "backdrop":
			hasBackdropImage = true
			backdropImage = image
		}
	}

	if hasPosterImage {
		return posterImage
	} else if hasBackdropImage {
		return backdropImage
	}
	return images[0]
}
