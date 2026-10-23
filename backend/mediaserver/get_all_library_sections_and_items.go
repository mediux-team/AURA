package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediaserver/ej"
	"aura/models"
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"
)

// libraryPageSize is how many items are asked for at a time
var libraryPageSize = 1000

var (
	warmupMu   sync.Mutex
	warmupDone bool
)

func GetAllLibrarySectionsAndItems(ctx context.Context, force bool) (success bool) {
	// If we already did a run that satisfies this request, skip.
	warmupMu.Lock()
	alreadyDone := warmupDone
	warmupMu.Unlock()
	if alreadyDone && !force {
		return true
	}

	success = getAllLibrarySectionsAndItemsImpl(ctx)
	if success {
		warmupMu.Lock()
		warmupDone = true
		warmupMu.Unlock()
	}

	return success
}

func getAllLibrarySectionsAndItemsImpl(ctx context.Context) (success bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Fetching All Library Sections and Items", logging.LevelDebug)
	defer logAction.Complete()

	// Main media server
	success = loadLibrariesIntoCache(ctx, config.Current.MediaServer, cache.LibraryStore, true)
	if !success {
		return false
	}
	cache.CollectionsStore.LastFullUpdate = time.Now().Unix()

	// Other media servers load in the background, so a slow or unreachable one doesn't hold up aura
	go RefreshOtherMediaServers()
	return true
}

// loadLibrariesIntoCache loads the libraries and items of a media server into a library cache.
// Collections and progress are only for the main media server.
func loadLibrariesIntoCache(ctx context.Context, server config.Config_MediaServer, store *cache.MediaServerLibraryCache, isMain bool) (success bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Loading Libraries: %s", server.Name), logging.LevelDebug)
	defer logAction.Complete()

	msClient, Err := NewMediaServerClient(&server)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return false
	}

	configuredSections := slices.Clone(server.Libraries)

	// Sort sections by Title to ensure consistent order
	sort.SliceStable(configuredSections, func(i, j int) bool {
		return configuredSections[i].Title < configuredSections[j].Title
	})

	logAction.AppendResult("num_sections", len(configuredSections))

	if isMain {
		titles := []string{}
		lastTotals := map[string]int{}
		for _, section := range configuredSections {
			titles = append(titles, section.Title)
			lastTotals[section.Title] = store.SectionItemCount(section.Title)
		}
		startLibraryProgress(titles, lastTotals)
		defer func() { finishLibraryProgress(success) }()
	}

	ejRanCollections := false
	// The collections found while loading, and whether every library's collections loaded
	loadedCollections := map[string]bool{}
	collectionsLoaded := true
	// When each library that loaded in full started loading
	loadedSince := map[string]int64{}
	// Items the main cache didn't have, a library loading for the first time has none
	newItems := []models.MediaItem{}

	for _, section := range configuredSections {
		sectionStartedAt := time.Now().Unix()
		found, Err := msClient.GetLibrarySectionDetails(ctx, &section)
		if Err.Message != "" || !found {
			if isMain {
				updateLibraryLoad(section.Title, 0, 0, true)
			}
			continue
		}

		// Update the collections cache for this section
		if isMain && (section.Type == "movie" || section.Type == "mixed") && !ejRanCollections {
			collections, collectionsErr := loadMovieCollections(ctx, server, section)
			if collectionsErr.Message != "" {
				collectionsLoaded = false
			}
			for _, collection := range collections {
				loadedCollections[collection.RatingKey] = true
			}
			if server.Type == "Emby" || server.Type == "Jellyfin" {
				ejRanCollections = true
			}
		}

		// Plex adds each movie to its collections while the library loads, they keep their movies until it's done
		if isMain && server.Type == "Plex" && (section.Type == "movie" || section.Type == "mixed") {
			cache.CollectionsStore.StartLoadingMembers(section.Title)
			defer cache.CollectionsStore.StopLoadingMembers(section.Title)
		}

		pageSize := libraryPageSize
		start := 0
		expectedTotal := 0
		seen := map[string]bool{}
		known := map[string]bool{}
		if isMain {
			known = store.SectionItemKeys(section.Title)
		}

		for {

			items, totalSize, fetched, Err := msClient.GetLibrarySectionItems(ctx, section, strconv.Itoa(start), strconv.Itoa(pageSize))
			if Err.Message != "" {
				return false
			}
			logging.LOGGER.Info().Timestamp().
				Str("media_server", server.Name).
				Str("section_title", section.Title).
				Str("section_id", section.ID).
				Int("fetched_items", fetched).
				Int("kept_items", len(items)).
				Int("start_index", start).
				Int("total_size", totalSize).
				Msg("Fetched library section items")

			if totalSize > 0 {
				expectedTotal = totalSize
			}
			if fetched == 0 {
				break
			}

			for i := range items {
				key := cache.MediaItemCacheKey(&items[i])
				seen[key] = true
				if len(known) > 0 && !known[key] {
					newItems = append(newItems, items[i])
				}
			}
			if len(items) > 0 {
				sectionForCache := section
				sectionForCache.TotalSize = expectedTotal
				sectionForCache.MediaItems = items

				// Update Library Cache
				store.UpdateSection(&sectionForCache)

				// Save the main media server's items to the database
				if isMain {
					if saveErr := database.SaveLibraryItems(ctx, items, time.Now().Unix()); saveErr.Message != "" {
						logAction.AppendWarning("save_library_items_error", saveErr.Message)
					}
				}
			}

			// The next page starts after every entry sent, skipped ones too
			start += fetched
			if isMain {
				updateLibraryLoad(section.Title, start, expectedTotal, false)
			}

			if expectedTotal > 0 && start >= expectedTotal {
				break
			}

		}
		// Items that are no longer on the media server
		if removed := store.RemoveItemsNotSeen(section.Title, seen); removed > 0 {
			logAction.AppendResult(fmt.Sprintf("removed_from_%s", section.Title), removed)
		}
		if isMain {
			updateLibraryLoad(section.Title, start, max(expectedTotal, start), true)
			loadedSince[section.Title] = sectionStartedAt
			cache.CollectionsStore.FinishLoadingMembers(section.Title)
		}

	}

	if isMain {
		usedLibraries := []string{}
		for _, section := range configuredSections {
			usedLibraries = append(usedLibraries, section.Title)
		}
		if finishErr := database.FinishLibraryLoad(ctx, usedLibraries, loadedSince); finishErr.Message != "" {
			logAction.AppendWarning("finish_library_load_error", finishErr.Message)
		}
		if collectionsLoaded {
			finishCollectionsLoad(ctx, msClient, server, loadedCollections)
		}
		if len(newItems) > 0 && OnNewItems != nil {
			logAction.AppendResult("new_items", len(newItems))
			OnNewItems(ctx, newItems)
		}
	}
	store.LastFullUpdate = time.Now().Unix()
	return true
}

