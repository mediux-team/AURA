package statistics

import (
	"aura/cache"
	"aura/models"
	"slices"
	"sort"
	"strings"
	"time"
)

// Items listed in a media server comparison, the rest are only counted
const comparisonListLimit = 200

// libraryItems is the items of one library
type libraryItems struct {
	ID       string
	Title    string
	Type     string
	Items    []models.MediaItem
	Episodes int // The library's episodes, when the media server can't give each show's
}

// storeLibraries copies the items of each library in a media server's cache, by title
func storeLibraries(store *cache.MediaServerLibraryCache) []libraryItems {
	byTitle := map[string]*libraryItems{}
	store.ForEachMediaItem(func(section models.LibrarySectionBase, item *models.MediaItem) {
		library, found := byTitle[section.Title]
		if !found {
			library = &libraryItems{ID: section.ID, Title: section.Title, Type: section.Type}
			byTitle[section.Title] = library
		}
		library.Items = append(library.Items, *item)
	})
	libraries := make([]libraryItems, 0, len(byTitle))
	for _, library := range byTitle {
		libraries = append(libraries, *library)
	}
	sort.Slice(libraries, func(i, j int) bool { return libraries[i].Title < libraries[j].Title })
	return libraries
}

// showCounts is a show's seasons and episodes, from the library listing or its details
func showCounts(item models.MediaItem) (seasons, episodes int) {
	seasons, episodes = item.SeasonCount, item.EpisodeCount
	if item.Series != nil {
		seasons = max(seasons, item.Series.SeasonCount, len(item.Series.Seasons))
		episodes = max(episodes, item.Series.EpisodeCount)
	}
	return seasons, episodes
}

// countItems counts the movies, shows, seasons and episodes
func countItems(items []models.MediaItem) (counts ItemCounts) {
	for _, item := range items {
		switch item.Type {
		case "movie":
			counts.Movies++
		case "show":
			counts.Shows++
			seasons, episodes := showCounts(item)
			counts.Seasons += seasons
			counts.Episodes += episodes
		default:
			continue
		}
		counts.Items++
	}
	return counts
}

// libraryCounts counts a library, with its episodes from the media server when the shows don't have them
func libraryCounts(library libraryItems) ItemCounts {
	counts := countItems(library.Items)
	if counts.Episodes == 0 {
		counts.Episodes = library.Episodes
	}
	return counts
}

// librariesCounts counts every library
func librariesCounts(libraries []libraryItems) (counts ItemCounts) {
	for _, library := range libraries {
		counts = addCounts(counts, libraryCounts(library))
	}
	return counts
}

func addCounts(a, b ItemCounts) ItemCounts {
	return ItemCounts{Items: a.Items + b.Items, Movies: a.Movies + b.Movies, Shows: a.Shows + b.Shows, Seasons: a.Seasons + b.Seasons, Episodes: a.Episodes + b.Episodes}
}

// allItems is the items of every library
func allItems(libraries []libraryItems) []models.MediaItem {
	var items []models.MediaItem
	for _, library := range libraries {
		items = append(items, library.Items...)
	}
	return items
}

// libraryStats counts each library, and how the libraries grew in the range
func libraryStats(libraries []libraryItems, since time.Time, period string, now time.Time) (stats LibraryStats) {
	for _, library := range libraries {
		counts := libraryCounts(library)
		stats.Libraries = append(stats.Libraries, LibraryCounts{Title: library.Title, Type: library.Type, Counts: counts})
		stats.Totals = addCounts(stats.Totals, counts)
	}
	stats.Growth = growth(allItems(libraries), since, period, now)
	return stats
}

// growth is how many movies and shows there were at the end of each day, week or month, by when they were added.
// Items without an added date count from the start.
func growth(items []models.MediaItem, since time.Time, period string, now time.Time) []GrowthPoint {
	var movies, shows []int64
	for _, item := range items {
		switch item.Type {
		case "movie":
			movies = append(movies, item.AddedAt)
		case "show":
			shows = append(shows, item.AddedAt)
		}
	}
	slices.Sort(movies)
	slices.Sort(shows)

	// A line needs two points, so a range of one day, week or month starts with the one before it
	starts := periodStarts(since, now, period)
	if len(starts) < 2 {
		previous := periodStart(since.AddDate(0, 0, -1), period)
		starts = append([]time.Time{previous}, starts...)
	}

	// How many were added by the end of a day, week or month
	addedBy := func(sorted []int64, end time.Time) int {
		return sort.Search(len(sorted), func(i int) bool { return sorted[i] >= end.Unix() })
	}
	var points []GrowthPoint
	for _, start := range starts {
		end := nextPeriod(start, period)
		points = append(points, GrowthPoint{Date: start.Format("2006-01-02"), Movies: addedBy(movies, end), Shows: addedBy(shows, end)})
	}
	return points
}

