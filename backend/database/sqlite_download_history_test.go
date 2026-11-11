package database

import (
	"aura/models"
	"database/sql"
	"testing"
	"time"
)

func TestDownloadHistoryFailures(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()

	entry := func(tmdbID, setID string) DownloadHistoryEntry {
		return DownloadHistoryEntry{
			TMDB_ID: tmdbID, LibraryTitle: "Series", MediaItemTitle: "Carrie", MediaItemYear: 2026,
			SetID: setID, SetTitle: "Carrie (2026) Set", SetCreator: "willtong93",
			SelectedTypes: models.SelectedTypes{Poster: true, Titlecard: true}, WarningCount: 2,
			ImagesSucceeded: 9, ImagesFailed: 1, FailedImages: []models.ImageDownloadResult{{ImageType: "titlecard", FailureReason: "timed out"}},
		}
	}
	for _, e := range []DownloadHistoryEntry{entry("1", "set"), entry("1", ""), entry("1", "other"), entry("2", "set")} {
		if Err := db.InsertDownloadHistoryEntry(ctx, e); Err.Message != "" {
			t.Fatal(Err.Message)
		}
	}

	entries, total, Err := db.GetDownloadHistory(ctx, 50, 0)
	if Err.Message != "" || total != 4 {
		t.Fatalf("GetDownloadHistory = %d %s, want 4", total, Err.Message)
	}
	// -1 gets every entry
	if all, _, _ := db.GetDownloadHistory(ctx, -1, 0); len(all) != 4 {
		t.Errorf("GetDownloadHistory(-1) = %d entries, want 4", len(all))
	}
	got := entries[0]
	if one, found, Err := db.GetDownloadHistoryEntry(ctx, got.ID); Err.Message != "" || !found || one.SetID != got.SetID || one.SetCreator != "willtong93" || !one.SelectedTypes.Titlecard {
		t.Errorf("GetDownloadHistoryEntry = %+v %v %s", one, found, Err.Message)
	}
	if _, found, _ := db.GetDownloadHistoryEntry(ctx, 999); found {
		t.Error("a missing entry shouldn't be found")
	}
	if got.SetCreator != "willtong93" || !got.SelectedTypes.Poster || !got.SelectedTypes.Titlecard || got.SelectedTypes.Backdrop || got.WarningCount != 2 || got.Status != "warning" {
		t.Errorf("entry = %+v, want the set's creator, selected types and warnings kept", got)
	}

	// The set downloading without failures clears its failures and the item's failures without a set
	if Err := db.DeleteDownloadHistoryForSet(ctx, "1", "Series", "", "set"); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	entries, _, _ = db.GetDownloadHistory(ctx, 50, 0)
	left := map[string]bool{}
	for _, e := range entries {
		left[e.TMDB_ID+"|"+e.SetID] = true
	}
	if len(entries) != 2 || !left["1|other"] || !left["2|set"] {
		t.Errorf("left = %v, want the other set and the other item", left)
	}
}

func TestDownloadHistoryRetries(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	failure := DownloadHistoryEntry{TMDB_ID: "1", LibraryTitle: "Movies", MediaItemTitle: "Hope", SetID: "set", ImagesFailed: 1}

	// waitFor is how long until the entry's next automatic retry, -1 when there's none
	waitFor := func(setID string) time.Duration {
		t.Helper()
		var retryCount int
		var next sql.NullTime
		if err := db.conn.QueryRow(`SELECT retry_count, next_retry_at FROM DownloadHistory WHERE set_id = ?`, setID).Scan(&retryCount, &next); err != nil {
			t.Fatal(err)
		}
		if !next.Valid {
			return -1
		}
		return time.Until(next.Time).Round(time.Minute)
	}
	record := func(entry DownloadHistoryEntry) {
		t.Helper()
		if Err := db.InsertDownloadHistoryEntry(ctx, entry); Err.Message != "" {
			t.Fatal(Err.Message)
		}
	}
	idOf := func(setID string) int64 {
		var id int64
		_ = db.conn.QueryRow(`SELECT id FROM DownloadHistory WHERE set_id = ?`, setID).Scan(&id)
		return id
	}

	// Each retry that fails again waits longer, after the last one it isn't retried automatically
	record(failure)
	for _, want := range []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour, -1} {
		if got := waitFor("set"); got != want {
			t.Fatalf("next retry in %v, want %v", got, want)
		}
		if Err := db.MarkDownloadHistoryRetried(ctx, idOf("set")); Err.Message != "" {
			t.Fatal(Err.Message)
		}
		if got := waitFor("set"); got != -1 {
			t.Errorf("a retried entry waits for its result, next retry in %v", got)
		}
		record(failure)
	}
	if entry, _, _ := db.GetDownloadHistoryEntry(ctx, idOf("set")); entry.RetryCount != 4 || entry.NextRetryAt != nil {
		t.Errorf("entry = %d retries, next %v", entry.RetryCount, entry.NextRetryAt)
	}

	// A failure without a set can't be retried
	record(DownloadHistoryEntry{TMDB_ID: "2", LibraryTitle: "Movies", MediaItemTitle: "Carrie", ImagesFailed: 1})
	if got := waitFor(""); got != -1 {
		t.Errorf("a failure without a set has a retry in %v", got)
	}

	// Only retries that are due are returned
	record(DownloadHistoryEntry{TMDB_ID: "3", LibraryTitle: "Movies", MediaItemTitle: "Due", SetID: "due", ImagesFailed: 1})
	record(DownloadHistoryEntry{TMDB_ID: "4", LibraryTitle: "Movies", MediaItemTitle: "Later", SetID: "later", ImagesFailed: 1})
	exec(t, db.conn, `UPDATE DownloadHistory SET next_retry_at = datetime('now', '-1 minute') WHERE set_id = 'due'`)
	due, Err := db.GetDueDownloadRetries(ctx)
	if Err.Message != "" || len(due) != 1 || due[0].SetID != "due" || due[0].NextRetryAt == nil {
		t.Errorf("due = %+v %s", due, Err.Message)
	}
	if Err := db.StopDownloadRetries(ctx, due[0].ID); Err.Message != "" || waitFor("due") != -1 {
		t.Errorf("StopDownloadRetries didn't stop the retries: %s", Err.Message)
	}
}

func TestDownloadHistoryKeepsTheSource(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()
	entry := DownloadHistoryEntry{TMDB_ID: "1", LibraryTitle: "Series", MediaItemTitle: "Show", SetID: "set", SetTitle: "Set",
		ImagesFailed: 1, FailedImages: []models.ImageDownloadResult{{ImageType: "titlecard", FailureReason: "timed out"}}}

	source := func() string {
		entries, _, Err := db.GetDownloadHistory(ctx, 10, 0)
		if Err.Message != "" || len(entries) != 1 {
			t.Fatalf("history = %+v %s", entries, Err.Message)
		}
		return entries[0].Source
	}

	entry.Source = "auto_download"
	if Err := db.InsertDownloadHistoryEntry(ctx, entry); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	// A retry that fails again keeps where the download came from
	entry.Source = "retry"
	if Err := db.InsertDownloadHistoryEntry(ctx, entry); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if got := source(); got != "auto_download" {
		t.Errorf("source after a failed retry = %q, want auto_download", got)
	}
	// Another download that fails replaces it
	entry.Source = "download_window"
	if Err := db.InsertDownloadHistoryEntry(ctx, entry); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if got := source(); got != "download_window" {
		t.Errorf("source = %q, want download_window", got)
	}
}
