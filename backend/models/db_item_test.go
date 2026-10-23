package models

import "testing"

func TestQueueEntry(t *testing.T) {
	movie := MediaItem{TMDB_ID: "863", LibraryTitle: "Movies", Edition: "4K", Title: "Toy Story 2"}
	collectionImages := &QueuedCollectionImages{
		Collection: CollectionItem{RatingKey: "c1", TMDB_ID: "10194", LibraryTitle: "Collections", Title: "Toy Story Collection"},
		Images:     []ImageFile{{Type: "collection_poster"}},
	}

	// An item with sets is listed by its media item, even with collection images
	withSets := DBSavedItem{MediaItem: movie, PosterSets: []DBPosterSetDetail{{}}, CollectionImages: collectionImages}
	if tmdbID, library, edition, title := withSets.QueueEntry(); withSets.IsCollectionOnly() || tmdbID != "863" || library != "Movies" || edition != "4K" || title != "Toy Story 2" {
		t.Errorf("with sets = %s %s %s %s", tmdbID, library, edition, title)
	}

	// An item with only collection images is listed by its collection
	collectionOnly := DBSavedItem{MediaItem: movie, CollectionImages: collectionImages}
	if tmdbID, library, edition, title := collectionOnly.QueueEntry(); !collectionOnly.IsCollectionOnly() || tmdbID != "10194" || library != "Collections" || edition != "" || title != "Toy Story Collection" {
		t.Errorf("collection only = %s %s %s %s", tmdbID, library, edition, title)
	}

	// No sets and no collection images isn't collection only
	if (DBSavedItem{MediaItem: movie, CollectionImages: &QueuedCollectionImages{}}).IsCollectionOnly() {
		t.Error("an item without collection images isn't collection only")
	}
}