// setsApplied counts the items with saved sets, and the image types selected for them
func setsApplied(libraries []libraryItems) (stats SetsAppliedStats) {
	typeItems := map[string]int{}
	movies, shows := 0, 0
	for _, library := range libraries {
		coverage := LibraryCoverage{Title: library.Title}
		for _, item := range library.Items {
			if item.Type != "movie" && item.Type != "show" {
				continue
			}
			if item.Type == "movie" {
				movies++
			} else {
				shows++
			}
			coverage.Items++
			if len(item.DBSavedSets) == 0 {
				continue
			}
			coverage.ItemsWithSets++
			stats.SavedSets += len(item.DBSavedSets)

			selected := map[string]bool{}
			for _, set := range item.DBSavedSets {
				for imageType, isSelected := range map[string]bool{
					"poster":                set.SelectedTypes.Poster,
					"backdrop":              set.SelectedTypes.Backdrop,
					"season_poster":         set.SelectedTypes.SeasonPoster,
					"special_season_poster": set.SelectedTypes.SpecialSeasonPoster,
					"titlecard":             set.SelectedTypes.Titlecard,
				} {
					selected[imageType] = selected[imageType] || isSelected
				}
			}
			for imageType, isSelected := range selected {
				// Movies only have posters and backdrops
				if isSelected && (item.Type == "show" || imageType == "poster" || imageType == "backdrop") {
					typeItems[imageType]++
				}
			}
		}
		stats.Items += coverage.Items
		stats.ItemsWithSets += coverage.ItemsWithSets
		stats.Libraries = append(stats.Libraries, coverage)
	}

	for _, imageType := range []string{"poster", "backdrop", "season_poster", "special_season_poster", "titlecard"} {
		total := shows
		if imageType == "poster" || imageType == "backdrop" {
			total += movies
		}
		stats.ImageTypes = append(stats.ImageTypes, TypeCoverage{Type: imageType, Items: typeItems[imageType], Total: total})
	}
	return stats
}

// compareServer compares another media server's libraries with the main one's, through its library map
func compareServer(main []libraryItems, other []libraryItems, libraryMap map[string]string) (stats MediaServerStats) {
	itemKey := func(item models.MediaItem) string { return item.Type + "|" + item.TMDB_ID }
	ref := func(item models.MediaItem, library string) ItemRef {
		return ItemRef{Title: item.Title, Year: item.Year, Type: item.Type, Library: library, TMDB_ID: item.TMDB_ID}
	}

	// The other media server's items, by the main library each of its libraries matches
	otherByMain := map[string]map[string]models.MediaItem{}
	for _, library := range other {
		mainLibrary := libraryMap[library.Title]
		if mainLibrary == "" {
			stats.UnmappedLibraries = append(stats.UnmappedLibraries, library.Title)
			continue
		}
		if otherByMain[mainLibrary] == nil {
			otherByMain[mainLibrary] = map[string]models.MediaItem{}
		}
		for _, item := range library.Items {
			otherByMain[mainLibrary][itemKey(item)] = item
		}
	}

	var onlyInMain, onlyInServer []ItemRef
	for _, library := range main {
		otherItems, mapped := otherByMain[library.Title]
		if !mapped {
			continue
		}
		mainKeys := map[string]bool{}
		for _, item := range library.Items {
			mainKeys[itemKey(item)] = true
			if _, found := otherItems[itemKey(item)]; found {
				stats.Matched++
			} else {
				onlyInMain = append(onlyInMain, ref(item, library.Title))
			}
		}
		for key, item := range otherItems {
			if !mainKeys[key] {
				onlyInServer = append(onlyInServer, ref(item, library.Title))
			}
		}
	}

	byTitle := func(items []ItemRef) []ItemRef {
		sort.Slice(items, func(i, j int) bool {
			if !strings.EqualFold(items[i].Title, items[j].Title) {
				return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
			}
			return items[i].Year < items[j].Year
		})
		if len(items) > comparisonListLimit {
			items = items[:comparisonListLimit]
		}
		if items == nil {
			items = []ItemRef{}
		}
		return items
	}
	stats.OnlyInMainCount, stats.OnlyInServerCount = len(onlyInMain), len(onlyInServer)
	stats.OnlyInMain, stats.OnlyInServer = byTitle(onlyInMain), byTitle(onlyInServer)
	sort.Strings(stats.UnmappedLibraries)
	return stats
}
