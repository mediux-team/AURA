package downloadqueue

import (
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"slices"
	"testing"
)

// retryStubs stand in for the database, with the history entries, the queued jobs and what was queued, retried and stopped
type retryStubs struct {
	entries map[int64]database.DownloadHistoryEntry
	jobs    []database.DownloadQueueJob
	queued  []models.DBSavedItem
	retried []int64
	stopped []int64
}

func setupRetryStubs(t *testing.T) (context.Context, *retryStubs) {
	t.Helper()
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))

	stubs := &retryStubs{entries: map[int64]database.DownloadHistoryEntry{
		1: {ID: 1, TMDB_ID: "100", LibraryTitle: "Movies", MediaItemTitle: "Hope", SetID: "a"},
		2: {ID: 2, TMDB_ID: "100", LibraryTitle: "Movies", MediaItemTitle: "Hope"},
		3: {ID: 3, TMDB_ID: "100", LibraryTitle: "Movies", MediaItemTitle: "Hope", SetID: "gone"},
	}}
	set := func(id string) models.DBPosterSetDetail {
		return models.DBPosterSetDetail{PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: id}}}
	}

	getHistoryEntry = func(_ context.Context, id int64) (database.DownloadHistoryEntry, bool, logging.LogErrorInfo) {
		entry, ok := stubs.entries[id]
		return entry, ok, logging.LogErrorInfo{}
	}
	getDueRetries = func(_ context.Context) ([]database.DownloadHistoryEntry, logging.LogErrorInfo) {
		return []database.DownloadHistoryEntry{stubs.entries[1], stubs.entries[3]}, logging.LogErrorInfo{}
	}
	markRetried = func(_ context.Context, id int64) logging.LogErrorInfo {
		stubs.retried = append(stubs.retried, id)
		return logging.LogErrorInfo{}
	}
	stopRetries = func(_ context.Context, id int64) logging.LogErrorInfo {
		stubs.stopped = append(stubs.stopped, id)
		return logging.LogErrorInfo{}
	}
	getSavedSets = func(_ context.Context, _ models.DBFilter) (database.PagedSavedItems, logging.LogErrorInfo) {
		return database.PagedSavedItems{Items: []models.DBSavedItem{
			{MediaItem: models.MediaItem{TMDB_ID: "100", LibraryTitle: "Movies", Edition: "4K"}, PosterSets: []models.DBPosterSetDetail{set("a")}},
			{MediaItem: models.MediaItem{TMDB_ID: "100", LibraryTitle: "Movies"}, PosterSets: []models.DBPosterSetDetail{set("b"), set("a")}},
		}}, logging.LogErrorInfo{}
	}
	getQueueJobs = func(_ context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo) {
		return stubs.jobs, logging.LogErrorInfo{}
	}
	addToQueue = func(_ context.Context, item models.DBSavedItem) logging.LogErrorInfo {
		stubs.queued = append(stubs.queued, item)
		return logging.LogErrorInfo{}
	}
	getLatestSet = func(_ context.Context, setID, _ string, _ models.MediaItem) (models.PosterSet, logging.LogErrorInfo) {
		return models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: setID}, Images: []models.ImageFile{{ID: "latest-" + setID, Type: "poster"}}}, logging.LogErrorInfo{}
	}
	t.Cleanup(func() {
		getLatestSet = mediux.GetLatestSet
		getHistoryEntry, getDueRetries, markRetried, stopRetries = database.GetDownloadHistoryEntry, database.GetDueDownloadRetries, database.MarkDownloadHistoryRetried, database.StopDownloadRetries
		getSavedSets, getQueueJobs, addToQueue = database.GetAllSavedSets, database.GetAllDownloadQueueJobs, AddToQueue
	})
	return ctx, stubs
}

