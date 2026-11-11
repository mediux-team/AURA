package autodownload

import (
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"testing"
)

func TestInsertRedownloadedSetSavesOnlyThatSet(t *testing.T) {
	oldUpsert := upsertSavedItem
	t.Cleanup(func() { upsertSavedItem = oldUpsert })
	var saved models.DBSavedItem
	upsertSavedItem = func(_ context.Context, item models.DBSavedItem) logging.LogErrorInfo {
		saved = item
		return logging.LogErrorInfo{}
	}

	item := models.MediaItem{TMDB_ID: "1", LibraryTitle: "Series", Type: "show", Title: "Show"}
	setA := models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "a"}}, SelectedTypes: models.SelectedTypes{Poster: true}, AutoDownload: true}
	setB := models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "b"}}, SelectedTypes: models.SelectedTypes{Titlecard: true}, AutoDownload: true}
	dbItem := models.DBSavedItem{MediaItem: item, PosterSets: []models.DBPosterSetDetail{setA, setB}}

	latestB := models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "b", Title: "B updated"}, Images: []models.ImageFile{{ID: "e1", Type: "titlecard"}}}
	if Err := insertRedownloadedSetIntoDB(testContext(), item, latestB, dbItem, setB); Err.Message != "" {
		t.Fatal(Err.Message)
	}

	// Set A isn't saved again, so an update it got earlier in the check stays
	if len(saved.PosterSets) != 1 || saved.PosterSets[0].ID != "b" || saved.PosterSets[0].Title != "B updated" {
		t.Fatalf("saved sets = %+v, want only the updated set b", saved.PosterSets)
	}
	if !saved.PosterSets[0].SelectedTypes.Titlecard || !saved.PosterSets[0].AutoDownload {
		t.Errorf("set b = %+v, want its selected types and Auto Download kept", saved.PosterSets[0])
	}
}

func TestLatestSavedItem(t *testing.T) {
	oldGet := getSavedSets
	t.Cleanup(func() { getSavedSets = oldGet })
	var filter models.DBFilter
	getSavedSets = func(_ context.Context, f models.DBFilter) (database.PagedSavedItems, logging.LogErrorInfo) {
		filter = f
		return database.PagedSavedItems{Items: []models.DBSavedItem{
			{MediaItem: models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies", Edition: "Director's Cut"}, PosterSets: []models.DBPosterSetDetail{{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "cut"}}}}},
			{MediaItem: models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies"}, PosterSets: []models.DBPosterSetDetail{{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "now"}}}}},
		}}, logging.LogErrorInfo{}
	}

	saved, found, Err := latestSavedItem(testContext(), models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies"})
	if Err.Message != "" || !found {
		t.Fatalf("latestSavedItem = %v %s", found, Err.Message)
	}
	if filter.ItemTMDB_ID != "1" || filter.ItemLibraryTitle != "Movies" {
		t.Errorf("filter = %+v, want the item's TMDB ID and library", filter)
	}
	// The item's own edition, as it is in the database now
	if len(saved.PosterSets) != 1 || saved.PosterSets[0].ID != "now" {
		t.Errorf("saved sets = %+v, want the set saved now for the item without an edition", saved.PosterSets)
	}

	if _, found, _ := latestSavedItem(testContext(), models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies", Edition: "Extended"}); found {
		t.Error("an edition without saved sets shouldn't be found")
	}
}
