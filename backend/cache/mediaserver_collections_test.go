package cache

import (
	"aura/models"
	"testing"
)

func memberKeys(collection *models.CollectionItem) []string {
	keys := []string{}
	for _, item := range collection.MediaItems {
		keys = append(keys, item.RatingKey)
	}
	return keys
}

func TestCollectionsCacheMembers(t *testing.T) {
	store := Cache_NewCollectionsCache()
	movie := func(key string) *models.MediaItem {
		return &models.MediaItem{RatingKey: key, LibraryTitle: "Movies", Type: "movie"}
	}

	// Plex stores collections by rating key, which isn't the same as the index
	store.UpsertCollection(&models.CollectionItem{RatingKey: "101", Index: "7", Title: "Toy Story Collection", LibraryTitle: "Movies"})
	store.UpdateMediaItemInCollectionByIndex("7", movie("m1"))
	collection, _ := store.GetCollectionByRatingKey("101")
	if keys := memberKeys(collection); len(keys) != 1 || keys[0] != "m1" {
		t.Fatalf("movies after adding by index = %v", keys)
	}

	// Fetching the collection again without its movies keeps them
	store.UpsertCollection(&models.CollectionItem{RatingKey: "101", Index: "7", Title: "Toy Story Collection", LibraryTitle: "Movies"})
	collection, _ = store.GetCollectionByRatingKey("101")
	if len(collection.MediaItems) != 1 {
		t.Errorf("movies after fetching again = %v", memberKeys(collection))
	}

	// While the library loads, the collection keeps its movies until the load is done
	store.StartLoadingMembers("Movies")
	store.UpdateMediaItemInCollectionByTitle("Toy Story Collection", movie("m2"))
	store.UpdateMediaItemInCollectionByTitle("Toy Story Collection", movie("m3"))
	collection, _ = store.GetCollectionByRatingKey("101")
	if keys := memberKeys(collection); len(keys) != 1 || keys[0] != "m1" {
		t.Errorf("movies while loading = %v, want the old ones", keys)
	}
	store.FinishLoadingMembers("Movies")
	collection, _ = store.GetCollectionByRatingKey("101")
	if keys := memberKeys(collection); len(keys) != 2 || keys[0] != "m2" || keys[1] != "m3" {
		t.Errorf("movies after loading = %v, want the new ones", keys)
	}

	// A load that stops keeps the movies the collection had
	store.StartLoadingMembers("Movies")
	store.UpdateMediaItemInCollectionByTitle("Toy Story Collection", movie("m4"))
	store.StopLoadingMembers("Movies")
	store.UpdateMediaItemInCollectionByTitle("Toy Story Collection", movie("m5"))
	collection, _ = store.GetCollectionByRatingKey("101")
	if keys := memberKeys(collection); len(keys) != 3 || keys[2] != "m5" {
		t.Errorf("movies after a stopped load = %v", keys)
	}

	// Collections that are gone are removed
	store.UpsertCollection(&models.CollectionItem{RatingKey: "102", Title: "Alien Collection", LibraryTitle: "Movies"})
	store.Retain(map[string]bool{"102": true})
	if _, found := store.GetCollectionByRatingKey("101"); found || len(store.GetAllCollections()) != 1 {
		t.Errorf("collections after Retain = %+v", store.GetAllCollections())
	}

	// The saved collections replace the cache
	store.SetAllCollections([]models.CollectionItem{{RatingKey: "103", Title: "Dune Collection", LibraryTitle: "4K Movies"}})
	store.SetCollectionMembers("103", []models.MediaItem{*movie("m6")})
	collection, found := store.GetCollectionByRatingKey("103")
	if !found || len(store.GetAllCollections()) != 1 || len(collection.MediaItems) != 1 {
		t.Errorf("collections after SetAllCollections = %+v", store.GetAllCollections())
	}
}

func TestFindCollectionForMovie(t *testing.T) {
	store := Cache_NewCollectionsCache()
	members := func(library string, tmdbIDs ...string) []models.MediaItem {
		items := []models.MediaItem{}
		for _, id := range tmdbIDs {
			items = append(items, models.MediaItem{RatingKey: library + id, TMDB_ID: id, LibraryTitle: library})
		}
		return items
	}
	toyStoryMovies := []string{"862", "863", "10193", "301528"}
	store.SetAllCollections([]models.CollectionItem{
		// A Kometa collection with the Toy Story movies and others
		{RatingKey: "1", Title: "Pixar", LibraryTitle: "Movies", MediaItems: members("Movies", "862", "863", "10193", "301528", "12", "14160")},
		{RatingKey: "2", Title: "Toy Story", LibraryTitle: "Movies", MediaItems: members("Movies", "862", "863", "10193", "301528")},
		// The same Dune collection in two libraries
		{RatingKey: "3", Title: "Dune", LibraryTitle: "Movies", MediaItems: members("Movies", "438631", "693134")},
		{RatingKey: "4", Title: "Dune", LibraryTitle: "4K Movies", MediaItems: members("4K Movies", "438631", "693134")},
		// A Jellyfin collection with its TMDB ID
		{RatingKey: "5", TMDB_ID: "8091", Title: "Alien Collection", LibraryTitle: "Movies"},
	})

	tests := []struct {
		name, collectionTMDBID, library, movie string
		collectionMovies                       []string
		want                                   string
	}{
		{"the collection with the fewest other movies", "10194", "Movies", "863", toyStoryMovies, "2"},
		{"the collection in the movie's library", "726871", "4K Movies", "693134", []string{"438631", "693134"}, "4"},
		{"the collection with the same TMDB ID", "8091", "Movies", "348", []string{"348", "679"}, "5"},
		{"no collection with the movie", "", "Movies", "999", []string{"999"}, ""},
	}
	for _, tt := range tests {
		got, found := store.FindCollectionForMovie(tt.collectionTMDBID, tt.library, tt.movie, tt.collectionMovies)
		if got.RatingKey != tt.want || found != (tt.want != "") {
			t.Errorf("%s: got %q, %v, want %q", tt.name, got.RatingKey, found, tt.want)
		}
		if found && got.MediaItems != nil {
			t.Errorf("%s: the collection should be returned without its movies", tt.name)
		}
	}
}
