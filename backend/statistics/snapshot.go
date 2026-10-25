package statistics

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"context"
	"encoding/json"
	"time"
)

// RecordSnapshot saves today's numbers, so the graphs can show how they change
func RecordSnapshot(ctx context.Context, counts ItemCounts, sets SetsAppliedStats) {
	// Nothing to save before the libraries are loaded
	if counts.Items == 0 {
		return
	}
	snapshot := Snapshot{Date: time.Now().Format("2006-01-02"), Counts: counts, ItemsWithSets: sets.ItemsWithSets, SavedSets: sets.SavedSets}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	if Err := database.UpsertStatisticsSnapshot(ctx, snapshot.Date, string(data)); Err.Message != "" {
		logging.LOGGER.Warn().Timestamp().Str("error", Err.Message).Msg("Failed to save today's statistics")
	}
}

// RecordLibrarySnapshot saves today's numbers from the main media server's libraries, after they're loaded
func RecordLibrarySnapshot(ctx context.Context) {
	libraries := storeLibraries(cache.LibraryStore)
	addSiloEpisodes(ctx, config.Current.MediaServer, libraries)
	RecordSnapshot(ctx, librariesCounts(libraries), setsApplied(libraries))
}

// snapshotsSince is the daily numbers from a day, the oldest first
func snapshotsSince(ctx context.Context, since time.Time) ([]Snapshot, logging.LogErrorInfo) {
	saved, Err := database.GetStatisticsSnapshots(ctx, since.Format("2006-01-02"))
	if Err.Message != "" {
		return nil, Err
	}
	snapshots := []Snapshot{}
	for _, row := range saved {
		var snapshot Snapshot
		if err := json.Unmarshal([]byte(row.Data), &snapshot); err != nil {
			continue
		}
		snapshot.Date = row.Date
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, logging.LogErrorInfo{}
}
