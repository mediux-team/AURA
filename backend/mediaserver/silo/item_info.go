package silo

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"context"
	"sync"
	"time"
)

// Silo's catalog listing does not include provider IDs, so the details of each item are needed to get them.
// The provider IDs are kept in memory and saved to the database, so they are not retrieved again after a restart.

type siloItemInfo struct {
	TMDB_ID      string
	IMDB_ID      string
	TVDB_ID      string
	FirstAirDate string
	SeasonCount  int
	EpisodeCount int
	UpdatedAt    time.Time
}

// Item info older than this is retrieved from Silo again
const itemInfoMaxAge = 7 * 24 * time.Hour

// Item info that has not changed is only saved to the database again after this
const itemInfoSaveAge = 24 * time.Hour

var (
	itemInfoMu     sync.RWMutex
	itemInfoStore  = map[string]siloItemInfo{} // Key: Silo Content ID
	itemInfoLoaded bool
)

func getItemInfo(ctx context.Context, msConfig config.Config_MediaServer, contentID string) (itemInfo siloItemInfo, Err logging.LogErrorInfo) {
	// Load the saved item info from the database
	loadItemInfo(ctx)

	// Check if the item info was already retrieved
	itemInfoMu.RLock()
	itemInfo, found := itemInfoStore[contentID]
	itemInfoMu.RUnlock()
	if found && time.Since(itemInfo.UpdatedAt) < itemInfoMaxAge {
		return itemInfo, logging.LogErrorInfo{}
	}

	// Get the Media Item details from Silo, which also saves the item info
	siloResp, Err := getCatalogItemDetail(ctx, msConfig, contentID, "")
	if Err.Message != "" {
		return itemInfo, Err
	}

	itemInfo = siloItemInfo{
		TMDB_ID:      siloResp.TMDB_ID,
		IMDB_ID:      siloResp.IMDB_ID,
		TVDB_ID:      siloResp.TVDB_ID,
		FirstAirDate: siloResp.FirstAirDate,
		SeasonCount:  siloResp.SeasonCount,
		EpisodeCount: siloResp.EpisodeCount,
		UpdatedAt:    time.Now(),
	}
	return itemInfo, logging.LogErrorInfo{}
}

func loadItemInfo(ctx context.Context) {
	itemInfoMu.Lock()
	defer itemInfoMu.Unlock()
	if itemInfoLoaded {
		return
	}

	// If the database is not ready yet, try again next time
	savedItems, Err := database.GetAllSiloItemIDs(ctx)
	if Err.Message != "" {
		return
	}

	for _, savedItem := range savedItems {
		// Keep any item info that was retrieved before the database was loaded
		if _, exists := itemInfoStore[savedItem.ContentID]; exists {
			continue
		}
		itemInfoStore[savedItem.ContentID] = siloItemInfo{
			TMDB_ID:      savedItem.TMDB_ID,
			IMDB_ID:      savedItem.IMDB_ID,
			TVDB_ID:      savedItem.TVDB_ID,
			FirstAirDate: savedItem.FirstAirDate,
			SeasonCount:  savedItem.SeasonCount,
			EpisodeCount: savedItem.EpisodeCount,
			UpdatedAt:    savedItem.UpdatedAt,
		}
	}
	itemInfoLoaded = true
}

func saveItemInfo(ctx context.Context, siloResp SiloCatalogItemDetailResponse) {
	if siloResp.ContentID == "" {
		return
	}
	itemInfo := siloItemInfo{
		TMDB_ID:      siloResp.TMDB_ID,
		IMDB_ID:      siloResp.IMDB_ID,
		TVDB_ID:      siloResp.TVDB_ID,
		FirstAirDate: siloResp.FirstAirDate,
		SeasonCount:  siloResp.SeasonCount,
		EpisodeCount: siloResp.EpisodeCount,
		UpdatedAt:    time.Now(),
	}

	// Only save the item info if it is new, has changed, or was saved a while ago
	itemInfoMu.Lock()
	existing, found := itemInfoStore[siloResp.ContentID]
	unchanged := found &&
		existing.TMDB_ID == itemInfo.TMDB_ID &&
		existing.IMDB_ID == itemInfo.IMDB_ID &&
		existing.TVDB_ID == itemInfo.TVDB_ID &&
		existing.FirstAirDate == itemInfo.FirstAirDate &&
		existing.SeasonCount == itemInfo.SeasonCount &&
		existing.EpisodeCount == itemInfo.EpisodeCount &&
		time.Since(existing.UpdatedAt) < itemInfoSaveAge
	if unchanged {
		itemInfoMu.Unlock()
		return
	}
	itemInfoStore[siloResp.ContentID] = itemInfo
	itemInfoMu.Unlock()

	// Save the item info to the database
	// If this fails, the item info is still kept in memory
	Err := database.UpsertSiloItemIDs(ctx, []database.SiloItemIDs{{
		ContentID:    siloResp.ContentID,
		TMDB_ID:      itemInfo.TMDB_ID,
		IMDB_ID:      itemInfo.IMDB_ID,
		TVDB_ID:      itemInfo.TVDB_ID,
		FirstAirDate: itemInfo.FirstAirDate,
		SeasonCount:  itemInfo.SeasonCount,
		EpisodeCount: itemInfo.EpisodeCount,
		UpdatedAt:    itemInfo.UpdatedAt,
	}})
	if Err.Message != "" {
		logging.LOGGER.Debug().Timestamp().Str("content_id", siloResp.ContentID).Str("error", Err.Message).
			Msg("Failed to save Silo item info to the database")
	}
}
