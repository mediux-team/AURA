package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
)

// LoadLibraryCacheFromDatabase fills the main library cache with the saved items that are on the server
func LoadLibraryCacheFromDatabase(ctx context.Context) (loaded int) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Loading the library cache from the database", logging.LevelInfo)
	defer logAction.Complete()

	items, Err := database.GetLibraryItemsOnServer(ctx)
	if Err.Message != "" {
		logAction.AppendWarning("message", "Could not load the library from the database, it will load from the media server")
		return 0
	}
	byLibrary := map[string][]models.MediaItem{}
	for _, item := range items {
		byLibrary[item.LibraryTitle] = append(byLibrary[item.LibraryTitle], item)
	}

	// Only the libraries that are still used
	for _, library := range config.Current.MediaServer.Libraries {
		libraryItems := byLibrary[library.Title]
		if len(libraryItems) == 0 {
			continue
		}
		dbStates, dbErr := database.GetLibraryDBStates(ctx, library.Title)
		if dbErr.Message != "" {
			logAction.AppendWarning("db_states_error", dbErr.Message)
		}
		for i := range libraryItems {
			database.ApplyDBState(dbStates, &libraryItems[i])
			libraryItems[i].HasMediuxSets = cache.MediuxItems.CheckItemExists(libraryItems[i].Type, libraryItems[i].TMDB_ID)
		}
		section := models.LibrarySection{LibrarySectionBase: library.LibrarySectionBase, MediaItems: libraryItems}
		section.TotalSize = len(libraryItems)
		cache.LibraryStore.UpdateSection(&section)
		loaded += len(libraryItems)
	}
	logAction.AppendResult("items", loaded)

	// Collections and the movies in them, with each movie from the library cache when it's there
	if loaded > 0 {
		collections, collectionsErr := database.GetCollections(ctx)
		if collectionsErr.Message != "" {
			logAction.AppendWarning("collections_error", collectionsErr.Message)
		} else {
			for i := range collections {
				for j, member := range collections[i].MediaItems {
					if item, found := cache.LibraryStore.GetMediaItemFromSectionByRatingKey(member.LibraryTitle, member.RatingKey); found {
						collections[i].MediaItems[j] = *item
					}
				}
			}
			cache.CollectionsStore.SetAllCollections(collections)
			logAction.AppendResult("collections", len(collections))
		}
	}
	return loaded
}
