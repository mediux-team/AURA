package plex

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (p *Plex) GetMediaItemDetails(ctx context.Context, item *models.MediaItem) (found bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Fetching Full Info for %s", utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	found = false
	Err = logging.LogErrorInfo{}

	// Construct the URL for the Plex API request
	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return found, *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "metadata", item.RatingKey)
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, config.Current.MediaServer, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return found, Err
	}
	defer resp.Body.Close()

	// Decode the Response
	var plexResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex Media Item Details Response")
	if Err.Message != "" {
		return found, *logAction.Error
	}

	// Check if any metadata was returned
	if len(plexResp.MediaContainer.Metadata) == 0 {
		logAction.SetError("No metadata found for the specified media item", "Verify the RatingKey is correct", nil)
		return found, *logAction.Error
	}

	// Populate the MediaItem details
	metadata := plexResp.MediaContainer.Metadata[0]
	extracted, Err := extractMediaItemFromResponse(ctx, metadata)
	if Err.Message != "" {
		return found, *logAction.Error
	}
	*item = *extracted

	// If no TMDB ID found, return an error
	if item.TMDB_ID == "" {
		logAction.SetError("No TMDB ID found for the media item", "Ensure the media item has a valid TMDB GUID in Plex",
			map[string]any{
				"rating_key":     item.RatingKey,
				"library_title":  item.LibraryTitle,
				"title":          item.Title,
				"provider_guids": item.Guids,
			})
		return found, *logAction.Error
	}

	// Check if Media Item exists in DB
	ignored, ignoredMode, sets, logErr := database.CheckIfMediaItemExists(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition)
	if logErr.Message != "" {
		logAction.AppendWarning("message", "Failed to check if media item exists in database")
		logAction.AppendWarning("error", Err)
	}
	if !ignored {
		item.DBSavedSets = sets
		item.IgnoredInDB = false
		item.IgnoredMode = ""
	} else {
		item.DBSavedSets = []models.DBSavedSet{}
		item.IgnoredInDB = true
		item.IgnoredMode = ignoredMode
	}

	// Check if Media Item exists in MediUX with a set
	if cache.MediuxItems.CheckItemExists(item.Type, item.TMDB_ID) {
		item.HasMediuxSets = true
	}

	// Update the item in the cache
	cache.LibraryStore.UpdateMediaItem(item.LibraryTitle, item)

	// Update the Media Item on Server in the DB
	updateErr := database.UpdateMediaItemOnServer(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition, true)
	if updateErr.Message != "" {
		logAction.AppendWarning("update_on_server_error", updateErr.Message)
	}

	// Mark as found
	found = true
	return found, logging.LogErrorInfo{}
}

