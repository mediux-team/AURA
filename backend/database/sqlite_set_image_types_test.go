package database

import (
	"aura/models"
	"testing"
)

func TestMovieAndCollectionSetsOnlyKeepPostersAndBackdrops(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	all := models.SelectedTypes{Poster: true, Backdrop: true, SeasonPoster: true, SpecialSeasonPoster: true, Titlecard: true}
	set := func(id, setType string) models.DBPosterSetDetail {
		return models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: id, Type: setType, Title: id, UserCreated: "creator"}}, SelectedTypes: all}
	}
	item := models.DBSavedItem{
		MediaItem: models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies", RatingKey: "1", Type: "movie", Title: "Movie", Year: 2020,
			Movie: &models.MediaItemMovie{File: models.MediaItemFile{Path: "/movies/Movie.mkv"}}},
		PosterSets: []models.DBPosterSetDetail{set("movieSet", "movie"), set("collectionSet", "collection")},
	}
	show := models.DBSavedItem{
		MediaItem: models.MediaItem{TMDB_ID: "2", LibraryTitle: "Series", RatingKey: "2", Type: "show", Title: "Show", Year: 2020,
			Series: &models.MediaItemSeries{}},
		PosterSets: []models.DBPosterSetDetail{set("showSet", "show")},
	}

	// Saved sets
	for _, saved := range []models.DBSavedItem{item, show} {
		if Err := db.UpsertSavedItem(ctx, saved); Err.Message != "" {
			t.Fatal(Err.Message)
		}
	}
	rows, err := db.conn.Query(`SELECT ps.set_id, si.poster_selected, si.backdrop_selected, si.season_poster_selected, si.special_season_poster_selected, si.titlecard_selected
FROM SavedItems si JOIN PosterSets ps ON ps.id = si.poster_set_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var setID string
		var poster, backdrop, season, special, titlecard bool
		if err := rows.Scan(&setID, &poster, &backdrop, &season, &special, &titlecard); err != nil {
			t.Fatal(err)
		}
		showSet := setID == "showSet"
		if !poster || !backdrop || season != showSet || special != showSet || titlecard != showSet {
			t.Errorf("%s saved poster=%v backdrop=%v season=%v special=%v titlecard=%v", setID, poster, backdrop, season, special, titlecard)
		}
	}

	// Download Queue jobs
	if _, Err := db.EnqueueDownloadQueueJob(ctx, item); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	jobs, Err := db.GetAllDownloadQueueJobs(ctx)
	if Err.Message != "" || len(jobs) != 1 {
		t.Fatalf("jobs = %d %s", len(jobs), Err.Message)
	}
	for _, set := range jobs[0].Payload.PosterSets {
		if set.SelectedTypes != (models.SelectedTypes{Poster: true, Backdrop: true}) {
			t.Errorf("queued %s set = %+v, want only poster and backdrop", set.Type, set.SelectedTypes)
		}
	}
	// The caller's item isn't changed
	if !item.PosterSets[0].SelectedTypes.Titlecard {
		t.Error("queueing changed the caller's selected types")
	}
}

func TestSavedSetAutoUpgradeSetting(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	// Each set owns its own image type, a set without one isn't saved
	set := func(id string, keep bool, types models.SelectedTypes) models.DBPosterSetDetail {
		return models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: id, Type: "movie", Title: id, UserCreated: "creator"}},
			SelectedTypes: types, NoAutoUpgrade: keep}
	}
	item := models.DBSavedItem{
		MediaItem: models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies", RatingKey: "1", Type: "movie", Title: "Movie", Year: 2020,
			Movie: &models.MediaItemMovie{File: models.MediaItemFile{Path: "/movies/Movie.mkv"}}},
		PosterSets: []models.DBPosterSetDetail{set("kept", true, models.SelectedTypes{Poster: true}), set("upgradable", false, models.SelectedTypes{Backdrop: true})},
	}
	if Err := db.UpsertSavedItem(ctx, item); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	keep := func() map[string]bool {
		t.Helper()
		out, Err := db.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
		if Err.Message != "" || len(out.Items) != 1 {
			t.Fatalf("saved = %d %s", len(out.Items), Err.Message)
		}
		got := map[string]bool{}
		for _, set := range out.Items[0].PosterSets {
			got[set.ID] = set.NoAutoUpgrade
		}
		return got
	}
	if got := keep(); len(got) != 2 || !got["kept"] || got["upgradable"] {
		t.Errorf("auto upgrade off = %v", got)
	}

	// A replaced set is removed and the item keeps its other sets
	if Err := db.DeletePosterSetForMediaItem(ctx, "1", "Movies", "", "upgradable"); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if got := keep(); len(got) != 1 || !got["kept"] {
		t.Errorf("after removing a set = %v", got)
	}
}

func TestSavingASetMovesOnlyItsTypes(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	set := func(id string, types models.SelectedTypes) models.DBPosterSetDetail {
		return models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: id, Type: "show", Title: id, UserCreated: id}}, SelectedTypes: types}
	}
	save := func(sets ...models.DBPosterSetDetail) map[string]models.SelectedTypes {
		t.Helper()
		item := models.DBSavedItem{
			MediaItem:  models.MediaItem{TMDB_ID: "2", LibraryTitle: "Series", RatingKey: "2", Type: "show", Title: "Show", Year: 2020, Series: &models.MediaItemSeries{}},
			PosterSets: sets,
		}
		if Err := db.UpsertSavedItem(ctx, item); Err.Message != "" {
			t.Fatal(Err.Message)
		}
		out, Err := db.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
		if Err.Message != "" || len(out.Items) != 1 {
			t.Fatalf("saved = %d %s", len(out.Items), Err.Message)
		}
		got := map[string]models.SelectedTypes{}
		for _, saved := range out.Items[0].PosterSets {
			got[saved.ID] = saved.SelectedTypes
		}
		return got
	}

	save(set("old", models.SelectedTypes{Poster: true, Titlecard: true}))

	// The new set takes the poster, the old set keeps its titlecards
	got := save(set("new", models.SelectedTypes{Poster: true}))
	if len(got) != 2 || got["old"] != (models.SelectedTypes{Titlecard: true}) || got["new"] != (models.SelectedTypes{Poster: true}) {
		t.Errorf("after the poster moved = %+v", got)
	}

	// Once the titlecards move too, the old set has nothing left and is removed
	got = save(set("new", models.SelectedTypes{Poster: true, Titlecard: true}))
	if len(got) != 1 || got["new"] != (models.SelectedTypes{Poster: true, Titlecard: true}) {
		t.Errorf("after the titlecards moved = %+v", got)
	}
}
