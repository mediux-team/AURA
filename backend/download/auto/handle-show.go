package autodownload

import (
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"runtime/debug"
	"strings"
)

func handleShow(ctx context.Context, mediaItem models.MediaItem, dbItem models.DBSavedItem) (result AutoDownloadResult) {
	result = AutoDownloadResult{}
	result.Item = utils.MediaItemInfo(dbItem.MediaItem)

	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().
				Timestamp().
				Str("item", utils.MediaItemInfo(dbItem.MediaItem)).
				Interface("recover", r).
				Str("stack", string(debug.Stack())).
				Msg("PANIC: in handleShow for AutoDownload Check")
			result = AutoDownloadResult{
				Item:           utils.MediaItemInfo(dbItem.MediaItem),
				OverallResult:  "error",
				OverallMessage: fmt.Sprintf("Panic occurred: %v", r),
			}
		}
	}()

	// Indicators that something has changed with the media item
	// If any of these are true, we will redownload the selected types for each set
	// If none of these are true, we will compare get the latest set and check the dates to see if there has been an update to an image
	changes := ShowChangeDetails{}

	_, actionCheckChanges := logging.AddSubActionToContext(ctx, "Checking if Media Item has changed", logging.LevelTrace)
	// Rating key change
	if mediaItem.RatingKey != dbItem.MediaItem.RatingKey {
		changes.RatingKeyChanged = true
		changes.OldRatingKey = dbItem.MediaItem.RatingKey
		changes.NewRatingKey = mediaItem.RatingKey
	}
	// Path + season/episode changes
	if mediaItem.Series != nil {
		oldPath := ""
		if dbItem.MediaItem.Series != nil {
			oldPath = dbItem.MediaItem.Series.Location
		}
		newPath := mediaItem.Series.Location
		if oldPath != newPath {
			changes.PathChanged = true
			changes.OldPath = oldPath
			changes.NewPath = newPath
		}

		type dbSeasonLookup struct {
			season   models.MediaItemSeason
			episodes map[int]models.MediaItemEpisode
		}

		dbSeasons := make(map[int]dbSeasonLookup)
		if dbItem.MediaItem.Series != nil {
			for _, s := range dbItem.MediaItem.Series.Seasons {
				dbEpisodes := make(map[int]models.MediaItemEpisode, len(s.Episodes))
				for _, e := range s.Episodes {
					dbEpisodes[e.EpisodeNumber] = e
				}
				dbSeasons[s.SeasonNumber] = dbSeasonLookup{
					season:   s,
					episodes: dbEpisodes,
				}
			}
		}

		for _, season := range mediaItem.Series.Seasons {
			seasonNumber := season.SeasonNumber
			dbSeasonData, seasonFound := dbSeasons[season.SeasonNumber]
			if !seasonFound {
				changes.AddedSeasons = append(changes.AddedSeasons, season.SeasonNumber)
				changes.SeasonsAdded = true
				continue
			}

			for _, episode := range season.Episodes {
				dbEpisode, episodeFound := dbSeasonData.episodes[episode.EpisodeNumber]
				if !episodeFound {
					changes.AddedEpisodes = append(changes.AddedEpisodes, EpisodeRef{
						SeasonNumber:  seasonNumber,
						EpisodeNumber: episode.EpisodeNumber,
					})
					changes.EpisodesAdded = true
					continue
				}
				ratingKeyChanged := episode.RatingKey != dbEpisode.RatingKey
				pathChanged := episode.File.Path != dbEpisode.File.Path
				sizeChanged := sizeReallyChanged(episode.File.Size, dbEpisode.File.Size)
				durationChanged := durationReallyChanged(episode.File.Duration, dbEpisode.File.Duration)
				if ratingKeyChanged || pathChanged || sizeChanged || durationChanged {
					episodeChange := EpisodeChangeDetails{
						EpisodeRef: EpisodeRef{
							SeasonNumber:  seasonNumber,
							EpisodeNumber: episode.EpisodeNumber,
						},
					}
					if ratingKeyChanged {
						episodeChange.RatingKeyChanged = true
						episodeChange.OldRatingKey = dbEpisode.RatingKey
						episodeChange.NewRatingKey = episode.RatingKey
					}
					if pathChanged {
						episodeChange.PathChanged = true
						episodeChange.OldPath = dbEpisode.File.Path
						episodeChange.NewPath = episode.File.Path
					}
					if sizeChanged {
						episodeChange.SizeChanged = true
						episodeChange.OldSize = dbEpisode.File.Size
						episodeChange.NewSize = episode.File.Size
					}
					if durationChanged {
						episodeChange.DurationChanged = true
						episodeChange.OldDuration = dbEpisode.File.Duration
						episodeChange.NewDuration = episode.File.Duration
					}
					changes.ChangedEpisodes = append(changes.ChangedEpisodes, episodeChange)
					changes.EpisodesChanged = true
				}
			}
		}
	}

	actionCheckChanges.AppendResult("changes", changes)
	actionCheckChanges.AppendResult("changes_summary", map[string]any{
		"changed_rating_key":     changes.RatingKeyChanged,
		"changed_path":           changes.PathChanged,
		"added_seasons_count":    len(changes.AddedSeasons),
		"added_episodes_count":   len(changes.AddedEpisodes),
		"changed_episodes_count": len(changes.ChangedEpisodes),
	})
	actionCheckChanges.Complete()
	logging.Dev().Timestamp().
		Bool("changed_rating_key", changes.RatingKeyChanged).
		Bool("changed_path", changes.PathChanged).
		Int("added_seasons_count", len(changes.AddedSeasons)).
		Int("added_episodes_count", len(changes.AddedEpisodes)).
		Int("changed_episodes_count", len(changes.ChangedEpisodes)).
		Msgf("Change details for %s", utils.MediaItemInfo(dbItem.MediaItem))

	for _, dbSet := range dbItem.PosterSets {
		var setResult AutoDownloadSetResult
		setResult.ID = dbSet.ID
		setResult.Title = dbSet.Title
		setResult.UserCreated = dbSet.UserCreated

		// If the set is not set to auto-download, then we skip it
		if !dbSet.AutoDownload {
			setResult.Result = "skipped"
			setResult.Reason = "Set is not set to auto-download, skipping check for this set"
			result.Sets = append(result.Sets, setResult)
			continue
		}

		// If no types are selected, then we skip
		if !dbSet.SelectedTypes.Poster && !dbSet.SelectedTypes.Backdrop && !dbSet.SelectedTypes.SeasonPoster && !dbSet.SelectedTypes.SpecialSeasonPoster && !dbSet.SelectedTypes.Titlecard {
			setResult.Result = "skipped"
			setResult.Reason = "No image types selected for this set, skipping check for this set"
			result.Sets = append(result.Sets, setResult)
			continue
		}

		// Get the latest set details from MediUX
		mediuxSet, _, Err := mediux.GetShowSetByID(ctx, dbSet.ID, mediaItem.LibraryTitle, mediaItem.Edition)
		if Err.Message != "" {
			setResult.Result = "error"
			setResult.Reason = "Failed to get latest set details from MediUX"
			result.Sets = append(result.Sets, setResult)
			continue
		}

		// If the Set IDs don't match, we have a problem and we skip this set
		if mediuxSet.ID != dbSet.ID {
			setResult.Result = "error"
			setResult.Reason = fmt.Sprintf("Set ID mismatch between database and MediUX for set '%s'", dbSet.ID)
			result.Sets = append(result.Sets, setResult)
			continue
		}

		imagesToRedownload := []ImageFileWithReason{}

		// Build fast lookup maps once (before looping images)
		allSeasonsInMediaItem := make(map[int]struct{})
		if mediaItem.Series != nil {
			for _, s := range mediaItem.Series.Seasons {
				allSeasonsInMediaItem[s.SeasonNumber] = struct{}{}
			}
		}

		addedSeasonSet := make(map[int]struct{}, len(changes.AddedSeasons))
		for _, s := range changes.AddedSeasons {
			addedSeasonSet[s] = struct{}{}
		}

		episodeKey := func(season, episode int) string {
			return fmt.Sprintf("%d:%d", season, episode)
		}

		allEpisodesInMediaItem := make(map[string]struct{})
		if mediaItem.Series != nil {
			for _, s := range mediaItem.Series.Seasons {
				for _, e := range s.Episodes {
					allEpisodesInMediaItem[episodeKey(s.SeasonNumber, e.EpisodeNumber)] = struct{}{}
				}
			}
		}

		addedEpisodeSet := make(map[string]struct{}, len(changes.AddedEpisodes))
		for _, ep := range changes.AddedEpisodes {
			addedEpisodeSet[episodeKey(ep.SeasonNumber, ep.EpisodeNumber)] = struct{}{}
		}

		changedEpisodeSet := make(map[string]struct{}, len(changes.ChangedEpisodes))
		for _, ep := range changes.ChangedEpisodes {
			changedEpisodeSet[episodeKey(ep.SeasonNumber, ep.EpisodeNumber)] = struct{}{}
		}

		// Create a map of the old images for easy lookup when checking if we need to redownload an image
		oldImageByKey := make(map[string]models.ImageFile, len(dbSet.Images))
		for _, oldImage := range dbSet.Images {
			key := oldImage.Type + "|" + oldImage.ID
			oldImageByKey[key] = oldImage
		}

		// Sort the images by type
		sortImagesSliceByType(mediuxSet.Images)

		_, actionImageChecks := logging.AddSubActionToContext(ctx, "Checking images in set", logging.LevelTrace)
		// Now we loop through all the images in the latest set details and do our checks
		for _, image := range mediuxSet.Images {
			imageName := utils.GetFileDownloadName(mediaItem.Title, image)
			check := ImageCheckResult{
				Type:    image.Type,
				Outcome: "skipped",
				Reason:  "",
			}

			if image.Type == "poster" && !dbSet.SelectedTypes.Poster {
				check.Reason = "Poster not selected for this set"
				actionImageChecks.AppendResult(imageName, check)
				continue
			} else if image.Type == "backdrop" && !dbSet.SelectedTypes.Backdrop {
				check.Reason = "Backdrop not selected for this set"
				actionImageChecks.AppendResult(imageName, check)
				continue
			} else if image.Type == "season_poster" && !dbSet.SelectedTypes.SeasonPoster && !dbSet.SelectedTypes.SpecialSeasonPoster {
				check.Reason = "Season Poster not selected for this set"
				actionImageChecks.AppendResult(imageName, check)
				continue
			} else if image.Type == "titlecard" && !dbSet.SelectedTypes.Titlecard {
				check.Reason = "Titlecard not selected for this set"
				actionImageChecks.AppendResult(imageName, check)
				continue
			}

			// Skip any season posters and titlecards if there is no season or episode information in the Media Item for this image, as there is nothing to match against and download for
			if (image.Type == "season_poster" || image.Type == "titlecard") && (image.SeasonNumber == nil || (image.Type == "titlecard" && image.EpisodeNumber == nil)) {
				check.Reason = "Image has no season/episode information to match against in the Media Item, skipping"
				actionImageChecks.AppendResult(imageName, check)
				continue
			}

			// Skip any season posters if the season they are for does not exist in the Media Item
			if image.Type == "season_poster" {
				if _, seasonExists := allSeasonsInMediaItem[*image.SeasonNumber]; !seasonExists {
					check.Reason = fmt.Sprintf("Season %d does not exist in the Media Item, skipping", *image.SeasonNumber)
					actionImageChecks.AppendResult(imageName, check)
					continue

				}
			}

			// Skip any titlecards if the season/episode they are for does not exist in the Media Item
			if image.Type == "titlecard" {
				sn := *image.SeasonNumber
				en := *image.EpisodeNumber
				if _, seasonExists := allSeasonsInMediaItem[sn]; !seasonExists {
					check.Reason = fmt.Sprintf("Titlecard for Season %d Episode %d but Season %d does not exist in the Media Item, skipping", sn, en, sn)
					actionImageChecks.AppendResult(imageName, check)
					continue
				}
				if _, episodeExists := allEpisodesInMediaItem[episodeKey(sn, en)]; !episodeExists {
					check.Reason = fmt.Sprintf("Titlecard for Season %d Episode %d but Episode %d does not exist in the Media Item, skipping", sn, en, en)
					actionImageChecks.AppendResult(imageName, check)
					continue
				}
			}

			// If we got here, it means the image type is selected for this set, so we check if the image needs to be re-downloaded based on the changes we detected earlier
			// If there are changes that indicate we should re-download, we add it to the list.
			// If there are no relevant changes, we check to see if the image has changed based on the dates and add it to the list if it has
			handled := false

			if changes.RatingKeyChanged || changes.PathChanged {
				check.Outcome = "redownload"
				check.Reason = getShowInfoChangeReason(changes)
				imagesToRedownload = append(imagesToRedownload, ImageFileWithReason{
					ImageFile:   image,
					ReasonTitle: "Show Info Changed",
					Reason:      check.Reason,
				})
				handled = true
				continue
			}

			switch image.Type {
			case "poster", "backdrop":
				// For posters and backdrop, we already checked for Series Path and RatingKey changes which would impact all images
			case "season_poster":
				if _, ok := addedSeasonSet[*image.SeasonNumber]; ok {
					seasonStr := fmt.Sprintf("Season %02d added since last download", *image.SeasonNumber)
					if *image.SeasonNumber == 0 {
						seasonStr = "Special Season added since last download"
					}
					check.Outcome = "redownload"
					check.Reason = seasonStr
					imagesToRedownload = append(imagesToRedownload, ImageFileWithReason{
						ImageFile:   image,
						ReasonTitle: "Season Added",
						Reason:      check.Reason,
					})
					handled = true
				}
			case "titlecard":
				sn := *image.SeasonNumber
				en := *image.EpisodeNumber

				_, seasonAdded := addedSeasonSet[sn]
				_, episodeAdded := addedEpisodeSet[episodeKey(sn, en)]
				_, episodeChanged := changedEpisodeSet[episodeKey(sn, en)]

				if seasonAdded || episodeAdded || episodeChanged {
					reasonTitle := ""
					reason := ""
					switch {
					case seasonAdded:
						reasonTitle = "Season Added"
						reason = fmt.Sprintf("Season %02d added since last download", sn)
						if sn == 0 {
							reasonTitle = "Special Season Added"
							reason = "Special Season added since last download"
						}
					case episodeAdded:
						reasonTitle = "Episode Added"
						reason = fmt.Sprintf("Season %02d Episode %02d added since last download", sn, en)
					default:
						reasonTitle = "Episode Changed"
						reason = fmt.Sprintf("Season %02d Episode %02d changed since last download\n", sn, en)
						// Get the changes episode details to include in the reason
						for _, epChange := range changes.ChangedEpisodes {
							if epChange.SeasonNumber == sn && epChange.EpisodeNumber == en {
								reason += getEpisodeInfoChangeReason(epChange)
								break
							}
						}
					}

					imagesToRedownload = append(imagesToRedownload, ImageFileWithReason{
						ImageFile:   image,
						ReasonTitle: reasonTitle,
						Reason:      reason,
					})
					check.Outcome = "redownload"
					check.Reason = reason
					handled = true
				}
			default:
				check.Reason = "Unsupported image type"
			}

			if !handled {
				checkImageDates(image, &dbSet, oldImageByKey, &imagesToRedownload, &check)
			}
			actionImageChecks.AppendResult(imageName, check)
		}
		actionImageChecks.AppendResult("images_to_redownload_count", len(imagesToRedownload))
		actionImageChecks.Complete()

		if len(imagesToRedownload) == 0 {
			setResult.Result = "skipped"
			setResult.Reason = "No changes detected that require redownloading images"
			result.Sets = append(result.Sets, setResult)
			continue
		}
		logging.Dev().Timestamp().
			Int("total_images_in_set", len(mediuxSet.Images)).
			Int("images_to_redownload", len(imagesToRedownload)).
			Msgf("Image check results for set %s", dbSet.ID)

		_, imageRedownloadsAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Downloading %d updated images for set %s (ID: %s)", len(imagesToRedownload), dbSet.Title, dbSet.ID), logging.LevelInfo)
		event := models.NewDownloadEvent(mediaItem, dbSet.PosterSet, models.SourceAutoDownload, models.DownloadKindUpdate)
		for idx, image := range imagesToRedownload {
			// Redownload the image
			imageRedownloadResult := make(map[string]any)
			imageRedownloadResult["image"] = utils.GetFileDownloadName(mediaItem.Title, image.ImageFile)
			imageRedownloadResult["image_type"] = image.Type
			imageRedownloadResult["redownload_reason"] = image.Reason
			Err.Message = ""
			Err := mediaserver.DownloadApplyImageToMediaItem(ctx, &mediaItem, image.ImageFile)
			event.AddImage(image.ImageFile, Err.Message == "")
			if Err.Message != "" {
				imageRedownloadResult["redownload_result"] = "error"
				imageRedownloadResult["redownload_error"] = Err.Message
				imageRedownloadsAction.AppendResult(fmt.Sprintf("image_redownload_%d", idx+1), imageRedownloadResult)
				setResult.Result = "error"
				setResult.Reason = fmt.Sprintf("Failed to redownload image %s: %s", utils.GetFileDownloadName(mediaItem.Title, image.ImageFile), Err.Message)
				result.Sets = append(result.Sets, setResult)
				logging.LOGGER.Error().Timestamp().Str("item", utils.MediaItemInfo(mediaItem)).Str("set_id", dbSet.ID).Str("image", utils.GetFileDownloadName(mediaItem.Title, image.ImageFile)).Str("error", Err.Message).Msg("Failed to redownload image for AutoDownload Check")
				continue
			} else {
				// Send a notification to all configured notification services
				// We do this asynchronously and don't wait for the result
				go func(image ImageFileWithReason) {
					defer logging.RecoverPanic("Auto Download Notification")
					sendFileDownloadNotification(mediaItem, dbSet, image)
				}(image)
			}
		}
		imageRedownloadsAction.Complete()
		database.RecordDownloadEvent(ctx, event)

		// Reinsert the set into the DB item with the updated image info and download date so that it is up to date for the next check
		Err = insertRedownloadedSetIntoDB(ctx, mediaItem, mediuxSet.PosterSet, dbItem, dbSet)

		setResult.Result = "success"
		setResult.Reason = fmt.Sprintf("%d images need to be redownloaded", len(imagesToRedownload))
		result.Sets = append(result.Sets, setResult)
	}

	getOverallResults(&result)
	return result
}

