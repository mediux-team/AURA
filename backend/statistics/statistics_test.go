package statistics

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"testing"
	"time"
)

func movie(tmdbID string, added time.Time, sets ...models.DBSavedSet) models.MediaItem {
	return models.MediaItem{TMDB_ID: tmdbID, Type: "movie", Title: "Movie " + tmdbID, AddedAt: added.Unix(), DBSavedSets: sets}
}

func show(tmdbID string, seasons, episodes int, sets ...models.DBSavedSet) models.MediaItem {
	return models.MediaItem{TMDB_ID: tmdbID, Type: "show", Title: "Show " + tmdbID, SeasonCount: seasons, EpisodeCount: episodes, DBSavedSets: sets}
}

func TestLibraryCountsAndCoverage(t *testing.T) {
	day := time.Date(2026, 10, 20, 12, 0, 0, 0, time.Local)
	posterSet := models.DBSavedSet{ID: "a", SelectedTypes: models.SelectedTypes{Poster: true}}
	titlecards := models.DBSavedSet{ID: "b", SelectedTypes: models.SelectedTypes{Poster: true, Titlecard: true}}
	libraries := []libraryItems{
		{Title: "Movies", Type: "movie", Items: []models.MediaItem{movie("1", day.AddDate(0, 0, -3), posterSet), movie("2", day)}},
		{Title: "TV Shows", Type: "show", Items: []models.MediaItem{show("10", 3, 30, titlecards), show("11", 1, 8)}},
	}

	stats := libraryStats(libraries, day.AddDate(0, 0, -6), "day", day)
	if stats.Totals != (ItemCounts{Items: 4, Movies: 2, Shows: 2, Seasons: 4, Episodes: 38}) || len(stats.Libraries) != 2 {
		t.Errorf("totals = %+v", stats.Totals)
	}
	// One point a day, with the shows counted from the start as they have no added date
	if len(stats.Growth) != 7 || stats.Growth[0].Movies != 0 || stats.Growth[3].Movies != 1 || stats.Growth[6].Movies != 2 || stats.Growth[0].Shows != 2 {
		t.Errorf("growth = %+v", stats.Growth)
	}

	coverage := setsApplied(libraries)
	if coverage.Items != 4 || coverage.ItemsWithSets != 2 || coverage.SavedSets != 2 {
		t.Errorf("coverage = %+v", coverage)
	}
	types := map[string]TypeCoverage{}
	for _, imageType := range coverage.ImageTypes {
		types[imageType.Type] = imageType
	}
	if types["poster"] != (TypeCoverage{Type: "poster", Items: 2, Total: 4}) || types["titlecard"] != (TypeCoverage{Type: "titlecard", Items: 1, Total: 2}) {
		t.Errorf("image types = %+v", coverage.ImageTypes)
	}
}

func TestCompareServer(t *testing.T) {
	now := time.Now()
	main := []libraryItems{
		{Title: "Movies", Items: []models.MediaItem{movie("1", now), movie("2", now)}},
		{Title: "TV Shows", Items: []models.MediaItem{show("10", 1, 1)}}, // Not in the library map
	}
	other := []libraryItems{
		{Title: "Films", Items: []models.MediaItem{movie("2", now), movie("3", now)}},
		{Title: "Kids", Items: []models.MediaItem{movie("4", now)}},
	}

	stats := compareServer(main, other, map[string]string{"Films": "Movies", "Kids": ""})
	if stats.Matched != 1 || stats.OnlyInMainCount != 1 || stats.OnlyInServerCount != 1 {
		t.Fatalf("comparison = %+v", stats)
	}
	if stats.OnlyInMain[0].TMDB_ID != "1" || stats.OnlyInServer[0].TMDB_ID != "3" || stats.OnlyInServer[0].Library != "Movies" {
		t.Errorf("lists = %+v and %+v", stats.OnlyInMain, stats.OnlyInServer)
	}
	if len(stats.UnmappedLibraries) != 1 || stats.UnmappedLibraries[0] != "Kids" {
		t.Errorf("unmapped = %v", stats.UnmappedLibraries)
	}
}

