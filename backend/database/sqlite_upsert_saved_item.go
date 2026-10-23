package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *SQliteDB) UpsertSavedItem(ctx context.Context, newItem models.DBSavedItem) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(
		ctx,
		fmt.Sprintf(
			"Upserting SavedItem '%s' (%s | %s | %d)",
			newItem.MediaItem.Title,
			newItem.MediaItem.RatingKey,
			newItem.MediaItem.LibraryTitle,
			newItem.MediaItem.Year,
		),
		logging.LevelDebug,
	)
	defer logAction.Complete()

	if s == nil || s.conn == nil {
		logAction.SetError("DB: connection is nil", "", map[string]any{})
		return *logAction.Error
	}

	// Movie and collection sets only have posters and backdrops
	newItem = newItem.WithSetImageTypes()

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("DB: TX BEGIN failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	// Basic counters for logging
	var (
		posterSetsDeleted      int64
		posterSetsUpserted     int
		imagesUpserted         int
		emptySavedItemsDeleted int64
	)

	// 1) Delete Ignore entry for this Media Item if it exists
	if errInfo := deleteIgnoredEntryForMediaItemIfExists(ctx, tx, newItem.MediaItem); errInfo.Message != "" {
		return errInfo
	}

	mediaItemRowID, errInfo := upsertMediaItem(ctx, tx, newItem.MediaItem)
	if errInfo.Message != "" {
		return errInfo
	}

	// Upsert per-type details
	switch newItem.MediaItem.Type {
	case "movie":
		if errInfo := upsertMovie(ctx, tx, newItem.MediaItem, mediaItemRowID); errInfo.Message != "" {
			return errInfo
		}
		logAction.AppendResult("action", "upsert_movie")
	case "show":
		seriesRowID, errInfo := upsertSeries(ctx, tx, newItem.MediaItem, mediaItemRowID)
		if errInfo.Message != "" {
			return errInfo
		}
		if errInfo := reconcileSeasonsAndEpisodes(ctx, tx, newItem.MediaItem, seriesRowID); errInfo.Message != "" {
			return errInfo
		}
		logAction.AppendResult("action", "upsert_show_reconcile")
	default:
		logAction.SetError("DB: unsupported media item type", newItem.MediaItem.Type, map[string]any{
			"type": newItem.MediaItem.Type,
		})
		return *logAction.Error
	}

	// Enforce uniqueness of SelectedTypes across sets for this item:
	// "last one wins" based on incoming slice order.
	typeOwnerSetID := map[string]string{} // key: poster/backdrop/season_poster/special_season_poster/titlecard -> set_id
	for _, ps := range newItem.PosterSets {
		if ps.ToDelete {
			continue
		}
		if ps.SelectedTypes.Poster {
			typeOwnerSetID["poster"] = ps.ID
		}
		if ps.SelectedTypes.Backdrop {
			typeOwnerSetID["backdrop"] = ps.ID
		}
		if ps.SelectedTypes.SeasonPoster {
			typeOwnerSetID["season_poster"] = ps.ID
		}
		if ps.SelectedTypes.SpecialSeasonPoster {
			typeOwnerSetID["special_season_poster"] = ps.ID
		}
		if ps.SelectedTypes.Titlecard {
			typeOwnerSetID["titlecard"] = ps.ID
		}
	}

	// 2) First, process deletions (so re-adds/upserts in same payload behave predictably)
	for _, ps := range newItem.PosterSets {
		if ps.ToDelete {
			continue
		}
		deletedLinks, errInfo := deleteSavedItemLinkAndImages(ctx, tx, newItem.MediaItem.TMDB_ID, newItem.MediaItem.LibraryTitle, newItem.MediaItem.Edition, ps.ID)
		if errInfo.Message != "" {
			logAction.SetError(errInfo.Message, "", errInfo.Detail)
			return *logAction.Error
		}
		posterSetsDeleted += deletedLinks
	}
	if posterSetsDeleted > 0 {
		logAction.AppendResult("poster_sets_deleted", posterSetsDeleted)
	}

	// 3) Then upsert non-deleted sets
	for _, ps := range newItem.PosterSets {
		if ps.ToDelete {
			continue
		}

		ps.DateUpdated = time.Now().UTC()

		posterSetRowID, errInfo := upsertPosterSet(ctx, tx, ps)
		if errInfo.Message != "" {
			logAction.SetError(errInfo.Message, "", errInfo.Detail)
			return *logAction.Error
		}
		posterSetsUpserted++

		// Upsert item+set link (SavedItems)
		if errInfo := upsertSavedItemEntry(ctx, tx, newItem.MediaItem, ps, posterSetRowID); errInfo.Message != "" {
			logAction.SetError(errInfo.Message, "", errInfo.Detail)
			return *logAction.Error
		}

		// Upsert images for this set, scoped to this item
		imagesUpserted += len(ps.Images)
		if errInfo := upsertImageFiles(ctx, tx, ps, posterSetRowID, newItem.MediaItem.TMDB_ID); errInfo.Message != "" {
			logAction.SetError(errInfo.Message, "", errInfo.Detail)
			return *logAction.Error
		}
	}

	logAction.AppendResult("poster_sets_upserted", posterSetsUpserted)
	logAction.AppendResult("images_upserted", imagesUpserted)

	// Apply SelectedTypes uniqueness across ALL sets for this media item
	if errInfo := clearSelectedTypesOnOtherSets(ctx, tx, newItem.MediaItem.TMDB_ID, newItem.MediaItem.LibraryTitle, newItem.MediaItem.Edition, typeOwnerSetID); errInfo.Message != "" {
		logAction.SetError(errInfo.Message, "", errInfo.Detail)
		return *logAction.Error
	}
	logAction.AppendResult("selected_types_uniqueness", "applied")

	// If no selected types remain, remove that SavedItems row
	deletedEmpty, errInfo := deleteEmptySavedItemLinks(ctx, tx, newItem.MediaItem.TMDB_ID, newItem.MediaItem.LibraryTitle, newItem.MediaItem.Edition)
	if errInfo.Message != "" {
		logAction.SetError(errInfo.Message, "", errInfo.Detail)
		return *logAction.Error
	}
	emptySavedItemsDeleted += deletedEmpty
	if emptySavedItemsDeleted > 0 {
		logAction.AppendResult("saved_items_deleted_empty", emptySavedItemsDeleted)
	}

	// Cleanup: remove orphan poster sets + their images not referenced by any SavedItems row
	orphanSetsDeleted, orphanImagesDeleted, errInfo := deleteOrphanPosterSetsAndImages(ctx, tx)
	if errInfo.Message != "" {
		logAction.SetError(errInfo.Message, "", errInfo.Detail)
		return *logAction.Error
	}
	if orphanSetsDeleted > 0 {
		logAction.AppendResult("orphan_poster_sets_deleted", orphanSetsDeleted)
	}
	if orphanImagesDeleted > 0 {
		logAction.AppendResult("orphan_images_deleted", orphanImagesDeleted)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		logAction.SetError("DB: TX COMMIT failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	logging.LOGGER.Info().Timestamp().
		Str("tmdb_id", newItem.MediaItem.TMDB_ID).
		Str("library_title", newItem.MediaItem.LibraryTitle).
		Msg("Upserted SavedItem entry for MediaItem")

	return logging.LogErrorInfo{}
}

func deleteIgnoredEntryForMediaItemIfExists(ctx context.Context, tx *sql.Tx, mediaItem models.MediaItem) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(
		ctx,
		fmt.Sprintf(
			"Deleting Ignored Entrys for MediaItem '%s' (%s | %s | %d)",
			mediaItem.Title,
			mediaItem.RatingKey,
			mediaItem.LibraryTitle,
			mediaItem.Year,
		),
		logging.LevelDebug,
	)
	defer logAction.Complete()

	// Check if an Ignore entry exists for this Media Item
	var count int
	query := `
		SELECT COUNT(*) FROM IgnoredItems
		WHERE tmdb_id = ? AND library_title = ? AND edition = ?
	`
	err := tx.QueryRowContext(ctx, query, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition).Scan(&count)
	if err != nil {
		logAction.SetError("DB: Failed to check for existing IgnoredItem", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	if count == 0 {
		logAction.AppendResult("delete_ignore_entry", "none found")
		return logging.LogErrorInfo{}
	}

	// Delete the Ignore entry
	deleteQuery := `
		DELETE FROM IgnoredItems
		WHERE tmdb_id = ? AND library_title = ? AND edition = ?
	`
	_, err = tx.ExecContext(ctx, deleteQuery, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition)
	if err != nil {
		logAction.SetError("DB: Failed to delete IgnoredItem", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	logAction.AppendResult("delete_ignore_entry", "deleted")
	logging.LOGGER.Info().Timestamp().
		Str("tmdb_id", mediaItem.TMDB_ID).
		Str("library_title", mediaItem.LibraryTitle).
		Msg("Deleted IgnoredItem entry for MediaItem during UpsertSavedItem")

	return logging.LogErrorInfo{}
}

func upsertMediaItem(ctx context.Context, tx *sql.Tx, mediaItem models.MediaItem) (rowID int64, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Upserting MediaItem '%s' (%s | %s | %d)",
			mediaItem.Title,
			mediaItem.RatingKey,
			mediaItem.LibraryTitle,
			mediaItem.Year,
		), logging.LevelDebug)
	defer logAction.Complete()

	q := `
INSERT INTO MediaItems (tmdb_id, library_title, edition, rating_key, type, title, year, on_server, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 1, CAST(strftime('%s','now') AS INTEGER), CAST(strftime('%s','now') AS INTEGER))
ON CONFLICT(tmdb_id, library_title, edition) DO UPDATE SET
  rating_key    = excluded.rating_key,
  type          = excluded.type,
  title         = excluded.title,
  year          = excluded.year,
  on_server     = 1,
  first_seen_at = CASE WHEN MediaItems.first_seen_at = 0 THEN excluded.first_seen_at ELSE MediaItems.first_seen_at END,
  last_seen_at  = excluded.last_seen_at
RETURNING id;
`
	err := tx.QueryRowContext(ctx, q,
		mediaItem.TMDB_ID,
		mediaItem.LibraryTitle,
		mediaItem.Edition,
		mediaItem.RatingKey,
		mediaItem.Type,
		mediaItem.Title,
		mediaItem.Year,
	).Scan(&rowID)
	if err != nil {
		logAction.SetError("DB: UPSERT MediaItems failed", err.Error(), map[string]any{"error": err.Error()})
		return 0, *logAction.Error
	}

	logging.LOGGER.Debug().Timestamp().
		Str("tmdb_id", mediaItem.TMDB_ID).
		Str("library_title", mediaItem.LibraryTitle).
		Int64("media_item_id", rowID).
		Msg("Upserted MediaItem")
	return rowID, logging.LogErrorInfo{}
}

func upsertMovie(ctx context.Context, tx *sql.Tx, mediaItem models.MediaItem, mediaItemRowID int64) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Upserting Movie for MediaItem '%s' (%s | %s | %d)",
			mediaItem.Title,
			mediaItem.RatingKey,
			mediaItem.LibraryTitle,
			mediaItem.Year,
		), logging.LevelDebug)
	defer logAction.Complete()

	q := `
INSERT INTO Movies (media_item_id, path, size, duration)
VALUES (?, ?, ?, ?)
ON CONFLICT(media_item_id) DO UPDATE SET
  path     = excluded.path,
  size     = excluded.size,
  duration = excluded.duration;
`
	_, err := tx.ExecContext(ctx, q,
		mediaItemRowID,
		mediaItem.Movie.File.Path,
		mediaItem.Movie.File.Size,
		mediaItem.Movie.File.Duration,
	)
	if err != nil {
		logAction.SetError("DB: UPSERT Movies failed", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	logging.LOGGER.Debug().Timestamp().
		Str("tmdb_id", mediaItem.TMDB_ID).
		Str("library_title", mediaItem.LibraryTitle).
		Msg("Upserted Movie")
	return logging.LogErrorInfo{}
}

func upsertSeries(ctx context.Context, tx *sql.Tx, mediaItem models.MediaItem, mediaItemRowID int64) (seriesRowID int64, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Upserting Series for MediaItem '%s' (%s | %s | %d)",
			mediaItem.Title,
			mediaItem.RatingKey,
			mediaItem.LibraryTitle,
			mediaItem.Year,
		), logging.LevelDebug)
	defer logAction.Complete()

	q := `
INSERT INTO Series (media_item_id, season_count, episode_count, location)
VALUES (?, ?, ?, ?)
ON CONFLICT(media_item_id) DO UPDATE SET
  season_count  = excluded.season_count,
  episode_count = excluded.episode_count,
  location      = excluded.location
RETURNING id;
`
	err := tx.QueryRowContext(ctx, q,
		mediaItemRowID,
		mediaItem.Series.SeasonCount,
		mediaItem.Series.EpisodeCount,
		mediaItem.Series.Location,
	).Scan(&seriesRowID)
	if err != nil {
		logAction.SetError("DB: UPSERT Series failed", err.Error(), map[string]any{"error": err.Error()})
		return 0, *logAction.Error
	}

	logging.LOGGER.Debug().Timestamp().
		Str("tmdb_id", mediaItem.TMDB_ID).
		Str("library_title", mediaItem.LibraryTitle).
		Msg("Upserted Series")
	return seriesRowID, logging.LogErrorInfo{}
}

