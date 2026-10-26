package autodownload

import (
	"aura/cache"
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"runtime/debug"
	"sort"
	"time"
)

type AutoDownloadResult struct {
	Item           string                  `json:"item"`
	Sets           []AutoDownloadSetResult `json:"sets"`
	OverallResult  string                  `json:"overall_result"`
	OverallMessage string                  `json:"overall_message"`
}

type AutoDownloadSetResult struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	UserCreated string `json:"user_created"`
	Result      string `json:"result"`
	Reason      string `json:"reason"`
}

type ImageFileWithReason struct {
	models.ImageFile
	ReasonTitle string
	Reason      string
}

// applyRedownloads downloads the images in parallel, and returns a result for each in the same order
func applyRedownloads(ctx context.Context, mediaItem *models.MediaItem, images []ImageFileWithReason) []mediaserver.ImageResult {
	files := make([]models.ImageFile, len(images))
	for i, image := range images {
		files[i] = image.ImageFile
	}
	results, updated := mediaserver.ApplyImages(ctx, ctx, *mediaItem, files, nil)
	*mediaItem = updated
	return results
}

func CheckAllItems(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, getAllItemAction := logging.AddSubActionToContext(ctx, " Getting all saved sets for AutoDownload Check", logging.LevelInfo)
	out, Err := database.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
	if Err.Message != "" {
		getAllItemAction.Complete()
		return Err
	}
	getAllItemAction.Complete()

	mediaserver.GetAllLibrarySectionsAndItems(ctx, true)

	errorCount := 0
	warningCount := 0
	successCount := 0
	skippedCount := 0
	for _, item := range out.Items {
		itemCtx, ld := logging.CreateLoggingContext(context.Background(), "AutoDownload - Check For Updates")
		itemAction := ld.AddAction(fmt.Sprintf("Checking Item %s", utils.MediaItemInfo(item.MediaItem)), logging.LevelInfo)
		itemCtx = logging.WithCurrentAction(itemCtx, itemAction)
		result := CheckItem(itemCtx, item)
		switch result.OverallResult {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "success":
			successCount++
		case "skipped":
			skippedCount++
		}
		itemAction.AppendResult("outcomes", result)
		ld.Log()
	}

	logging.LOGGER.Info().Timestamp().Int("error_count", errorCount).
		Int("warning_count", warningCount).
		Int("success_count", successCount).
		Int("skipped_count", skippedCount).
		Msg("Completed AutoDownload Check for all items")
	return logging.LogErrorInfo{}
}

func CheckItem(ctx context.Context, dbItem models.DBSavedItem) (result AutoDownloadResult) {
	result = AutoDownloadResult{}
	result.Item = utils.MediaItemInfo(dbItem.MediaItem)

	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().
				Timestamp().
				Str("item", utils.MediaItemInfo(dbItem.MediaItem)).
				Interface("recover", r).
				Str("stack", string(debug.Stack())).
				Msg("PANIC: in CheckItem for AutoDownload Check")
			result = AutoDownloadResult{
				Item:           utils.MediaItemInfo(dbItem.MediaItem),
				OverallResult:  "error",
				OverallMessage: fmt.Sprintf("Panic occurred: %v", r),
			}
		}
	}()

	// If there are no Poster Sets in the database for this item, we will skip the check process as there is no existing data to compare against, and just return the result
	if len(dbItem.PosterSets) == 0 {
		result.OverallResult = "skipped"
		result.OverallMessage = "No sets in this item, try deleting and re-adding if this is an error"
		return result
	}

	// If none of the Poster Sets are set to be auto-downloaded, we will skip the check process as there is no need to check for updates if we are not going to download anything, and just return the result
	autoDownloadSetExists := false
	for _, s := range dbItem.PosterSets {
		if s.AutoDownload {
			autoDownloadSetExists = true
			break
		}
	}
	if !autoDownloadSetExists {
		result.OverallResult = "skipped"
		result.OverallMessage = "No sets in this item are set to auto-download, try updating the item if this is an error"
		return result
	}

	// Get the base Show Media Item from the cache
	_, actionGetFromCache := logging.AddSubActionToContext(ctx, fmt.Sprintf("Getting %s Item from cache", utils.MediaItemInfo(dbItem.MediaItem)), logging.LevelTrace)
	mediaItem, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(dbItem.MediaItem.LibraryTitle, dbItem.MediaItem.TMDB_ID, dbItem.MediaItem.Edition)
	if !found || mediaItem == nil {
		result.OverallResult = "error"
		result.OverallMessage = "Media Item not found in cache"
		actionGetFromCache.SetError("Media Item not found in cache", "Try refreshing the cache if this issue persists", nil)
		return result
	}
	actionGetFromCache.Complete()

	// Get the latest Show Media Item from the media server
	found, Err := mediaserver.GetMediaItemDetails(ctx, mediaItem)
	if Err.Message != "" {
		result.OverallResult = "error"
		result.OverallMessage = "Failed to get latest Media Item details from media server"
		return result
	}
	if !found {
		result.OverallResult = "error"
		result.OverallMessage = "Media Item not found on media server"
		return result
	}

	switch dbItem.MediaItem.Type {
	case "movie":
		result = handleMovie(ctx, *mediaItem, dbItem)
	case "show":
		result = handleShow(ctx, *mediaItem, dbItem)
	default:
		result.OverallResult = "error"
		result.OverallMessage = "Unknown media type"
	}
	return result
}