func TestDownloadTotalsAndTimeline(t *testing.T) {
	totals := downloadTotals([]database.DownloadEventTotal{
		{Kind: "download", Events: 3, ImagesSucceeded: 10, ImagesFailed: 1, Bytes: 100, Estimated: 2},
		{Kind: "update", Events: 1, ImagesSucceeded: 2},
		{Kind: "reapply", Events: 4, ImagesSucceeded: 9},
		{Kind: "saved", Events: 1},
		{Kind: "download", Collection: true, Events: 2, ImagesSucceeded: 2},
	})
	want := DownloadTotals{Sets: 3, Images: 14, Updates: 1, UpdatedImages: 2, Reapplies: 4, SavedOnly: 1, CollectionImages: 2, ImagesFailed: 1, Bytes: 100, EstimatedSets: 2}
	if totals != want {
		t.Errorf("totals = %+v, want %+v", totals, want)
	}

	// Every day of the range, with the days without downloads too
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.Local)
	timeline := downloadTimeline([]database.DownloadEventBucket{{Date: "2026-10-23", Kind: "download", Events: 2, ImagesSucceeded: 5}}, now.AddDate(0, 0, -2), "day", now)
	if len(timeline) != 3 || timeline[0].Date != "2026-10-22" || timeline[1].Sets != 2 || timeline[1].Images != 5 || timeline[2].Sets != 0 {
		t.Errorf("timeline = %+v", timeline)
	}
}

func TestCreatorStats(t *testing.T) {
	stats := creatorStats([]database.DownloadEventGroup{{Name: "Fav", Sets: 5, Images: 50}, {Name: "other", Sets: 2, Images: 4}}, []string{"fav"})
	if stats.FavoriteSets != 5 || stats.OtherSets != 2 || stats.FavoriteCount != 1 || !stats.Top[0].Favorite || stats.Top[1].Favorite {
		t.Errorf("creators = %+v", stats)
	}
}

func TestPeriods(t *testing.T) {
	// Weeks start on Monday, like the database groups them
	sunday := time.Date(2026, 10, 25, 15, 0, 0, 0, time.Local)
	if start := periodStart(sunday, "week"); start.Weekday() != time.Monday || start.Day() != 19 {
		t.Errorf("week of %v starts %v", sunday, start)
	}
	if start := periodStart(sunday, "month"); start.Day() != 1 || start.Month() != time.October {
		t.Errorf("month starts %v", start)
	}
	// Days are grouped by day, then week, then month
	for days, want := range map[int]struct {
		period string
		points int
	}{1: {"day", 1}, 7: {"day", 7}, 45: {"day", 45}, 90: {"week", 13}, 365: {"week", 53}, 1000: {"month", 34}} {
		since, period := rangeStart(days, sunday)
		if points := len(periodStarts(since, sunday, period)); period != want.period || points != want.points {
			t.Errorf("%d days = %s with %d points, want %s with %d", days, period, points, want.period, want.points)
		}
	}

	// One day still draws a line, from the day before
	if points := growth([]models.MediaItem{movie("1", sunday)}, startOfDay(sunday), "day", sunday); len(points) != 2 || points[0].Movies != 0 || points[1].Movies != 1 {
		t.Errorf("one day growth = %+v", points)
	}
}

func TestSiloEpisodesFromTheLibrary(t *testing.T) {
	previous := siloLibraryEpisodeCount
	t.Cleanup(func() { siloLibraryEpisodeCount = previous })
	asked := []string{}
	siloLibraryEpisodeCount = func(ctx context.Context, server config.Config_MediaServer, libraryID string) (int, logging.LogErrorInfo) {
		asked = append(asked, libraryID)
		return 120, logging.LogErrorInfo{}
	}
	libraries := []libraryItems{
		{ID: "movies", Title: "Movies", Type: "movie", Items: []models.MediaItem{movie("1", time.Now())}},
		{ID: "shows", Title: "TV Shows", Type: "show", Items: []models.MediaItem{show("10", 3, 0)}},
	}

	// Only Silo, and only its show libraries
	addSiloEpisodes(context.Background(), config.Config_MediaServer{Type: "Plex"}, libraries)
	addSiloEpisodes(context.Background(), config.Config_MediaServer{Type: "Silo"}, libraries)
	if len(asked) != 1 || asked[0] != "shows" {
		t.Fatalf("asked for %v", asked)
	}
	if counts := librariesCounts(libraries); counts.Episodes != 120 || counts.Seasons != 3 {
		t.Errorf("counts = %+v", counts)
	}
}
