package plex

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
)

func (p *Plex) GetMovieCollections(ctx context.Context, library models.LibrarySection) (collections []models.CollectionItem, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Plex: Fetching Movie Collections for Library '%s' [ID: %s]", library.Title, library.ID), logging.LevelDebug)
	defer logAction.Complete()

	collections = []models.CollectionItem{}
	Err = logging.LogErrorInfo{}

	if library.Type != "movie" {
		return collections, Err
	}

	// Construct the URL for the Plex API request
	u, err := url.Parse(p.Config.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return collections, *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "sections", library.ID, "collections")
	query := u.Query()
	query.Set("includeCollections", "1")
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, p.Config, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return collections, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var plexResp PlexLibraryItemsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex Movie Collections Response")
	if Err.Message != "" {
		return collections, Err
	}

	for _, collection := range plexResp.MediaContainer.Metadata {
		if collection.Type != "collection" || collection.ChildCount < 1 {
			continue
		}

		var collectionItem models.CollectionItem
		collectionItem.Index = strconv.Itoa(collection.Index)
		collectionItem.RatingKey = collection.RatingKey
		collectionItem.Title = collection.Title
		collectionItem.Summary = collection.Summary
		collectionItem.ChildCount = collection.ChildCount
		collectionItem.LibraryTitle = library.Title

		// Update the collections cache
		cache.CollectionsStore.UpsertCollection(&collectionItem)

		collections = append(collections, collectionItem)
	}

	return collections, logging.LogErrorInfo{}
}
