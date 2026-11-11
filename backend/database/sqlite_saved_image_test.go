package database

import (
	"aura/models"
	"testing"
	"time"
)

func TestSaveDownloadedImage(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	item := models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies", RatingKey: "1", Type: "movie", Title: "Movie", Year: 2020,
		Movie: &models.MediaItemMovie{File: models.MediaItemFile{Path: "/movies/Movie.mkv"}}}
	if Err := db.UpsertSavedItem(ctx, models.DBSavedItem{MediaItem: item, PosterSets: []models.DBPosterSetDetail{{
		PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "set", Type: "movie", Title: "Set"}, Images: []models.ImageFile{
			{ID: "p", Type: "poster", Modified: day(1)},
			{ID: "b", Type: "backdrop", Modified: day(1)},
		}},
		SelectedTypes:  models.SelectedTypes{Poster: true, Backdrop: true},
		LastDownloaded: day(1),
	}}}); Err.Message != "" {
		t.Fatal(Err.Message)
	}

	saved, Err := db.SaveDownloadedImage(ctx, item, "set", models.ImageFile{ID: "p", Type: "poster", Modified: day(9)})
	if Err.Message != "" || !saved {
		t.Fatalf("SaveDownloadedImage = %v %s", saved, Err.Message)
	}

	out, Err := db.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
	if Err.Message != "" || len(out.Items) != 1 || len(out.Items[0].PosterSets) != 1 {
		t.Fatalf("saved sets = %+v %s", out.Items, Err.Message)
	}
	set := out.Items[0].PosterSets[0]
	modified := map[string]time.Time{}
	for _, image := range set.Images {
		modified[image.ID] = image.Modified
	}
	// The downloaded image is updated, the other keeps what was saved
	if !modified["p"].Equal(day(9)) || !modified["b"].Equal(day(1)) {
		t.Errorf("images = %v, want only the poster updated", modified)
	}
	if !set.LastDownloaded.After(day(1)) {
		t.Errorf("last downloaded = %v, want it updated", set.LastDownloaded)
	}

	// A set that isn't saved for the item is saved when the job finishes
	if saved, Err := db.SaveDownloadedImage(ctx, item, "other", models.ImageFile{ID: "x", Type: "poster"}); saved || Err.Message != "" {
		t.Errorf("SaveDownloadedImage for a set that isn't saved = %v %s", saved, Err.Message)
	}
}
