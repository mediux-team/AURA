package cache

import (
	"aura/models"
	"sync"
)

// ---   Cache Global Variables (Backend Library Collections Cache) --- ---
var CollectionsStore *MediaServerCollectionsCache

type MediaServerCollectionsCache struct {
	// libraryTitle -> ratingKey -> CollectionItem
	collections map[string]map[string]*models.CollectionItem
	// The movies added to each collection while a library loads, by library then collection title.
	// They replace the collection's movies when the library finishes loading.
	loadingMembers map[string]map[string][]models.MediaItem
	mu             sync.RWMutex
	LastFullUpdate int64
}

// NewCollectionsCache creates a new CollectionsCache instance
func Cache_NewCollectionsCache() *MediaServerCollectionsCache {
	return &MediaServerCollectionsCache{
		collections:    make(map[string]map[string]*models.CollectionItem),
		loadingMembers: make(map[string]map[string][]models.MediaItem),
		LastFullUpdate: 0,
	}
}

func init() {
	CollectionsStore = Cache_NewCollectionsCache()
}

// ClearAllCollections removes every collection
func (msc *MediaServerCollectionsCache) ClearAllCollections() {
	msc.mu.Lock()
	defer msc.mu.Unlock()
	msc.collections = make(map[string]map[string]*models.CollectionItem)
	msc.loadingMembers = make(map[string]map[string][]models.MediaItem)
	msc.LastFullUpdate = 0
}

func (msc *MediaServerCollectionsCache) GetAllCollections() []models.CollectionItem {
	msc.mu.RLock()
	defer msc.mu.RUnlock()

	out := []models.CollectionItem{}
	for _, lib := range msc.collections {
		for _, coll := range lib {
			out = append(out, *coll)
		}
	}
	return out
}

func (msc *MediaServerCollectionsCache) GetCollectionByRatingKey(ratingKey string) (*models.CollectionItem, bool) {
	msc.mu.RLock()
	defer msc.mu.RUnlock()

	for _, lib := range msc.collections {
		for _, coll := range lib {
			if coll.RatingKey == ratingKey {
				return coll, true
			}
		}
	}
	return nil, false
}

func (msc *MediaServerCollectionsCache) UpsertCollection(collection *models.CollectionItem) {
	msc.mu.Lock()
	defer msc.mu.Unlock()

	lib := msc.collections[collection.LibraryTitle]
	if lib == nil {
		lib = make(map[string]*models.CollectionItem)
		msc.collections[collection.LibraryTitle] = lib
	}
	// A collection fetched without its movies keeps the ones it has
	if existing, ok := lib[collection.RatingKey]; ok && len(collection.MediaItems) == 0 {
		collection.MediaItems = existing.MediaItems
	}
	lib[collection.RatingKey] = collection
}

func (msc *MediaServerCollectionsCache) UpdateMediaItemInCollectionByIndex(collectionIndex string, item *models.MediaItem) {
	msc.mu.Lock()
	defer msc.mu.Unlock()

	if item == nil || item.LibraryTitle == "" {
		return
	}

	lib := msc.collections[item.LibraryTitle]
	if lib == nil {
		return
	}

	// Collections are stored by rating key, which isn't always the same as the index
	var collection *models.CollectionItem
	for _, coll := range lib {
		if coll != nil && coll.Index == collectionIndex {
			collection = coll
			break
		}
	}
	if collection == nil {
		return
	}

	for i := range collection.MediaItems {
		if collection.MediaItems[i].RatingKey == item.RatingKey {
			// Update existing item
			collection.MediaItems[i] = *item
			return
		}
	}

	// If not found, append new item
	collection.MediaItems = append(collection.MediaItems, *item)
}

func (msc *MediaServerCollectionsCache) UpdateMediaItemInCollectionByTitle(collectionTitle string, item *models.MediaItem) {
	msc.mu.Lock()
	defer msc.mu.Unlock()

	if item == nil || item.LibraryTitle == "" {
		return
	}

	// While the library loads, the movies are kept aside until it finishes
	if loading, ok := msc.loadingMembers[item.LibraryTitle]; ok {
		members := loading[collectionTitle]
		for i := range members {
			if members[i].RatingKey == item.RatingKey {
				members[i] = *item
				return
			}
		}
		loading[collectionTitle] = append(members, *item)
		return
	}

	lib := msc.collections[item.LibraryTitle]
	if lib == nil {
		return
	}

	for _, collection := range lib {
		if collection == nil || collection.Title != collectionTitle {
			continue
		}

		for i := range collection.MediaItems {
			if collection.MediaItems[i].RatingKey == item.RatingKey {
				collection.MediaItems[i] = *item
				return
			}
		}

		collection.MediaItems = append(collection.MediaItems, *item)
		return
	}

}

