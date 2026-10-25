package database

import (
	"aura/logging"
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DownloadEventTotal is the downloads of one kind, with collection images on their own
type DownloadEventTotal struct {
	Kind            string
	Collection      bool // A collection's own images
	Events          int
	ImagesSucceeded int
	ImagesFailed    int
	Bytes           int64
	Estimated       int // Events counted from the saved sets
}

// DownloadEventBucket is the downloads of one kind in a day, week or month
type DownloadEventBucket struct {
	Date            string // The first day of the day, week or month, as YYYY-MM-DD
	Kind            string
	Collection      bool
	Events          int
	ImagesSucceeded int
	ImagesFailed    int
	Bytes           int64
}

// DownloadEventGroup is the downloads of one creator or source
type DownloadEventGroup struct {
	Name   string
	Sets   int
	Images int
}

// NameCount is a name with how many times it was counted
type NameCount struct {
	Name  string
	Count int
}

// StatisticsSnapshot is one day of numbers, as JSON
type StatisticsSnapshot struct {
	Date string
	Data string
}

// periodSQL is the SQL that turns created_at into the first day of its day, week (from Monday) or month
func periodSQL(period string) string {
	switch period {
	case "week":
		return "date(created_at, 'localtime', 'weekday 0', '-6 days')"
	case "month":
		return "strftime('%Y-%m-01', created_at, 'localtime')"
	default:
		return "date(created_at, 'localtime')"
	}
}

// sinceSQL limits the events to those from since, or all of them when since is zero
func sinceSQL(since time.Time) (string, []any) {
	if since.IsZero() {
		return "1 = 1", nil
	}
	return "created_at >= ?", []any{since.UTC().Format("2006-01-02 15:04:05")}
}

// statisticsQuery runs a query for the statistics
func statisticsQuery(ctx context.Context, query string, args ...any) (*sql.Rows, logging.LogErrorInfo) {
	conn, _, Err := GetDBConnection(ctx)
	if Err.Message != "" {
		return nil, Err
	}
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, logging.LogErrorInfo{Message: "Failed to get the statistics", Detail: map[string]any{"error": err.Error()}}
	}
	return rows, logging.LogErrorInfo{}
}

// GetDownloadEventTotals returns the downloads of each kind since a time
func GetDownloadEventTotals(ctx context.Context, since time.Time) (totals []DownloadEventTotal, Err logging.LogErrorInfo) {
	where, args := sinceSQL(since)
	rows, Err := statisticsQuery(ctx, fmt.Sprintf(`
SELECT kind, media_item_type = 'collection', COUNT(*), SUM(images_succeeded), SUM(images_failed), SUM(bytes), SUM(estimated)
FROM DownloadEvents WHERE %s
GROUP BY kind, media_item_type = 'collection';`, where), args...)
	if Err.Message != "" {
		return nil, Err
	}
	defer rows.Close()
	for rows.Next() {
		var total DownloadEventTotal
		if err := rows.Scan(&total.Kind, &total.Collection, &total.Events, &total.ImagesSucceeded, &total.ImagesFailed, &total.Bytes, &total.Estimated); err != nil {
			return nil, logging.LogErrorInfo{Message: "Failed to read the download totals", Detail: map[string]any{"error": err.Error()}}
		}
		totals = append(totals, total)
	}
	return totals, logging.LogErrorInfo{}
}

// GetDownloadEventTimeline returns the downloads of each kind for each day, week or month since a time
func GetDownloadEventTimeline(ctx context.Context, since time.Time, period string) (buckets []DownloadEventBucket, Err logging.LogErrorInfo) {
	where, args := sinceSQL(since)
	rows, Err := statisticsQuery(ctx, fmt.Sprintf(`
SELECT %s AS bucket, kind, media_item_type = 'collection', COUNT(*), SUM(images_succeeded), SUM(images_failed), SUM(bytes)
FROM DownloadEvents WHERE %s
GROUP BY bucket, kind, media_item_type = 'collection'
ORDER BY bucket;`, periodSQL(period), where), args...)
	if Err.Message != "" {
		return nil, Err
	}
	defer rows.Close()
	for rows.Next() {
		var bucket DownloadEventBucket
		if err := rows.Scan(&bucket.Date, &bucket.Kind, &bucket.Collection, &bucket.Events, &bucket.ImagesSucceeded, &bucket.ImagesFailed, &bucket.Bytes); err != nil {
			return nil, logging.LogErrorInfo{Message: "Failed to read the download timeline", Detail: map[string]any{"error": err.Error()}}
		}
		buckets = append(buckets, bucket)
	}
	return buckets, logging.LogErrorInfo{}
}

