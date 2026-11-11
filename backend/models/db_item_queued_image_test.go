package models

import "testing"

func TestDownloads(t *testing.T) {
	poster := ImageFile{ID: "p", Type: "poster"}
	backdrop := ImageFile{ID: "b", Type: "backdrop"}

	whole := DBPosterSetDetail{}
	if !whole.Downloads(poster) || !whole.Downloads(backdrop) {
		t.Error("a job without OnlyImages downloads every image")
	}

	update := DBPosterSetDetail{OnlyImages: []QueuedImage{{ID: "b", Type: "backdrop", ReasonTitle: "Image Updated"}}}
	if update.Downloads(poster) || !update.Downloads(backdrop) {
		t.Error("a job with OnlyImages downloads only those images")
	}
	if queued, ok := update.QueuedImage(backdrop); !ok || queued.ReasonTitle != "Image Updated" {
		t.Errorf("QueuedImage = %+v %v, want the backdrop's reason", queued, ok)
	}
	// The same ID with another type isn't the same image
	if update.Downloads(ImageFile{ID: "b", Type: "poster"}) {
		t.Error("an image matches by ID and type")
	}
}
