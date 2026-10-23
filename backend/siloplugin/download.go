package siloplugin

import (
	"aura/database"
	autodownload "aura/download/auto"
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"fmt"
	"strings"
	"sync"
)

// Set in tests
var (
	getImage     = mediux.GetImage
	getQueueJobs = database.GetAllDownloadQueueJobs
	addToQueue   = downloadqueue.AddToQueue
)

// queueMu makes sure two downloads at once don't queue the same set twice
var queueMu sync.Mutex

// Download returns the original image for a token, then queues its set in the background when the item doesn't have it saved
func Download(ctx context.Context, token DownloadToken, autoDownload bool) (imageData []byte, imageType string, Err logging.LogErrorInfo) {
	imageData, imageType, Err = getImage(ctx, token.ImageID, token.Modified.Format("20060102150405"), mediux.ImageQualityOriginal)
	if Err.Message != "" {
		return nil, "", Err
	}

	QueueInBackground(token, autoDownload)
	return imageData, imageType, logging.LogErrorInfo{}
}

// QueueInBackground queues the token's set without holding up the request
func QueueInBackground(token DownloadToken, autoDownload bool) {
	go func() {
		defer logging.RecoverPanic("Silo Plugin Download")
		ctx, ld := logging.CreateLoggingContext(context.Background(), "Silo Plugin Download")
		logAction := ld.AddAction(fmt.Sprintf("Queue Set %s for %s %s", token.SetID, token.ItemType, token.TMDB_ID), logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, logAction)
		defer ld.Log()
		QueueSetForDownload(ctx, token, autoDownload)
	}()
}

// QueueSetForDownload adds the token's set to the Download Queue for each library item that doesn't have it saved.
// From a refresh, only a favorite creator's set is queued.
// An item without a saved set gets the whole set, from the Download Defaults.
// An item with another saved set only gets the downloaded image's type, so its saved set isn't replaced.
// With autoDownload, the set is saved with Auto Download on.
func QueueSetForDownload(ctx context.Context, token DownloadToken, autoDownload bool) (queued int) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Silo Plugin: Queueing the Downloaded Set", logging.LevelDebug)
	defer logAction.Complete()

	// Silo picked the image on a refresh, only a favorite creator's set is saved for it
	fromFavorite := !token.Refresh || isFavorite(ctx, token.Creator)

	queueMu.Lock()
	defer queueMu.Unlock()

	items := libraryItems(token.TMDB_ID, NormalizeItemType(token.ItemType))
	if len(items) == 0 {
		logAction.AppendResult("skipped", "the item isn't in aura's libraries")
		return 0
	}

	// Items already waiting in the Download Queue
	waiting := map[string]bool{}
	jobs, _ := getQueueJobs(ctx)
	for _, job := range jobs {
		if job.Status == "pending" || job.Status == "processing" {
			waiting[job.TMDB_ID+"|"+job.LibraryTitle+"|"+job.Edition] = true
		}
	}

	var set *models.SetRef
	for _, item := range items {
		key := item.TMDB_ID + "|" + item.LibraryTitle + "|" + item.Edition
		switch {
		case item.IgnoredInDB:
			logAction.AppendResult(key, "ignored")
			continue
		case hasSavedSet(item, token.SetID):
			logAction.AppendResult(key, "the set is already saved")
			continue
		case waiting[key]:
			logAction.AppendResult(key, "already in the Download Queue")
			continue
		case !fromFavorite:
			logAction.AppendResult(key, fmt.Sprintf("%s isn't a favorite creator", token.Creator))
			continue
		}

		// The set is fetched once, for the first item that needs it
		if set == nil {
			found, Err := findSet(ctx, item, token.SetID)
			if Err.Message != "" {
				logAction.SetErrorFromInfo(Err)
				return queued
			}
			set = &found
		}

		selected := autodownload.DownloadDefaultTypes()
		if len(item.DBSavedSets) > 0 {
			selected = onlyType(token.ImageType, token.SeasonNumber)
		}
		Err := addToQueue(ctx, models.DBSavedItem{
			MediaItem:  item,
			PosterSets: []models.DBPosterSetDetail{{PosterSet: set.PosterSet, SelectedTypes: selected, AutoDownload: autoDownload}},
			Source:     models.SourceSiloPlugin,
		})
		if Err.Message != "" {
			logAction.AppendWarning(key, Err.Message)
			continue
		}
		queued++
		logAction.AppendResult(key, fmt.Sprintf("queued %s by %s", set.Title, set.UserCreated))
	}
	return queued
}

func isFavorite(ctx context.Context, username string) bool {
	for _, favorite := range getFavorites(ctx) {
		if strings.EqualFold(favorite, username) {
			return true
		}
	}
	return false
}

func hasSavedSet(item models.MediaItem, setID string) bool {
	for _, set := range item.DBSavedSets {
		if set.ID == setID {
			return true
		}
	}
	return false
}

// findSet returns one of the item's sets by ID
func findSet(ctx context.Context, item models.MediaItem, setID string) (models.SetRef, logging.LogErrorInfo) {
	sets, Err := itemSets(ctx, item)
	if Err.Message != "" {
		return models.SetRef{}, Err
	}
	for _, set := range sets {
		if set.ID == setID {
			return set, logging.LogErrorInfo{}
		}
	}
	return models.SetRef{}, logging.LogErrorInfo{Message: fmt.Sprintf("Set %s isn't one of the item's sets on MediUX", setID)}
}
