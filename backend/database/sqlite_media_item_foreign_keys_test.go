package database

import (
	"strings"
	"testing"
)

func TestRebuildMovieAndSeriesTables(t *testing.T) {
	db := openTestMediaDB(t)
	// The tables as the v5 to v6 migration left them
	exec(t, db.conn, `DROP TABLE Movies;`)
	exec(t, db.conn, `CREATE TABLE Movies (id INTEGER PRIMARY KEY AUTOINCREMENT, media_item_id INTEGER NOT NULL UNIQUE, path TEXT NOT NULL, size INTEGER NOT NULL, duration INTEGER NOT NULL,
		FOREIGN KEY (media_item_id) REFERENCES "MediaItems_old"(id) ON DELETE CASCADE ON UPDATE CASCADE);`)
	exec(t, db.conn, `DROP TABLE Series;`)
	exec(t, db.conn, `CREATE TABLE Series (id INTEGER PRIMARY KEY AUTOINCREMENT, media_item_id INTEGER NOT NULL UNIQUE, season_count INTEGER, episode_count INTEGER, location TEXT,
		FOREIGN KEY (media_item_id) REFERENCES "MediaItems_old"(id) ON DELETE CASCADE ON UPDATE CASCADE);`)

	exec(t, db.conn, `INSERT INTO MediaItems (id, tmdb_id, library_title, rating_key, type, title, year) VALUES
		(1, '1', 'Movies', 'm1', 'movie', 'Movie', 2020),
		(2, '2', 'Series', 's2', 'show', 'Show', 2020);`)
	exec(t, db.conn, `INSERT INTO Movies (id, media_item_id, path, size, duration) VALUES (5, 1, '/movies/Movie.mkv', 10, 20), (6, 99, '/movies/Deleted.mkv', 1, 1);`)
	exec(t, db.conn, `INSERT INTO Series (id, media_item_id, season_count, episode_count, location) VALUES (7, 2, 1, 1, '/series/Show'), (8, 98, 1, 1, '/series/Deleted');`)
	exec(t, db.conn, `INSERT INTO Seasons (id, series_id, rating_key, season_number) VALUES (11, 7, 's2-1', 1), (12, 8, 'deleted-1', 1);`)
	exec(t, db.conn, `INSERT INTO Episodes (id, season_id, rating_key, episode_number, title, path, size, duration) VALUES
		(21, 11, 's2-1-1', 1, 'Pilot', '/series/Show/S01E01.mkv', 1, 1), (22, 12, 'deleted-1-1', 1, 'Gone', '/series/Deleted/S01E01.mkv', 1, 1);`)

	if Err := v17_RebuildMovieAndSeriesTables(testContext(), db.conn); Err.Message != "" {
		t.Fatal(Err.Message)
	}

	for _, table := range []string{"Movies", "Series"} {
		var schema string
		if err := db.conn.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&schema); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(schema, "MediaItems_old") || !strings.Contains(schema, "REFERENCES MediaItems(id)") {
			t.Errorf("%s schema = %s, want it to reference MediaItems", table, schema)
		}
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE '%\_new' ESCAPE '\'`); n != 0 {
		t.Errorf("%d new tables were left", n)
	}

	// The items' rows stay with their IDs, the deleted items' rows are removed
	if n := count(t, db.conn, `SELECT COUNT(*) FROM Movies WHERE id = 5 AND media_item_id = 1 AND path = '/movies/Movie.mkv'`); n != 1 {
		t.Error("the movie's file should be kept")
	}
	if n := count(t, db.conn, `SELECT COUNT(*) FROM Series WHERE id = 7 AND media_item_id = 2 AND location = '/series/Show'`); n != 1 {
		t.Error("the show should be kept")
	}
	for query, want := range map[string]int{
		`SELECT COUNT(*) FROM Movies`:   1,
		`SELECT COUNT(*) FROM Series`:   1,
		`SELECT COUNT(*) FROM Seasons`:  1,
		`SELECT COUNT(*) FROM Episodes`: 1,
		// The season still belongs to its show
		`SELECT COUNT(*) FROM Seasons s JOIN Series sr ON sr.id = s.series_id WHERE sr.media_item_id = 2`: 1,
	} {
		if n := count(t, db.conn, query); n != want {
			t.Errorf("%s = %d, want %d", query, n, want)
		}
	}

	// New rows don't reuse the old IDs
	exec(t, db.conn, `INSERT INTO MediaItems (id, tmdb_id, library_title, rating_key, type, title, year) VALUES (3, '3', 'Movies', 'm3', 'movie', 'Other', 2021);`)
	exec(t, db.conn, `INSERT INTO Movies (media_item_id, path, size, duration) VALUES (3, '/movies/Other.mkv', 1, 1);`)
	if n := count(t, db.conn, `SELECT id FROM Movies WHERE media_item_id = 3`); n <= 6 {
		t.Errorf("the new movie got ID %d, want one after the old IDs", n)
	}
}
