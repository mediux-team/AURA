package downloadqueue

import (
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"fmt"
	"slices"
)

// Set in tests
var (
	getHistoryEntry = database.GetDownloadHistoryEntry
	getDueRetries   = database.GetDueDownloadRetries
	markRetried     = database.MarkDownloadHistoryRetried
	stopRetries     = database.StopDownloadRetries
	getSavedSets    = database.GetAllSavedSets
	getQueueJobs    = database.GetAllDownloadQueueJobs
	addToQueue      = AddToQueue
	getLatestSet    = mediux.GetLatestSet

	saveDownloadedImage = database.SaveDownloadedImage
)

// RetryFailedDownload adds a failed download's set back to the Download Queue, from the item's saved set
func RetryFailedDownload(ctx context.Context, historyID int64) (title string, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Retry Failed Download %d", historyID), logging.LevelInfo)
	defer logAction.Complete()

	entry, found, Err := getHistoryEntry(ctx, historyID)
	if Err.Message != "" {
		return "", Err
	}
	if !found {
		logAction.SetError("This failed download no longer exists", "Refresh the page", map[string]any{"id": historyID})
		return "", *logAction.Error
	}

	alreadyQueued, Err := retryEntry(ctx, entry)
	if alreadyQueued {
		logAction.SetError(fmt.Sprintf("%s is already in the Download Queue", entry.MediaItemTitle), "Wait for it to finish", map[string]any{"id": historyID})
		return entry.MediaItemTitle, *logAction.Error
	}
	return entry.MediaItemTitle, Err
}

// retryDueDownloads adds the failed downloads due for an automatic retry back to the Download Queue
func retryDueDownloads(ctx context.Context) {
	ctx, ld := logging.CreateLoggingContext(ctx, "Download Queue Worker")
	logAction := ld.AddAction("Retry Failed Downloads", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	entries, Err := getDueRetries(ctx)
	if Err.Message == "" && len(entries) == 0 {
		return
	}
	defer ld.Log()
	defer logAction.Complete()

	for _, entry := range entries {
		key := fmt.Sprintf("%s (set %s)", entry.MediaItemTitle, entry.SetID)
		alreadyQueued, Err := retryEntry(ctx, entry)
		switch {
		case alreadyQueued:
			// Tried again on the next check
			logAction.AppendResult(key, "already in the Download Queue")
		case Err.Message != "":
			// It can't be retried, like a set that isn't saved anymore
			_ = stopRetries(ctx, entry.ID)
		default:
			logAction.AppendResult(key, fmt.Sprintf("retry %d of %d", entry.RetryCount+1, len(database.DownloadRetryDelays)))
		}
	}
}

// retryEntry queues the failed set again and counts the retry, alreadyQueued is whether the item is waiting in the Download Queue
func retryEntry(ctx context.Context, entry database.DownloadHistoryEntry) (alreadyQueued bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Retry %s", entry.MediaItemTitle), logging.LevelDebug)
	defer logAction.Complete()

	// A failure from before a set was downloaded has nothing to retry
	if entry.SetID == "" {
		logAction.SetError(fmt.Sprintf("%s has no set to retry", entry.MediaItemTitle), "Download a set from the item's page", map[string]any{"id": entry.ID})
		return false, *logAction.Error
	}

	jobs, Err := getQueueJobs(ctx)
	if Err.Message != "" {
		return false, Err
	}
	for _, job := range jobs {
		if job.TMDB_ID == entry.TMDB_ID && job.LibraryTitle == entry.LibraryTitle && job.Edition == entry.Edition &&
			(job.Status == "pending" || job.Status == "processing") {
			return true, logging.LogErrorInfo{}
		}
	}

	saved, Err := getSavedSets(ctx, models.DBFilter{
		ItemTMDB_ID:      entry.TMDB_ID,
		ItemLibraryTitle: entry.LibraryTitle,
		SetID:            entry.SetID,
		ItemsPerPage:     -1,
	})
	if Err.Message != "" {
		return false, Err
	}
	for _, item := range saved.Items {
		if item.MediaItem.Edition != entry.Edition {
			continue
		}
		for _, set := range item.PosterSets {
			if set.ID != entry.SetID {
				continue
			}
			// The set's latest images, as images that failed before weren't saved
			if latest, latestErr := getLatestSet(ctx, set.ID, set.Type, item.MediaItem); latestErr.Message == "" && latest.ID == set.ID {
				set.PosterSet = latest
				// Only the images that failed, the others downloaded fine
				set.OnlyImages = failedImages(latest.Images, entry.FailedImages)
			} else {
				logAction.AppendWarning("latest_set", "Couldn't get the set's latest images from MediUX, retrying with the saved images")
			}
			// Only the failed set, the item's other saved sets downloaded fine
			Err = addToQueue(ctx, models.DBSavedItem{MediaItem: item.MediaItem, PosterSets: []models.DBPosterSetDetail{set}, Source: models.SourceRetry})
			if Err.Message != "" {
				return false, Err
			}
			if Err = markRetried(ctx, entry.ID); Err.Message != "" {
				return false, Err
			}
			logAction.AppendResult("queued", fmt.Sprintf("%s set %s", entry.MediaItemTitle, set.ID))
			return false, logging.LogErrorInfo{}
		}
	}

	logAction.SetError(fmt.Sprintf("Set %s isn't saved for %s", entry.SetID, entry.MediaItemTitle), "Download the set again from the item's page", map[string]any{"id": entry.ID})
	return false, *logAction.Error
}

// failedImages is the set's images that failed, or nil to download the whole set when a failure isn't for one of its images
func failedImages(images []models.ImageFile, failed []models.ImageDownloadResult) []models.QueuedImage {
	if len(failed) == 0 {
		return nil
	}
	queued := []models.QueuedImage{}
	for _, failure := range failed {
		i := slices.IndexFunc(images, func(image models.ImageFile) bool {
			return image.Type == failure.ImageType && sameNumber(image.SeasonNumber, failure.SeasonNumber) && sameNumber(image.EpisodeNumber, failure.EpisodeNumber)
		})
		if failure.ImageType == "" || i < 0 {
			return nil
		}
		queued = append(queued, models.QueuedImage{
			ID:          images[i].ID,
			Type:        images[i].Type,
			ReasonTitle: "Retry",
			Reason:      "The image failed to download before",
		})
	}
	return queued
}

func sameNumber(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
