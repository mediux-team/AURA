package ej

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
)

func (e *EJ) GetMediaItemDetails(ctx context.Context, item *models.MediaItem) (found bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Fetching Full Info for %s", config.Current.MediaServer.Type,
		utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	found = false
	Err = logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return found, *logAction.Error
	}
	u.Path = path.Join(u.Path, "Users", config.Current.MediaServer.UserID, "Items", item.RatingKey)
	query := u.Query()
	query.Set("fields", "ShareLevel")
	query.Set("ExcludeFields", "VideoChapters,VideoMediaSources,MediaStreams")
	query.Set("Fields", "DateLastContentAdded,PremiereDate,DateCreated,ProviderIds,BasicSyncInfo,CanDelete,CanDownload,PrimaryImageAspectRatio,ProductionYear,Status,EndDate")
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, config.Current.MediaServer, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return found, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var ejResp EmbyJellyItemContentResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &ejResp, fmt.Sprintf("%s Media Item Details Response", config.Current.MediaServer.Type))
	if Err.Message != "" {
		return found, *logAction.Error
	}

	// If item doesn't have a TMDB ID, skip it
	if ejResp.ProviderIds.Tmdb == "" {
		logAction.SetError("Media item does not have a TMDB ID", "Only items with TMDB IDs are supported",
			map[string]any{"tmdb_id": ejResp.ProviderIds.Tmdb,
				"library_title": item.LibraryTitle,
				"title":         item.Title,
				"rating_key":    item.RatingKey,
			})
		return found, *logAction.Error
	}
	found = true

	item.RatingKey = ejResp.ID
	item.Type = ejResp.Type
	item.Title = ejResp.Name
	item.Year = ejResp.ProductionYear
	item.ContentRating = ejResp.OfficialRating
	item.UpdatedAt = ejResp.DateCreated.UnixMilli()
	item.ReleasedAt = ejResp.PremiereDate.UnixMilli()
	item.Summary = ejResp.Overview
	if ejResp.ProviderIds.Imdb != "" {
		item.Guids = append(item.Guids, models.MediaItemGuid{Provider: "imdb", ID: ejResp.ProviderIds.Imdb})
	}
	if ejResp.ProviderIds.Tmdb != "" {
		item.Guids = append(item.Guids, models.MediaItemGuid{Provider: "tmdb", ID: ejResp.ProviderIds.Tmdb})
		item.TMDB_ID = ejResp.ProviderIds.Tmdb
	}
	if ejResp.ProviderIds.Tvdb != "" {
		item.Guids = append(item.Guids, models.MediaItemGuid{Provider: "tvdb", ID: ejResp.ProviderIds.Tvdb})
	}
	item.Guids = append(item.Guids, models.MediaItemGuid{Provider: "community", Rating: fmt.Sprintf("%.1f", float64(ejResp.CommunityRating))})
	item.Guids = append(item.Guids, models.MediaItemGuid{Provider: "critic", Rating: fmt.Sprintf("%.1f", float64(ejResp.CriticRating)/10.0)})

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
		logAction.SetError("No TMDB ID found for the media item", fmt.Sprintf("Ensure the media item has a valid TMDB GUID in %s", config.Current.MediaServer.Type),
			map[string]any{
				"rating_key":     item.RatingKey,
				"library_title":  item.LibraryTitle,
				"title":          item.Title,
				"provider_guids": item.Guids,
			})
		return found, *logAction.Error
	}

	if ejResp.Type == "Movie" {
		item.Type = "movie"
		item.Movie = &models.MediaItemMovie{
			File: models.MediaItemFile{
				Path:     ejResp.Path,
				Size:     ejResp.Size,
				Duration: ejResp.RunTimeTicks / 10000,
			},
		}
	}
	if ejResp.Type == "Series" {
		item.Type = "show"
		Err := fetchSeasonsForShow(ctx, item)
		if Err.Message != "" {
			return found, Err
		}
		item.Series.Location = ejResp.Path
		item.Series.SeasonCount = ejResp.ChildCount
		item.Series.EpisodeCount = 0
		for _, season := range item.Series.Seasons {
			for _, episode := range season.Episodes {
				if episode.File.Path != "" {
					item.Series.EpisodeCount++
				}
			}
		}

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
		item.DBSavedSets = []models.DBSavedSet{}
		item.IgnoredInDB = true
		item.IgnoredMode = ignoredMode
	}

	// Check if Media Item exists in MediUX with a set
	if cache.MediuxItems.CheckItemExists(item.Type, item.TMDB_ID) {
		item.HasMediuxSets = true
	}

	// Update the Media Item on Server in the DB
	updateErr := database.UpdateMediaItemOnServer(ctx, item.TMDB_ID, item.LibraryTitle, true)
	if updateErr.Message != "" {
		logAction.AppendWarning("update_on_server_error", updateErr.Message)
	}

	// Update item in cache
	cache.LibraryStore.UpdateMediaItem(item.LibraryTitle, item)
	return found, logging.LogErrorInfo{}
}

