package autodownload

import (
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"testing"
	"time"
)

func TestQueuedUpdate(t *testing.T) {
	saved := models.DBPosterSetDetail{
		PosterSet:     models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "b", Title: "B"}, Images: []models.ImageFile{{ID: "e1", Type: "titlecard"}}},
		SelectedTypes: models.SelectedTypes{Titlecard: true},
		AutoDownload:  true,
	}
	latest := models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "b", Title: "B updated"}, Images: []models.ImageFile{{ID: "e1", Type: "titlecard"}, {ID: "e2", Type: "titlecard"}}}
	changed := []ImageFileWithReason{{ImageFile: models.ImageFile{ID: "e2", Type: "titlecard"}, ReasonTitle: "Episode Added", Reason: "Season 01 Episode 02 added"}}

	update := queuedUpdate(saved, latest, changed)
	// The set is saved with its latest images, and keeps its options
	if update.Title != "B updated" || len(update.Images) != 2 || !update.SelectedTypes.Titlecard || !update.AutoDownload {
		t.Errorf("update = %+v, want the latest set with the saved options", update)
	}
	// Only the changed image is downloaded, with why
	if len(update.OnlyImages) != 1 || update.OnlyImages[0].ID != "e2" || update.OnlyImages[0].ReasonTitle != "Episode Added" {
		t.Errorf("OnlyImages = %+v, want only the added episode", update.OnlyImages)
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

func TestFormatReasonDate(t *testing.T) {
	oldLocal := time.Local
	t.Cleanup(func() { time.Local = oldLocal })
	time.Local = time.FixedZone("EST", -5*60*60)

	// A MediUX date in UTC is shown in the server's time zone
	if got := formatReasonDate(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)); got != "2026-01-01 22:04:05 EST" {
		t.Errorf("formatReasonDate = %q", got)
	}
}