func upsertSeason(ctx context.Context, tx *sql.Tx, seriesRowID int64, ratingKey string, seasonNumber int, episodeCount int) (seasonRowID int64, Err logging.LogErrorInfo) {
	q := `
INSERT INTO Seasons (series_id, rating_key, season_number, episode_count)
VALUES (?, ?, ?, ?)
ON CONFLICT(series_id, season_number) DO UPDATE SET
  episode_count = excluded.episode_count,
  rating_key   = excluded.rating_key
RETURNING id;
`
	err := tx.QueryRowContext(ctx, q, seriesRowID, ratingKey, seasonNumber, episodeCount).Scan(&seasonRowID)
	if err != nil {
		return 0, logging.LogErrorInfo{Message: "DB: UPSERT Seasons failed", Detail: map[string]any{"error": err.Error()}}
	}
	return seasonRowID, logging.LogErrorInfo{}
}

func upsertEpisode(ctx context.Context, tx *sql.Tx, seasonRowID int64, ep models.MediaItemEpisode) (Err logging.LogErrorInfo) {
	q := `
INSERT INTO Episodes (season_id, rating_key, episode_number, title, path, size, duration)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(season_id, episode_number) DO UPDATE SET
  rating_key = excluded.rating_key,
  title    = excluded.title,
  path     = excluded.path,
  size     = excluded.size,
  duration = excluded.duration;
`
	_, err := tx.ExecContext(ctx, q,
		seasonRowID,
		ep.RatingKey,
		ep.EpisodeNumber,
		ep.Title,
		ep.File.Path,
		ep.File.Size,
		ep.File.Duration,
	)
	if err != nil {
		return logging.LogErrorInfo{Message: "DB: UPSERT Episodes failed", Detail: map[string]any{"error": err.Error()}}
	}
	return logging.LogErrorInfo{}
}

