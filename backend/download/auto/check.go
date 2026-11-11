package autodownload

import (
	"aura/cache"
	"aura/config"
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
	// Silo has one item for every library it is in, and saving its sets saves them in the other libraries,
	// so an item checked in one library isn't checked again in the others
	checkedSiloItems := map[string]bool{}
	for _, item := range out.Items {
		if config.Current.MediaServer.Type == "Silo" && item.MediaItem.RatingKey != "" {
			key := item.MediaItem.RatingKey + "|" + item.MediaItem.Edition
			if checkedSiloItems[key] {
				skippedCount++
				continue
			}
			checkedSiloItems[key] = true
		}

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

	// The saved sets may have changed since the check started, so use them as they are now
	latest, found, Err := latestSavedItem(ctx, dbItem.MediaItem)
	if Err.Message != "" {
		result.OverallResult = "error"
		result.OverallMessage = "Failed to get the item's saved sets"
		return result
	}
	if !found {
		result.OverallResult = "skipped"
		result.OverallMessage = "The item has no saved sets anymore"
		return result
	}
	dbItem = latest

	// A job already waiting or running for the item downloads what it needs, so it's checked again next time
	if queuedItems(ctx)[dbItem.MediaItem.TMDB_ID+"|"+dbItem.MediaItem.LibraryTitle+"|"+dbItem.MediaItem.Edition] {
		result.OverallResult = "skipped"
		result.OverallMessage = "The item is in the Download Queue, it's checked again next time"
		return result
	}

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
	found, Err = mediaserver.GetMediaItemDetails(ctx, mediaItem)
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

	var updates []models.DBPosterSetDetail
	switch dbItem.MediaItem.Type {
	case "movie":
		result, updates = handleMovie(ctx, *mediaItem, dbItem)
	case "show":
		result, updates = handleShow(ctx, *mediaItem, dbItem)
	default:
		result.OverallResult = "error"
		result.OverallMessage = "Unknown media type"
	}

	// One job downloads the changed images of every set
	if len(updates) > 0 {
		if Err := addToQueue(ctx, models.DBSavedItem{MediaItem: *mediaItem, PosterSets: updates, Source: models.SourceAutoDownload}); Err.Message != "" {
			result.OverallResult = "error"
			result.OverallMessage = fmt.Sprintf("Failed to add the changed images to the Download Queue: %s", Err.Message)
		}
	}
	return result
}

// queuedUpdate is the set as a Download Queue job saves it, downloading only the images that changed, with why
func queuedUpdate(dbSet models.DBPosterSetDetail, latest models.PosterSet, images []ImageFileWithReason) models.DBPosterSetDetail {
	update := dbSet
	update.PosterSet = latest
	update.OnlyImages = make([]models.QueuedImage, 0, len(images))
	for _, image := range images {
		update.OnlyImages = append(update.OnlyImages, models.QueuedImage{
			ID:          image.ID,
			Type:        image.Type,
			ReasonTitle: image.ReasonTitle,
			Reason:      image.Reason,
		})
	}
	return update
}

// latestSavedItem is the item's saved sets as they are in the database now
func latestSavedItem(ctx context.Context, item models.MediaItem) (saved models.DBSavedItem, found bool, Err logging.LogErrorInfo) {
	out, Err := getSavedSets(ctx, models.DBFilter{ItemTMDB_ID: item.TMDB_ID, ItemLibraryTitle: item.LibraryTitle, ItemsPerPage: -1})
	if Err.Message != "" {
		return saved, false, Err
	}
	for _, savedItem := range out.Items {
		if savedItem.MediaItem.Edition == item.Edition {
			return savedItem, true, logging.LogErrorInfo{}
		}
	}
	return saved, false, logging.LogErrorInfo{}
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

// formatReasonDate is a date for a reason, in the server's time zone with the zone, as MediUX dates are in UTC
func formatReasonDate(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05 MST")
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
			formatReasonDate(image.Modified),
			formatReasonDate(dbSet.LastDownloaded),
		)
		check.Details = map[string]any{
			"image_modified_new": formatReasonDate(image.Modified),
			"last_downloaded":    formatReasonDate(dbSet.LastDownloaded),
		}
		*imagesToRedownload = append(*imagesToRedownload, ImageFileWithReason{
			ImageFile:   image,
			ReasonTitle: "New Image in Set",
			Reason:      check.Reason,
		})
		return
	}

	check.Details = map[string]any{
		"image_modified_new": formatReasonDate(image.Modified),
		"image_modified_old": formatReasonDate(matchingOldImage.Modified),
		"last_downloaded":    formatReasonDate(dbSet.LastDownloaded),
	}

	if !image.Modified.Equal(matchingOldImage.Modified) && image.Modified.After(matchingOldImage.Modified) {
		check.Outcome = "redownload"
		check.Reason = fmt.Sprintf(
			"Image modified date is newer than previous image\nImage Updated New: %s\nImage Updated Old: %s",
			formatReasonDate(image.Modified),
			formatReasonDate(matchingOldImage.Modified),
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
			formatReasonDate(image.Modified),
			formatReasonDate(dbSet.LastDownloaded),
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