// GetDownloadEventGroups returns the sets and images by creator or source since a time, the most sets first.
// Sets are downloads, collection images and updates only count as images.
func GetDownloadEventGroups(ctx context.Context, since time.Time, groupBy string, limit int) (groups []DownloadEventGroup, Err logging.LogErrorInfo) {
	column := map[string]string{"creator": "set_creator", "source": "source"}[groupBy]
	if column == "" {
		return nil, logging.LogErrorInfo{Message: fmt.Sprintf("Unknown download group '%s'", groupBy)}
	}
	where, args := sinceSQL(since)
	rows, Err := statisticsQuery(ctx, fmt.Sprintf(`
SELECT %[1]s, SUM(CASE WHEN kind = 'download' AND media_item_type != 'collection' THEN 1 ELSE 0 END), SUM(images_succeeded)
FROM DownloadEvents WHERE %[2]s AND kind != 'saved' AND %[1]s != ''
GROUP BY %[1]s
ORDER BY 2 DESC, 3 DESC
LIMIT ?;`, column, where), append(args, limit)...)
	if Err.Message != "" {
		return nil, Err
	}
	defer rows.Close()
	for rows.Next() {
		var group DownloadEventGroup
		if err := rows.Scan(&group.Name, &group.Sets, &group.Images); err != nil {
			return nil, logging.LogErrorInfo{Message: "Failed to read the downloads by " + groupBy, Detail: map[string]any{"error": err.Error()}}
		}
		groups = append(groups, group)
	}
	return groups, logging.LogErrorInfo{}
}

// GetDownloadEventImageTypes returns the images downloaded of each type since a time
func GetDownloadEventImageTypes(ctx context.Context, since time.Time) (types []NameCount, Err logging.LogErrorInfo) {
	where, args := sinceSQL(since)
	rows, Err := statisticsQuery(ctx, fmt.Sprintf(`
SELECT image_type.key, SUM(image_type.value)
FROM DownloadEvents, json_each(DownloadEvents.image_types) AS image_type
WHERE %s AND kind IN ('download', 'update')
GROUP BY image_type.key
ORDER BY 2 DESC;`, where), args...)
	if Err.Message != "" {
		return nil, Err
	}
	defer rows.Close()
	for rows.Next() {
		var count NameCount
		if err := rows.Scan(&count.Name, &count.Count); err != nil {
			return nil, logging.LogErrorInfo{Message: "Failed to read the image types", Detail: map[string]any{"error": err.Error()}}
		}
		types = append(types, count)
	}
	return types, logging.LogErrorInfo{}
}

// GetRedownloadCount returns how many set downloads since a time were of a set already downloaded for the item
func GetRedownloadCount(ctx context.Context, since time.Time) (redownloads int, Err logging.LogErrorInfo) {
	where, args := sinceSQL(since)
	rows, Err := statisticsQuery(ctx, fmt.Sprintf(`
SELECT COUNT(*)
FROM DownloadEvents event
WHERE %s AND event.kind = 'download' AND event.media_item_type != 'collection'
  AND EXISTS (
    SELECT 1 FROM DownloadEvents earlier
    WHERE earlier.kind = 'download'
      AND earlier.tmdb_id = event.tmdb_id AND earlier.library_title = event.library_title
      AND earlier.edition = event.edition AND earlier.set_id = event.set_id
      AND earlier.id < event.id
  );`, where), args...)
	if Err.Message != "" {
		return 0, Err
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&redownloads); err != nil {
			return 0, logging.LogErrorInfo{Message: "Failed to read the redownloads", Detail: map[string]any{"error": err.Error()}}
		}
	}
	return redownloads, logging.LogErrorInfo{}
}