func TestRetryFailedDownload(t *testing.T) {
	ctx, stubs := setupRetryStubs(t)

	// Only the failed set is queued, for the entry's edition, and the retry is counted
	if title, Err := RetryFailedDownload(ctx, 1); Err.Message != "" || title != "Hope" {
		t.Fatalf("retry = %q %q", title, Err.Message)
	}
	if len(stubs.queued) != 1 || stubs.queued[0].MediaItem.Edition != "" || len(stubs.queued[0].PosterSets) != 1 || stubs.queued[0].PosterSets[0].ID != "a" {
		t.Errorf("queued = %+v", stubs.queued)
	}
	if !slices.Equal(stubs.retried, []int64{1}) {
		t.Errorf("retried = %v, want entry 1 counted", stubs.retried)
	}
	// With the set's latest images, as images that failed before weren't saved
	if images := stubs.queued[0].PosterSets[0].Images; len(images) != 1 || images[0].ID != "latest-a" {
		t.Errorf("queued images = %+v, want the latest images", images)
	}

	// Nothing is queued without a set, for a set that isn't saved, for a missing entry, or while the item is in the queue
	for _, id := range []int64{2, 3, 4} {
		if _, Err := RetryFailedDownload(ctx, id); Err.Message == "" {
			t.Errorf("retry %d should fail", id)
		}
	}
	stubs.jobs = []database.DownloadQueueJob{{ID: 9, TMDB_ID: "100", LibraryTitle: "Movies", Status: "pending"}}
	if _, Err := RetryFailedDownload(ctx, 1); Err.Message == "" {
		t.Error("retry while queued should fail")
	}
	if len(stubs.queued) != 1 || len(stubs.retried) != 1 {
		t.Errorf("queued %d times and counted %d retries, want 1 of each", len(stubs.queued), len(stubs.retried))
	}
}

func TestRetryDueDownloads(t *testing.T) {
	ctx, stubs := setupRetryStubs(t)

	// The due set is queued and counted, the set that isn't saved stops being retried
	retryDueDownloads(ctx)
	if len(stubs.queued) != 1 || !slices.Equal(stubs.retried, []int64{1}) || !slices.Equal(stubs.stopped, []int64{3}) {
		t.Errorf("queued %d, retried %v, stopped %v", len(stubs.queued), stubs.retried, stubs.stopped)
	}

	// While the item is in the queue, it's left to be tried on the next check
	stubs.jobs = []database.DownloadQueueJob{{ID: 9, TMDB_ID: "100", LibraryTitle: "Movies", Status: "processing"}}
	stubs.retried, stubs.stopped = nil, nil
	retryDueDownloads(ctx)
	if len(stubs.retried) != 0 || slices.Contains(stubs.stopped, 1) {
		t.Errorf("a queued item shouldn't be retried or stopped, retried %v, stopped %v", stubs.retried, stubs.stopped)
	}
}

func TestFailedImages(t *testing.T) {
	num := func(n int) *int { return &n }
	images := []models.ImageFile{
		{ID: "p", Type: "poster"},
		{ID: "e1", Type: "titlecard", SeasonNumber: num(1), EpisodeNumber: num(1)},
		{ID: "e2", Type: "titlecard", SeasonNumber: num(1), EpisodeNumber: num(2)},
	}

	queued := failedImages(images, []models.ImageDownloadResult{{ImageType: "titlecard", SeasonNumber: num(1), EpisodeNumber: num(2), FailureReason: "timed out"}})
	if len(queued) != 1 || queued[0].ID != "e2" {
		t.Errorf("queued = %+v, want only the failed title card", queued)
	}

	// A failure that isn't for one of the set's images downloads the whole set
	for _, failed := range [][]models.ImageDownloadResult{
		nil,
		{{FailureReason: "Media Item missing required fields"}},
		{{ImageType: "titlecard", SeasonNumber: num(2), EpisodeNumber: num(1)}},
	} {
		if queued := failedImages(images, failed); queued != nil {
			t.Errorf("failedImages(%+v) = %+v, want the whole set", failed, queued)
		}
	}
}
