package routes_db

import (
	"aura/models"
	"testing"
)

func TestDownloadWindowEvent(t *testing.T) {
	item := models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies", Type: "movie", Title: "Movie"}
	set := models.DBPosterSetDetail{
		PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "set", Type: "collection", UserCreated: "creator"}, Images: []models.ImageFile{
			{ID: "poster", Type: "poster", ItemTMDB_ID: "1", FileSize: 100},
			{ID: "backdrop", Type: "backdrop", ItemTMDB_ID: "1", FileSize: 200},
			{ID: "other", Type: "poster", ItemTMDB_ID: "2", FileSize: 300}, // Another movie in the collection
		}},
		SelectedTypes: models.SelectedTypes{Poster: true, Backdrop: true},
	}

	// The images the window downloaded
	event := downloadWindowEvent(addItemRequest{MediaItem: item, DownloadedImageIDs: []string{"poster"}, FailedImages: 1}, set)
	if event.Source != models.SourceDownloadWindow || event.Kind != models.DownloadKindDownload || event.ImagesSucceeded != 1 || event.ImagesFailed != 1 || event.Bytes != 100 || event.Estimated {
		t.Errorf("downloaded event = %+v", event)
	}

	// Without them, the item's images of the selected types
	event = downloadWindowEvent(addItemRequest{MediaItem: item}, set)
	if event.ImagesSucceeded != 2 || event.Bytes != 300 || !event.Estimated {
		t.Errorf("estimated event = %+v", event)
	}

	// Add to Database Only saves the set without downloading
	event = downloadWindowEvent(addItemRequest{MediaItem: item, AddToDBOnly: true}, set)
	if event.Kind != models.DownloadKindSaved || event.Attempted() {
		t.Errorf("saved event = %+v", event)
	}
}
