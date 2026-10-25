package database

import (
	"aura/logging"
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SiloItemIDs is the provider IDs of a Silo catalog item.
// Silo's catalog listing does not include provider IDs, so they are saved here to avoid
// getting the details of every item from Silo after each restart.
type SiloItemIDs struct {
	ContentID    string    `json:"content_id"`
	TMDB_ID      string    `json:"tmdb_id"`
	IMDB_ID      string    `json:"imdb_id"`
	TVDB_ID      string    `json:"tvdb_id"`
	FirstAirDate string    `json:"first_air_date"`
	SeasonCount  int       `json:"season_count"`
	EpisodeCount int       `json:"episode_count"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *SQliteDB) GetAllSiloItemIDs(ctx context.Context) (items []SiloItemIDs, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get All Silo Item IDs", logging.LevelDebug)
	defer logAction.Complete()

	items = []SiloItemIDs{}

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return items, *logAction.Error
	}

	rows, err := s.conn.QueryContext(ctx, `
SELECT content_id, tmdb_id, imdb_id, tvdb_id, first_air_date, season_count, episode_count, updated_at
FROM SiloItemIDs;
`)
	if err != nil {
		logAction.SetError("DB: SELECT SiloItemIDs failed", err.Error(), map[string]any{"error": err.Error()})
		return items, *logAction.Error
	}
	defer rows.Close()

	for rows.Next() {
		var item SiloItemIDs
		if err := rows.Scan(&item.ContentID, &item.TMDB_ID, &item.IMDB_ID, &item.TVDB_ID, &item.FirstAirDate, &item.SeasonCount, &item.EpisodeCount, &item.UpdatedAt); err != nil {
			logAction.SetError("DB: scan SiloItemIDs failed", err.Error(), map[string]any{"error": err.Error()})
			return items, *logAction.Error
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("DB: iterate SiloItemIDs rows failed", err.Error(), map[string]any{"error": err.Error()})
		return items, *logAction.Error
	}

	logAction.AppendResult("count", len(items))
	return items, logging.LogErrorInfo{}
}

func (s *SQliteDB) UpsertSiloItemIDs(ctx context.Context, items []SiloItemIDs) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Upsert %d Silo Item IDs", len(items)), logging.LevelTrace)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("DB: TX BEGIN failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO SiloItemIDs (content_id, tmdb_id, imdb_id, tvdb_id, first_air_date, season_count, episode_count, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(content_id) DO UPDATE SET
    tmdb_id = excluded.tmdb_id,
    imdb_id = excluded.imdb_id,
    tvdb_id = excluded.tvdb_id,
    first_air_date = excluded.first_air_date,
    season_count = excluded.season_count,
    episode_count = excluded.episode_count,
    updated_at = excluded.updated_at;
`)
	if err != nil {
		logAction.SetError("DB: prepare UPSERT SiloItemIDs failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer stmt.Close()

	for _, item := range items {
		if _, err := stmt.ExecContext(ctx, item.ContentID, item.TMDB_ID, item.IMDB_ID, item.TVDB_ID, item.FirstAirDate, item.SeasonCount, item.EpisodeCount, item.UpdatedAt); err != nil {
			logAction.SetError("DB: UPSERT SiloItemIDs failed", err.Error(), map[string]any{"error": err.Error(), "content_id": item.ContentID})
			return *logAction.Error
		}
	}

	if err := tx.Commit(); err != nil {
		logAction.SetError("DB: TX COMMIT failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

// siloItemCountColumnsSQL adds the season and episode counts of shows, for the statistics
const siloItemCountColumnsSQL = `
ALTER TABLE SiloItemIDs ADD COLUMN season_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE SiloItemIDs ADD COLUMN episode_count INTEGER NOT NULL DEFAULT 0;`

func v16_AddSiloItemCountColumns(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Adding Season and Episode Counts to SiloItemIDs", logging.LevelTrace)
	defer logAction.Complete()

	if _, err := conn.ExecContext(ctx, siloItemCountColumnsSQL); err != nil {
		logAction.SetError("Failed to add the season and episode counts to SiloItemIDs", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}

// AddSiloItemCountColumns adds the season and episode counts, for the v15 to v16 migration.
// The saved item info is marked as old, so it's retrieved again with the counts.
func AddSiloItemCountColumns(ctx context.Context) (Err logging.LogErrorInfo) {
	conn, _, Err := GetDBConnection(ctx)
	if Err.Message != "" {
		return Err
	}
	if Err := v16_AddSiloItemCountColumns(ctx, conn); Err.Message != "" {
		return Err
	}
	if _, err := conn.ExecContext(ctx, `UPDATE SiloItemIDs SET updated_at = '1970-01-01 00:00:00';`); err != nil {
		return logging.LogErrorInfo{Message: "Failed to mark the Silo item info as old", Detail: map[string]any{"error": err.Error()}}
	}
	return logging.LogErrorInfo{}
}
