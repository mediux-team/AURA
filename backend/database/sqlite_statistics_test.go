package database

import (
	"aura/models"
	"testing"
	"time"
)

func TestStatisticsQueries(t *testing.T) {
	db := openTestMediaDB(t)
	previousClient := Client
	Client = db
	t.Cleanup(func() { Client = previousClient })
	ctx := testContext()

	day := time.Date(2026, 10, 20, 12, 0, 0, 0, time.Local)
	item := models.MediaItem{TMDB_ID: "1", LibraryTitle: "Movies", Type: "movie", Title: "Movie"}
	set := models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "set", Type: "movie", UserCreated: "creator"}}
	add := func(kind string, created time.Time, images ...string) {
		event := models.NewDownloadEvent(item, set, models.SourceDownloadWindow, kind)
		for _, imageType := range images {
			event.AddImage(models.ImageFile{Type: imageType, FileSize: 10}, true)
		}
		event.CreatedAt = created
		if Err := db.InsertDownloadEvent(ctx, event); Err.Message != "" {
			t.Fatal(Err.Message)
		}
	}
	add("download", day.AddDate(0, 0, -10), "poster", "backdrop")
	add("download", day, "poster") // The same set again
	add("update", day, "backdrop")

	totals, Err := GetDownloadEventTotals(ctx, time.Time{})
	if Err.Message != "" || len(totals) != 2 {
		t.Fatalf("totals = %+v, %s", totals, Err.Message)
	}
	// Since a day, only the newer events
	recent, _ := GetDownloadEventTotals(ctx, day.AddDate(0, 0, -1))
	recentSets := 0
	for _, total := range recent {
		if total.Kind == "download" {
			recentSets = total.Events
		}
	}
	if recentSets != 1 {
		t.Errorf("recent totals = %+v", recent)
	}

	if redownloads, _ := GetRedownloadCount(ctx, time.Time{}); redownloads != 1 {
		t.Errorf("redownloads = %d, want 1", redownloads)
	}

	timeline, _ := GetDownloadEventTimeline(ctx, time.Time{}, "day")
	if len(timeline) != 3 || timeline[len(timeline)-1].Date != day.Format("2006-01-02") {
		t.Errorf("timeline = %+v", timeline)
	}

	types, _ := GetDownloadEventImageTypes(ctx, time.Time{})
	counts := map[string]int{}
	for _, imageType := range types {
		counts[imageType.Name] = imageType.Count
	}
	if counts["poster"] != 2 || counts["backdrop"] != 2 {
		t.Errorf("image types = %+v", types)
	}

	creators, _ := GetDownloadEventGroups(ctx, time.Time{}, "creator", 10)
	if len(creators) != 1 || creators[0].Name != "creator" || creators[0].Sets != 2 || creators[0].Images != 4 {
		t.Errorf("creators = %+v", creators)
	}

	// A failed image in the Download History
	if Err := db.InsertDownloadHistoryEntry(ctx, DownloadHistoryEntry{TMDB_ID: "1", LibraryTitle: "Movies", MediaItemTitle: "Movie", SetID: "set", SetTitle: "Set",
		ImagesFailed: 1, FailedImages: []models.ImageDownloadResult{{ImageType: "poster", FailureReason: "Access denied"}}}); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	reasons, _ := GetFailureReasons(ctx, 5)
	if len(reasons) != 1 || reasons[0].Name != "Access denied" || reasons[0].Count != 1 {
		t.Errorf("reasons = %+v", reasons)
	}
	if count, _ := CountDownloadHistory(ctx); count != 1 {
		t.Errorf("history count = %d", count)
	}

	// Each day's numbers replace that day's earlier ones
	_ = UpsertStatisticsSnapshot(ctx, "2026-10-20", `{"items_with_sets":1}`)
	_ = UpsertStatisticsSnapshot(ctx, "2026-10-20", `{"items_with_sets":2}`)
	snapshots, _ := GetStatisticsSnapshots(ctx, "2026-10-01")
	if len(snapshots) != 1 || snapshots[0].Data != `{"items_with_sets":2}` {
		t.Errorf("snapshots = %+v", snapshots)
	}
}