// Only mark size as changed if the size differs by at least 256KB to avoid minor Media Server metadata/container drift.
// If Old and New sizes are both under 256KB, consider that no change
// If one of the sizes is under 256KB and the other is over, consider that a change regardless of the difference, since that could indicate a change from a stub/small file to a real file or vice versa.
// If both sizes are over 256KB, then check if the difference is over 256KB to consider it a change.
func sizeReallyChanged(newSize int64, oldSize int64) bool {
	const sizeThresholdBytes int64 = 256 * 1024 // 256 KB threshold
	sizeDifference := newSize - oldSize
	if sizeDifference < 0 {
		sizeDifference = -sizeDifference
	}

	if newSize < sizeThresholdBytes && oldSize < sizeThresholdBytes {
		return false
	} else if (newSize < sizeThresholdBytes && oldSize >= sizeThresholdBytes) || (newSize >= sizeThresholdBytes && oldSize < sizeThresholdBytes) {
		return true
	} else if sizeDifference > sizeThresholdBytes {
		return true
	}
	return false
}

func durationReallyChanged(newDuration, oldDuration int64) bool {
	const threshold int64 = 10

	diff := newDuration - oldDuration
	if diff < 0 {
		diff = -diff
	}

	return diff > threshold
}

func sortImagesSliceByType(images []models.ImageFile) {
	sort.SliceStable(images, func(i, j int) bool {
		a := images[i]
		b := images[j]

		typeRank := func(t string) int {
			switch t {
			case "poster":
				return 0
			case "backdrop":
				return 1
			case "season_poster":
				return 2
			case "titlecard":
				return 3
			default:
				return 4
			}
		}

		ar, br := typeRank(a.Type), typeRank(b.Type)
		if ar != br {
			return ar < br
		}

		seasonNum := func(img models.ImageFile) int {
			if img.SeasonNumber == nil {
				return -1
			}
			return *img.SeasonNumber
		}

		// For season posters/titlecards, sort by season number
		if a.Type == "season_poster" || a.Type == "titlecard" {
			as, bs := seasonNum(a), seasonNum(b)
			if as != bs {
				return as < bs
			}
		}

		// Optional: keep titlecards ordered within season
		if a.Type == "titlecard" && b.Type == "titlecard" &&
			a.EpisodeNumber != nil && b.EpisodeNumber != nil &&
			*a.EpisodeNumber != *b.EpisodeNumber {
			return *a.EpisodeNumber < *b.EpisodeNumber
		}

		// Stable fallback
		return a.ID < b.ID
	})
}

func insertRedownloadedSetIntoDB(ctx context.Context, newMediaItem models.MediaItem, newMediuxSet models.PosterSet, dbItem models.DBSavedItem, dbSet models.DBPosterSetDetail) (Err logging.LogErrorInfo) {
	_, insertRedownloadAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Inserting redownloaded set %s into database for item %s", newMediuxSet.Title, utils.MediaItemInfo(newMediaItem)), logging.LevelInfo)
	defer insertRedownloadAction.Complete()

	dbItem.MediaItem = newMediaItem
	newSetInfo := models.DBPosterSetDetail{
		PosterSet: models.PosterSet{
			BaseSetInfo: models.BaseSetInfo{
				ID:          newMediuxSet.ID,
				Type:        newMediuxSet.Type,
				Title:       newMediuxSet.Title,
				UserCreated: newMediuxSet.UserCreated,
				DateCreated: newMediuxSet.DateCreated,
				DateUpdated: newMediuxSet.DateUpdated,
			},
			Images: newMediuxSet.Images,
		},
		LastDownloaded:            time.Now(),
		SelectedTypes:             dbSet.SelectedTypes,
		AutoDownload:              dbSet.AutoDownload,
		AutoAddNewCollectionItems: dbSet.AutoAddNewCollectionItems,
		NoAutoUpgrade:             dbSet.NoAutoUpgrade,
		ToDelete:                  false,
	}
	var found bool
	found, dbItem.PosterSets = utils.UpdatePosterSetInDBItem(dbItem.PosterSets, newSetInfo)
	if !found {
		logging.LOGGER.Error().Timestamp().Str("item", utils.MediaItemInfo(newMediaItem)).Str("set_id", dbSet.ID).Msg("Failed to update set info in DB item after redownloading images for AutoDownload Check, set not found in DB item")
	} else {
		// Update the set info in the database for this item
		Err = database.UpsertSavedItem(ctx, dbItem)
		if Err.Message != "" {
			logging.LOGGER.Error().Timestamp().Str("item", utils.MediaItemInfo(newMediaItem)).Str("set_id", dbSet.ID).Str("error", Err.Message).Msg("Failed to update DB item after redownloading images for AutoDownload Check")
			return Err
		}
	}
	return logging.LogErrorInfo{}
}

