package database

import (
	"aura/logging"
	"context"
	"database/sql"
)

// RebuildMovieAndSeriesTables points the Movies and Series foreign keys at MediaItems again
func RebuildMovieAndSeriesTables(ctx context.Context) (Err logging.LogErrorInfo) {
	conn, _, Err := GetDBConnection(ctx)
	if Err.Message != "" {
		return Err
	}
	return v17_RebuildMovieAndSeriesTables(ctx, conn)
}

// v17_RebuildMovieAndSeriesTables rebuilds the Movies and Series tables, as renaming MediaItems in the v5 to v6
// migration moved their foreign keys to MediaItems_old. The rows of deleted items are removed, as the cascades would have.
func v17_RebuildMovieAndSeriesTables(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Rebuilding the Movies and Series tables", logging.LevelInfo)
	defer logAction.Complete()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to begin the transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer tx.Rollback()

	steps := []struct{ name, query string }{
		{"remove the movies of deleted items", `DELETE FROM Movies WHERE media_item_id NOT IN (SELECT id FROM MediaItems);`},
		{"remove the shows of deleted items", `DELETE FROM Series WHERE media_item_id NOT IN (SELECT id FROM MediaItems);`},
		{"remove the seasons of deleted shows", `DELETE FROM Seasons WHERE series_id NOT IN (SELECT id FROM Series);`},
		{"remove the episodes of deleted seasons", `DELETE FROM Episodes WHERE season_id NOT IN (SELECT id FROM Seasons);`},

		{"create the new Movies table", `
CREATE TABLE Movies_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL UNIQUE,
    path TEXT NOT NULL,
    size INTEGER NOT NULL,
    duration INTEGER NOT NULL,
    FOREIGN KEY (media_item_id) REFERENCES MediaItems(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE
);`},
		{"copy the movies", `INSERT INTO Movies_new (id, media_item_id, path, size, duration) SELECT id, media_item_id, path, size, duration FROM Movies;`},
		{"keep the next Movies ID", `DELETE FROM sqlite_sequence WHERE name = 'Movies_new'; INSERT INTO sqlite_sequence (name, seq) SELECT 'Movies_new', seq FROM sqlite_sequence WHERE name = 'Movies';`},
		{"drop the old Movies table", `DROP TABLE Movies;`},
		{"rename the new Movies table", `ALTER TABLE Movies_new RENAME TO Movies;`},

		// The IDs are kept, so the seasons still belong to their shows
		{"create the new Series table", `
CREATE TABLE Series_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL UNIQUE,
    season_count INTEGER,
    episode_count INTEGER,
    location TEXT,
    FOREIGN KEY (media_item_id) REFERENCES MediaItems(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE
);`},
		{"copy the shows", `INSERT INTO Series_new (id, media_item_id, season_count, episode_count, location) SELECT id, media_item_id, season_count, episode_count, location FROM Series;`},
		{"keep the next Series ID", `DELETE FROM sqlite_sequence WHERE name = 'Series_new'; INSERT INTO sqlite_sequence (name, seq) SELECT 'Series_new', seq FROM sqlite_sequence WHERE name = 'Series';`},
		{"drop the old Series table", `DROP TABLE Series;`},
		{"rename the new Series table", `ALTER TABLE Series_new RENAME TO Series;`},
	}
	for _, step := range steps {
		result, err := tx.ExecContext(ctx, step.query)
		if err != nil {
			logAction.SetError("Failed to "+step.name, "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		if rows, _ := result.RowsAffected(); rows > 0 {
			logAction.AppendResult(step.name, rows)
		}
	}

	if err := tx.Commit(); err != nil {
		logAction.SetError("Failed to commit the transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}