func fetchSeasonsForShow(ctx context.Context, itemInfo *models.MediaItem) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Fetching Seasons for Show %s", config.Current.MediaServer.Type,
		utils.MediaItemInfo(*itemInfo),
	), logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	u.Path = path.Join("Shows", itemInfo.RatingKey, "Seasons")
	query := u.Query()
	query.Set("Fields", "BasicSyncInfo,CanDelete,CanDownload,PrimaryImageAspectRatio,Overview")
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, config.Current.MediaServer, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return Err
	}
	defer resp.Body.Close()

	// Decode the Response
	var ejResp EmbyJellyItemContentChildResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &ejResp, fmt.Sprintf("%s Show Seasons Response", config.Current.MediaServer.Type))
	if Err.Message != "" {
		return Err
	}

	var seasons []models.MediaItemSeason
	var latestEpisodeAddedAt int64
	for _, season := range ejResp.Items {
		season := models.MediaItemSeason{
			RatingKey:    season.ID,
			SeasonNumber: season.IndexNumber,
			Title:        season.Name,
			Episodes:     []models.MediaItemEpisode{},
		}
		season, Err := fetchEpisodesForSeason(ctx, itemInfo.RatingKey, season)
		if Err.Message != "" {
			return *logAction.Error
		}
		if len(season.Episodes) == 0 {
			continue
		}
		for _, ep := range season.Episodes {
			if ep.AddedAt > latestEpisodeAddedAt {
				latestEpisodeAddedAt = ep.AddedAt
			}
		}
		seasons = append(seasons, season)
	}
	itemInfo.LatestEpisodeAddedAt = latestEpisodeAddedAt

	itemInfo.Series = &models.MediaItemSeries{Seasons: seasons}

	return logging.LogErrorInfo{}
}

func fetchEpisodesForSeason(ctx context.Context, showRatingKey string, season models.MediaItemSeason) (models.MediaItemSeason, logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Fetching Episodes for Season %d of Show RatingKey %s", config.Current.MediaServer.Type,
		season.SeasonNumber, showRatingKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	Err := logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(config.Current.MediaServer.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return season, *logAction.Error
	}
	u.Path = path.Join(u.Path, "Shows", showRatingKey, "Episodes")
	query := u.Query()
	query.Set("SeasonId", season.RatingKey)
	query.Set("Fields", "ID,Name,IndexNumber,IndexNumberEnd,ParentIndexNumber,Path,Size,RunTimeTicks,DateCreated,MediaSources")
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, config.Current.MediaServer, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return season, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var ejResp EmbyJellyItemContentChildResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &ejResp, fmt.Sprintf("%s Season Episodes Response", config.Current.MediaServer.Type))
	if Err.Message != "" {
		return season, *logAction.Error
	}

	// Jellyfin/Emby number a merged multi-episode file (e.g. "S01E01-E02") across
	// multiple index slots (IndexNumber..IndexNumberEnd), while MediUX (TMDB-based)
	// counts it as a single slot. That collapses every subsequent episode's MediUX
	// number by the difference, so track a running offset to translate between the
	// two numbering schemes.
	mediuxNumberOffset := 0
	for _, rawEpisode := range ejResp.Items {
		mediuxEpisodeNumber := rawEpisode.IndexNumber - mediuxNumberOffset
		if rawEpisode.IndexNumberEnd > rawEpisode.IndexNumber {
			mediuxNumberOffset += rawEpisode.IndexNumberEnd - rawEpisode.IndexNumber
		}

		episode := models.MediaItemEpisode{
			RatingKey:            rawEpisode.ID,
			Title:                rawEpisode.Name,
			SeasonNumber:         rawEpisode.ParentIndexNumber,
			EpisodeNumber:        rawEpisode.IndexNumber,
			MediuxEpisodeNumber:  mediuxEpisodeNumber,
			AddedAt:              rawEpisode.DateCreated.Unix(),
			File: models.MediaItemFile{
				Path:     rawEpisode.Path,
				Size:     rawEpisode.MediaSources[0].Size,
				Duration: rawEpisode.RunTimeTicks / 10000,
			},
		}

		if episode.File.Path == "" {
			continue
		}

		// For Emby/Jellyfin, the file path is not directly available in the response.
		// We need to fetch the base Item and then get the file path.
		//fetchEpisodeInfo(&episode)
		season.Episodes = append(season.Episodes, episode)
	}

	return season, logging.LogErrorInfo{}
}
