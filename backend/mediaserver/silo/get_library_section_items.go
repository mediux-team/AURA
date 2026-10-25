package silo

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"fmt"
	"strconv"
	"time"
)

func (s *Silo) GetLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string) (items []models.MediaItem, totalSize int, fetched int, Err logging.LogErrorInfo) {
	return s.getLibrarySectionItems(ctx, section, sectionStartIndex, limit, false)
}

// GetNewestLibrarySectionItems gets a library's items, newest first
func (s *Silo) GetNewestLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string) (items []models.MediaItem, totalSize int, fetched int, Err logging.LogErrorInfo) {
	return s.getLibrarySectionItems(ctx, section, sectionStartIndex, limit, true)
}

func (s *Silo) getLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string, newestFirst bool) (items []models.MediaItem, totalSize int, fetched int, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Fetching Items for Library Section: %s", section.Title,
	), logging.LevelInfo)
	defer logAction.Complete()

	items = []models.MediaItem{}
	totalSize = 0
	Err = logging.LogErrorInfo{}

	// If limit is empty, set a default limit
	if limit == "" {
		limit = "500"
	}
	startIndex, err := strconv.Atoi(sectionStartIndex)
	if err != nil || startIndex < 0 {
		logAction.SetError("Invalid start index", "The start index must be a positive number", map[string]any{"start_index": sectionStartIndex})
		return items, totalSize, fetched, *logAction.Error
	}
	limitCount, err := strconv.Atoi(limit)
	if err != nil || limitCount < 1 {
		logAction.SetError("Invalid limit", "The limit must be a positive number", map[string]any{"limit": limit})
		return items, totalSize, fetched, *logAction.Error
	}

	// Get the Catalog Items for the Library Section
	sort := "title"
	if newestFirst {
		sort = "-added_at"
	}
	catalogItems, totalSize, Err := getCatalogItems(ctx, s.Config, section, startIndex, limitCount, sort)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return items, 0, 0, *logAction.Error
	}
	fetched = len(catalogItems)

	// Check to see if any items were returned
	if len(catalogItems) == 0 {
		if startIndex == 0 {
			logAction.AppendWarning("message", fmt.Sprintf("Library Section '%s' is empty", section.Title))
		}
		return items, totalSize, fetched, logging.LogErrorInfo{}
	}

	// Saved sets and ignored state for the whole page
	dbStates, dbErr := database.GetLibraryDBStates(ctx, section.Title)
	if dbErr.Message != "" {
		logAction.AppendWarning("db_states_error", dbErr.Message)
	}

	for _, catalogItem := range catalogItems {
		var item models.MediaItem
		item.RatingKey = catalogItem.ContentID
		item.Type = map[string]string{
			"movie":  "movie",
			"series": "show",
		}[catalogItem.Type]
		item.Title = catalogItem.Title
		item.Year = catalogItem.Year
		item.LibraryTitle = section.Title
		item.ContentRating = catalogItem.ContentRating
		item.Summary = catalogItem.Overview
		if !catalogItem.AddedAt.IsZero() {
			item.AddedAt = catalogItem.AddedAt.Unix()
		}

		if item.Type == "" {
			logAction.AppendWarning(fmt.Sprintf("skipped '%s'", catalogItem.Title), map[string]string{"type": catalogItem.Type})
			continue
		}

		// Get the Provider IDs for the item
		// If this fails, return an error instead of skipping the item so that it is not treated as removed
		itemInfo, Err := getItemInfo(ctx, s.Config, catalogItem.ContentID)
		if Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return []models.MediaItem{}, 0, 0, *logAction.Error
		}

		// Movies have a release date, Series have a first air date
		releaseDate := catalogItem.ReleaseDate
		if releaseDate == "" {
			releaseDate = itemInfo.FirstAirDate
		}
		if t, err := time.Parse("2006-01-02", releaseDate); err == nil {
			item.ReleasedAt = t.Unix()
		}

		if itemInfo.TMDB_ID != "" {
			item.Guids = append(item.Guids, models.MediaItemGuid{Provider: "tmdb", ID: itemInfo.TMDB_ID})
			item.TMDB_ID = itemInfo.TMDB_ID
		}
		if item.Type == "show" {
			item.SeasonCount, item.EpisodeCount = itemInfo.SeasonCount, itemInfo.EpisodeCount
		}
		if itemInfo.IMDB_ID != "" {
			item.Guids = append(item.Guids, models.MediaItemGuid{Provider: "imdb", ID: itemInfo.IMDB_ID})
		}
		if itemInfo.TVDB_ID != "" {
			item.Guids = append(item.Guids, models.MediaItemGuid{Provider: "tvdb", ID: itemInfo.TVDB_ID})
		}

		// If no TMDB ID found, get the value from MediUX using the GUID[tvdb]
		if item.TMDB_ID == "" {
			for _, guid := range item.Guids {
				if guid.Provider == "tvdb" {
					tmdbID, found, Err := mediux.SearchTMDBIDByTVDBID(ctx, guid.ID, item.Type)
					if Err.Message != "" {
						logAction.AppendWarning("search_tmdb_id_error", "Failed to search TMDB ID from MediUX")
					}
					if found {
						item.TMDB_ID = tmdbID
						break
					}
				}
			}
		}
		if item.TMDB_ID == "" {
			logging.LOGGER.Warn().Timestamp().Str("item_title", item.Title).Str("library_section", section.Title).Msgf("Skipping item in '%s' as no TMDB ID could be found", section.Title)
			continue // Skip items without TMDB ID
		}

		// Saved sets and ignored state from the database
		database.ApplyDBState(dbStates, &item)

		// Check if Media Item exists in MediUX with a set
		if cache.MediuxItems.CheckItemExists(item.Type, item.TMDB_ID) {
			item.HasMediuxSets = true
		}

		// Only the main media server's items go in the main library cache
		if s.Config.IsMain() {
			cache.LibraryStore.UpdateMediaItem(section.Title, &item)
		}
		items = append(items, item)
	}

	return items, totalSize, fetched, logging.LogErrorInfo{}
}

