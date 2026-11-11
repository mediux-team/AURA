package database

import (
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type PagedSavedItems struct {
	Items []models.DBSavedItem
	Total int
}

func (s *SQliteDB) GetAllSavedSets(ctx context.Context, filter models.DBFilter) (out PagedSavedItems, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Getting All Saved Sets from Database", logging.LevelInfo)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}
	out.Items = make([]models.DBSavedItem, 0)

	if s == nil || s.conn == nil {
		logAction.SetError("Database connection is nil", "", map[string]any{})
		return out, *logAction.Error
	}

	whereSQL, baseArgs := buildSavedItemsWhere(filter)

	// Sorting (whitelist only)
	sortCol := "mi.title"
	switch strings.ToLower(strings.TrimSpace(filter.SortOption)) {
	case "", "title":
		sortCol = "mi.title"
	case "year":
		sortCol = "mi.year"
	case "library":
		sortCol = "mi.library_title"
	case "last_downloaded", "date_downloaded":
		sortCol = "max_last_downloaded"
	}

	// Sort direction
	sortDir := "ASC"
	if strings.EqualFold(filter.SortOrder, "desc") {
		sortDir = "DESC"
	}

	pageItems := filter.ItemsPerPage
	if pageItems < 0 {
		// -1 means "all items" (no limit in SQLite)
		pageItems = -1
	}
	if pageItems == 0 {
		pageItems = 25
	}
	if pageItems > 250 {
		pageItems = 250
	}
	pageNumber := filter.PageNumber
	if pageNumber <= 0 {
		pageNumber = 1
	}
	limit := pageItems
	offset := (pageNumber - 1) * pageItems

	// Total count (filtered)
	countSQL := fmt.Sprintf(`
WITH base AS (
  SELECT
    mi.id,
    mi.tmdb_id,
    mi.library_title,
    mi.edition,
    mi.rating_key,
    mi.type,
    mi.title,
    mi.year,
	mi.on_server,
    (SELECT COUNT(*)
     FROM SavedItems si
     WHERE si.tmdb_id = mi.tmdb_id AND si.library_title = mi.library_title AND si.edition = mi.edition
    ) AS set_count
  FROM MediaItems mi
)
SELECT COUNT(*)
FROM base mi
%s;
`, whereSQL)

	if err := s.conn.QueryRowContext(ctx, countSQL, baseArgs...).Scan(&out.Total); err != nil {
		logAction.SetError("Failed to scan total count of saved sets", "", map[string]any{"error": err.Error(), "query": countSQL})
		return out, *logAction.Error
	}

	// Data query
	dataSQL := fmt.Sprintf(`
WITH base AS (
  SELECT
    mi.id,
    mi.tmdb_id,
    mi.library_title,
    mi.edition,
    mi.rating_key,
    mi.type,
    mi.title,
    mi.year,
	mi.on_server,

    mv.path     AS movie_path,
    mv.size     AS movie_size,
    mv.duration AS movie_duration,

    sr.id            AS series_id,
    sr.season_count  AS season_count,
    sr.episode_count AS episode_count,
    sr.location      AS location,

    (SELECT COUNT(*)
     FROM SavedItems si
     WHERE si.tmdb_id = mi.tmdb_id AND si.library_title = mi.library_title AND si.edition = mi.edition
    ) AS set_count,

    -- julianday compares the moment, the dates are saved with different time zones
    (SELECT MAX(julianday(si.last_downloaded))
     FROM SavedItems si
     WHERE si.tmdb_id = mi.tmdb_id AND si.library_title = mi.library_title AND si.edition = mi.edition
    ) AS max_last_downloaded

  FROM MediaItems mi
  LEFT JOIN Movies mv ON mv.media_item_id = mi.id
  LEFT JOIN Series sr ON sr.media_item_id = mi.id
)
SELECT
  mi.tmdb_id AS tmdb_id,
  mi.library_title AS library_title,

  json_object(
    'tmdb_id', mi.tmdb_id,
    'library_title', mi.library_title,
    'edition', mi.edition,
    'rating_key', mi.rating_key,
    'type', mi.type,
    'title', mi.title,
    'year', mi.year,

    'movie', json(CASE WHEN mi.type = 'movie' THEN
      json_object(
        'file', json_object(
          'path', mi.movie_path,
          'size', mi.movie_size,
          'duration', mi.movie_duration
        )
      )
    ELSE NULL END),

    'series', json(CASE WHEN mi.type = 'show' THEN
      json_object(
        'season_count', mi.season_count,
        'episode_count', mi.episode_count,
        'location', mi.location,
        'seasons', json((
          SELECT json_group_array(
            json_object(
			  'rating_key', sn.rating_key,
              'season_number', sn.season_number,
              'episode_count', sn.episode_count,
              'episodes', json((
                SELECT json_group_array(
                  json_object(
				    'rating_key', ep.rating_key,
                    'episode_number', ep.episode_number,
                    'season_number', sn.season_number,
                    'title', ep.title,
                    'file', json_object(
                      'path', ep.path,
                      'size', ep.size,
                      'duration', ep.duration
                    )
                  )
                )
                FROM Episodes ep
                WHERE ep.season_id = sn.id
                ORDER BY ep.episode_number
              ))
            )
          )
          FROM Seasons sn
          WHERE sn.series_id = mi.series_id
          ORDER BY sn.season_number
        ))
      )
    ELSE NULL END)
  ) AS media_item,

  COALESCE(
    (
      SELECT json_group_array(
        json_object(
          'id', ps.set_id,
          'title', ps.title,
          'type', ps.type,
          'user_created', ps.user,

          'date_created', replace(ps.date_created, ' ', 'T'),
          'date_updated', replace(ps.date_updated, ' ', 'T'),

          'last_downloaded', replace(si.last_downloaded, ' ', 'T'),

          -- IMPORTANT: emit JSON booleans, not 0/1 numbers
          'selected_types', json_object(
            'poster', json(CASE WHEN si.poster_selected = 1 THEN 'true' ELSE 'false' END),
            'backdrop', json(CASE WHEN si.backdrop_selected = 1 THEN 'true' ELSE 'false' END),
            'season_poster', json(CASE WHEN si.season_poster_selected = 1 THEN 'true' ELSE 'false' END),
            'special_season_poster', json(CASE WHEN si.special_season_poster_selected = 1 THEN 'true' ELSE 'false' END),
            'titlecard', json(CASE WHEN si.titlecard_selected = 1 THEN 'true' ELSE 'false' END)
          ),

          'auto_download', json(CASE WHEN si.autodownload = 1 THEN 'true' ELSE 'false' END),
		  'auto_add_new_collection_items', json(CASE WHEN si.auto_add_new_collection_items = 1 THEN 'true' ELSE 'false' END),
		  'no_auto_upgrade', json(CASE WHEN si.no_auto_upgrade = 1 THEN 'true' ELSE 'false' END),

          'images', json(COALESCE(
            (
              SELECT json_group_array(
                json_object(
                  'id', im.image_id,
                  'type', im.image_type,
                  'modified', replace(im.image_last_updated, ' ', 'T'),
                  'season_number', im.image_season_number,
                  'episode_number', im.image_episode_number
                )
              )
              FROM ImageFiles im
              WHERE im.poster_set_id = ps.id
                AND im.item_tmdb_id = mi.tmdb_id
            ),
            '[]'
          ))
        )
      )
      FROM SavedItems si
      JOIN PosterSets ps ON ps.id = si.poster_set_id
      WHERE si.tmdb_id = mi.tmdb_id
        AND si.library_title = mi.library_title
        AND si.edition = mi.edition
    ),
    json('[]')
  ) AS poster_sets

FROM base mi
%s
ORDER BY %s %s, mi.tmdb_id ASC, mi.library_title ASC, mi.edition ASC
LIMIT ? OFFSET ?;
`, whereSQL, sortCol, sortDir)

	dataArgs := make([]any, 0, len(baseArgs)+2)
	dataArgs = append(dataArgs, baseArgs...)
	dataArgs = append(dataArgs, limit, offset)

	rows, err := s.conn.QueryContext(ctx, dataSQL, dataArgs...)
	if err != nil {
		logAction.SetError("Failed to query all saved sets", "", map[string]any{"error": err.Error(), "query": dataSQL})
		return out, *logAction.Error
	}
	defer rows.Close()

	type rowT struct {
		TMDBID       string
		LibraryTitle string
		MediaItem    string
		PosterSets   sql.NullString
	}

	for rows.Next() {
		var r rowT
		if err := rows.Scan(&r.TMDBID, &r.LibraryTitle, &r.MediaItem, &r.PosterSets); err != nil {
			logAction.SetError("Failed to scan saved item row", "", map[string]any{"error": err.Error()})
			return out, *logAction.Error
		}

		var mi models.MediaItem
		if err := json.Unmarshal([]byte(r.MediaItem), &mi); err != nil {
			logAction.SetError("Failed to unmarshal media_item JSON", "", map[string]any{"error": err.Error()})
			return out, *logAction.Error
		}

		var posterSets []models.DBPosterSetDetail
		if r.PosterSets.Valid && strings.TrimSpace(r.PosterSets.String) != "" {
			if err := json.Unmarshal([]byte(r.PosterSets.String), &posterSets); err != nil {
				logAction.SetError("Failed to unmarshal poster_sets JSON", "", map[string]any{"error": err.Error()})
				return out, *logAction.Error
			}
		}

		if len(posterSets) == 0 {
			out.Total -= 1
			continue
		}
		out.Items = append(out.Items, models.DBSavedItem{
			//TMDB_ID:      r.TMDBID,
			//LibraryTitle: r.LibraryTitle,
			MediaItem:  mi,
			PosterSets: posterSets,
		})
	}

	if err := rows.Err(); err != nil {
		logAction.SetError("Row iteration error", "", map[string]any{"error": err.Error()})
		return out, *logAction.Error
	}

	return out, Err
}

func buildSavedItemsWhere(filter models.DBFilter) (whereSQL string, args []any) {
	clauses := make([]string, 0, 16)
	args = make([]any, 0, 32)

	// The outer query uses "base mi" as alias.
	add := func(sql string, a ...any) {
		clauses = append(clauses, sql)
		args = append(args, a...)
	}

	// Only items with saved sets
	add("mi.set_count > 0")

	if strings.TrimSpace(filter.ItemTMDB_ID) != "" {
		add("mi.tmdb_id = ?", strings.TrimSpace(filter.ItemTMDB_ID))
	}
	if strings.TrimSpace(filter.ItemLibraryTitle) != "" {
		add("mi.library_title = ?", filter.ItemLibraryTitle)
	}
	if filter.ItemYear > 0 {
		add("mi.year = ?", filter.ItemYear)
	}
	if strings.TrimSpace(filter.ItemTitle) != "" {
		add("mi.title LIKE ?", "%"+strings.TrimSpace(filter.ItemTitle)+"%")
	}
	if len(filter.LibraryTitles) > 0 {
		// Preserve exact library title values (including intentional leading spaces).
		inSQL, inArgs := makeInClauseNoTrim(filter.LibraryTitles)
		add("mi.library_title IN ("+inSQL+")", inArgs...)
	}

	// Filters that apply to poster sets/images via EXISTS
	posterSetExists := make([]string, 0, 8)
	posterArgs := make([]any, 0, 16)

	if filter.SetID != "" {
		posterSetExists = append(posterSetExists, "ps.set_id = ?")
		posterArgs = append(posterArgs, filter.SetID)
	}

	if len(filter.Usernames) > 0 {
		inSQL, inArgs := makeInClause(filter.Usernames)
		posterSetExists = append(posterSetExists, "ps.user IN ("+inSQL+")")
		posterArgs = append(posterArgs, inArgs...)
	}

	// IMPORTANT: autodownload is now stored on SavedItems (si), not PosterSets (ps)
	switch strings.ToLower(strings.TrimSpace(filter.Autodownload)) {
	case "true", "1", "yes", "on":
		posterSetExists = append(posterSetExists, "si.autodownload = 1")
	case "false", "0", "no", "off":
		posterSetExists = append(posterSetExists, "si.autodownload = 0")
	}

	if len(filter.ImageTypes) > 0 {
		inSQL, inArgs := makeInClause(filter.ImageTypes)

		// IMPORTANT: ImageFiles must be filtered to the current item TMDB id,
		// otherwise a collection set would match due to images that belong to other items.
		posterSetExists = append(posterSetExists, fmt.Sprintf(`
EXISTS (
  SELECT 1
  FROM ImageFiles im
  WHERE im.poster_set_id = ps.id
    AND im.item_tmdb_id = mi.tmdb_id
    AND im.image_type IN (%s)
)`, inSQL))
		posterArgs = append(posterArgs, inArgs...)
	}

	if len(posterSetExists) > 0 {
		add(fmt.Sprintf(`
EXISTS (
  SELECT 1
  FROM SavedItems si
  JOIN PosterSets ps ON ps.id = si.poster_set_id
  WHERE si.tmdb_id = mi.tmdb_id
    AND si.library_title = mi.library_title
    AND si.edition = mi.edition
    AND %s
)`, strings.Join(posterSetExists, " AND ")), posterArgs...)
	}

	if filter.MultiSetOnly {
		add("mi.set_count > 1")
	}

	if filter.MediaItemOnServer != "" {
		switch strings.ToLower(filter.MediaItemOnServer) {
		case "true", "1", "yes", "on":
			add("mi.on_server = 1")
		case "false", "0", "no", "off":
			add("mi.on_server = 0")
		}
	}

	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func makeInClause(values []string) (placeholders string, args []any) {
	args = make([]any, 0, len(values))
	ph := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		ph = append(ph, "?")
		args = append(args, v)
	}
	if len(ph) == 0 {
		// Caller should avoid using this if empty; keep safe fallback.
		return "NULL", []any{}
	}
	return strings.Join(ph, ","), args
}

func makeInClauseNoTrim(values []string) (placeholders string, args []any) {
	args = make([]any, 0, len(values))
	ph := make([]string, 0, len(values))
	for _, v := range values {
		// Keep exact value; skip only truly empty entries.
		if v == "" {
			continue
		}
		ph = append(ph, "?")
		args = append(args, v)
	}
	if len(ph) == 0 {
		return "NULL", []any{}
	}
	return strings.Join(ph, ","), args
}
