package mediux

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	_ "embed"
	"slices"
)

//go:embed gen_movie_collection_sets_by_tmdbid.graphql
var queryMovieCollectionSetsByTMDBID string

type movieCollectionSetsByTMDBID_Response struct {
	Data   movieCollectionSetsByTMDBID_Data `json:"data"`
	Errors []ErrorResponse                  `json:"errors,omitempty"`
}

type movieCollectionSetsByTMDBID_Data struct {
	Base movieCollectionSetsByTMDBID_MoviesByID `json:"movies_by_id"`
}

type movieCollectionSetsByTMDBID_MoviesByID struct {
	Collection movieCollectionSetsByTMDBID_Collection `json:"collection_id"`
}

type movieCollectionSetsByTMDBID_Collection struct {
	ID        string                                        `json:"id"`
	Posters   []movieCollectionSetsByTMDBID_ImageCollection `json:"posters"`
	Backdrops []movieCollectionSetsByTMDBID_ImageCollection `json:"backdrops"`
	Movies    []movieCollectionSetsByTMDBID_Movie           `json:"movies"`
}

type movieCollectionSetsByTMDBID_Movie struct {
	BaseMovieInfo
	Posters   []movieCollectionSetsByTMDBID_ImageCollection `json:"posters"`
	Backdrops []movieCollectionSetsByTMDBID_ImageCollection `json:"backdrops"`
}

type movieCollectionSetsByTMDBID_ImageCollection struct {
	ImageAsset
	Collection BaseSetInfo `json:"collection_set"`
}