// Reconcile seasons/episodes: delete rows no longer present, upsert present ones.
func reconcileSeasonsAndEpisodes(ctx context.Context, tx *sql.Tx, mediaItem models.MediaItem, seriesRowID int64) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Reconciling Seasons/Episodes", logging.LevelTrace)
	defer logAction.Complete()

	seasons := mediaItem.Series.Seasons
	if seasons == nil {
		// if caller doesn't send seasons, do nothing (avoid destructive deletes)
		return logging.LogErrorInfo{}
	}

	// Build keep list for seasons
	keepSeasonNums := make([]any, 0, len(seasons))
	for _, sn := range seasons {
		keepSeasonNums = append(keepSeasonNums, sn.SeasonNumber)
	}

	// Delete seasons not in incoming (cascades episodes)
	if len(keepSeasonNums) > 0 {
		inClause := placeholders(len(keepSeasonNums))
		qDel := fmt.Sprintf(`DELETE FROM Seasons WHERE series_id = ? AND season_number NOT IN (%s);`, inClause)
		args := append([]any{seriesRowID}, keepSeasonNums...)
		if _, err := tx.ExecContext(ctx, qDel, args...); err != nil {
			logAction.SetError("DB: delete missing seasons failed", err.Error(), map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	}

	// Upsert seasons and reconcile episodes per season
	for _, sn := range seasons {
		seasonRowID, errInfo := upsertSeason(ctx, tx, seriesRowID, sn.RatingKey, sn.SeasonNumber, lenOr0(sn.Episodes))
		if errInfo.Message != "" {
			return errInfo
		}

		if sn.Episodes == nil {
			continue
		}

		keepEpisodeNums := make([]any, 0, len(sn.Episodes))
		for _, ep := range sn.Episodes {
			keepEpisodeNums = append(keepEpisodeNums, ep.EpisodeNumber)
		}

		if len(keepEpisodeNums) > 0 {
			inClause := placeholders(len(keepEpisodeNums))
			qDelEp := fmt.Sprintf(`DELETE FROM Episodes WHERE season_id = ? AND episode_number NOT IN (%s);`, inClause)
			args := append([]any{seasonRowID}, keepEpisodeNums...)
			if _, err := tx.ExecContext(ctx, qDelEp, args...); err != nil {
				logAction.SetError("DB: delete missing episodes failed", err.Error(), map[string]any{"error": err.Error()})
				return *logAction.Error
			}
		}

		for _, ep := range sn.Episodes {
			if errInfo := upsertEpisode(ctx, tx, seasonRowID, ep); errInfo.Message != "" {
				return errInfo
			}
		}
	}

	return logging.LogErrorInfo{}
}

func upsertPosterSet(ctx context.Context, tx *sql.Tx, ps models.DBPosterSetDetail) (posterSetRowID int64, Err logging.LogErrorInfo) {
	q := `
INSERT INTO PosterSets (set_id, type, title, user, date_created, date_updated)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(set_id) DO UPDATE SET
  type        = excluded.type,
  title       = excluded.title,
  user        = excluded.user,
  -- preserve original creation time
  date_created = PosterSets.date_created,
  date_updated = excluded.date_updated
RETURNING id;
`
	err := tx.QueryRowContext(ctx, q,
		ps.ID,
		ps.Type,
		ps.Title,
		ps.UserCreated,
		ps.DateCreated,
		ps.DateUpdated,
	).Scan(&posterSetRowID)
	if err != nil {
		return 0, logging.LogErrorInfo{Message: "DB: UPSERT PosterSets failed", Detail: map[string]any{"error": err.Error(), "set_id": ps.ID}}
	}
	return posterSetRowID, logging.LogErrorInfo{}
}

func upsertSavedItemEntry(ctx context.Context, tx *sql.Tx, mediaItem models.MediaItem, ps models.DBPosterSetDetail, posterSetRowID int64) (Err logging.LogErrorInfo) {
	q := `
INSERT INTO SavedItems (
  tmdb_id, library_title, edition, poster_set_id,
  poster_selected, backdrop_selected, season_poster_selected, special_season_poster_selected, titlecard_selected,
	autodownload, auto_add_new_collection_items, no_auto_upgrade, last_downloaded
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(tmdb_id, library_title, edition, poster_set_id) DO UPDATE SET
  poster_selected               = excluded.poster_selected,
  backdrop_selected             = excluded.backdrop_selected,
  season_poster_selected        = excluded.season_poster_selected,
  special_season_poster_selected= excluded.special_season_poster_selected,
  titlecard_selected            = excluded.titlecard_selected,
  autodownload                  = excluded.autodownload,
	auto_add_new_collection_items = excluded.auto_add_new_collection_items,
  no_auto_upgrade               = excluded.no_auto_upgrade,
  last_downloaded               = excluded.last_downloaded;
`
	_, err := tx.ExecContext(ctx, q,
		mediaItem.TMDB_ID,
		mediaItem.LibraryTitle,
		mediaItem.Edition,
		posterSetRowID,
		boolToInt(ps.SelectedTypes.Poster),
		boolToInt(ps.SelectedTypes.Backdrop),
		boolToInt(ps.SelectedTypes.SeasonPoster),
		boolToInt(ps.SelectedTypes.SpecialSeasonPoster),
		boolToInt(ps.SelectedTypes.Titlecard),
		boolToInt(ps.AutoDownload),
		boolToInt(ps.AutoAddNewCollectionItems),
		boolToInt(ps.NoAutoUpgrade),
		lastDownloaded(ps.LastDownloaded),
	)
	if err != nil {
		return logging.LogErrorInfo{Message: "DB: UPSERT SavedItems failed", Detail: map[string]any{"error": err.Error()}}
	}
	return logging.LogErrorInfo{}
}

func upsertImageFiles(ctx context.Context, tx *sql.Tx, ps models.DBPosterSetDetail, posterSetRowID int64, itemTMDBID string) (Err logging.LogErrorInfo) {
	if len(ps.Images) == 0 {
		return logging.LogErrorInfo{}
	}

	// NOTE: schema table is ImageFiles (not Images)
	q := `
INSERT INTO ImageFiles (
  poster_set_id, item_tmdb_id,
  image_id, image_type, image_last_updated, image_season_number, image_episode_number
)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(poster_set_id, image_id, item_tmdb_id) DO UPDATE SET
  image_type         = excluded.image_type,
  image_last_updated = excluded.image_last_updated,
  image_season_number= excluded.image_season_number,
  image_episode_number= excluded.image_episode_number;
`
	for _, im := range ps.Images {
		// Force scope: item_tmdb_id should match the media item being saved
		_, err := tx.ExecContext(ctx, q,
			posterSetRowID,
			itemTMDBID,
			im.ID,
			im.Type,
			im.Modified,
			im.SeasonNumber,
			im.EpisodeNumber,
		)
		if err != nil {
			return logging.LogErrorInfo{Message: "DB: UPSERT ImageFiles failed", Detail: map[string]any{"error": err.Error(), "set_id": ps.ID}}
		}
	}
	return logging.LogErrorInfo{}
}

func clearSelectedTypesOnOtherSets(ctx context.Context, tx *sql.Tx, tmdbID, libraryTitle, edition string, owner map[string]string) (Err logging.LogErrorInfo) {
	// For each type, find owner poster_set_id, then clear that type on all other sets for this item
	type col struct {
		key string
		sql string
	}
	cols := []col{
		{"poster", "poster_selected"},
		{"backdrop", "backdrop_selected"},
		{"season_poster", "season_poster_selected"},
		{"special_season_poster", "special_season_poster_selected"},
		{"titlecard", "titlecard_selected"},
	}

	for _, c := range cols {
		ownerSetID, ok := owner[c.key]
		if !ok || ownerSetID == "" {
			continue
		}

		var ownerPosterSetRowID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM PosterSets WHERE set_id = ? LIMIT 1;`, ownerSetID).Scan(&ownerPosterSetRowID); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return logging.LogErrorInfo{Message: "DB: lookup owner poster set id failed", Detail: map[string]any{"error": err.Error(), "set_id": ownerSetID}}
		}

		q := fmt.Sprintf(`
UPDATE SavedItems
SET %s = 0
WHERE tmdb_id = ? AND library_title = ? AND edition = ? AND poster_set_id != ?;
`, c.sql)

		if _, err := tx.ExecContext(ctx, q, tmdbID, libraryTitle, edition, ownerPosterSetRowID); err != nil {
			return logging.LogErrorInfo{Message: "DB: clear selected types failed", Detail: map[string]any{"error": err.Error(), "type": c.key}}
		}
	}

	return logging.LogErrorInfo{}
}

// deleteSavedItemLinkAndImages deletes:
// - SavedItems link for (tmdb_id, library_title, set_id)
// - ImageFiles rows for that (poster_set_id, item_tmdb_id)
// If the poster set becomes orphaned (no SavedItems rows reference it), it also deletes:
// - ALL ImageFiles for that poster_set_id
// - the PosterSets row
//
// Returns number of SavedItems links deleted (0 or 1).
func deleteSavedItemLinkAndImages(ctx context.Context, tx *sql.Tx, tmdbID, libraryTitle, edition, setID string) (deletedLinks int64, Err logging.LogErrorInfo) {
	// Find poster_set PK by set_id
	var posterSetRowID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM PosterSets WHERE set_id = ? LIMIT 1;`, setID).Scan(&posterSetRowID); err != nil {
		if err == sql.ErrNoRows {
			return 0, logging.LogErrorInfo{}
		}
		return 0, logging.LogErrorInfo{Message: "DB: lookup PosterSets.id failed", Detail: map[string]any{"error": err.Error(), "set_id": setID}}
	}

	// Delete item-scoped images for this set + item
	if _, err := tx.ExecContext(ctx, `DELETE FROM ImageFiles WHERE poster_set_id = ? AND item_tmdb_id = ?;`, posterSetRowID, tmdbID); err != nil {
		return 0, logging.LogErrorInfo{Message: "DB: delete ImageFiles (item-scoped) failed", Detail: map[string]any{"error": err.Error()}}
	}

	// Delete SavedItems link (for this item)
	res, err := tx.ExecContext(ctx, `DELETE FROM SavedItems WHERE tmdb_id = ? AND library_title = ? AND edition = ? AND poster_set_id = ?;`, tmdbID, libraryTitle, edition, posterSetRowID)
	if err != nil {
		return 0, logging.LogErrorInfo{Message: "DB: delete SavedItems link failed", Detail: map[string]any{"error": err.Error()}}
	}
	if n, _ := res.RowsAffected(); n > 0 {
		deletedLinks = n
	}

	// If the set is no longer referenced anywhere, delete ALL its images + the set row
	var refCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM SavedItems WHERE poster_set_id = ?;`, posterSetRowID).Scan(&refCount); err != nil {
		return deletedLinks, logging.LogErrorInfo{Message: "DB: check PosterSets references failed", Detail: map[string]any{"error": err.Error()}}
	}

	if refCount == 0 {
		// Delete all images for that set across items
		if _, err := tx.ExecContext(ctx, `DELETE FROM ImageFiles WHERE poster_set_id = ?;`, posterSetRowID); err != nil {
			return deletedLinks, logging.LogErrorInfo{Message: "DB: delete ImageFiles (all for set) failed", Detail: map[string]any{"error": err.Error()}}
		}
		// Delete the set row itself
		if _, err := tx.ExecContext(ctx, `DELETE FROM PosterSets WHERE id = ?;`, posterSetRowID); err != nil {
			return deletedLinks, logging.LogErrorInfo{Message: "DB: delete PosterSets failed", Detail: map[string]any{"error": err.Error(), "poster_set_id": posterSetRowID}}
		}
	}

	return deletedLinks, logging.LogErrorInfo{}
}

// deleteOrphanPosterSetsAndImages removes orphan poster sets (not referenced by SavedItems) AND their images.
// Returns (setsDeleted, imagesDeleted).
func deleteOrphanPosterSetsAndImages(ctx context.Context, tx *sql.Tx) (setsDeleted int64, imagesDeleted int64, Err logging.LogErrorInfo) {
	// 1) Delete orphan images first
	resImg, err := tx.ExecContext(ctx, `
DELETE FROM ImageFiles
WHERE poster_set_id NOT IN (SELECT DISTINCT poster_set_id FROM SavedItems);
`)
	if err != nil {
		return 0, 0, logging.LogErrorInfo{Message: "DB: delete orphan ImageFiles failed", Detail: map[string]any{"error": err.Error()}}
	}
	if n, _ := resImg.RowsAffected(); n > 0 {
		imagesDeleted = n
	}

	// 2) Delete orphan sets
	resSet, err := tx.ExecContext(ctx, `
DELETE FROM PosterSets
WHERE id NOT IN (SELECT DISTINCT poster_set_id FROM SavedItems);
`)
	if err != nil {
		return 0, imagesDeleted, logging.LogErrorInfo{Message: "DB: delete orphan PosterSets failed", Detail: map[string]any{"error": err.Error()}}
	}
	if n, _ := resSet.RowsAffected(); n > 0 {
		setsDeleted = n
	}

	return setsDeleted, imagesDeleted, logging.LogErrorInfo{}
}

// Change deleteEmptySavedItemLinks to return rows deleted so caller can log it.
func deleteEmptySavedItemLinks(ctx context.Context, tx *sql.Tx, tmdbID, libraryTitle, edition string) (deleted int64, Err logging.LogErrorInfo) {
	q := `
DELETE FROM SavedItems
WHERE tmdb_id = ? AND library_title = ? AND edition = ?
  AND poster_selected = 0
  AND backdrop_selected = 0
  AND season_poster_selected = 0
  AND special_season_poster_selected = 0
  AND titlecard_selected = 0;
`
	res, err := tx.ExecContext(ctx, q, tmdbID, libraryTitle, edition)
	if err != nil {
		return 0, logging.LogErrorInfo{Message: "DB: delete empty SavedItems links failed", Detail: map[string]any{"error": err.Error()}}
	}
	if n, _ := res.RowsAffected(); n > 0 {
		deleted = n
	}
	return deleted, logging.LogErrorInfo{}
}

// Helpers
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}

func lenOr0[T any](s []T) int {
	if s == nil {
		return 0
	}
	return len(s)
}

// lastDownloaded is when a set was downloaded, a set saved without a date was saved now
func lastDownloaded(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}