func getShowInfoChangeReason(changes ShowChangeDetails) string {
	lines := []string{"Change detected in show info:"}

	if changes.RatingKeyChanged {
		lines = append(lines, fmt.Sprintf(
			"Rating Key changed:\n- old: %s\n- new: %s",
			changes.OldRatingKey, changes.NewRatingKey,
		))
	}
	if changes.PathChanged {
		lines = append(lines, formatChangedPath(changes.OldPath, changes.NewPath))
	}

	if changes.SeasonsAdded {
		lines = append(lines, fmt.Sprintf("Added Seasons: %v", changes.AddedSeasons))
	}

	return strings.Join(lines, "\n")
}

func getEpisodeInfoChangeReason(changes EpisodeChangeDetails) string {
	lines := []string{"Change detected in episode info:"}

	if changes.RatingKeyChanged {
		lines = append(lines, fmt.Sprintf(
			"Rating Key changed:\n- old: %s\n- new: %s",
			changes.OldRatingKey, changes.NewRatingKey,
		))
	}

	if changes.PathChanged {
		lines = append(lines, formatChangedPath(changes.OldPath, changes.NewPath))
	}

	if changes.SizeChanged {
		lines = append(lines, fmt.Sprintf(
			"Size changed:\n- old: %d\n- new: %d",
			changes.OldSize, changes.NewSize,
		))
	}

	if changes.DurationChanged {
		lines = append(lines, fmt.Sprintf(
			"Duration changed:\n- old: %d\n- new: %d",
			changes.OldDuration, changes.NewDuration,
		))
	}

	return strings.Join(lines, "\n")
}

