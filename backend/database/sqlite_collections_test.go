package database

import (
	"aura/models"
	"testing"
)

func TestSaveAndGetCollections(t *testing.T) {
	db := &SQliteDB{conn: openTestDB(t)}
	if Err := db.CreateTables(testContext()); Err.Message != "" {
		t.Fatalf("CreateTables: %s", Err.Message)
	}

	toyStory := models.CollectionItem{
		RatingKey: "c1", Index: "7", TMDB_ID: "10194", Title: "Toy Story Collection", ChildCount: 2, LibraryTitle: "Movies",
		MediaItems: []models.MediaItem{
			{RatingKey: "m2", TMDB_ID: "863", LibraryTitle: "Movies", Title: "Toy Story 2", Year: 1999},
			{RatingKey: "m1", TMDB_ID: "862", LibraryTitle: "Movies", Title: "Toy Story", Year: 1995},
		},
	}
	alien := models.CollectionItem{RatingKey: "c2", Title: "Alien Collection", LibraryTitle: "Movies"}
	if Err := db.SaveCollections(testContext(), []models.CollectionItem{toyStory, alien}); Err.Message != "" {
		t.Fatalf("SaveCollections: %s", Err.Message)
	}

	collections, Err := db.GetCollections(testContext())
	if Err.Message != "" || len(collections) != 2 {
		t.Fatalf("GetCollections = %d collections, %s", len(collections), Err.Message)
	}
	got := collections[1]
	if got.RatingKey != "c1" || got.Index != "7" || got.TMDB_ID != "10194" || got.ChildCount != 2 {
		t.Errorf("collection = %+v", got)
	}
	// The movies are oldest first, with what's needed to find them in the library
	if len(got.MediaItems) != 2 || got.MediaItems[0].TMDB_ID != "862" || got.MediaItems[1].RatingKey != "m2" ||
		got.MediaItems[0].Type != "movie" || got.MediaItems[0].LibraryTitle != "Movies" {
		t.Errorf("movies = %+v", got.MediaItems)
	}

	// Saving again replaces everything, a collection that's gone is removed with its movies
	if Err := db.SaveCollections(testContext(), []models.CollectionItem{alien}); Err.Message != "" {
		t.Fatalf("SaveCollections: %s", Err.Message)
	}
	collections, _ = db.GetCollections(testContext())
	var movies int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM CollectionItems`).Scan(&movies); err != nil {
		t.Fatal(err)
	}
	if len(collections) != 1 || collections[0].RatingKey != "c2" || movies != 0 {
		t.Errorf("after replacing: %+v, %d movies", collections, movies)
	}
}
