package downloadqueue

import (
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/models"
	sonarr_radarr "aura/sonarr-radarr"
	"aura/utils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
)

func finalizeQueueFile(filePath, fileName string, hasErrors, hasWarnings bool) error {
	if hasErrors {
		return os.Rename(filePath, path.Join(FolderPath, fmt.Sprintf("error_%s", fileName)))
	}
	if hasWarnings {
		return os.Rename(filePath, path.Join(FolderPath, fmt.Sprintf("warning_%s", fileName)))
	}
	return os.Remove(filePath)
}

func ProcessQueueItems() {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Download Queue Processing")
	logAction := ld.AddAction("Processing Download Queue", logging.LevelInfo)
	defer logAction.Complete()
	ctx = logging.WithCurrentAction(ctx, logAction)

	// Read all JSON files in the download-queue directory
	files, err := os.ReadDir(FolderPath)
	if err != nil {
		logging.LOGGER.Warn().Timestamp().Err(err).Msg("Failed to read download queue directory")
		logAction.SetError("Failed to read download queue directory", "Ensure the directory exists and is accessible",
			map[string]any{
				"error":      err.Error(),
				"folderPath": FolderPath,
			})
		return
	}

	if len(files) == 0 {
		logAction.AppendResult("result", "queue is empty")
		return
	}

	// Process each file in the directory
	for _, file := range files {
		if file.IsDir() || path.Ext(file.Name()) != ".json" {
			continue
		}

		// If a file starts with "error_" or "warning_", skip it
		if len(file.Name()) > 6 && (file.Name()[:6] == "error_" || file.Name()[:8] == "warning_") {
			continue
		}

		ctx, ld := logging.CreateLoggingContext(context.Background(), "Download Queue - Processing")
		subAction := ld.AddAction(fmt.Sprintf("Processing file: %s", file.Name()), logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, subAction)

		// Reset the Latest Info for this file
		LatestInfo.Status = LAST_STATUS_PROCESSING
		LatestInfo.Message = fmt.Sprintf("Processing file: %s", file.Name())
		LatestInfo.Errors = []string{}
		LatestInfo.Warnings = []string{}

		// Create an array of errors and warnings for this file
		fileErrors := []string{}
		fileWarnings := []string{}

		filePath := path.Join(FolderPath, file.Name())

		finalizeAndNotify := func(
			mediaItem models.MediaItem,
			set models.DBPosterSetDetail,
			tmdbPoster string,
			tmdbBackdrop string,
		) {
			issues := FileIssues{Errors: fileErrors, Warnings: fileWarnings}
			SendNotification(issues, mediaItem, set, tmdbPoster, tmdbBackdrop)

			if err := finalizeQueueFile(filePath, file.Name(), len(fileErrors) > 0, len(fileWarnings) > 0); err != nil {
				subAction.AppendWarning(fmt.Sprintf("file_%s", file.Name()), "Failed to move or delete processed file")
			}
			ld.Log()
		}

		// Read and parse the JSON file
		data, err := os.ReadFile(filePath)
		if err != nil {
			fileErrors = append(fileErrors, fmt.Sprintf("read file failed: %v", err))
			finalizeAndNotify(models.MediaItem{}, models.DBPosterSetDetail{}, "", "")
			continue
		}

		var queueItem models.DBSavedItem
		if err := json.Unmarshal(data, &queueItem); err != nil {
			fileErrors = append(fileErrors, fmt.Sprintf("parse json failed: %v", err))
			finalizeAndNotify(models.MediaItem{}, models.DBPosterSetDetail{}, "", "")
			continue
		}

		if queueItem.MediaItem.RatingKey == "" || queueItem.MediaItem.Title == "" || queueItem.MediaItem.LibraryTitle == "" || queueItem.MediaItem.TMDB_ID == "" {
			fileErrors = append(fileErrors, "media item missing required fields: ratingKey/title/libraryTitle/tmdbId")
			finalizeAndNotify(queueItem.MediaItem, models.DBPosterSetDetail{}, "", "")
			continue
		}

		if len(queueItem.PosterSets) == 0 {
			fileWarnings = append(fileWarnings, "no poster sets found")
			finalizeAndNotify(queueItem.MediaItem, models.DBPosterSetDetail{}, "", "")
			continue
		}

		mediuxItemInfo, mErr := mediux.GetBaseItemInfoByTMDB_ID(queueItem.MediaItem.TMDB_ID, queueItem.MediaItem.Type)
		if mErr.Message != "" {
			fileWarnings = append(fileWarnings, fmt.Sprintf("mediux lookup failed: %s", mErr.Message))
		}

		found, mediaErr := mediaserver.GetMediaItemDetails(ctx, &queueItem.MediaItem)
		if mediaErr.Message != "" || !found {
			fileErrors = append(fileErrors, fmt.Sprintf("media server lookup failed for '%s' in '%s': %s", queueItem.MediaItem.Title, queueItem.MediaItem.LibraryTitle, mediaErr.Message))
			// Stop retry flood: mark file as error_ immediately
			finalizeAndNotify(
				queueItem.MediaItem,
				models.DBPosterSetDetail{},
				mediuxItemInfo.TMDB_PosterPath,
				mediuxItemInfo.TMDB_BackdropPath,
			)
			continue
		}

		for _, posterSet := range queueItem.PosterSets {
			setErrors := []string{}
			setWarnings := []string{}

			if posterSet.ID == "" || posterSet.Type == "" || posterSet.Title == "" {
				setErrors = append(setErrors, "poster set missing required fields: id/type/title")
				fileErrors = append(fileErrors, setErrors...)
				SendNotification(
					FileIssues{Errors: setErrors, Warnings: setWarnings},
					queueItem.MediaItem,
					posterSet,
					mediuxItemInfo.TMDB_PosterPath,
					mediuxItemInfo.TMDB_BackdropPath,
				)
				continue
			}

			if !posterSet.SelectedTypes.Poster &&
				!posterSet.SelectedTypes.Backdrop &&
				!posterSet.SelectedTypes.SeasonPoster &&
				!posterSet.SelectedTypes.SpecialSeasonPoster &&
				!posterSet.SelectedTypes.Titlecard {
				setWarnings = append(setWarnings, "poster set has no selected image types")
				fileWarnings = append(fileWarnings, setWarnings...)
				SendNotification(
					FileIssues{Errors: setErrors, Warnings: setWarnings},
					queueItem.MediaItem,
					posterSet,
					mediuxItemInfo.TMDB_PosterPath,
					mediuxItemInfo.TMDB_BackdropPath,
				)
				continue
			}

			LatestInfo.Message = fmt.Sprintf("%s (Set: %s)", queueItem.MediaItem.Title, posterSet.ID)

			for idx, image := range posterSet.Images {
				switch image.Type {
				case "poster":
					if !posterSet.SelectedTypes.Poster {
						continue
					}
				case "backdrop":
					if !posterSet.SelectedTypes.Backdrop {
						continue
					}
				case "season_poster":
					if image.SeasonNumber == nil {
						continue
					}
					// Check if the Media Item contains the season number for this image, if not skip it
					mediaItemHasSeason := false
					if queueItem.MediaItem.Series != nil {
						for _, season := range queueItem.MediaItem.Series.Seasons {
							if *image.SeasonNumber == season.SeasonNumber {
								mediaItemHasSeason = true
								break
							}
						}
					}
					if !mediaItemHasSeason {
						continue
					}
					if *image.SeasonNumber == 0 {
						if !posterSet.SelectedTypes.SpecialSeasonPoster {
							continue
						}
					} else {
						if !posterSet.SelectedTypes.SeasonPoster {
							continue
						}
					}
				case "titlecard":
					// Check if the Media Item contains the Season and Episode numbers for this image, if not skip it
					mediaItemHasEpisode := false
					if queueItem.MediaItem.Series != nil {
						for _, season := range queueItem.MediaItem.Series.Seasons {
							for _, episode := range season.Episodes {
								if image.SeasonNumber != nil && *image.SeasonNumber != season.SeasonNumber {
									continue
								}
								if image.EpisodeNumber != nil && *image.EpisodeNumber != episode.MediuxEpisodeNumber {
									continue
								}
								mediaItemHasEpisode = true
								break
							}
							if mediaItemHasEpisode {
								break
							}
						}
					}
					if !mediaItemHasEpisode {
						continue
					}
					if !posterSet.SelectedTypes.Titlecard {
						continue
					}
				default:
					subAction.AppendWarning(fmt.Sprintf("file_%s_image_%d", file.Name(), idx), fmt.Sprintf("Image has unrecognized type '%s'", image.Type))
					fileWarnings = append(fileWarnings, fmt.Sprintf("Image '%s' has unrecognized type '%s'", image.Src, image.Type))
					continue
				}

				downloadFileName := utils.GetFileDownloadName(queueItem.MediaItem.Title, image)
				Err := mediaserver.DownloadApplyImageToMediaItem(ctx, &queueItem.MediaItem, image)
				if Err.Message != "" {
					setErrors = append(setErrors, fmt.Sprintf("%s: %s", downloadFileName, Err.Message))
				}
			}

			// Per-set notification (success/warning/error)
			SendNotification(
				FileIssues{Errors: setErrors, Warnings: setWarnings},
				queueItem.MediaItem,
				posterSet,
				mediuxItemInfo.TMDB_PosterPath,
				mediuxItemInfo.TMDB_BackdropPath,
			)

			fileErrors = append(fileErrors, setErrors...)
			fileWarnings = append(fileWarnings, setWarnings...)
		}

		Err := database.UpsertSavedItem(ctx, queueItem)
		if Err.Message != "" {
			fileErrors = append(fileErrors, fmt.Sprintf("db upsert failed: %s", Err.Message))
			finalizeAndNotify(
				queueItem.MediaItem,
				models.DBPosterSetDetail{},
				mediuxItemInfo.TMDB_PosterPath,
				mediuxItemInfo.TMDB_BackdropPath,
			)
			continue
		}

		if err := finalizeQueueFile(filePath, file.Name(), len(fileErrors) > 0, len(fileWarnings) > 0); err != nil {
			fileWarnings = append(fileWarnings, fmt.Sprintf("finalize file failed: %v", err))
		}

		// Handle any labels and tags asynchronously
		go func() {
			ctx, ld := logging.CreateLoggingContext(context.Background(), "Download Queue - Labels and Tags Handling")
			logAction := ld.AddAction("Handle Labels and Tags for Added Item", logging.LevelInfo)
			ctx = logging.WithCurrentAction(ctx, logAction)
			defer ld.Log()
			selectedTypes := models.SelectedTypes{}
			for _, posterSet := range queueItem.PosterSets {
				selectedTypes.Poster = selectedTypes.Poster || posterSet.SelectedTypes.Poster
				selectedTypes.Backdrop = selectedTypes.Backdrop || posterSet.SelectedTypes.Backdrop
				selectedTypes.SeasonPoster = selectedTypes.SeasonPoster || posterSet.SelectedTypes.SeasonPoster
				selectedTypes.SpecialSeasonPoster = selectedTypes.SpecialSeasonPoster || posterSet.SelectedTypes.SpecialSeasonPoster
				selectedTypes.Titlecard = selectedTypes.Titlecard || posterSet.SelectedTypes.Titlecard
			}

			mediaserver.AddLabelToMediaItem(ctx, queueItem.MediaItem, selectedTypes)
			sonarr_radarr.HandleTags(ctx, queueItem.MediaItem, selectedTypes)
		}()

		ld.Log()
	}
}