func getCatalogItems(ctx context.Context, msConfig config.Config_MediaServer, section models.LibrarySection, startIndex int, limit int, sort string) (catalogItems []SiloCatalogItem, totalSize int, Err logging.LogErrorInfo) {
	catalogItems = []SiloCatalogItem{}
	totalSize = 0

	catalogType := map[string]string{
		"movie": "movie",
		"show":  "series",
		"mixed": "video",
	}[section.Type]

	// Silo returns at most 200 items per page
	// Seek to the start index, then follow the cursor until the limit is reached
	// The cursor only works with the same page size, so keep it the same for every page
	pageSize := min(limit, 200)
	cursor := ""
	for len(catalogItems) < limit {
		// Construct the URL for the Silo server API request
		u, err := apiURL(msConfig.URL, "catalog")
		if err != nil {
			message, help, detail := logging.Error_BaseUrlParsing(err)
			return catalogItems, totalSize, logging.LogErrorInfo{Message: message, Help: help, Detail: detail}
		}
		query := u.Query()
		query.Set("library_id", section.ID)
		if catalogType != "" {
			query.Set("type", catalogType)
		}
		query.Set("sort", sort)
		query.Set("limit", strconv.Itoa(pageSize))
		if cursor == "" {
			query.Set("seek", strconv.Itoa(startIndex))
		} else {
			query.Set("cursor", cursor)
		}
		u.RawQuery = query.Encode()

		// Make the HTTP Request to Silo
		_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
		if Err.Message != "" {
			return catalogItems, totalSize, Err
		}

		// Decode the Response
		var siloResp SiloCatalogResponse
		Err = httpx.DecodeResponseToJSON(ctx, respBody, &siloResp, "Silo Library Section Items Response")
		if Err.Message != "" {
			return catalogItems, totalSize, Err
		}

		if cursor == "" {
			totalSize = siloResp.Total
		}
		catalogItems = append(catalogItems, siloResp.Items...)

		if !siloResp.Page.HasMore || siloResp.Page.NextCursor == "" || len(siloResp.Items) == 0 {
			break
		}
		cursor = siloResp.Page.NextCursor
	}

	// The last page may have more items than needed
	if len(catalogItems) > limit {
		catalogItems = catalogItems[:limit]
	}

	return catalogItems, totalSize, logging.LogErrorInfo{}
}
