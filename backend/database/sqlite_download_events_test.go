package database

import (
	"aura/models"
	"testing"
	"time"
)

func TestBackfillDownloadEvents(t *testing.T) {
	db := openTestMediaDB(t)
	previousClient := Client
	Client = db
	t.Cleanup(func() { Client = previousClient })
	ctx := testContext()

	num := func(n int) *int { return &n }
	downloaded := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	show := models.DBSavedItem{
		MediaItem: models.MediaItem{TMDB_ID: "2", LibraryTitle: "Series", RatingKey: "2", Type: "show", Title: "Show", Year: 2020,
			Series: &models.MediaItemSeries{Seasons: []models.MediaItemSeason{{SeasonNumber: 1, RatingKey: "21", Episodes: []models.MediaItemEpisode{{SeasonNumber: 1, EpisodeNumber: 1, RatingKey: "211"}}}}}},
		PosterSets: []models.DBPosterSetDetail{{
			PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "show-set", Type: "show", Title: "Show Set", UserCreated: "creator"}, Images: []models.ImageFile{
				{ID: "p", Type: "poster", ItemTMDB_ID: "2"},
				{ID: "s1", Type: "season_poster", SeasonNumber: num(1), ItemTMDB_ID: "2"},
				{ID: "e1", Type: "titlecard", SeasonNumber: num(1), EpisodeNumber: num(1), ItemTMDB_ID: "2"},
				{ID: "e2", Type: "titlecard", SeasonNumber: num(1), EpisodeNumber: num(2), ItemTMDB_ID: "2"}, // The show doesn't have this episode
			}},
			SelectedTypes:  models.SelectedTypes{Poster: true, Titlecard: true},
			LastDownloaded: downloaded,
		}},
	}
	movie := models.DBSavedItem{
		MediaItem: models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies", RatingKey: "1", Type: "movie", Title: "Movie", Year: 2020,
			Movie: &models.MediaItemMovie{File: models.MediaItemFile{Path: "/movies/Movie.mkv"}}},
		PosterSets: []models.DBPosterSetDetail{{
			PosterSet:     models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "movie-set", Type: "movie", Title: "Movie Set", UserCreated: "other"}, Images: []models.ImageFile{{ID: "mp", Type: "poster", ItemTMDB_ID: "1"}}},
			SelectedTypes: models.SelectedTypes{Backdrop: true}, // Saved for a backdrop the set doesn't have
		}},
	}
	for _, item := range []models.DBSavedItem{show, movie} {
		if Err := db.UpsertSavedItem(ctx, item); Err.Message != "" {
			t.Fatal(Err.Message)
		}
	}

	events, Err := BackfillDownloadEvents(ctx)
	if Err.Message != "" || events != 2 {
		t.Fatalf("BackfillDownloadEvents = %d, %s", events, Err.Message)
	}

	type row struct {
		kind, source, imageTypes, createdAt string
		succeeded, estimated                int
	}
	got := map[string]row{}
	rows, err := db.conn.Query(`SELECT set_id, kind, source, image_types, created_at, images_succeeded, estimated FROM DownloadEvents`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var setID string
		var r row
		if err := rows.Scan(&setID, &r.kind, &r.source, &r.imageTypes, &r.createdAt, &r.succeeded, &r.estimated); err != nil {
			t.Fatal(err)
		}
		got[setID] = r
	}

	// The poster and the one title card the show has, dated by the last download
	if r := got["show-set"]; r.kind != "download" || r.source != "backfill" || r.succeeded != 2 || r.imageTypes != `{"poster":1,"titlecard":1}` || r.estimated != 1 || r.createdAt[:10] != "2026-03-01" {
		t.Errorf("show event = %+v", r)
	}
	// Nothing matched the selected types, so the set was only saved
	if r := got["movie-set"]; r.kind != "saved" || r.succeeded != 0 {
		t.Errorf("movie event = %+v", r)
	}
}
