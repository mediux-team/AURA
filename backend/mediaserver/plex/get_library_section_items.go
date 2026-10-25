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

func (p *Plex) GetLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string) (items []models.MediaItem, totalSize int, fetched int, Err logging.LogErrorInfo) {
	return p.getLibrarySectionItems(ctx, section, sectionStartIndex, limit, false)
}

// GetNewestLibrarySectionItems gets a library's items, newest first
func (p *Plex) GetNewestLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string) (items []models.MediaItem, totalSize int, fetched int, Err logging.LogErrorInfo) {
	return p.getLibrarySectionItems(ctx, section, sectionStartIndex, limit, true)
}

func (p *Plex) getLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string, newestFirst bool) (items []models.MediaItem, totalSize int, fetched int, Err logging.LogErrorInfo) {
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
	u, err := url.Parse(p.Config.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return items, totalSize, fetched, *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "sections", section.ID, "all")
	query := u.Query()
	query.Set("X-Plex-Container-Start", sectionStartIndex)
	query.Set("X-Plex-Container-Size", limit)
	query.Set("includeGuids", "1")
	if newestFirst {
		query.Set("sort", "addedAt:desc")
	}
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, p.Config, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return items, totalSize, fetched, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var plexResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex Library Items Response")
	if Err.Message != "" {
		return items, totalSize, fetched, Err
	}

	totalSize = plexResp.MediaContainer.TotalSize
	fetched = len(plexResp.MediaContainer.Metadata)

	// Saved sets and ignored state for the whole page
	dbStates, dbErr := database.GetLibraryDBStates(ctx, section.Title)
	if dbErr.Message != "" {
		logAction.AppendWarning("db_states_error", dbErr.Message)
	}

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
		item.Edition = metadata.Edition

		if item.Title == "" {
			if metadata.OriginalTitle != "" {
				item.Title = metadata.OriginalTitle
			} else {
				item.Title = "<Unknown Title>"
			}
		}

		if t, err := time.Parse("2006-01-02", metadata.OriginallyAvailableAt); err == nil {
			item.ReleasedAt = t.Unix()
		} else {
			item.ReleasedAt = 0
		}

		if metadata.Type == "movie" {
			item.Movie = &models.MediaItemMovie{File: firstFile(metadata.Media)}
		}
		if metadata.Type == "show" {
			item.SeasonCount, item.EpisodeCount = metadata.ChildCount, metadata.LeafCount
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
			continue // Skip items without TMDB ID
		}

		// Saved sets and ignored state from the database
		database.ApplyDBState(dbStates, &item)

		// Check if Media Item exists in MediUX with a set
		if cache.MediuxItems.CheckItemExists(item.Type, item.TMDB_ID) {
			item.HasMediuxSets = true
		}

		// If the item is a movie, update the movie collections cache
		if item.Type == "movie" && p.Config.IsMain() {
			if len(metadata.Collections) > 0 {
				for _, coll := range metadata.Collections {
					cache.CollectionsStore.UpdateMediaItemInCollectionByTitle(coll.Tag, &item)
				}
			}
		}

		// Only the main media server's items go in the main library cache
		if p.Config.IsMain() {
			cache.LibraryStore.UpdateMediaItem(section.Title, &item)
		}
		items = append(items, item)
	}

	// For show sections, bulk-fetch all episodes to compute LatestEpisodeAddedAt per show.
	// Skipped for the newest items, it gets every episode of the library
	if section.Type == "show" && p.Config.EnableSortByEpisodeAddedDate && !newestFirst {
		latestEpAdded, fetchErr := fetchLatestEpisodeAddedAtByShow(ctx, p.Config, section.ID)
		if fetchErr.Message != "" {
			logAction.AppendWarning("latest_episode_added_at", "Failed to bulk-fetch latest episode addedAt for shows")
		} else {
			for i := range items {
				items[i].LatestEpisodeAddedAt = latestEpAdded[items[i].RatingKey]
			}
		}
	}

	return items, totalSize, fetched, logging.LogErrorInfo{}
}

// fetchLatestEpisodeAddedAtByShow fetches all episodes for a library section in one bulk
// request and returns a map of show RatingKey -> latest episode addedAt timestamp.
func fetchLatestEpisodeAddedAtByShow(ctx context.Context, msConfig config.Config_MediaServer, sectionID string) (map[string]int64, logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Bulk-fetching episode addedAt for section %s", sectionID,
	), logging.LevelDebug)
	defer logAction.Complete()

	logging.DevMsgf("Bulk-fetching latest episode addedAt for shows in section %s", sectionID)

	u, err := url.Parse(msConfig.URL)
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

	resp, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return nil, *logAction.Error
	}
	resp.Body.Close()

	var countResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &countResp, "Plex Episode Count Response")
	if Err.Message != "" {
		return nil, Err
	}
	totalEpisodes := countResp.MediaContainer.TotalSize
	if totalEpisodes == 0 {
		return map[string]int64{}, logging.LogErrorInfo{}
	}

	// Second pass: fetch all episodes in one shot
	query.Set("X-Plex-Container-Size", fmt.Sprintf("%d", totalEpisodes))
	u.RawQuery = query.Encode()

	resp, respBody, Err = makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return nil, *logAction.Error
	}
	resp.Body.Close()

	var episodesResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &episodesResp, "Plex All Episodes Response")
	if Err.Message != "" {
		return nil, Err
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
