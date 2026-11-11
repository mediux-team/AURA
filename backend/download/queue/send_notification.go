package downloadqueue

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/notification"
	"aura/utils"
	"context"
	"fmt"
)

// SendNotification sends the result of a Download Queue set, sets from other sources use their own notification.
// downloaded is the set's images that downloaded.
func SendNotification(
	fileIssues FileIssues,
	mediaItem models.MediaItem,
	posterSet models.DBPosterSetDetail,
	tmdbPoster string,
	tmdbBackdrop string,
	source string,
	downloaded []models.ImageFile,
) {
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

	fromFavoriteCreator := source == models.SourceFavoriteCreatorsNewItem || source == models.SourceFavoriteCreatorsAllItems
	template := config.Current.Notifications.NotificationTemplate.DownloadQueue
	switch {
	case fromFavoriteCreator:
		template = config.Current.Notifications.NotificationTemplate.NewItemAutoDownload
	case source == models.SourceSiloPlugin:
		template = config.Current.Notifications.NotificationTemplate.SiloPlugin
	case source == models.SourceAutoDownload:
		template = config.Current.Notifications.NotificationTemplate.Autodownload
	case source == models.SourceSonarrWebhook:
		template = config.Current.Notifications.NotificationTemplate.SonarrNotification
	case source == models.SourcePlexRefresh:
		// Images applied again after Plex refreshed an item don't send a notification
		return
	}

	// If this notification is disabled, skip
	if !template.Enabled {
		logging.LOGGER.Debug().Timestamp().Msg("Download queue notification is disabled, skipping the notification")
		return
	}

	if posterSet.ID == "" {
		posterSet.ID = "Unknown Set ID"
	}
	if mediaItem.Title == "" {
		mediaItem.Title = "Unknown Title"
	}
	if mediaItem.LibraryTitle == "" {
		mediaItem.LibraryTitle = "Unknown Library"
	}
	if mediaItem.TMDB_ID == "" {
		mediaItem.TMDB_ID = "Unknown TMDB ID"
	}
	if mediaItem.RatingKey == "" {
		mediaItem.RatingKey = "Unknown RatingKey"
	}
	if mediaItem.Type == "" {
		mediaItem.Type = "Unknown Type"
	}

	vars := utils.TemplateVars_DownloadQueue(mediaItem, posterSet, fileIssues.Errors, fileIssues.Warnings)
	switch {
	case fromFavoriteCreator:
		vars = utils.TemplateVars_NewItemAutoDownload(mediaItem, posterSet, fileIssues.Errors, fileIssues.Warnings, source == models.SourceFavoriteCreatorsNewItem)
	case source == models.SourceSiloPlugin:
		vars = utils.TemplateVars_SiloPlugin(mediaItem, posterSet, fileIssues.Errors, fileIssues.Warnings)
	case source == models.SourceAutoDownload:
		vars = utils.TemplateVars_AutodownloadSet(mediaItem, posterSet, downloaded, fileIssues.Errors, fileIssues.Warnings)
	case source == models.SourceSonarrWebhook:
		vars = utils.TemplateVars_SonarrSet(mediaItem, posterSet, downloaded, fileIssues.Errors, fileIssues.Warnings)
	}
	// What added the set, for the Download Queue notification
	vars["Source"] = models.SourceLabel(source)
	title := utils.RenderTemplate(template.Title, vars)
	message := utils.RenderTemplate(template.Message, vars)
	imageURL := ""
	if template.IncludeImage {
		imageURL = getImageURLFromPosterSet(posterSet, tmdbPoster, tmdbBackdrop)
		// An update shows the first image it downloaded
		if (source == models.SourceAutoDownload || source == models.SourceSonarrWebhook) && len(downloaded) > 0 {
			imageURL = fmt.Sprintf("%s/%s?v=%s&key=jpg", "https://images.mediux.io/assets", downloaded[0].ID, downloaded[0].Modified.Format("20060102150405"))
		}
	}

	ctx, ld := logging.CreateLoggingContext(context.Background(), "Notification - Send Download Queue Update")
	logAction := ld.AddAction("Sending Download Queue Notification", logging.LevelInfo)
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

func getImageURLFromPosterSet(posterSet models.DBPosterSetDetail, tmdbPoster, tmdbBackdrop string) string {
	item_tmdb_id := ""
	posterURL := ""
	backdropURL := ""
	seasonURL := ""
	titlecardURL := ""
	tmdbPosterURL := tmdbPoster
	tmdbBackdropURL := tmdbBackdrop

	for _, img := range posterSet.Images {
		switch img.Type {
		case "poster":
			posterURL = mediux.GetImageURLFromSrc(img.Src)
			if item_tmdb_id == "" {
				item_tmdb_id = img.ItemTMDB_ID
			}
		case "backdrop":
			backdropURL = mediux.GetImageURLFromSrc(img.Src)
			if item_tmdb_id == "" {
				item_tmdb_id = img.ItemTMDB_ID
			}
		case "seasonPoster":
			if seasonURL == "" {
				seasonURL = mediux.GetImageURLFromSrc(img.Src)
				if item_tmdb_id == "" {
					item_tmdb_id = img.ItemTMDB_ID
				}
			}
		case "titlecard":
			if titlecardURL == "" {
				titlecardURL = mediux.GetImageURLFromSrc(img.Src)
				if item_tmdb_id == "" {
					item_tmdb_id = img.ItemTMDB_ID
				}
			}
		}
	}

	if item_tmdb_id != "" && tmdbPoster == "" && tmdbBackdrop == "" {
		itemType := posterSet.Type
		if posterSet.Type == "collection" {
			itemType = "movie"
		}

		itemInfo, Err := mediux.GetBaseItemInfoByTMDB_ID(item_tmdb_id, itemType)
		if Err.Message != "" {
			tmdbPosterURL = itemInfo.TMDB_PosterPath
			tmdbBackdropURL = itemInfo.TMDB_BackdropPath
		}
	}

	// Single-image fallback order:
	// poster -> tmdb poster -> backdrop -> tmdb backdrop -> season -> titlecard
	if posterURL != "" {
		return posterURL
	}
	if tmdbPosterURL != "" {
		return fmt.Sprintf("https://image.tmdb.org/t/p/original%s", tmdbPosterURL)
	}
	if backdropURL != "" {
		return backdropURL
	}
	if tmdbBackdropURL != "" {
		return fmt.Sprintf("https://image.tmdb.org/t/p/original%s", tmdbBackdropURL)
	}
	if seasonURL != "" {
		return seasonURL
	}
	if titlecardURL != "" {
		return titlecardURL
	}
	return ""
}
