package ej

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
	"slices"
	"strings"
)

func (e *EJ) GetLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string) (items []models.MediaItem, totalSize int, fetched int, Err logging.LogErrorInfo) {
	return e.getLibrarySectionItems(ctx, section, sectionStartIndex, limit, false)
}

// GetNewestLibrarySectionItems gets a library's items, newest first
func (e *EJ) GetNewestLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string) (items []models.MediaItem, totalSize int, fetched int, Err logging.LogErrorInfo) {
	return e.getLibrarySectionItems(ctx, section, sectionStartIndex, limit, true)
}

func (e *EJ) getLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string, newestFirst bool) (items []models.MediaItem, totalSize int, fetched int, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Fetching Items for Library Section: %s", e.Config.Type, section.Title,
	), logging.LevelInfo)
	defer logAction.Complete()

	items = []models.MediaItem{}
	totalSize = 0
	Err = logging.LogErrorInfo{}

	// If limit is empty, set a default limit
	if limit == "" {
		limit = "500"
	}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(e.Config.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return items, totalSize, fetched, *logAction.Error
	}
	u.Path = path.Join(u.Path, "Users", e.Config.UserID, "Items")
	query := u.Query()
	query.Add("Recursive", "true")
	if newestFirst {
		query.Add("SortBy", "DateCreated")
		query.Add("SortOrder", "Descending")
	} else {
		query.Add("SortBy", "Name")
		query.Add("SortOrder", "Ascending")
	}
	query.Add("IncludeItemTypes", "Movie,Series")
	query.Add("Fields", "DateLastContentAdded,PremiereDate,DateCreated,ProviderIds,BasicSyncInfo,CanDelete,CanDownload,PrimaryImageAspectRatio,ProductionYear,Status,EndDate")
	query.Add("ParentId", section.ID)
	query.Add("StartIndex", sectionStartIndex)
	query.Add("Limit", limit)
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, e.Config, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return items, totalSize, fetched, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var ejResp EmbyJellyLibraryItemsResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &ejResp, fmt.Sprintf("%s Library Section Items Response", e.Config.Type))
	if Err.Message != "" {
		return items, totalSize, fetched, Err
	}

	// Check to see if any items were returned
	if len(ejResp.Items) == 0 {
		logAction.AppendWarning("message", fmt.Sprintf("Library Section '%s' is empty", section.Title))
		return items, totalSize, fetched, Err
	}

	totalSize = ejResp.TotalRecordCount
	fetched = len(ejResp.Items)

	// Saved sets and ignored state for the whole page
	dbStates, dbErr := database.GetLibraryDBStates(ctx, section.Title)
	if dbErr.Message != "" {
		logAction.AppendWarning("db_states_error", dbErr.Message)
	}

	for _, ejItem := range ejResp.Items {
		var item models.MediaItem

		// If Type is Boxset, then split them up
		if ejItem.Type == "BoxSet" {
			// Split the BoxSet into individual items
			boxSetItems, boxSetErr := splitCollectionIntoIndividualItems(ctx, e.Config, ejItem.Name, ejItem.ID, section.Title, dbStates)
			if boxSetErr.Message != "" {
				return nil, 0, 0, boxSetErr
			}
			// Only include unique items from the BoxSet split (some servers may return duplicate items in the BoxSet response vs the main section items response)
			existingRatingKeys := make(map[string]bool)
			for _, existingItem := range items {
				existingRatingKeys[existingItem.RatingKey] = true
			}
			uniqueBoxSetItems := []models.MediaItem{}
			for _, boxSetItem := range boxSetItems {
				if _, exists := existingRatingKeys[boxSetItem.RatingKey]; !exists {
					uniqueBoxSetItems = append(uniqueBoxSetItems, boxSetItem)
				}
			}
			items = append(items, uniqueBoxSetItems...)
			continue
		}

		item.RatingKey = ejItem.ID
		item.Type = map[string]string{
			"Movie":  "movie",
			"Series": "show",
		}[ejItem.Type]
		item.Title = ejItem.Name
		item.Year = ejItem.ProductionYear
		item.LibraryTitle = section.Title
		if ejItem.ProviderIds.Tmdb != "" {
			item.Guids = append(item.Guids, models.MediaItemGuid{Provider: "tmdb", ID: ejItem.ProviderIds.Tmdb})
			item.Guids = append(item.Guids, models.MediaItemGuid{Provider: "tvdb", ID: ejItem.ProviderIds.Tvdb})
			item.TMDB_ID = ejItem.ProviderIds.Tmdb
		}
		item.AddedAt = unixSeconds(ejItem.DateCreated)
		item.ReleasedAt = unixSeconds(ejItem.PremiereDate)
		if item.Type == "show" && !ejItem.DateLastContentAdded.IsZero() {
			item.LatestEpisodeAddedAt = unixSeconds(ejItem.DateLastContentAdded)
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
		if e.Config.IsMain() {
			cache.LibraryStore.UpdateMediaItem(section.Title, &item)
		}
		items = append(items, item)
	}

	return items, totalSize, fetched, logging.LogErrorInfo{}
}

