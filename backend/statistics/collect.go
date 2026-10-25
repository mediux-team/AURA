package statistics

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediaserver/silo"
	"aura/mediux"
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// Creators listed by the most sets
const topCreatorsLimit = 10

// Get works out the statistics, with the graphs going back a number of days
func Get(ctx context.Context, days int) (stats Statistics, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Getting the Statistics", logging.LevelDebug)
	defer logAction.Complete()

	now := time.Now()
	if days < 1 {
		days = defaultRangeDays
	}
	since, period := rangeStart(days, now)
	stats = Statistics{GeneratedAt: now, Days: days, Period: period}

	// The media servers
	mainLibraries := storeLibraries(cache.LibraryStore)
	addSiloEpisodes(ctx, config.Current.MediaServer, mainLibraries)
	stats.Library = libraryStats(mainLibraries, since, period, now)
	stats.SetsApplied = setsApplied(mainLibraries)
	stats.MediaServers = mediaServerStats(ctx, mainLibraries, stats.Library.Totals)

	// The downloads
	if Err := downloadStats(ctx, &stats, since, period, now); Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return stats, Err
	}

	// The collections
	stats.Collections.Collections = len(cache.CollectionsStore.GetAllCollections())
	withImages, savedCollectionSets, Err := database.CollectionCounts(ctx)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return stats, Err
	}
	stats.Collections.WithImages, stats.Collections.SavedCollectionSet = withImages, savedCollectionSets
	stats.Collections.CollectionImages = stats.Downloads.AllTime.CollectionImages

	// The MediUX images cached on disk
	stats.Data.ImageCacheBytes, stats.Data.ImageCacheFiles = folderSize(mediux.FullTempImageFolder, mediux.ThumbsTempImageFolder)

	// Today's numbers, then the daily numbers of the range
	RecordSnapshot(ctx, stats.Library.Totals, stats.SetsApplied)
	stats.History, Err = snapshotsSince(ctx, since)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return stats, Err
	}
	return stats, logging.LogErrorInfo{}
}

// mediaServerStats is the items of each enabled media server, and how the others compare to the main one
func mediaServerStats(ctx context.Context, mainLibraries []libraryItems, mainCounts ItemCounts) []MediaServerStats {
	current := config.Current.Clone()
	servers := []MediaServerStats{}
	for _, server := range current.MediaServers {
		if server.ID == current.MainMediaServer {
			servers = append(servers, MediaServerStats{ID: server.ID, Name: server.Name, Type: server.Type, Main: true, Counts: mainCounts,
				OnlyInMain: []ItemRef{}, OnlyInServer: []ItemRef{}})
			continue
		}
		if !server.Enabled {
			continue
		}
		libraries := storeLibraries(cache.ServerLibraryStore(server.ID))
		addSiloEpisodes(ctx, server, libraries)
		stats := compareServer(mainLibraries, libraries, server.LibraryMap)
		stats.ID, stats.Name, stats.Type = server.ID, server.Name, server.Type
		stats.Counts = librariesCounts(libraries)
		servers = append(servers, stats)
	}
	return servers
}

// downloadStats fills in the downloads, creators, sources, failures and data
func downloadStats(ctx context.Context, stats *Statistics, since time.Time, period string, now time.Time) logging.LogErrorInfo {
	allTime, Err := database.GetDownloadEventTotals(ctx, time.Time{})
	if Err.Message != "" {
		return Err
	}
	inRange, Err := database.GetDownloadEventTotals(ctx, since)
	if Err.Message != "" {
		return Err
	}
	stats.Downloads.AllTime, stats.Downloads.InRange = downloadTotals(allTime), downloadTotals(inRange)
	if stats.Downloads.AllTime.Redownloads, Err = database.GetRedownloadCount(ctx, time.Time{}); Err.Message != "" {
		return Err
	}
	if stats.Downloads.InRange.Redownloads, Err = database.GetRedownloadCount(ctx, since); Err.Message != "" {
		return Err
	}

	buckets, Err := database.GetDownloadEventTimeline(ctx, since, period)
	if Err.Message != "" {
		return Err
	}
	stats.Downloads.Timeline = downloadTimeline(buckets, since, period, now)

	imageTypes, Err := database.GetDownloadEventImageTypes(ctx, since)
	if Err.Message != "" {
		return Err
	}
	stats.Downloads.ImageTypes = []GroupCount{}
	for _, imageType := range imageTypes {
		stats.Downloads.ImageTypes = append(stats.Downloads.ImageTypes, GroupCount{Name: imageType.Name, Images: imageType.Count})
	}

	// Creators, with the favorites marked
	creators, Err := database.GetDownloadEventGroups(ctx, since, "creator", 10000)
	if Err.Message != "" {
		return Err
	}
	favorites := config.Current.UserPreferences.FavoriteCreators
	stats.Creators = creatorStats(creators, favorites)

	sources, Err := database.GetDownloadEventGroups(ctx, since, "source", 100)
	if Err.Message != "" {
		return Err
	}
	stats.Sources = []GroupCount{}
	for _, source := range sources {
		stats.Sources = append(stats.Sources, GroupCount{Name: source.Name, Sets: source.Sets, Images: source.Images})
	}

	// Failures
	stats.Failures.ImagesFailed = stats.Downloads.InRange.ImagesFailed
	if tried := stats.Downloads.InRange.Images + stats.Downloads.InRange.ImagesFailed; tried > 0 {
		stats.Failures.FailureRate = float64(stats.Downloads.InRange.ImagesFailed) / float64(tried)
	}
	if stats.Failures.OpenFailures, Err = database.CountDownloadHistory(ctx); Err.Message != "" {
		return Err
	}
	reasons, Err := database.GetFailureReasons(ctx, 5)
	if Err.Message != "" {
		return Err
	}
	stats.Failures.TopReasons = []NameCount{}
	for _, reason := range reasons {
		stats.Failures.TopReasons = append(stats.Failures.TopReasons, NameCount{Name: reason.Name, Count: reason.Count})
	}

	stats.Data.BytesInRange, stats.Data.BytesAllTime = stats.Downloads.InRange.Bytes, stats.Downloads.AllTime.Bytes
	return logging.LogErrorInfo{}
}

