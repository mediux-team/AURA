package ej

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/url"
	"path"
)

func (e *EJ) GetMovieCollectionChildrenItems(ctx context.Context, collection *models.CollectionItem) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Fetching Collection Children for '%s' | %s [%s | %s]",
		e.Config.Type,
		collection.Title, collection.LibraryTitle, collection.TMDB_ID, collection.RatingKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(e.Config.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "Users", e.Config.UserID, "Items")
	query := u.Query()
	query.Set("ParentId", collection.RatingKey)
	query.Set("IncludeItemTypes", "Movie")
	query.Set("Fields", "DateLastContentAdded,PremiereDate,DateCreated,ProviderIds,BasicSyncInfo,CanDelete,CanDownload,PrimaryImageAspectRatio,ProductionYear,Status,EndDate,ProviderIds,Overview")
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, e.Config, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var ejResp EmbyJellyLibraryItemsResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &ejResp, fmt.Sprintf("%s Collection Children Response", e.Config.Type))
	if Err.Message != "" {
		return Err
	}

	var items []models.MediaItem
	for _, item := range ejResp.Items {
		if item.Type != "Movie" {
			continue
		}
		if item.ProviderIds.Tmdb == "" {
			continue
		}
		var itemInfo models.MediaItem
		itemInfo.RatingKey = item.ID
		itemInfo.TMDB_ID = item.ProviderIds.Tmdb
		itemInfo.Type = map[string]string{
			"Movie":  "movie",
			"Series": "show",
		}[item.Type]
		itemInfo.Title = item.Name
		itemInfo.Year = item.ProductionYear
		itemInfo.LibraryTitle = collection.LibraryTitle

		itemInfo.AddedAt = unixSeconds(item.DateCreated)
		itemInfo.ReleasedAt = unixSeconds(item.PremiereDate)

		items = append(items, itemInfo)

		// Update the collection cache with the item
		cache.CollectionsStore.UpdateMediaItemInCollectionByIndex(collection.Index, &itemInfo)
	}

	collection.MediaItems = items

	return logging.LogErrorInfo{}
}
