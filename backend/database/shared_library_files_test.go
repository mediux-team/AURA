package database

import (
	"aura/cache"
	"aura/config"
	"aura/models"
	"context"
	"testing"
)

// A Silo item's sets are saved in its other libraries with the file in each library
func TestUpsertSavedItemUsesEachLibrarysFile(t *testing.T) {
	db := openTestMediaDB(t)
	previousClient, previousStore, previousType, previousDetails := Client, cache.LibraryStore, config.Current.MediaServer.Type, GetMediaItemDetails
	t.Cleanup(func() {
		Client, cache.LibraryStore, config.Current.MediaServer.Type, GetMediaItemDetails = previousClient, previousStore, previousType, previousDetails
	})
	Client = db
	cache.LibraryStore = cache.Cache_NewLibraryCache()
	config.Current.MediaServer.Type = "Silo"
	ctx := testContext()

	files := map[string]string{"4K Movies": "/data/4kmovies/Movie.mkv", "Movies": "/data/movies/Movie.mkv"}
	for _, library := range []string{"4K Movies", "Movies"} {
		cache.LibraryStore.UpdateSection(&models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: library, Type: "movie"}, MediaItems: []models.MediaItem{
			// The library listing doesn't have the file
			{TMDB_ID: "1", LibraryTitle: library, RatingKey: "movie-tmdb-1", Type: "movie", Title: "Movie", Year: 2025},
		}})
	}
	GetMediaItemDetails = func(_ context.Context, item *models.MediaItem) bool {
		item.Movie = &models.MediaItemMovie{File: models.MediaItemFile{Path: files[item.LibraryTitle]}}
		return true
	}

	saved := models.DBSavedItem{
		MediaItem: models.MediaItem{TMDB_ID: "1", LibraryTitle: "4K Movies", RatingKey: "movie-tmdb-1", Type: "movie", Title: "Movie", Year: 2025,
			Movie: &models.MediaItemMovie{File: models.MediaItemFile{Path: files["4K Movies"]}}},
		PosterSets: []models.DBPosterSetDetail{{
			PosterSet:     models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "set", Type: "movie", Title: "Set", UserCreated: "creator"}, Images: []models.ImageFile{{ID: "p", Type: "poster", ItemTMDB_ID: "1"}}},
			SelectedTypes: models.SelectedTypes{Poster: true},
		}},
	}
	if Err := UpsertSavedItem(ctx, saved); Err.Message != "" {
		t.Fatal(Err.Message)
	}

	for library, want := range files {
		var path string
		if err := db.conn.QueryRow(`SELECT mv.path FROM Movies mv JOIN MediaItems mi ON mi.id = mv.media_item_id WHERE mi.library_title = ?`, library).Scan(&path); err != nil {
			t.Fatalf("%s: %v", library, err)
		}
		if path != want {
			t.Errorf("%s file = %s, want %s", library, path, want)
		}
	}

	// Without the file in the other library, the sets aren't saved there
	exec(t, db.conn, `DELETE FROM SavedItems WHERE library_title = 'Movies'`)
	GetMediaItemDetails = func(context.Context, *models.MediaItem) bool { return false }
	if Err := UpsertSavedItem(ctx, saved); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM SavedItems WHERE library_title = 'Movies'`); n != 0 {
		t.Errorf("sets were saved in Movies without its file")
	}
}