// finishCollectionsLoad removes the collections that are gone, gets the movies in each collection, and saves them
func finishCollectionsLoad(ctx context.Context, msClient MediaServerInterface, server config.Config_MediaServer, loadedCollections map[string]bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Saving Collections", logging.LevelDebug)
	defer logAction.Complete()

	cache.CollectionsStore.Retain(loadedCollections)

	// Plex added the movies while the libraries loaded, the other media servers are asked for each collection
	if server.Type != "Plex" {
		for _, collection := range cache.CollectionsStore.GetAllCollections() {
			if Err := msClient.GetMovieCollectionChildrenItems(ctx, &collection); Err.Message != "" {
				logAction.AppendWarning(fmt.Sprintf("collection_%s", collection.RatingKey), Err.Message)
				continue
			}
			cache.CollectionsStore.SetCollectionMembers(collection.RatingKey, collection.MediaItems)
		}
	}

	collections := cache.CollectionsStore.GetAllCollections()
	logAction.AppendResult("collections", len(collections))
	if Err := database.SaveCollections(ctx, collections); Err.Message != "" {
		logAction.AppendWarning("save_collections_error", Err.Message)
	}
}

// loadMovieCollections gets the collections for a movie library. Emby and Jellyfin keep every collection
// in their own Collections library, so that library is used instead.
func loadMovieCollections(ctx context.Context, server config.Config_MediaServer, section models.LibrarySection) ([]models.CollectionItem, logging.LogErrorInfo) {
	if server.Type == "Emby" || server.Type == "Jellyfin" {
		collectionSection, Err := ej.GetMovieCollectionSection(ctx, server)
		if Err.Message != "" {
			return nil, Err
		}
		// No Collections library, so there are no collections
		if collectionSection.ID == "" {
			return []models.CollectionItem{}, logging.LogErrorInfo{}
		}
		section = collectionSection
	}
	return GetMovieCollections(ctx, section)
}
