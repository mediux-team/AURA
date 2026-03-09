package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"context"
	"sort"
	"strconv"
	"sync"
	"time"
)

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

	success = true

	configuredSections := config.Current.MediaServer.Libraries

	// Sort sections by Title to ensure consistent order
	sort.SliceStable(configuredSections, func(i, j int) bool {
		return configuredSections[i].Title < configuredSections[j].Title
	})

	logAction.AppendResult("num_sections", len(configuredSections))

	ejRanCollections := false

	for _, section := range configuredSections {
		found, Err := GetLibrarySectionDetails(ctx, &section)
		if Err.Message != "" || !found {
			continue
		}

		// Update the collections cache for this section
		if section.Type == "movie" && !ejRanCollections {
			GetMovieCollections(ctx, section)
			if config.Current.MediaServer.Type == "Emby" || config.Current.MediaServer.Type == "Jellyfin" {
				ejRanCollections = true
			}
		}

		pageSize := 1000
		start := 0
		expectedTotal := 0

		for {

			items, totalSize, Err := GetLibrarySectionItems(ctx, section, strconv.Itoa(start), strconv.Itoa(pageSize), true)
			if Err.Message != "" {
				return false
			}
			logging.LOGGER.Info().Timestamp().
				Str("section_title", section.Title).
				Str("section_id", section.ID).
				Int("fetched_items", len(items)).
				Int("start_index", start).
				Int("total_size", totalSize).
				Msg("Fetched library section items")

			if totalSize > 0 {
				expectedTotal = totalSize
			}
			if len(items) == 0 {
				break
			}

			sectionForCache := section
			sectionForCache.TotalSize = expectedTotal
			sectionForCache.MediaItems = items

			// Update Library Cache
			cache.LibraryStore.UpdateSection(&sectionForCache)

			start += len(items)

			if expectedTotal > 0 && start >= expectedTotal {
				break
			}

		}

	}
	cache.LibraryStore.LastFullUpdate = time.Now().Unix()
	cache.CollectionsStore.LastFullUpdate = time.Now().Unix()
	return true
}