// GetFailureReasons returns the most common reasons images failed, from the Download History
func GetFailureReasons(ctx context.Context, limit int) (reasons []NameCount, Err logging.LogErrorInfo) {
	rows, Err := statisticsQuery(ctx, `
SELECT COALESCE(NULLIF(json_extract(failed.value, '$.failure_reason'), ''), 'Unknown'), COUNT(*)
FROM DownloadHistory, json_each(DownloadHistory.failed_images) AS failed
GROUP BY 1
ORDER BY 2 DESC
LIMIT ?;`, limit)
	if Err.Message != "" {
		return nil, Err
	}
	defer rows.Close()
	for rows.Next() {
		var reason NameCount
		if err := rows.Scan(&reason.Name, &reason.Count); err != nil {
			return nil, logging.LogErrorInfo{Message: "Failed to read the failure reasons", Detail: map[string]any{"error": err.Error()}}
		}
		reasons = append(reasons, reason)
	}
	return reasons, logging.LogErrorInfo{}
}

// UpsertStatisticsSnapshot saves the numbers of a day, replacing that day's earlier numbers
func UpsertStatisticsSnapshot(ctx context.Context, date string, data string) logging.LogErrorInfo {
	conn, _, Err := GetDBConnection(ctx)
	if Err.Message != "" {
		return Err
	}
	if _, err := conn.ExecContext(ctx, `
INSERT INTO StatisticsSnapshots (snapshot_date, data, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(snapshot_date) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at;`, date, data); err != nil {
		return logging.LogErrorInfo{Message: "Failed to save the statistics snapshot", Detail: map[string]any{"error": err.Error()}}
	}
	return logging.LogErrorInfo{}
}

// GetStatisticsSnapshots returns the daily numbers from a day, the oldest first
func GetStatisticsSnapshots(ctx context.Context, sinceDate string) (snapshots []StatisticsSnapshot, Err logging.LogErrorInfo) {
	rows, Err := statisticsQuery(ctx, `
SELECT snapshot_date, data FROM StatisticsSnapshots WHERE snapshot_date >= ? ORDER BY snapshot_date;`, sinceDate)
	if Err.Message != "" {
		return nil, Err
	}
	defer rows.Close()
	for rows.Next() {
		var snapshot StatisticsSnapshot
		if err := rows.Scan(&snapshot.Date, &snapshot.Data); err != nil {
			return nil, logging.LogErrorInfo{Message: "Failed to read the statistics snapshots", Detail: map[string]any{"error": err.Error()}}
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, logging.LogErrorInfo{}
}

// CollectionCounts is the collections aura applied their own images to, and the collection sets saved for movies
func CollectionCounts(ctx context.Context) (withImages int, savedCollectionSets int, Err logging.LogErrorInfo) {
	rows, Err := statisticsQuery(ctx, `
SELECT
  (SELECT COUNT(DISTINCT tmdb_id || '|' || library_title) FROM DownloadEvents WHERE media_item_type = 'collection' AND images_succeeded > 0),
  (SELECT COUNT(*) FROM SavedItems si JOIN PosterSets ps ON ps.id = si.poster_set_id WHERE ps.type = 'collection');`)
	if Err.Message != "" {
		return 0, 0, Err
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&withImages, &savedCollectionSets); err != nil {
			return 0, 0, logging.LogErrorInfo{Message: "Failed to read the collection counts", Detail: map[string]any{"error": err.Error()}}
		}
	}
	return withImages, savedCollectionSets, logging.LogErrorInfo{}
}

// CountDownloadHistory returns how many downloads are in the Download History
func CountDownloadHistory(ctx context.Context) (count int, Err logging.LogErrorInfo) {
	rows, Err := statisticsQuery(ctx, `SELECT COUNT(*) FROM DownloadHistory;`)
	if Err.Message != "" {
		return 0, Err
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, logging.LogErrorInfo{Message: "Failed to count the Download History", Detail: map[string]any{"error": err.Error()}}
		}
	}
	return count, logging.LogErrorInfo{}
}
