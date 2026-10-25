package database

import (
	"testing"
	"time"
)

func TestSiloItemCounts(t *testing.T) {
	db := openTestMediaDB(t)
	ctx := testContext()

	// Saved and loaded with the item's provider IDs
	updated := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if Err := db.UpsertSiloItemIDs(ctx, []SiloItemIDs{{ContentID: "series-1", TMDB_ID: "1399", SeasonCount: 8, EpisodeCount: 73, UpdatedAt: updated}}); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	items, Err := db.GetAllSiloItemIDs(ctx)
	if Err.Message != "" || len(items) != 1 || items[0].SeasonCount != 8 || items[0].EpisodeCount != 73 {
		t.Fatalf("items = %+v, %s", items, Err.Message)
	}
}

func TestAddSiloItemCountColumns(t *testing.T) {
	conn := openTestDB(t)
	previousClient := Client
	Client = &SQliteDB{conn: conn}
	t.Cleanup(func() { Client = previousClient })
	ctx := testContext()

	// A v15 database, before the counts
	if Err := v8_CreateSiloItemIDsTable(ctx, conn); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	exec(t, conn, `INSERT INTO SiloItemIDs (content_id, tmdb_id, updated_at) VALUES ('series-1', '1399', '2026-10-01 00:00:00')`)

	if Err := AddSiloItemCountColumns(ctx); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	// The saved item info is old now, so it's retrieved again with the counts
	if n := count(t, conn, `SELECT COUNT(*) FROM SiloItemIDs WHERE season_count = 0 AND episode_count = 0 AND updated_at < '2000-01-01'`); n != 1 {
		t.Errorf("rows marked as old with no counts = %d, want 1", n)
	}
}
