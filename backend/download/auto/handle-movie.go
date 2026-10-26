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
	"time"
)

func handleMovie(ctx context.Context, mediaItem models.MediaItem, dbItem models.DBSavedItem) (result AutoDownloadResult) {
	result = AutoDownloadResult{}
	result.Item = utils.MediaItemInfo(dbItem.MediaItem)

	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().
				Timestamp().
				Str("item", utils.MediaItemInfo(dbItem.MediaItem)).
				Interface("recover", r).
				Str("stack", string(debug.Stack())).
				Msg("PANIC: in handleMovie for AutoDownload Check")
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
	changes := MovieChangeDetails{}

	_, actionCheckChanges := logging.AddSubActionToContext(ctx, "Checking if Media Item has changed", logging.LevelTrace)
	// Check to see if the Rating Key has changed
	if mediaItem.RatingKey != dbItem.MediaItem.RatingKey {
		changes.RatingKeyChanged = true
		changes.OldRatingKey = dbItem.MediaItem.RatingKey
		changes.NewRatingKey = mediaItem.RatingKey
	}
	// Check to see if the file details have changed
	if mediaItem.Movie != nil && dbItem.MediaItem.Movie != nil {
		pathChanged := mediaItem.Movie.File.Path != dbItem.MediaItem.Movie.File.Path
		durationChanged := durationReallyChanged(mediaItem.Movie.File.Duration, dbItem.MediaItem.Movie.File.Duration)
		sizeChanged := sizeReallyChanged(mediaItem.Movie.File.Size, dbItem.MediaItem.Movie.File.Size)
		if pathChanged {
			changes.PathChanged = true
			changes.OldPath = dbItem.MediaItem.Movie.File.Path
			changes.NewPath = mediaItem.Movie.File.Path
		}
		if durationChanged {
			changes.DurationChanged = true
			changes.OldDuration = dbItem.MediaItem.Movie.File.Duration
			changes.NewDuration = mediaItem.Movie.File.Duration
		}
		if sizeChanged {
			changes.SizeChanged = true
			changes.OldSize = dbItem.MediaItem.Movie.File.Size
			changes.NewSize = mediaItem.Movie.File.Size
		}
	}
	actionCheckChanges.AppendResult("changes", changes)
	actionCheckChanges.AppendResult("changes_summary", map[string]any{
		"changed_rating_key": changes.RatingKeyChanged,
		"changed_path":       changes.PathChanged,
		"changed_duration":   changes.DurationChanged,
		"changed_size":       changes.SizeChanged,
	})
	actionCheckChanges.Complete()
	logging.Dev().Timestamp().
		Bool("changed_rating_key", changes.RatingKeyChanged).
		Bool("changed_path", changes.PathChanged).
		Bool("changed_duration", changes.DurationChanged).
		Bool("changed_size", changes.SizeChanged).
		Msg("Completed checking for changes in Movie info for AutoDownload Check")

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
		if !dbSet.SelectedTypes.Poster && !dbSet.SelectedTypes.Backdrop {
			setResult.Result = "skipped"
			setResult.Reason = "No image types selected for this set, skipping check for this set"
			result.Sets = append(result.Sets, setResult)
			continue
		}

		mediuxSet := models.SetRef{}
		includedItems := map[string]models.IncludedItem{}
		Err := logging.LogErrorInfo{}
		// Get the latest set details from MediUX
		switch dbSet.Type {
		case "movie":
			mediuxSet, _, Err = mediux.GetMovieSetByID(ctx, dbSet.ID, mediaItem.LibraryTitle, mediaItem.Edition)
			if Err.Message != "" {
				setResult.Result = "error"
				setResult.Reason = "Failed to get latest set details from MediUX"
				result.Sets = append(result.Sets, setResult)
				continue
			}
		case "collection":
			mediuxSet, includedItems, Err = mediux.GetMovieCollectionSetByID(ctx, dbSet.ID, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition, false)
			if Err.Message != "" {
				setResult.Result = "error"
				setResult.Reason = "Failed to get latest set details from MediUX"
				result.Sets = append(result.Sets, setResult)
				continue
			}
		default:
			setResult.Result = "error"
			setResult.Reason = "Unknown set type"
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

		// Create a map of the old images for easy lookup when checking if we need to redownload an image
		oldImageByKey := make(map[string]models.ImageFile, len(dbSet.Images))
		for _, oldImage := range dbSet.Images {
			key := oldImage.Type + "|" + oldImage.ID
			oldImageByKey[key] = oldImage
		}

		// Sort the images by type
		sortImagesSliceByType(mediuxSet.Images)

		possibleImages := []models.ImageFile{}
		for _, image := range mediuxSet.Images {
			if image.ItemTMDB_ID != mediaItem.TMDB_ID {
				continue
			}
			possibleImages = append(possibleImages, image)
		}

		imagesToRedownload := []ImageFileWithReason{}
		_, actionImageChecks := logging.AddSubActionToContext(ctx, "Checking images in set", logging.LevelTrace)
		// Now we loop through all the images in the latest set details and do our checks
		for _, image := range possibleImages {
			imageName := utils.GetFileDownloadName(mediaItem.Title, image)
			check := ImageCheckResult{
				Type:    image.Type,
				Outcome: "skipped",
				Reason:  "",
			}

			if image.Type != "poster" && image.Type != "backdrop" {
				check.Reason = "Image type is not poster or backdrop"
				actionImageChecks.AppendResult(imageName, check)
				continue
			} else if image.Type == "poster" && !dbSet.SelectedTypes.Poster {
				check.Reason = "Poster not selected for this set"
				actionImageChecks.AppendResult(imageName, check)
				continue
			} else if image.Type == "backdrop" && !dbSet.SelectedTypes.Backdrop {
				check.Reason = "Backdrop not selected for this set"
				actionImageChecks.AppendResult(imageName, check)
				continue
			} else if image.ItemTMDB_ID != mediaItem.TMDB_ID {
				// We don't log this as its not needed
				continue
			}

			// If we got here, it means the image type is selected for this set, so we check if the image needs to be re-downloaded based on the changes we detected earlier
			// If there are changes that indicate we should re-download, we add it to the list.
			// If there are no relevant changes, we check to see if the image has changed based on the dates and add it to the list if it has
			handled := false
			if changes.RatingKeyChanged || changes.PathChanged || changes.DurationChanged || changes.SizeChanged {
				check.Outcome = "redownload"
				check.Reason = getMovieInfoReasons(changes)
				imagesToRedownload = append(imagesToRedownload, ImageFileWithReason{
					ImageFile:   image,
					ReasonTitle: "Movie Info Changed",
					Reason:      check.Reason,
				})
				handled = true
			}
			if !handled {
				checkImageDates(image, &dbSet, oldImageByKey, &imagesToRedownload, &check)
			}
			actionCheckChanges.AppendResult(imageName, check)
		}
		actionCheckChanges.AppendResult("images_to_redownload_count", len(imagesToRedownload))
		actionCheckChanges.Complete()

		defer func() {
			logging.DevMsgf("Checking if we need to add new collection items for set %s (ID: %s)", dbSet.Title, dbSet.ID)
			handleCollectionAutoAddNewItems(ctx, dbSet, includedItems, mediuxSet)
		}()

		// If no images need to be redownloaded, we will skip the redownload process and move on to the next set
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
		results := applyRedownloads(ctx, &mediaItem, imagesToRedownload)
		for idx, image := range imagesToRedownload {
			// The redownloaded image
			imageRedownloadResult := make(map[string]any)
			imageRedownloadResult["image"] = utils.GetFileDownloadName(mediaItem.Title, image.ImageFile)
			imageRedownloadResult["image_type"] = image.Type
			imageRedownloadResult["redownload_reason"] = image.Reason

			Err := results[idx].Err
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

		// We remove the images that are for other items in the set and then update the set in the database with the new image info and download date so that it is up to date for the next check
		mediuxSet.PosterSet.Images = possibleImages

		// Reinsert the set into the DB item with the updated image info and download date so that it is up to date for the next check
		Err = insertRedownloadedSetIntoDB(ctx, mediaItem, mediuxSet.PosterSet, dbItem, dbSet)

		setResult.Result = "success"
		setResult.Reason = fmt.Sprintf("%d images need to be redownloaded", len(imagesToRedownload))
		result.Sets = append(result.Sets, setResult)
	}

	getOverallResults(&result)
	return result
}

func getMovieInfoReasons(changes MovieChangeDetails) string {
	lines := []string{"Changes in Movie Info:"}

	if changes.RatingKeyChanged {
		lines = append(lines, fmt.Sprintf(
			"Rating Key changed:\n- old: %s\n- new: %s",
			changes.OldRatingKey, changes.NewRatingKey,
		))
	}
	if changes.PathChanged {
		lines = append(lines, fmt.Sprintf(
			"File Path changed:\n- old: %s\n- new: %s",
			changes.OldPath, changes.NewPath,
		))
	}
	if changes.DurationChanged {
		lines = append(lines, fmt.Sprintf(
			"File Duration changed:\n- old: %d\n- new: %d",
			changes.OldDuration, changes.NewDuration,
		))
	}
	if changes.SizeChanged {
		lines = append(lines, fmt.Sprintf(
			"File Size changed:\n- old: %d\n- new: %d",
			changes.OldSize, changes.NewSize,
		))
	}

	return strings.Join(lines, "\n")
}

func handleCollectionAutoAddNewItems(ctx context.Context, dbSet models.DBPosterSetDetail, includedItems map[string]models.IncludedItem, mediuxSet models.SetRef) {
	if dbSet.Type != "collection" || !dbSet.AutoAddNewCollectionItems {
		logging.DevMsgf("Checking for new collection members to add for set %s (ID: %s) type: %s %v", dbSet.Title, dbSet.ID, dbSet.Type, dbSet.AutoAddNewCollectionItems)
		return
	}
	logging.DevMsgf("Checking for new collection members to add for set %s (ID: %s)", dbSet.Title, dbSet.ID)

	ctx, action := logging.AddSubActionToContext(ctx, fmt.Sprintf("Checking for new collection members in set %s (%s)", dbSet.Title, dbSet.ID), logging.LevelDebug)
	defer action.Complete()

	dbOut, dbErr := database.GetAllSavedSets(ctx, models.DBFilter{SetID: dbSet.ID, ItemsPerPage: -1})
	if dbErr.Message != "" {
		action.AppendWarning("collection_auto_add_lookup_error", dbErr.Message)
		return
	}
	logging.DevMsgf("Found %d existing items in the database for collection set %s (ID: %s)", len(dbOut.Items), dbSet.Title, dbSet.ID)

	existing := map[string]struct{}{}
	for _, existingItem := range dbOut.Items {
		key := existingItem.MediaItem.TMDB_ID + "|" + existingItem.MediaItem.LibraryTitle
		existing[key] = struct{}{}
	}

	for _, includedItem := range includedItems {
		item := includedItem.MediaItem
		if item.TMDB_ID == "" || item.LibraryTitle == "" {
			logging.DevMsgf("Skipping included item in collection set %s (ID: %s) because it is missing TMDB ID or Library Title", dbSet.Title, dbSet.ID)
			continue
		}

		// Skip items that are already in the database for this set (we only want to add new items that have been added to the collection since the last check)
		itemKey := item.TMDB_ID + "|" + item.LibraryTitle
		if _, exists := existing[itemKey]; exists {
			logging.DevMsgf("Skipping included item in collection set %s (ID: %s) because it is already in the database", dbSet.Title, dbSet.ID)
			continue
		}

		itemImages := make([]models.ImageFile, 0)
		for _, image := range mediuxSet.Images {
			if image.Type != "poster" && image.Type != "backdrop" {
				continue
			}
			if image.Type == "poster" && !dbSet.SelectedTypes.Poster {
				continue
			}
			if image.Type == "backdrop" && !dbSet.SelectedTypes.Backdrop {
				continue
			}
			if image.ItemTMDB_ID != item.TMDB_ID {
				continue
			}
			itemImages = append(itemImages, image)
		}

		if len(itemImages) == 0 {
			action.AppendResult("collection_auto_add_skipped", map[string]any{
				"tmdb_id":       item.TMDB_ID,
				"library_title": item.LibraryTitle,
				"reason":        "No selected images for item in this collection set",
			})
			continue
		}

		event := models.NewDownloadEvent(item, mediuxSet.PosterSet, models.SourceAutoDownload, models.DownloadKindDownload)
		results, updatedItem := mediaserver.ApplyImages(ctx, ctx, item, itemImages, nil)
		item = updatedItem
		for i, image := range itemImages {
			downloadErr := results[i].Err
			event.AddImage(image, downloadErr.Message == "")
			if downloadErr.Message != "" {
				action.AppendWarning("collection_auto_add_download_failed", map[string]any{
					"tmdb_id":       item.TMDB_ID,
					"library_title": item.LibraryTitle,
					"image_type":    image.Type,
					"image_id":      image.ID,
					"error":         downloadErr.Message,
				})
				continue
			}

			go func(mediaItem models.MediaItem, imageFile models.ImageFile) {
				defer logging.RecoverPanic("Auto Download Notification")
				sendFileDownloadNotification(mediaItem, dbSet, ImageFileWithReason{
					ImageFile:   imageFile,
					ReasonTitle: "New Collection Item",
					Reason:      "Item was added to a collection set with auto-add enabled",
				})
			}(item, image)
		}
		database.RecordDownloadEvent(ctx, event)

		newSavedItem := models.DBSavedItem{
			MediaItem: item,
			PosterSets: []models.DBPosterSetDetail{
				{
					PosterSet: models.PosterSet{
						BaseSetInfo: models.BaseSetInfo{
							ID:          mediuxSet.ID,
							Type:        mediuxSet.Type,
							Title:       mediuxSet.Title,
							UserCreated: mediuxSet.UserCreated,
							DateCreated: mediuxSet.DateCreated,
							DateUpdated: mediuxSet.DateUpdated,
						},
						Images: itemImages,
					},
					LastDownloaded:            time.Now(),
					SelectedTypes:             dbSet.SelectedTypes,
					AutoDownload:              dbSet.AutoDownload,
					AutoAddNewCollectionItems: dbSet.AutoAddNewCollectionItems,
					NoAutoUpgrade:             dbSet.NoAutoUpgrade,
					ToDelete:                  false,
				},
			},
		}

		upsertErr := database.UpsertSavedItem(ctx, newSavedItem)
		if upsertErr.Message != "" {
			action.AppendWarning("collection_auto_add_db_upsert_failed", map[string]any{
				"tmdb_id":       item.TMDB_ID,
				"library_title": item.LibraryTitle,
				"error":         upsertErr.Message,
			})
			continue
		}

		existing[itemKey] = struct{}{}
		action.AppendResult("collection_auto_add_added_item", map[string]any{
			"tmdb_id":       item.TMDB_ID,
			"library_title": item.LibraryTitle,
			"set_id":        dbSet.ID,
			"images":        len(itemImages),
		})
	}
}

type MovieChangeDetails struct {
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
