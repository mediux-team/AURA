package routes_sonarr_radarr

import (
	"aura/cache"
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"aura/utils/httpx"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type SonarrWebHookOnUpgradePayload struct {
	DeletedFiles []SonarrFile    `json:"deletedFiles"`
	EpisodeFile  SonarrFile      `json:"episodeFile"`
	Episodes     []SonarrEpisode `json:"episodes"`
	EventType    string          `json:"eventType"`
	InstanceName string          `json:"instanceName"`
	IsUpgrade    bool            `json:"isUpgrade"`
	Series       SonarrSeries    `json:"series"`
}

type SonarrFile struct {
	Path         string `json:"path"`
	RelativePath string `json:"relativePath"`
}

type SonarrEpisode struct {
	EpisodeNumber int `json:"episodeNumber"`
	SeasonNumber  int `json:"seasonNumber"`
}

type SonarrSeries struct {
	Path   string `json:"path"`
	Title  string `json:"title"`
	TmdbID int    `json:"tmdbId"`
}

type mediaItemFetchInfo struct {
	FromCache bool
	FetchErr  string
	CacheHit  bool
}

// SonarrWebhookHandler godoc
// @Summary      Sonarr/Radarr Webhook
// @Description  Receives Sonarr/Radarr "On Import"/"On Upgrade" webhook events. Requires HTTP Basic Auth since Sonarr/Radarr's built-in Webhook connection type has no custom-header support - set the Username field to anything, and the Password field to your AURA API key (Settings > Auth).
// @Tags         Sonarr/Radarr
// @Accept       json
// @Security     BasicAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized - missing or invalid Basic Auth"
// @Success      200  {object}  httpx.JSONResponse
// @Router       /api/sonarr/webhook [post]
func SonarrWebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Handle Sonarr Webhook", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	// Get the Library from the URL params
	libraryTitle := r.URL.Query().Get("library")
	if libraryTitle == "" {
		logAction.SetError("Missing library parameter", "The 'library' URL parameter is required", nil)
		httpx.SendResponse(w, ld, nil)
		return
	}

	// Decode into typed struct
	var payload SonarrWebHookOnUpgradePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Run Validation on Payload to determine if we could/should proceed
	// We only want to run this when EventType is Download
	if payload.EventType != "Download" {
		logAction.AppendResult("event_type", payload.EventType)
		w.WriteHeader(http.StatusOK)
		return
	} else if payload.Series == (SonarrSeries{}) || payload.Series.TmdbID == 0 {
		logAction.AppendResult("series_info", "missing or invalid")
		w.WriteHeader(http.StatusOK)
		return
	} else if len(payload.Episodes) == 0 {
		logAction.AppendResult("episodes_info", "no episodes in payload")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Now we want to check if this TMDB ID + Library Title exists in the Aura DB
	dbFilter := models.DBFilter{
		ItemTMDB_ID:      strconv.Itoa(payload.Series.TmdbID),
		ItemLibraryTitle: libraryTitle,
	}

	db, Err := database.GetAllSavedSets(ctx, dbFilter)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, nil)
		return
	}

	if len(db.Items) == 0 {
		logAction.AppendResult("items_found", 0)
		w.WriteHeader(http.StatusOK)
		return
	} else if len(db.Items) > 1 {
		logAction.SetError("Multiple DB items found", "Multiple items matched the TMDB ID and Library Title. This should not happen.", map[string]any{
			"items_found":   len(db.Items),
			"library_title": libraryTitle,
			"tmdb_id":       payload.Series.TmdbID,
		})
		httpx.SendResponse(w, ld, nil)
		return
	}

	dbItem := db.Items[0]

	// Validate the DB Item Media Item details to ensure it has the necessary info to proceed
	if dbItem.MediaItem.TMDB_ID == "" || dbItem.MediaItem.Title == "" || dbItem.MediaItem.LibraryTitle == "" || dbItem.MediaItem.RatingKey == "" {
		logAction.SetError("DB item is missing necessary media item info", "The DB item must have a MediaItem with TMDB_ID, Title, LibraryTitle, and RatingKey", map[string]any{
			"db_item": dbItem,
		})
		httpx.SendResponse(w, ld, nil)
		return
	}

	// Validate the DB Item Poster Sets to ensure it has the necessary info to proceed
	if len(dbItem.PosterSets) == 0 {
		logAction.SetError("DB item is missing poster sets", "The DB item must have at least one poster set to proceed", map[string]any{
			"db_item": dbItem,
		})
		httpx.SendResponse(w, ld, nil)
		return
	}

	// If we made it here, we can proceed with processing the webhook event
	// We will respond to Sonarr immediately and then continue processing the event in the background since Sonarr only cares about the response status code and not the response body

	// Respond to Sonarr immediately
	w.WriteHeader(http.StatusOK)

	// Check to see if Season Poster or Titlecards are selected in any of the Poster Sets
	// If they are not, then we can skip the background processing since there is nothing to download
	seasonPostersSelected := false
	specialSeasonSelected := false
	titlecardsSelected := false
	for _, posterSet := range dbItem.PosterSets {
		if posterSet.SelectedTypes.SeasonPoster {
			seasonPostersSelected = true
		}
		if posterSet.SelectedTypes.SpecialSeasonPoster {
			specialSeasonSelected = true
		}
		if posterSet.SelectedTypes.Titlecard {
			titlecardsSelected = true
		}
	}

	if !seasonPostersSelected && !specialSeasonSelected && !titlecardsSelected {
		logAction.AppendResult("background_processing_skipped", true)
		logAction.AppendResult("reason", "No season posters, special season posters, or titlecards selected in any poster set")
		return
	}

	sonarrContainsSpecialSeason := false
	for _, episode := range payload.Episodes {
		if episode.SeasonNumber == 0 {
			sonarrContainsSpecialSeason = true
			break
		}
	}
	if !seasonPostersSelected && !titlecardsSelected && specialSeasonSelected && !sonarrContainsSpecialSeason {
		logAction.AppendResult("background_processing_skipped", true)
		logAction.AppendResult("reason", "Only special season posters are selected but no episodes in the Sonarr payload are from the special season")
		return
	}

	go func(
		dbItem models.DBSavedItem,
		payload SonarrWebHookOnUpgradePayload,
	) {
		defer logging.RecoverPanic("Sonarr Webhook")
		eventTypeStr := ""
		switch payload.IsUpgrade {
		case true:
			eventTypeStr = "Upgrade"
		case false:
			eventTypeStr = "Download"
		}

		bgCtx, bgLd := logging.CreateLoggingContext(context.Background(), "Handle Sonarr Webhook Background Task")
		bgAction := bgLd.AddAction(fmt.Sprintf("Sonarr Webhook: Processing %s for %s", eventTypeStr, utils.MediaItemInfo(dbItem.MediaItem)), logging.LevelInfo)
		bgCtx = logging.WithCurrentAction(bgCtx, bgAction)

		// Handle Panic to prevent crashing the app since this is running in the background
		defer func() {
			if r := recover(); r != nil {
				logging.LOGGER.Error().Timestamp().Msgf("PANIC: in SonarrWebhookHandler background processing: %v", r)
			}
		}()

		processSonarrDownloadEvent(bgCtx, payload, dbItem)
		bgAction.Complete()
		bgLd.Log()
	}(dbItem, payload)

}

