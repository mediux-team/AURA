package models

import "testing"

func TestDownloadEventCountsImages(t *testing.T) {
	special, first := 0, 1
	event := NewDownloadEvent(MediaItem{TMDB_ID: "1", Title: "Show", Type: "show"}, PosterSet{BaseSetInfo: BaseSetInfo{ID: "set", UserCreated: "creator"}}, "", DownloadKindDownload)
	if event.Source != SourceDownloadQueue || event.SetCreator != "creator" || event.Attempted() {
		t.Fatalf("new event = %+v", event)
	}

	event.AddImage(ImageFile{Type: "poster", FileSize: 100}, true)
	event.AddImage(ImageFile{Type: "season_poster", SeasonNumber: &special, FileSize: 10}, true)
	event.AddImage(ImageFile{Type: "season_poster", SeasonNumber: &first, FileSize: 10}, true)
	event.AddImage(ImageFile{Type: "titlecard", FileSize: 5}, false)

	if event.ImagesSucceeded != 3 || event.ImagesFailed != 1 || event.Bytes != 120 || !event.Attempted() {
		t.Errorf("counts = %d succeeded, %d failed, %d bytes", event.ImagesSucceeded, event.ImagesFailed, event.Bytes)
	}
	want := map[string]int{"poster": 1, "special_season_poster": 1, "season_poster": 1}
	for imageType, n := range want {
		if event.ImageTypes[imageType] != n {
			t.Errorf("%s = %d, want %d (all %v)", imageType, event.ImageTypes[imageType], n, event.ImageTypes)
		}
	}

	// A collection's images are counted under the collection, with the set they're from
	collection := NewCollectionDownloadEvent(CollectionItem{TMDB_ID: "10", Title: "Collection", LibraryTitle: "Movies"}, &BaseSetInfo{ID: "c", UserCreated: "maker"}, SourceDownloadWindow)
	if collection.MediaItemType != "collection" || collection.SetID != "c" || collection.SetCreator != "maker" || collection.Source != SourceDownloadWindow {
		t.Errorf("collection event = %+v", collection)
	}
}