// StartLoadingMembers keeps the movies added to a library's collections aside until FinishLoadingMembers,
// so the collections keep their current movies while the library loads
func (msc *MediaServerCollectionsCache) StartLoadingMembers(libraryTitle string) {
	msc.mu.Lock()
	defer msc.mu.Unlock()
	msc.loadingMembers[libraryTitle] = map[string][]models.MediaItem{}
}

// FinishLoadingMembers replaces the movies of the library's collections with the ones added while it loaded
func (msc *MediaServerCollectionsCache) FinishLoadingMembers(libraryTitle string) {
	msc.mu.Lock()
	defer msc.mu.Unlock()
	loading, ok := msc.loadingMembers[libraryTitle]
	if !ok {
		return
	}
	delete(msc.loadingMembers, libraryTitle)
	for _, collection := range msc.collections[libraryTitle] {
		if collection != nil {
			collection.MediaItems = loading[collection.Title]
		}
	}
}

// StopLoadingMembers stops keeping a library's movies aside, without changing its collections
func (msc *MediaServerCollectionsCache) StopLoadingMembers(libraryTitle string) {
	msc.mu.Lock()
	defer msc.mu.Unlock()
	delete(msc.loadingMembers, libraryTitle)
}

// SetCollectionMembers replaces the movies of a collection
func (msc *MediaServerCollectionsCache) SetCollectionMembers(ratingKey string, items []models.MediaItem) {
	msc.mu.Lock()
	defer msc.mu.Unlock()
	for _, lib := range msc.collections {
		if collection, ok := lib[ratingKey]; ok && collection != nil {
			collection.MediaItems = items
			return
		}
	}
}

// Retain removes the collections whose rating keys aren't in keep
func (msc *MediaServerCollectionsCache) Retain(keep map[string]bool) {
	msc.mu.Lock()
	defer msc.mu.Unlock()
	for libraryTitle, lib := range msc.collections {
		for ratingKey := range lib {
			if !keep[ratingKey] {
				delete(lib, ratingKey)
			}
		}
		if len(lib) == 0 {
			delete(msc.collections, libraryTitle)
		}
	}
}

// SetAllCollections replaces every collection, for example with the ones saved in the database
func (msc *MediaServerCollectionsCache) SetAllCollections(collections []models.CollectionItem) {
	msc.mu.Lock()
	defer msc.mu.Unlock()
	msc.collections = make(map[string]map[string]*models.CollectionItem)
	for i := range collections {
		collection := collections[i]
		lib := msc.collections[collection.LibraryTitle]
		if lib == nil {
			lib = make(map[string]*models.CollectionItem)
			msc.collections[collection.LibraryTitle] = lib
		}
		lib[collection.RatingKey] = &collection
	}
}

// FindCollectionForMovie returns the media server collection that a MediUX collection's images apply to, for a movie in it.
// Only collections with the movie, or with the same TMDB ID, are used. The best one has the same TMDB ID, then the most
// of the MediUX collection's movies, then is in the movie's library, then has the fewest other movies.
// It's returned without its movies.
func (msc *MediaServerCollectionsCache) FindCollectionForMovie(collectionTMDBID, libraryTitle, movieTMDBID string, collectionMovieTMDBIDs []string) (models.CollectionItem, bool) {
	msc.mu.RLock()
	defer msc.mu.RUnlock()

	inMediuxCollection := make(map[string]bool, len(collectionMovieTMDBIDs))
	for _, id := range collectionMovieTMDBIDs {
		inMediuxCollection[id] = true
	}

	type score struct {
		sameTMDB, overlap, sameLibrary, others int
	}
	better := func(a, b score) bool {
		if a.sameTMDB != b.sameTMDB {
			return a.sameTMDB > b.sameTMDB
		}
		if a.overlap != b.overlap {
			return a.overlap > b.overlap
		}
		if a.sameLibrary != b.sameLibrary {
			return a.sameLibrary > b.sameLibrary
		}
		return a.others < b.others
	}

	var best *models.CollectionItem
	var bestScore score
	for _, lib := range msc.collections {
		for _, collection := range lib {
			if collection == nil {
				continue
			}
			s := score{}
			hasMovie := false
			for _, member := range collection.MediaItems {
				if member.TMDB_ID == movieTMDBID {
					hasMovie = true
				}
				if inMediuxCollection[member.TMDB_ID] {
					s.overlap++
				} else {
					s.others++
				}
			}
			if collectionTMDBID != "" && collection.TMDB_ID == collectionTMDBID {
				s.sameTMDB = 1
			}
			if !hasMovie && s.sameTMDB == 0 {
				continue
			}
			if collection.LibraryTitle == libraryTitle {
				s.sameLibrary = 1
			}
			// Ties go to the lowest rating key, so the same collection is always picked
			if best == nil || better(s, bestScore) || (!better(bestScore, s) && collection.RatingKey < best.RatingKey) {
				best, bestScore = collection, s
			}
		}
	}
	if best == nil {
		return models.CollectionItem{}, false
	}
	match := *best
	match.MediaItems = nil
	return match, true
}