func getOverallResults(result *AutoDownloadResult) {
	if len(result.Sets) == 0 {
		result.OverallResult = "skipped"
		result.OverallMessage = "No sets to check"
		return
	} else {
		errorCount := 0
		warningCount := 0
		successCount := 0
		skippedCount := 0
		for _, setResult := range result.Sets {
			switch setResult.Result {
			case "error":
				errorCount++
			case "warning":
				warningCount++
			case "success":
				successCount++
			case "skipped":
				skippedCount++
			}
		}
		if errorCount > 0 {
			result.OverallResult = "error"
			result.OverallMessage = fmt.Sprintf("%d sets had errors, %d sets were successful, %d sets were skipped", errorCount, successCount, skippedCount)
		} else if warningCount > 0 {
			result.OverallResult = "warning"
			result.OverallMessage = fmt.Sprintf("%d sets had warnings, %d sets were successful, %d sets were skipped", warningCount, successCount, skippedCount)
		} else if successCount > 0 {
			result.OverallResult = "success"
			result.OverallMessage = fmt.Sprintf("%d sets were successful, %d sets were skipped", successCount, skippedCount)
		} else {
			result.OverallResult = "skipped"
			result.OverallMessage = "All sets were skipped, no changes detected that require redownloading images"
		}
	}
}

func checkImageDates(
	image models.ImageFile,
	dbSet *models.DBPosterSetDetail,
	oldImageByKey map[string]models.ImageFile,
	imagesToRedownload *[]ImageFileWithReason,
	check *ImageCheckResult,
) {
	key := image.Type + "|" + image.ID
	matchingOldImage, found := oldImageByKey[key]

	if !found {
		check.Outcome = "redownload"
		check.Reason = fmt.Sprintf(
			"New image added to set since last download\nImage Updated New: %s\nLast Downloaded: %s",
			image.Modified.Format("2006-01-02 15:04:05"),
			dbSet.LastDownloaded.Format("2006-01-02 15:04:05"),
		)
		check.Details = map[string]any{
			"image_modified_new": image.Modified.Format("2006-01-02 15:04:05"),
			"last_downloaded":    dbSet.LastDownloaded.Format("2006-01-02 15:04:05"),
		}
		*imagesToRedownload = append(*imagesToRedownload, ImageFileWithReason{
			ImageFile:   image,
			ReasonTitle: "New Image in Set",
			Reason:      check.Reason,
		})
		return
	}

	check.Details = map[string]any{
		"image_modified_new": image.Modified.Format("2006-01-02 15:04:05"),
		"image_modified_old": matchingOldImage.Modified.Format("2006-01-02 15:04:05"),
		"last_downloaded":    dbSet.LastDownloaded.Format("2006-01-02 15:04:05"),
	}

	if !image.Modified.Equal(matchingOldImage.Modified) && image.Modified.After(matchingOldImage.Modified) {
		check.Outcome = "redownload"
		check.Reason = fmt.Sprintf(
			"Image modified date is newer than previous image\nImage Updated New: %s\nImage Updated Old: %s",
			image.Modified.Format("2006-01-02 15:04:05"),
			matchingOldImage.Modified.Format("2006-01-02 15:04:05"),
		)
		*imagesToRedownload = append(*imagesToRedownload, ImageFileWithReason{
			ImageFile:   image,
			ReasonTitle: "Image Updated",
			Reason:      check.Reason,
		})
		return
	} else if image.Modified.After(dbSet.LastDownloaded) {
		check.Outcome = "redownload"
		check.Reason = fmt.Sprintf(
			"Image modified date is newer than last downloaded date\nImage Updated New: %s\nLast Downloaded: %s",
			image.Modified.Format("2006-01-02 15:04:05"),
			dbSet.LastDownloaded.Format("2006-01-02 15:04:05"),
		)

		*imagesToRedownload = append(*imagesToRedownload, ImageFileWithReason{
			ImageFile:   image,
			ReasonTitle: "Image Updated Since Last Download",
			Reason:      check.Reason,
		})
		return
	}

	check.Outcome = "skipped"
	check.Reason = "No changes detected"
}
