package mediaserver

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func withSimultaneousDownloads(t *testing.T, n int) {
	t.Helper()
	old := config.Current.Mediux.SimultaneousDownloads
	config.Current.Mediux.SimultaneousDownloads = n
	t.Cleanup(func() { config.Current.Mediux.SimultaneousDownloads = old })
}

func titlecards(n int) []models.ImageFile {
	images := []models.ImageFile{}
	for i := 1; i <= n; i++ {
		season, episode := 1, i
		images = append(images, models.ImageFile{ID: string(rune('a' + i - 1)), Type: "titlecard", SeasonNumber: &season, EpisodeNumber: &episode})
	}
	return images
}

func TestApplyImagesReturnsEachResultInOrder(t *testing.T) {
	withSimultaneousDownloads(t, 3)
	images := titlecards(6)

	results, _ := applyImages(context.Background(), context.Background(), models.MediaItem{}, images, func(_ context.Context, _ *models.MediaItem, image models.ImageFile) logging.LogErrorInfo {
		// Later images finish first
		time.Sleep(time.Duration(10-*image.EpisodeNumber) * time.Millisecond)
		if *image.EpisodeNumber%2 == 0 {
			return logging.LogErrorInfo{Message: "failed " + image.ID}
		}
		return logging.LogErrorInfo{}
	}, nil)

	for i, result := range results {
		if result.Image.ID != images[i].ID {
			t.Errorf("result %d is image %s, want %s", i, result.Image.ID, images[i].ID)
		}
		wantFailed := (i+1)%2 == 0
		if (result.Err.Message != "") != wantFailed || result.Stopped {
			t.Errorf("result %d = %+v, want failed %v", i, result, wantFailed)
		}
	}
}

func TestApplyImagesDownloadsAtMostTheSimultaneousDownloads(t *testing.T) {
	withSimultaneousDownloads(t, 2)
	var active, most atomic.Int32
	var updates sync.Map

	applyImages(context.Background(), context.Background(), models.MediaItem{}, titlecards(6), func(context.Context, *models.MediaItem, models.ImageFile) logging.LogErrorInfo {
		n := active.Add(1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		active.Add(-1)
		return logging.LogErrorInfo{}
	}, func(u ImageUpdate) {
		key := u.Image.ID
		if u.Finished {
			key += " finished"
		}
		updates.Store(key, true)
	})

	if most.Load() != 2 {
		t.Errorf("at most %d images downloaded at once, want 2", most.Load())
	}
	for _, image := range titlecards(6) {
		if _, ok := updates.Load(image.ID); !ok {
			t.Errorf("no update when image %s started", image.ID)
		}
		if _, ok := updates.Load(image.ID + " finished"); !ok {
			t.Errorf("no update when image %s finished", image.ID)
		}
	}
}

func TestApplyImagesAppliesImagesForTheSameItemOneAtATime(t *testing.T) {
	withSimultaneousDownloads(t, 3)
	season := 1
	images := []models.ImageFile{
		{ID: "poster", Type: "poster"},
		{ID: "backdrop", Type: "backdrop"},
		{ID: "season", Type: "season_poster", SeasonNumber: &season},
	}
	var itemActive atomic.Int32
	overlapped := false

	applyImages(context.Background(), context.Background(), models.MediaItem{}, images, func(_ context.Context, _ *models.MediaItem, image models.ImageFile) logging.LogErrorInfo {
		if image.Type == "poster" || image.Type == "backdrop" {
			if itemActive.Add(1) > 1 {
				overlapped = true
			}
			time.Sleep(10 * time.Millisecond)
			itemActive.Add(-1)
		}
		return logging.LogErrorInfo{}
	}, nil)

	if overlapped {
		t.Error("the poster and backdrop of the same item were applied at the same time")
	}
}

func TestApplyImagesStopsImagesThatHaventStarted(t *testing.T) {
	withSimultaneousDownloads(t, 1)
	stop, stopNow := context.WithCancel(context.Background())
	images := titlecards(3)

	results, _ := applyImages(context.Background(), stop, models.MediaItem{}, images, func(_ context.Context, _ *models.MediaItem, image models.ImageFile) logging.LogErrorInfo {
		// The first image is downloading when the job is stopped, so it finishes
		stopNow()
		return logging.LogErrorInfo{}
	}, nil)

	if results[0].Stopped || results[0].Err.Message != "" {
		t.Errorf("the downloading image = %+v, want it to finish", results[0])
	}
	for _, result := range results[1:] {
		if !result.Stopped {
			t.Errorf("image %s = %+v, want it stopped", result.Image.ID, result)
		}
	}
}

func TestApplyImagesTurnsAPanicIntoAFailedImage(t *testing.T) {
	withSimultaneousDownloads(t, 2)
	results, _ := applyImages(context.Background(), context.Background(), models.MediaItem{}, titlecards(2), func(_ context.Context, _ *models.MediaItem, image models.ImageFile) logging.LogErrorInfo {
		if image.ID == "a" {
			panic("broken")
		}
		return logging.LogErrorInfo{}
	}, nil)

	if results[0].Err.Message == "" || results[0].Stopped {
		t.Errorf("the image that panicked = %+v, want it failed", results[0])
	}
	if results[1].Err.Message != "" {
		t.Errorf("the other image = %+v, want it downloaded", results[1])
	}
}

func TestApplyImagesGivesEachImageItsOwnItem(t *testing.T) {
	withSimultaneousDownloads(t, 3)
	item := models.MediaItem{RatingKey: "old", Series: &models.MediaItemSeries{Seasons: []models.MediaItemSeason{{SeasonNumber: 1}}}}

	_, updated := applyImages(context.Background(), context.Background(), item, titlecards(3), func(_ context.Context, item *models.MediaItem, image models.ImageFile) logging.LogErrorInfo {
		// Like Silo finding the item's new content ID
		item.Series.Seasons[0].Title = image.ID
		if image.ID == "b" {
			item.RatingKey = "new"
		}
		return logging.LogErrorInfo{}
	}, nil)

	if item.Series.Seasons[0].Title != "" {
		t.Error("the item passed in was changed")
	}
	if updated.RatingKey != "new" {
		t.Errorf("updated rating key = %q, want the media server's new one", updated.RatingKey)
	}
}

func TestDownloadSlotsWaitForAFreeSlot(t *testing.T) {
	withSimultaneousDownloads(t, 1)
	s := &slots{changed: make(chan struct{})}
	if !s.acquire(context.Background()) {
		t.Fatal("the first slot should be free")
	}

	// A stopped wait gives up
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if s.acquire(ctx) {
		t.Fatal("there should be no free slot")
	}

	got := make(chan bool)
	go func() { got <- s.acquire(context.Background()) }()
	s.release()
	select {
	case ok := <-got:
		if !ok {
			t.Error("the freed slot should be taken")
		}
	case <-time.After(time.Second):
		t.Error("the wait didn't get the freed slot")
	}
}
