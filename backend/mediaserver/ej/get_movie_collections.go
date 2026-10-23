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

func (e *EJ) GetMovieCollections(ctx context.Context, library models.LibrarySection) (collections []models.CollectionItem, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("%s: Fetching Movie Collections for Library '%s' [ID: %s]",
		e.Config.Type, library.Title, library.ID), logging.LevelDebug)
	defer logAction.Complete()

	collections = []models.CollectionItem{}
	Err = logging.LogErrorInfo{}

	// Construct the URL for the Emby/Jellyfin API request
	u, err := url.Parse(e.Config.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return collections, *logAction.Error
	}
	u.Path = path.Join(u.Path, "Users", e.Config.UserID, "Items")
	query := u.Query()
	query.Add("Recursive", "true")
	query.Add("SortBy", "Name")
	query.Add("SortOrder", "Ascending")
	query.Add("IncludeItemTypes", "BoxSet")
	query.Add("ParentId", library.ID)
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to Emby/Jellyfin
	resp, respBody, Err := makeRequest(ctx, e.Config, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return collections, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var ejResp EmbyJellyLibraryItemsResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &ejResp, "Emby/Jellyfin Movie Collections Response")
	if Err.Message != "" {
		return collections, Err
	}

	// Now we loop through the items and build out the collection items
	// Note: Emby/Jellyfin does not have collections in the same way Plex does.
	// So we will need to loop through each boxset and get the details within.
	// This will tell us if the Boxset has a TMDB ID associated with it.

	for _, item := range ejResp.Items {
		if item.Type != "BoxSet" || item.ID == "" {
			continue
		}

		ctx, subAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Processing BoxSet '%s' (ID: %s)", item.Name, item.ID), logging.LevelTrace)
		logging.DevMsgf("Processing BoxSet '%s' (ID: %s)", item.Name, item.ID)

		// Construct the URL for the Emby/Jellyfin API request
		itemURL, err := url.Parse(e.Config.URL)
		if err != nil {
			subAction.SetError("Failed to parse base URL for item details", "Ensure the URL is valid", map[string]any{"error": err.Error()})
			continue
		}
		itemURL.Path = path.Join(itemURL.Path, "Users", e.Config.UserID, "Items", item.ID)
		detailURL := itemURL.String()

		// Make the HTTP Request to Emby/Jellyfin for item details
		detailResp, detailRespBody, detailErr := makeRequest(ctx, e.Config, detailURL, "GET", nil)
		if detailErr.Message != "" {
			subAction.SetError("Failed to fetch BoxSet details", detailErr.Message, nil)
			continue
		}
		defer detailResp.Body.Close()

		// Decode the Detail Response
		var itemDetail EmbyJellyItemContentResponse
		detailErr = httpx.DecodeResponseToJSON(ctx, detailRespBody, &itemDetail, "Emby/Jellyfin BoxSet Detail Response")
		if detailErr.Message != "" {
			subAction.SetError("Failed to decode BoxSet detail response", detailErr.Message, nil)
			continue
		}

		// If item doesn't have a TMDB ID, skip it
		if itemDetail.ProviderIds.Tmdb == "" {
			subAction.AppendWarning("message", "boxset does not have a TMDB ID; skipping")
			continue
		}

		var collectionItem models.CollectionItem
		collectionItem.RatingKey = itemDetail.ID
		collectionItem.Index = itemDetail.ID // Emby/Jellyfin does not have an index, so we use the RatingKey
		collectionItem.TMDB_ID = itemDetail.ProviderIds.Tmdb
		collectionItem.Title = itemDetail.Name
		collectionItem.Summary = itemDetail.Overview
		collectionItem.ChildCount = itemDetail.ChildCount
		collectionItem.LibraryTitle = library.Title

		// Update the collections cache
		cache.CollectionsStore.UpsertCollection(&collectionItem)

		collections = append(collections, collectionItem)
		subAction.Complete()
	}
	logging.DevMsgf("Total collections found: %d", len(collections))

	return collections, logging.LogErrorInfo{}
}
