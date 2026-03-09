package plex

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
	"net/url"
	"path"
	"strings"
	"time"
)

func (p *Plex) GetLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string, enableSortByNewEpisode bool) (items []models.MediaItem, totalSize int, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Fetching Items for Library Section: %s", section.Title,
	), logging.LevelInfo)
	defer logAction.Complete()

	items = []models.MediaItem{}
	totalSize = 0
	Err = logging.LogErrorInfo{}

	// If limit is empty, set a default limit
	if limit == "" {
		limit = "1000"
	}

	// Construct the URL for the Plex library sections API request
	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return items, totalSize, *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "sections", section.ID, "all")
	query := u.Query()
	query.Set("X-Plex-Container-Start", sectionStartIndex)
	query.Set("X-Plex-Container-Size", limit)
	query.Set("includeGuids", "1")
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, config.Current.MediaServer, URL, "GET", nil)
	if Err.Message != "" {
		return items, totalSize, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var plexResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex Library Items Response")
	if Err.Message != "" {
		return items, totalSize, *logAction.Error
	}

	totalSize = plexResp.MediaContainer.TotalSize

	for _, metadata := range plexResp.MediaContainer.Metadata {
		var item models.MediaItem
		item.RatingKey = metadata.RatingKey
		item.Type = metadata.Type
		item.Title = metadata.Title
		item.Year = metadata.Year
		item.LibraryTitle = plexResp.MediaContainer.LibrarySectionTitle
		item.UpdatedAt = metadata.UpdatedAt
		item.AddedAt = metadata.AddedAt
		item.ContentRating = metadata.ContentRating
		item.Summary = metadata.Summary

		if t, err := time.Parse("2006-01-02", metadata.OriginallyAvailableAt); err == nil {
			item.ReleasedAt = t.Unix()
		} else {
			item.ReleasedAt = 0
		}

		if metadata.Type == "movie" {
			item.Movie = &models.MediaItemMovie{
				File: models.MediaItemFile{
					Path:     metadata.Media[0].Part[0].File,
					Size:     metadata.Media[0].Part[0].Size,
					Duration: metadata.Media[0].Part[0].Duration,
				},
			}
		}

		if len(metadata.Guid) > 0 {
			for _, guid := range metadata.Guids {
				if guid.ID != "" {
					// Sample guid.id : tmdb://######
					// Split into provider and id
					parts := strings.Split(guid.ID, "://")
					if len(parts) == 2 {
						provider := parts[0]
						id := parts[1]
						item.Guids = append(item.Guids, models.MediaItemGuid{
							Provider: provider,
							ID:       id,
						})
						if provider == "tmdb" {
							item.TMDB_ID = id
						}
					}
				}
			}
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
			totalSize-- // Decrement total size as this item will be skipped
			continue    // Skip items without TMDB ID
		}

		// Check if Media Item exists in DB
		ignored, ignoredMode, sets, logErr := database.CheckIfMediaItemExists(ctx, item.TMDB_ID, item.LibraryTitle)
		if logErr.Message != "" {
			logAction.AppendWarning("message", "Failed to check if media item exists in database")
			logAction.AppendWarning("error", Err)
		}
		if !ignored {
			item.DBSavedSets = sets
		} else {
			item.IgnoredInDB = true
			item.IgnoredMode = ignoredMode
		}

		// Check if Media Item exists in MediUX with a set
		if cache.MediuxItems.CheckItemExists(item.Type, item.TMDB_ID) {
			item.HasMediuxSets = true
		}

		// If the item is a movie, update the movie collections cache
		if item.Type == "movie" {
			if len(metadata.Collections) > 0 {
				for _, coll := range metadata.Collections {
					cache.CollectionsStore.UpdateMediaItemInCollectionByTitle(coll.Tag, &item)
				}
			}
		}

		cache.LibraryStore.UpdateMediaItem(section.Title, &item)
		items = append(items, item)
	}

	// For show sections, bulk-fetch all episodes to compute LatestEpisodeAddedAt per show.
	// This requires an extra API call and can be slow for large libraries.
	// Skip it when the user has disabled "Sort by New Episode Added".
	if section.Type == "show" && enableSortByNewEpisode {
		latestEpAdded, fetchErr := fetchLatestEpisodeAddedAtByShow(ctx, section.ID)
		if fetchErr.Message != "" {
			logAction.AppendWarning("latest_episode_added_at", "Failed to bulk-fetch latest episode addedAt for shows")
		} else {
			for i := range items {
				items[i].LatestEpisodeAddedAt = latestEpAdded[items[i].RatingKey]
			}
		}
	}

	return items, totalSize, logging.LogErrorInfo{}
}

// fetchLatestEpisodeAddedAtByShow fetches all episodes for a library section in one bulk
// request and returns a map of show RatingKey -> latest episode addedAt timestamp.
func fetchLatestEpisodeAddedAtByShow(ctx context.Context, sectionID string) (map[string]int64, logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Bulk-fetching episode addedAt for section %s", sectionID,
	), logging.LevelDebug)
	defer logAction.Complete()

	logging.DevMsgf("Bulk-fetching latest episode addedAt for shows in section %s", sectionID)

	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return nil, *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "sections", sectionID, "all")

	// First pass: get total episode count (size=0 returns totalSize without data)
	query := u.Query()
	query.Set("type", "4") // 4 = episode
	query.Set("X-Plex-Container-Start", "0")
	query.Set("X-Plex-Container-Size", "0")
	u.RawQuery = query.Encode()

	resp, respBody, Err := makeRequest(ctx, config.Current.MediaServer, u.String(), "GET", nil)
	if Err.Message != "" {
		return nil, *logAction.Error
	}
	resp.Body.Close()

	var countResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &countResp, "Plex Episode Count Response")
	if Err.Message != "" {
		return nil, *logAction.Error
	}
	totalEpisodes := countResp.MediaContainer.TotalSize
	if totalEpisodes == 0 {
		return map[string]int64{}, logging.LogErrorInfo{}
	}

	// Second pass: fetch all episodes in one shot
	query.Set("X-Plex-Container-Size", fmt.Sprintf("%d", totalEpisodes))
	u.RawQuery = query.Encode()

	resp, respBody, Err = makeRequest(ctx, config.Current.MediaServer, u.String(), "GET", nil)
	if Err.Message != "" {
		return nil, *logAction.Error
	}
	resp.Body.Close()

	var episodesResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &episodesResp, "Plex All Episodes Response")
	if Err.Message != "" {
		return nil, *logAction.Error
	}

	latest := make(map[string]int64)
	for _, ep := range episodesResp.MediaContainer.Metadata {
		showKey := ep.GrandParentRatingKey
		if ep.AddedAt > latest[showKey] {
			latest[showKey] = ep.AddedAt
		}
	}

	return latest, logging.LogErrorInfo{}
}
