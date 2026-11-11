package downloadqueue

import (
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"context"
	"testing"
	"time"
)

func TestImagesToSave(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	setImages := []models.ImageFile{
		{ID: "poster", Type: "poster", Modified: day(9)},
		{ID: "backdrop", Type: "backdrop", Modified: day(9)},
		{ID: "e1", Type: "titlecard", Modified: day(9)},
		{ID: "logo", Type: "logo", Modified: day(9)},
	}
	results := []mediaserver.ImageResult{
		{Image: setImages[0]},
		{Image: setImages[1], Err: logging.LogErrorInfo{Message: "failed"}},
		{Image: setImages[2], Err: logging.LogErrorInfo{Message: "failed"}},
	}
	previous := map[string]models.ImageFile{
		"backdrop|backdrop": {ID: "backdrop", Type: "backdrop", Modified: day(1)},
	}

	saved := map[string]time.Time{}
	for _, image := range imagesToSave(setImages, results, previous) {
		saved[image.ID] = image.Modified
	}
	// The poster downloaded, so it's saved with its new date
	if !saved["poster"].Equal(day(9)) {
		t.Errorf("poster saved with %v, want the new date", saved["poster"])
	}
	// The backdrop failed, so it keeps what was saved before
	if !saved["backdrop"].Equal(day(1)) {
		t.Errorf("backdrop saved with %v, want the date saved before", saved["backdrop"])
	}
	// The title card failed and wasn't saved before, so it isn't saved
	if _, ok := saved["e1"]; ok {
		t.Error("the failed new title card shouldn't be saved")
	}
	// Images the job didn't download are saved as they are
	if !saved["logo"].Equal(day(9)) {
		t.Errorf("logo saved with %v, want it saved as it is", saved["logo"])
	}
}

func TestSavedImagesBySet(t *testing.T) {
	oldGet := getSavedSets
	t.Cleanup(func() { getSavedSets = oldGet })
	getSavedSets = func(_ context.Context, _ models.DBFilter) (database.PagedSavedItems, logging.LogErrorInfo) {
		return database.PagedSavedItems{Items: []models.DBSavedItem{
			{MediaItem: models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies", Edition: "4K"}, PosterSets: []models.DBPosterSetDetail{
				{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "a"}, Images: []models.ImageFile{{ID: "other-edition", Type: "poster"}}}},
			}},
			{MediaItem: models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies"}, PosterSets: []models.DBPosterSetDetail{
				{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "a"}, Images: []models.ImageFile{{ID: "p", Type: "poster"}}}},
			}},
		}}, logging.LogErrorInfo{}
	}

	bySet := savedImagesBySet(context.Background(), models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies"})
	if _, ok := bySet["a"]["poster|p"]; !ok || len(bySet["a"]) != 1 {
		t.Errorf("saved images = %+v, want only the item's own edition", bySet)
	}
}