func GetMovieItemCollectionSets(ctx context.Context, tmdbID string, itemLibraryTitle string, edition string, setItems *map[string]models.IncludedItem) (sets []models.SetRef, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get Movie Item Collection Sets", logging.LevelInfo)
	defer logAction.Complete()

	sets = []models.SetRef{}
	Err = logging.LogErrorInfo{}

	// Send the GraphQL request
	resp, Err := makeGraphQLRequest(ctx, MediuxGraphQLQueryBody{
		Query:     queryMovieCollectionSetsByTMDBID,
		Variables: map[string]any{"tmdb_id": tmdbID},
		QueryName: "getMovieItemCollectionSetsByTMDBID",
	})
	if Err.Message != "" {
		return sets, Err
	}

	// Decode the response
	var movieCollectionSetsResponse movieCollectionSetsByTMDBID_Response
	Err = httpx.DecodeResponseToJSON(ctx, resp.Body(), &movieCollectionSetsResponse, "MediUX Movie Collection Sets By TMDB ID Response")
	if Err.Message != "" {
		return sets, Err
	}

	// If any errors returned from MediUX, log and return
	if len(movieCollectionSetsResponse.Errors) > 0 {
		logAction.SetError("Errors returned from MediUX API", "Review the errors for more details",
			map[string]any{
				"tmdb_id":   tmdbID,
				"item_type": "movie",
				"errors":    movieCollectionSetsResponse.Errors,
			})
		return sets, *logAction.Error
	}

	logAction.AppendResult("collections_found", len(movieCollectionSetsResponse.Data.Base.Collection.Movies))
	// If no collection sets, return
	if len(movieCollectionSetsResponse.Data.Base.Collection.Movies) == 0 {
		return sets, Err
	}

	// 1) Populate included items with the movie info
	if *setItems == nil {
		*setItems = map[string]models.IncludedItem{}
	}
	for _, mediuxMovie := range movieCollectionSetsResponse.Data.Base.Collection.Movies {
		baseItem := convertMediuxBaseItemToResponseBaseItem(mediuxMovie.BaseItemInfo, "movie")
		baseItem.ReleaseDate = mediuxMovie.ReleaseDate
		includedItem := (*setItems)[mediuxMovie.ID]
		includedItem.MediuxInfo = baseItem
		(*setItems)[mediuxMovie.ID] = includedItem

		// Find the Media Item info from the cache. The primary item being browsed
		// (matching tmdbID) resolves to the exact edition; other collection
		// members are distinct movies, so we fall back to an arbitrary match.
		var mediaItem *models.MediaItem
		var found bool
		if mediuxMovie.ID == tmdbID {
			mediaItem, found = cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(itemLibraryTitle, mediuxMovie.ID, edition)
		} else {
			mediaItem, found = cache.LibraryStore.GetMediaItemFromSectionByTMDBID(itemLibraryTitle, mediuxMovie.ID)
		}
		if found {
			includedItem := (*setItems)[mediuxMovie.ID]
			includedItem.MediaItem = *mediaItem
			(*setItems)[mediuxMovie.ID] = includedItem
		}
	}

	// 2) Aggregate sets by collection set ID
	type setAgg struct {
		info    models.BaseSetInfo
		images  []models.ImageFile
		itemIDs map[string]struct{}
	}
	setMap := map[string]*setAgg{}

	getOrCreateSet := func(set BaseSetInfo) *setAgg {
		if agg, ok := setMap[set.ID]; ok {
			return agg
		}
		agg := &setAgg{
			info: models.BaseSetInfo{
				ID:               set.ID,
				Title:            set.SetTitle,
				Type:             "collection",
				UserCreated:      set.UserCreated.Username,
				DateCreated:      set.DateCreated,
				DateUpdated:      set.DateUpdated,
				Popularity:       set.Popularity,
				PopularityGlobal: set.PopularityGlobal,
			},
			images:  []models.ImageFile{},
			itemIDs: map[string]struct{}{},
		}
		setMap[set.ID] = agg
		return agg
	}

	for _, mediuxMovie := range movieCollectionSetsResponse.Data.Base.Collection.Movies {
		// Posters
		for _, poster := range mediuxMovie.Posters {
			agg := getOrCreateSet(poster.Collection)
			img := convertMediuxImageAssetToImageFile(&poster.ImageAsset, "poster")
			if img != nil {
				img.ItemTMDB_ID = mediuxMovie.ID
				agg.images = append(agg.images, *img)
				agg.itemIDs[mediuxMovie.ID] = struct{}{}
			}
		}
		// Backdrops
		for _, backdrop := range mediuxMovie.Backdrops {
			agg := getOrCreateSet(backdrop.Collection)
			img := convertMediuxImageAssetToImageFile(&backdrop.ImageAsset, "backdrop")
			if img != nil {
				img.ItemTMDB_ID = mediuxMovie.ID
				agg.images = append(agg.images, *img)
				agg.itemIDs[mediuxMovie.ID] = struct{}{}
			}
		}
	}

	// The collection's own poster and backdrop are in the same sets as the movies' images
	mediuxCollection := movieCollectionSetsResponse.Data.Base.Collection
	for _, poster := range mediuxCollection.Posters {
		if img := convertMediuxImageAssetToImageFile(&poster.ImageAsset, "collection_poster"); img != nil {
			agg := getOrCreateSet(poster.Collection)
			agg.images = append(agg.images, *img)
		}
	}
	for _, backdrop := range mediuxCollection.Backdrops {
		if img := convertMediuxImageAssetToImageFile(&backdrop.ImageAsset, "collection_backdrop"); img != nil {
			agg := getOrCreateSet(backdrop.Collection)
			agg.images = append(agg.images, *img)
		}
	}

	// The media server collection the collection images apply to
	collectionMovieIDs := make([]string, 0, len(mediuxCollection.Movies))
	for _, mediuxMovie := range mediuxCollection.Movies {
		collectionMovieIDs = append(collectionMovieIDs, mediuxMovie.ID)
	}
	var serverCollection *models.CollectionItem
	if match, found := cache.CollectionsStore.FindCollectionForMovie(mediuxCollection.ID, itemLibraryTitle, tmdbID, collectionMovieIDs); found {
		serverCollection = &match
		logAction.AppendResult("server_collection", match.Title)
	}

	// 3) Flatten into []SetRef, but only include sets that contain the requested tmdbID
	for _, agg := range setMap {
		itemIDs := make([]string, 0, len(agg.itemIDs))
		for id := range agg.itemIDs {
			itemIDs = append(itemIDs, id)
		}
		// Only include the set if tmdbID is present in itemIDs
		if slices.Contains(itemIDs, tmdbID) {
			setRef := models.SetRef{
				PosterSet: models.PosterSet{
					BaseSetInfo: agg.info,
					Images:      agg.images,
				},
				ItemIDs: itemIDs,
			}
			if serverCollection != nil && slices.ContainsFunc(agg.images, func(img models.ImageFile) bool {
				return img.Type == "collection_poster" || img.Type == "collection_backdrop"
			}) {
				setRef.Collection = serverCollection
			}
			sets = append(sets, setRef)
		}
	}
	logAction.AppendResult("collections_returned", len(sets))
	return sets, Err
}