func extractMediaItemFromResponse(ctx context.Context, metadata PlexLibraryItemsMetadata) (item *models.MediaItem, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Plex: Extracting Media Item '%s' [RatingKey: %s]", metadata.Title, metadata.RatingKey), logging.LevelDebug)
	defer logAction.Complete()

	item = &models.MediaItem{}
	Err = logging.LogErrorInfo{}

	item.RatingKey = metadata.RatingKey
	item.Type = metadata.Type
	item.Title = metadata.Title
	item.Year = metadata.Year
	item.LibraryTitle = metadata.LibrarySectionTitle
	item.UpdatedAt = metadata.UpdatedAt
	item.AddedAt = metadata.AddedAt
	item.ContentRating = metadata.ContentRating
	item.Summary = metadata.Summary
	item.Edition = metadata.Edition
	// Calculate ReleasedAt from OriginallyAvailableAt if available
	if t, err := time.Parse("2006-01-02", metadata.OriginallyAvailableAt); err == nil {
		item.ReleasedAt = t.Unix()
	} else {
		item.ReleasedAt = 0
	}

	if item.Title == "" {
		if metadata.OriginalTitle != "" {
			item.Title = metadata.OriginalTitle
		} else {
			item.Title = "<Unknown Title>"
		}
	}

	// Depending on the Type, populate Movie or Series info
	switch item.Type {
	case "movie":
		item.Movie = &models.MediaItemMovie{
			File: models.MediaItemFile{
				Path:     metadata.Media[0].Part[0].File,
				Size:     metadata.Media[0].Part[0].Size,
				Duration: metadata.Media[0].Part[0].Duration,
			},
		}
	case "show":
		// For shows, fetch seasons and episodes
		Err = fetchSeasonsAndEpisodesForShow(ctx, item)
		if Err.Message != "" {
			return item, *logAction.Error
		}
		item.Series.SeasonCount = metadata.ChildCount
		item.Series.EpisodeCount = metadata.LeafCount
		item.Series.Location = metadata.Location[0].Path
	}

	// Extract GUIDs and Ratings from the response
	guids, _ := getGUIDsAndRatingsFromResponse(ctx, metadata.Guids, metadata.Ratings,
		fmt.Sprintf("%.1f", metadata.AudienceRating))
	item.Guids = guids

	// Check to see if the item has a valid TMDB ID
	for _, guid := range item.Guids {
		if guid.Provider == "tmdb" && guid.ID != "" {
			item.TMDB_ID = guid.ID
			break
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

	// If the item has a user rating from Plex, set the community rating
	if metadata.UserRating > 0 {
		userRatingStr := strconv.FormatFloat(metadata.UserRating, 'f', -1, 64)
		item.Guids = append(item.Guids, models.MediaItemGuid{
			Provider: "user",
			Rating:   userRatingStr,
		})
	}

	return item, Err
}

func fetchSeasonsAndEpisodesForShow(ctx context.Context, itemInfo *models.MediaItem) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Fetching Seasons and Episodes for Show '%s' (%s | %s | %d)",
		itemInfo.Title, itemInfo.RatingKey, itemInfo.LibraryTitle, itemInfo.Year,
	), logging.LevelDebug)
	defer logAction.Complete()

	// Construct the URL for the Plex API request
	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "metadata", itemInfo.RatingKey, "allLeaves")
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, config.Current.MediaServer, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var plexResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex Show Seasons and Episodes Response")
	if Err.Message != "" {
		return *logAction.Error
	}

	// Group Episodes by Season Number
	seasonsMap := make(map[int]*models.MediaItemSeason)
	var latestEpisodeAddedAt int64
	for _, episode := range plexResp.MediaContainer.Metadata {
		seasonNumber := episode.ParentIndex
		ep := models.MediaItemEpisode{
			RatingKey:            episode.RatingKey,
			Title:                episode.Title,
			SeasonNumber:         seasonNumber,
			EpisodeNumber:        episode.Index,
			// Plex splits merged multi-episode files into separate, sequentially
			// numbered items (see backend/mediaserver/ej for why Jellyfin/Emby
			// need a separate offset), so Plex's MediUX-matching number is always
			// the same as its native episode number.
			MediuxEpisodeNumber: episode.Index,
			AddedAt:              episode.AddedAt,
			File: models.MediaItemFile{
				Path:     episode.Media[0].Part[0].File,
				Size:     episode.Media[0].Part[0].Size,
				Duration: episode.Media[0].Part[0].Duration,
			},
		}

		if episode.AddedAt > latestEpisodeAddedAt {
			latestEpisodeAddedAt = episode.AddedAt
		}

		if _, exists := seasonsMap[seasonNumber]; !exists {
			seasonsMap[seasonNumber] = &models.MediaItemSeason{
				RatingKey:    episode.ParentRatingKey,
				Title:        episode.ParentTitle,
				SeasonNumber: seasonNumber,
				Episodes:     []models.MediaItemEpisode{},
			}
		}
		seasonsMap[seasonNumber].Episodes = append(seasonsMap[seasonNumber].Episodes, ep)
	}
	itemInfo.LatestEpisodeAddedAt = latestEpisodeAddedAt

	// Convert map to slice and sort by Season Number
	var seasons []models.MediaItemSeason
	for _, season := range seasonsMap {
		seasons = append(seasons, *season)
	}
	sort.Slice(seasons, func(i, j int) bool {
		return seasons[i].SeasonNumber < seasons[j].SeasonNumber
	})

	itemInfo.Series = &models.MediaItemSeries{Seasons: seasons}
	return logging.LogErrorInfo{}
}
func getGUIDsAndRatingsFromResponse(ctx context.Context, plexGUIDS []PlexTagField, plexRatings []PlexRatings, audienceRating string) ([]models.MediaItemGuid, error) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Processing GUIDs and Ratings from Plex Response", logging.LevelTrace)
	defer logAction.Complete()

	// Example Ratings From GUIDS:
	// type PlexRatings struct {
	// 	Image string `json:"image"` // Use this to get the provider name as well
	// 	Value string `json:"value"`
	// 	Type  string `json:"type"`
	// }
	// Use the image field to determine the provider
	// Example Ratings:
	// Rating: {image:imdb://image.rating value:7.9 type:audience}
	// Rating: {image:rottentomatoes://image.rating.ripe value:8.1 type:critic}
	// Rating: {image:rottentomatoes://image.rating.upright value:8.2 type:audience}
	// Rating: {image:themoviedb://image.rating value:7.6 type:audience}

	var returnGUIDs []models.MediaItemGuid

	// First, process the GUIDs to add them to the returnGUIDs slice
	for _, plexGUID := range plexGUIDS {
		parts := strings.SplitN(plexGUID.ID, "://", 2)
		if len(parts) == 2 {
			provider := strings.ToLower(parts[0])
			id := parts[1]
			returnGUIDs = append(returnGUIDs, models.MediaItemGuid{
				Provider: provider,
				ID:       id,
			})
		}
	}

	// Next, process the Ratings to associate them with the correct GUIDs
	for _, plexRating := range plexRatings {
		// Extract provider from the image field
		parts := strings.SplitN(plexRating.Image, "://", 2)
		if len(parts) != 2 {
			continue // Skip if the format is unexpected
		}

		provider := strings.ToLower(parts[0])
		ratingValue := strconv.FormatFloat(plexRating.Value, 'f', -1, 64)

		// Normalize provider if needed
		if provider == "themoviedb" {
			provider = "tmdb"
		}

		// Check if the provider already exists in the returnGUIDs slice using an index-based loop
		found := false
		for i := 0; i < len(returnGUIDs); i++ {
			if returnGUIDs[i].Provider == provider {
				returnGUIDs[i].Rating = ratingValue // assign rating as a single string
				found = true
				break
			}
		}

		// If the provider was not found, add a new GUID with the rating.
		if !found {
			returnGUIDs = append(returnGUIDs, models.MediaItemGuid{
				Provider: provider,
				Rating:   ratingValue,
			})
		}
	}

	// Finally, handle the audienceRating if it's provided and valid
	if audienceRating != "" {
		returnGUIDs = append(returnGUIDs, models.MediaItemGuid{
			Provider: "community",
			Rating:   audienceRating,
		})
	}

	// Return the final slice of GUIDs with associated ratings
	return returnGUIDs, nil
}