func formatChangedPath(oldPath, newPath string) string {
	if oldPath == newPath {
		return ""
	}

	return fmt.Sprintf("Path changed:\n- old: %s\n- new: %s", oldPath, newPath)
}

type ShowChangeDetails struct {
	RatingKeyChanged bool                   `json:"rating_key_changed"`
	OldRatingKey     string                 `json:"old_rating_key,omitempty"`
	NewRatingKey     string                 `json:"new_rating_key,omitempty"`
	PathChanged      bool                   `json:"path_changed"`
	OldPath          string                 `json:"old_path,omitempty"`
	NewPath          string                 `json:"new_path,omitempty"`
	SeasonsAdded     bool                   `json:"seasons_added"`
	EpisodesAdded    bool                   `json:"episodes_added"`
	EpisodesChanged  bool                   `json:"episodes_changed"`
	AddedSeasons     []int                  `json:"added_seasons,omitempty"`
	AddedEpisodes    []EpisodeRef           `json:"added_episodes,omitempty"`
	ChangedEpisodes  []EpisodeChangeDetails `json:"changed_episodes,omitempty"`
}

type EpisodeRef struct {
	SeasonNumber  int `json:"season_number"`
	EpisodeNumber int `json:"episode_number"`
}

type EpisodeChangeDetails struct {
	EpisodeRef
	RatingKeyChanged bool   `json:"rating_key_changed"`
	OldRatingKey     string `json:"old_rating_key,omitempty"`
	NewRatingKey     string `json:"new_rating_key,omitempty"`
	PathChanged      bool   `json:"path_changed"`
	OldPath          string `json:"old_path,omitempty"`
	NewPath          string `json:"new_path,omitempty"`
	SizeChanged      bool   `json:"size_changed"`
	OldSize          int64  `json:"old_size,omitempty"`
	NewSize          int64  `json:"new_size,omitempty"`
	DurationChanged  bool   `json:"duration_changed"`
	OldDuration      int64  `json:"old_duration,omitempty"`
	NewDuration      int64  `json:"new_duration,omitempty"`
}

type ImageCheckResult struct {
	Type    string         `json:"type"`
	Outcome string         `json:"outcome"`
	Reason  string         `json:"reason,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}