// downloadTotals adds up the downloads of each kind
func downloadTotals(totals []database.DownloadEventTotal) (out DownloadTotals) {
	for _, total := range totals {
		out.ImagesFailed += total.ImagesFailed
		out.Bytes += total.Bytes
		if total.Collection {
			out.CollectionImages += total.ImagesSucceeded
			out.Images += total.ImagesSucceeded
			continue
		}
		switch total.Kind {
		case "download":
			out.Sets += total.Events
			out.Images += total.ImagesSucceeded
			out.EstimatedSets += total.Estimated
		case "update":
			out.Updates += total.Events
			out.UpdatedImages += total.ImagesSucceeded
			out.Images += total.ImagesSucceeded
		case "reapply":
			out.Reapplies += total.Events
		case "saved":
			out.SavedOnly += total.Events
		}
	}
	return out
}

// downloadTimeline is the downloads of each day, week or month in the range, with the ones without downloads too
func downloadTimeline(buckets []database.DownloadEventBucket, since time.Time, period string, now time.Time) []DownloadPoint {
	byDate := map[string]*DownloadPoint{}
	for _, bucket := range buckets {
		point, found := byDate[bucket.Date]
		if !found {
			point = &DownloadPoint{Date: bucket.Date}
			byDate[bucket.Date] = point
		}
		point.ImagesFailed += bucket.ImagesFailed
		point.Bytes += bucket.Bytes
		switch {
		case bucket.Collection:
			point.Images += bucket.ImagesSucceeded
		case bucket.Kind == "download":
			point.Sets += bucket.Events
			point.Images += bucket.ImagesSucceeded
		case bucket.Kind == "update":
			point.Updates += bucket.Events
			point.Images += bucket.ImagesSucceeded
		case bucket.Kind == "reapply":
			point.Reapplies += bucket.Events
		}
	}

	timeline := []DownloadPoint{}
	for _, start := range periodStarts(since, now, period) {
		date := start.Format("2006-01-02")
		if point, found := byDate[date]; found {
			timeline = append(timeline, *point)
		} else {
			timeline = append(timeline, DownloadPoint{Date: date})
		}
	}
	return timeline
}

// creatorStats is the creators with the most sets, and how many sets came from favorite creators
func creatorStats(creators []database.DownloadEventGroup, favorites []string) (stats CreatorStats) {
	isFavorite := func(name string) bool {
		for _, favorite := range favorites {
			if strings.EqualFold(favorite, name) {
				return true
			}
		}
		return false
	}
	stats.FavoriteCount = len(favorites)
	stats.Top = []GroupCount{}
	for _, creator := range creators {
		favorite := isFavorite(creator.Name)
		if favorite {
			stats.FavoriteSets += creator.Sets
		} else {
			stats.OtherSets += creator.Sets
		}
		if len(stats.Top) < topCreatorsLimit {
			stats.Top = append(stats.Top, GroupCount{Name: creator.Name, Sets: creator.Sets, Images: creator.Images, Favorite: favorite})
		}
	}
	return stats
}

// folderSize is the size and number of the files in folders
func folderSize(folders ...string) (bytes int64, files int) {
	for _, folder := range folders {
		_ = filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			if info, err := entry.Info(); err == nil {
				bytes += info.Size()
				files++
			}
			return nil
		})
	}
	return bytes, files
}

// Asks Silo for a show library's episodes. Replaced in tests.
var siloLibraryEpisodeCount = silo.LibraryEpisodeCount

// addSiloEpisodes asks Silo for the episodes of its show libraries, as its shows don't have them
func addSiloEpisodes(ctx context.Context, server config.Config_MediaServer, libraries []libraryItems) {
	if server.Type != "Silo" {
		return
	}
	for i := range libraries {
		if libraries[i].Type != "show" || libraries[i].ID == "" {
			continue
		}
		episodes, Err := siloLibraryEpisodeCount(ctx, server, libraries[i].ID)
		if Err.Message != "" {
			logging.LOGGER.Debug().Timestamp().Str("library", libraries[i].Title).Str("error", Err.Message).Msg("Failed to get the episodes of a Silo library")
			continue
		}
		libraries[i].Episodes = episodes
	}
}