func processSonarrDownloadEvent(ctx context.Context, payload SonarrWebHookOnUpgradePayload, dbItem models.DBSavedItem) {
	// Initial wait to give media server time to ingest new files.
	initialSleep := 10 * time.Second
	_, sleepAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Sleeping for %v to give time for media server to update", initialSleep), logging.LevelTrace)
	time.Sleep(initialSleep)
	sleepAction.Complete()

	// Get the base Show Media Item from the cache
	_, actionGetFromCache := logging.AddSubActionToContext(ctx, fmt.Sprintf("Getting %s Item from cache", utils.MediaItemInfo(dbItem.MediaItem)), logging.LevelTrace)
	mediaItem, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(dbItem.MediaItem.LibraryTitle, dbItem.MediaItem.TMDB_ID, dbItem.MediaItem.Edition)
	if !found || mediaItem == nil {
		actionGetFromCache.SetError("Media Item not found in cache", "Try refreshing the cache if this issue persists", nil)
		actionGetFromCache.Complete()
		return
	}
	actionGetFromCache.Complete()

	_, actionGetFromMediaServer := logging.AddSubActionToContext(
		ctx,
		fmt.Sprintf("Getting %s Item details from MediaServer", utils.MediaItemInfo(dbItem.MediaItem)),
		logging.LevelTrace,
	)

	retrySleep := 10 * time.Second
	maxRetries := 6
	var Err logging.LogErrorInfo

	for attempt := 1; attempt <= maxRetries; attempt++ {
		found, Err = mediaserver.GetMediaItemDetails(ctx, mediaItem)
		if Err.Message == "" && found {
			actionGetFromMediaServer.AppendResult("attempt", attempt)
			actionGetFromMediaServer.Complete()
			break
		}

		if Err.Message != "" {
			actionGetFromMediaServer.AppendWarning(
				fmt.Sprintf("attempt_%d_error", attempt),
				Err.Message,
			)
		} else {
			actionGetFromMediaServer.AppendWarning(
				fmt.Sprintf("attempt_%d_not_found", attempt),
				"Media item not found yet",
			)
		}

		// No more retries left
		if attempt == maxRetries {
			actionGetFromMediaServer.SetError(
				"Media item not found after retries",
				"The media server did not return the item within retry window",
				map[string]any{
					"max_retries": maxRetries,
					"retry_sleep": retrySleep.String(),
					"item":        utils.MediaItemInfo(*mediaItem),
				},
			)
			actionGetFromMediaServer.Complete()
			return
		}

		_, retrySleepAction := logging.AddSubActionToContext(
			ctx,
			fmt.Sprintf("Media item not ready; sleeping %v before retry %d/%d", retrySleep, attempt, maxRetries),
			logging.LevelTrace,
		)
		time.Sleep(retrySleep)
		retrySleepAction.Complete()
	}

	for _, dbSet := range dbItem.PosterSets {
		if !dbSet.SelectedTypes.SeasonPoster && !dbSet.SelectedTypes.SpecialSeasonPoster && !dbSet.SelectedTypes.Titlecard {
			continue
		}

		// Get the latest set details from MediUX
		mediuxSet, _, Err := mediux.GetShowSetByID(ctx, dbSet.ID, mediaItem.LibraryTitle, mediaItem.Edition)
		if Err.Message != "" {
			logging.LOGGER.Error().Timestamp().Msgf("Error fetching set details from MediUX for set ID %s: %s", dbSet.ID, Err.Message)
			continue
		}

		// If the set doesn't have any images, then we can skip it since there is nothing to download
		if len(mediuxSet.Images) == 0 {
			logging.LOGGER.Info().Timestamp().Msgf("Skipping set ID %s because it doesn't have any images", dbSet.ID)
			continue
		}

		// We need to do the following for each episode in the Sonarr payload:
		// - If Season Posters are selected and it is a New Download event, download the season poster from the set, if it exists.
		// - If Special Season Posters are selected and it is a New Download event, download the special season poster from the set, if it exists.
		// - If Titlecards are selected, download the titlecard for the episode from the set, if it exists.
		var imagesToDownload []models.ImageFile
		_, actionCheck := logging.AddSubActionToContext(ctx, "Checking which images to download for this Sonarr event", logging.LevelTrace)
		for _, sonarrEpisode := range payload.Episodes {
			seasonNumber := sonarrEpisode.SeasonNumber
			episodeNumber := sonarrEpisode.EpisodeNumber

			// Check if the Media Item in the DB Item has this season already downloaded - if it doesn't, then we download the season poster if it's selected, otherwise we skip downloading the season poster since its not new
			seasonInDBItem := false
			if dbItem.MediaItem.Series != nil {
				for _, season := range dbItem.MediaItem.Series.Seasons {
					if season.SeasonNumber == seasonNumber {
						seasonInDBItem = true
						break
					}
				}
			}

			if dbSet.SelectedTypes.SeasonPoster && seasonNumber != 0 && payload.IsUpgrade == false {
				var seasonPosterImage *models.ImageFile
				for _, image := range mediuxSet.Images {
					if image.Type == "season_poster" && image.SeasonNumber != nil && *image.SeasonNumber == seasonNumber {
						seasonPosterImage = &image
						break
					}
				}

				if seasonPosterImage != nil && !seasonInDBItem {
					imagesToDownload = append(imagesToDownload, *seasonPosterImage)
				} else {
					logging.LOGGER.Info().Timestamp().Msgf("No season poster found in set ID %s for season %d, skipping season poster download for this episode", dbSet.ID, seasonNumber)
				}
			}

			if dbSet.SelectedTypes.SpecialSeasonPoster && seasonNumber == 0 && payload.IsUpgrade == false {
				var specialSeasonPosterImage *models.ImageFile
				for _, image := range mediuxSet.Images {
					if image.Type == "season_poster" && image.SeasonNumber != nil && *image.SeasonNumber == 0 {
						specialSeasonPosterImage = &image
						break
					}
				}

				if specialSeasonPosterImage != nil && !seasonInDBItem {
					imagesToDownload = append(imagesToDownload, *specialSeasonPosterImage)
				} else {
					logging.LOGGER.Info().Timestamp().Msgf("No special season poster found in set ID %s, skipping special season poster download for this episode", dbSet.ID)
				}
			}

			if dbSet.SelectedTypes.Titlecard {
				var titlecardImage *models.ImageFile
				for _, image := range mediuxSet.Images {
					if image.Type == "titlecard" && image.SeasonNumber != nil && *image.SeasonNumber == seasonNumber && image.EpisodeNumber != nil && *image.EpisodeNumber == episodeNumber {
						titlecardImage = &image
						break
					}
				}

				if titlecardImage != nil {
					imagesToDownload = append(imagesToDownload, *titlecardImage)
				} else {
					logging.LOGGER.Info().Timestamp().Msgf("No titlecard found in set ID %s for season %d episode %d, skipping titlecard download for this episode", dbSet.ID, seasonNumber, episodeNumber)
				}
			}
		}

		if len(imagesToDownload) == 0 {
			logging.LOGGER.Info().Timestamp().Msgf("No images to download for set ID %s for any of the episodes in the Sonarr payload, skipping", dbSet.ID)
			continue
		}
		actionCheck.AppendResult(fmt.Sprintf("images_to_download_for_set_%s", dbSet.ID), len(imagesToDownload))
		actionCheck.Complete()

		dbUpdateRequired := false
		event := models.NewDownloadEvent(*mediaItem, dbSet.PosterSet, models.SourceSonarrWebhook, models.DownloadKindReapply)
		results, updatedItem := mediaserver.ApplyImages(ctx, ctx, *mediaItem, imagesToDownload, nil)
		*mediaItem = updatedItem
		for i, image := range imagesToDownload {
			result := ""
			Err := results[i].Err
			event.AddImage(image, Err.Message == "")
			if Err.Message != "" {
				logging.LOGGER.Error().Timestamp().Msgf("Error downloading/applying image from set ID %s to media item %s: %s", dbSet.ID, utils.MediaItemInfo(*mediaItem), Err.Message)
				result = "Error: " + Err.Message
			} else {
				dbUpdateRequired = true
				result = "Success"
			}

			go func(image models.ImageFile, result string) {
				defer logging.RecoverPanic("Sonarr Webhook Notification")
				sendFileDownloadNotification(*mediaItem, dbSet, image, payload.IsUpgrade, result)
			}(image, result)
		}
		database.RecordDownloadEvent(ctx, event)

		if dbUpdateRequired {
			dbItem.MediaItem = *mediaItem
			newSetInfo := models.DBPosterSetDetail{
				PosterSet: models.PosterSet{
					BaseSetInfo: models.BaseSetInfo{
						ID:               dbSet.ID,
						Type:             dbSet.Type,
						Title:            dbSet.Title,
						UserCreated:      dbSet.UserCreated,
						DateCreated:      dbSet.DateCreated,
						DateUpdated:      dbSet.DateUpdated,
						Popularity:       dbSet.Popularity,
						PopularityGlobal: dbSet.PopularityGlobal,
					},
					Images: mediuxSet.Images,
				},
				LastDownloaded:            time.Now(),
				SelectedTypes:             dbSet.SelectedTypes,
				AutoDownload:              dbSet.AutoDownload,
				AutoAddNewCollectionItems: dbSet.AutoAddNewCollectionItems,
				NoAutoUpgrade:             dbSet.NoAutoUpgrade,
				ToDelete:                  false,
			}
			found, dbItem.PosterSets = utils.UpdatePosterSetInDBItem(dbItem.PosterSets, newSetInfo)
			if !found {
				logging.LOGGER.Error().Timestamp().Str("item", utils.MediaItemInfo(*mediaItem)).Str("set_id", dbSet.ID).Msg("Failed to update set info in DB item after redownloading images for Sonarr Webhook Check, set not found in DB item")
			} else {
				// Update the set info in the database for this item
				Err = database.UpsertSavedItem(ctx, dbItem)
				if Err.Message != "" {
					logging.LOGGER.Error().Timestamp().Str("item", utils.MediaItemInfo(*mediaItem)).Str("set_id", dbSet.ID).Str("error", Err.Message).Msg("Failed to update DB item after redownloading images for Sonarr Webhook Check")
					continue
				}
			}
		}
	}
}
