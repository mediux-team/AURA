package sandbox

import (
	"aura/database"
	"aura/logging"
	"context"
	"database/sql"
	"fmt"
)

// The changes the sandbox can make to an item's saved data
const (
	ChangeImageUpdated       = "image_updated"        // The image was updated on MediUX since the last download
	ChangeImageNew           = "image_new"            // The image was added to the set since the last download
	ChangeMovieFileChanged   = "movie_file_changed"   // The movie's file changed, like an upgrade
	ChangeEpisodeFileChanged = "episode_file_changed" // The episode's file changed, like an upgrade
	ChangeEpisodeAdded       = "episode_added"        // The episode was added since the last download
	ChangeSeasonAdded        = "season_added"         // The season was added since the last download
)

// Change is a change to an item's saved data
type Change struct {
	TMDB_ID       string `json:"tmdb_id"`
	LibraryTitle  string `json:"library_title"`
	Edition       string `json:"edition"`
	Change        string `json:"change"`
	ImageID       string `json:"image_id,omitempty"`
	ImageType     string `json:"image_type,omitempty"`
	SeasonNumber  int    `json:"season_number,omitempty"`
	EpisodeNumber int    `json:"episode_number,omitempty"`
}

// The item's row in MediaItems, and one of its seasons
const (
	itemRowSQL   = `(SELECT id FROM MediaItems WHERE tmdb_id = ? AND library_title = ? AND edition = ?)`
	seasonRowSQL = `(SELECT s.id FROM Seasons s JOIN Series sr ON sr.id = s.series_id WHERE sr.media_item_id = ` + itemRowSQL + ` AND s.season_number = ?)`
)

// Simulate changes the item's saved data, so the next Auto Download check sees the change
func Simulate(ctx context.Context, change Change) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Sandbox: simulate %s", change.Change), logging.LevelInfo)
	defer logAction.Complete()

	conn, _, Err := database.GetDBConnection(ctx)
	if Err.Message != "" {
		return Err
	}

	item := []any{change.TMDB_ID, change.LibraryTitle, change.Edition}
	season := append(item, change.SeasonNumber)
	var queries []struct {
		query string
		args  []any
	}
	add := func(query string, args ...any) {
		queries = append(queries, struct {
			query string
			args  []any
		}{query, args})
	}

	switch change.Change {
	case ChangeImageUpdated:
		add(`UPDATE ImageFiles SET image_last_updated = '2020-01-01 00:00:00+00:00' WHERE item_tmdb_id = ? AND image_id = ? AND image_type = ?;`,
			change.TMDB_ID, change.ImageID, change.ImageType)
	case ChangeImageNew:
		add(`DELETE FROM ImageFiles WHERE item_tmdb_id = ? AND image_id = ? AND image_type = ?;`,
			change.TMDB_ID, change.ImageID, change.ImageType)
	case ChangeMovieFileChanged:
		add(`UPDATE Movies SET path = path || '.sandbox', size = size + 1048576 WHERE media_item_id = `+itemRowSQL+`;`, item...)
	case ChangeEpisodeFileChanged:
		add(`UPDATE Episodes SET path = path || '.sandbox', size = size + 1048576 WHERE episode_number = ? AND season_id = `+seasonRowSQL+`;`,
			append([]any{change.EpisodeNumber}, season...)...)
	case ChangeEpisodeAdded:
		add(`DELETE FROM Episodes WHERE episode_number = ? AND season_id = `+seasonRowSQL+`;`,
			append([]any{change.EpisodeNumber}, season...)...)
	case ChangeSeasonAdded:
		add(`DELETE FROM Episodes WHERE season_id = `+seasonRowSQL+`;`, season...)
		add(`DELETE FROM Seasons WHERE id = `+seasonRowSQL+`;`, season...)
	default:
		logAction.SetError(fmt.Sprintf("Unknown change '%s'", change.Change), "Pick one of the changes on the sandbox page", nil)
		return *logAction.Error
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to begin the transaction", err.Error(), nil)
		return *logAction.Error
	}
	defer tx.Rollback()

	changed := int64(0)
	for _, q := range queries {
		result, err := tx.ExecContext(ctx, q.query, q.args...)
		if err != nil {
			logAction.SetError("Failed to change the saved data", err.Error(), map[string]any{"change": change.Change})
			return *logAction.Error
		}
		if rows, _ := result.RowsAffected(); rows > 0 {
			changed += rows
		}
	}
	if changed == 0 {
		logAction.SetError("Nothing was changed", "The item doesn't have that image, file, season or episode saved", map[string]any{"change": change})
		return *logAction.Error
	}
	if err := tx.Commit(); err != nil && err != sql.ErrTxDone {
		logAction.SetError("Failed to commit the change", err.Error(), nil)
		return *logAction.Error
	}
	logAction.AppendResult("rows_changed", changed)
	return logging.LogErrorInfo{}
}