func splitCollectionIntoIndividualItems(ctx context.Context, msConfig config.Config_MediaServer, collectionName, parentID, sectionTitle string, dbStates map[string]database.MediaItemDBState) (items []models.MediaItem, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Splitting BoxSet Collection: %s in Section: %s into Individual Items", collectionName, sectionTitle,
	), logging.LevelInfo)
	defer logAction.Complete()

	items = []models.MediaItem{}
	Err = logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return items, *logAction.Error
	}
	u.Path = path.Join(u.Path, "Users", msConfig.UserID, "Items")
	query := u.Query()
	query.Add("Recursive", "true")
	query.Add("SortBy", "Name")
	query.Add("SortOrder", "Ascending")
	query.Add("IncludeItemTypes", "Movie,Series")
	query.Add("Fields", "Path,DateLastContentAdded,PremiereDate,DateCreated,ProviderIds,BasicSyncInfo,CanDelete,CanDownload,PrimaryImageAspectRatio,ProductionYear,Status,EndDate")
	query.Add("ParentId", parentID)
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, msConfig, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return items, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var ejResp EmbyJellyLibraryItemsResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &ejResp, fmt.Sprintf("%s BoxSet Individual Items Response", msConfig.Type))
	if Err.Message != "" {
		return items, Err
	}

	// Check to see if any items were returned
	if len(ejResp.Items) == 0 {
		logAction.AppendWarning("message", fmt.Sprintf("BoxSet Collection '%s' is empty", collectionName))
		return items, Err
	}

	validLibraryPaths := []string{}
	for _, lib := range msConfig.Libraries {
		if len(lib.Paths) > 0 {
			validLibraryPaths = append(validLibraryPaths, lib.Paths...)
		}
	}
	logging.LOGGER.Info().Timestamp().Str("boxset_collection", collectionName).Strs("valid_library_paths", validLibraryPaths).Msg("Valid library paths to check against for items in BoxSet collection")

	for _, item := range ejResp.Items {
		var itemInfo models.MediaItem

		// If the item type is "show", skip it since we only want movies in the BoxSet collections
		showTypes := []string{"show", "series", "season", "episode"}
		if slices.ContainsFunc(showTypes, func(str string) bool { return strings.EqualFold(str, item.Type) }) {
			logAction.AppendWarning("message", fmt.Sprintf("Skipping item '%s' in BoxSet Collection '%s' because it is of type '%s'", item.Name, collectionName, item.Type))
			continue
		}

		// Get the item path
		itemPath := item.Path
		if itemPath == "" {
			logAction.AppendWarning("message", fmt.Sprintf("Item '%s' in BoxSet Collection '%s' does not have a path, skipping", item.Name, collectionName))
			continue
		}
		// Check to see if the path starts with one of the known library paths, if not, skip the item
		validPath := false
		for _, lib := range msConfig.Libraries {
			for _, libPath := range lib.Paths {
				if libPath != "" && strings.HasPrefix(itemPath, libPath) {
					validPath = true
					break
				}
			}
			if validPath {
				break
			}
		}
		if !validPath {
			logging.LOGGER.Warn().Timestamp().Str("item_title", item.Name).
				Str("item_path", itemPath).
				Str("boxset_collection", collectionName).
				Msg("Skipping item in BoxSet collection because its path does not start with any of the known library paths")
			continue
		}

		itemInfo.RatingKey = item.ID
		itemInfo.Type = map[string]string{"Movie": "movie"}[item.Type]

		itemInfo.Title = item.Name
		itemInfo.Year = item.ProductionYear
		itemInfo.LibraryTitle = sectionTitle
		if item.ProviderIds.Tmdb != "" {
			itemInfo.Guids = append(itemInfo.Guids, models.MediaItemGuid{Provider: "tmdb", ID: item.ProviderIds.Tmdb})
			itemInfo.Guids = append(itemInfo.Guids, models.MediaItemGuid{Provider: "tvdb", ID: item.ProviderIds.Tvdb})
			itemInfo.TMDB_ID = item.ProviderIds.Tmdb
		}
		itemInfo.AddedAt = unixSeconds(item.DateCreated)
		itemInfo.ReleasedAt = unixSeconds(item.PremiereDate)

		// If no TMDB ID found, get the value from MediUX using the GUID[tvdb]
		if itemInfo.TMDB_ID == "" {
			for _, guid := range itemInfo.Guids {
				if guid.Provider == "tvdb" {
					tmdbID, found, Err := mediux.SearchTMDBIDByTVDBID(ctx, guid.ID, itemInfo.Type)
					if Err.Message != "" {
						logAction.AppendWarning("search_tmdb_id_error", "Failed to search TMDB ID from MediUX")
					}
					if found {
						itemInfo.TMDB_ID = tmdbID
						break
					}
				}
			}
		}
		if itemInfo.TMDB_ID == "" {
			logging.LOGGER.Warn().Timestamp().Str("item_title", itemInfo.Title).Str("library_section", sectionTitle).Msg("Skipping item in BoxSet collection because it does not have a TMDB ID")
			continue // Skip items without TMDB ID
		}

		// Saved sets and ignored state from the database
		database.ApplyDBState(dbStates, &itemInfo)

		// Check if Media Item exists in MediUX with a set
		if cache.MediuxItems.CheckItemExists(itemInfo.Type, itemInfo.TMDB_ID) {
			itemInfo.HasMediuxSets = true
		}

		// Only the main media server's items go in the main library cache
		if msConfig.IsMain() {
			cache.LibraryStore.UpdateMediaItem(sectionTitle, &itemInfo)
		}
		items = append(items, itemInfo)
	}
	return items, logging.LogErrorInfo{}
}
