package autodownload

import (
	"aura/config"
	"aura/creator"
	"aura/database"
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Set in tests
var (
	getCreatorSets = creator.Sets
	getFollowing   = mediux.GetUserFollowingAndHiding
	getQueueJobs   = database.GetAllDownloadQueueJobs
	addToQueue     = downloadqueue.AddToQueue
)

// FavoriteCreatorsResult is what a run did
type FavoriteCreatorsResult struct {
	Checked int `json:"checked"` // Items without a saved set
	Queued  int `json:"queued"`  // Items a set was added to the Download Queue for
	NoSet   int `json:"no_set"`  // Items none of the favorite creators have a set for
}

// favoriteCreatorsMu makes sure only one run goes at a time, so an item isn't queued twice
var favoriteCreatorsMu sync.Mutex

// DownloadFromFavoriteCreators queues the best set from a favorite creator for each item without a saved set
func DownloadFromFavoriteCreators(ctx context.Context, items []models.MediaItem, source string) (result FavoriteCreatorsResult, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Downloading Sets from Favorite Creators", logging.LevelInfo)
	defer logAction.Complete()

	favoriteCreatorsMu.Lock()
	defer favoriteCreatorsMu.Unlock()

	prefs := config.Current.UserPreferences
	if !prefs.AutoDownloadNewItems {
		logAction.AppendResult("skipped", "Automatically Download Images for New Items is off")
		return result, logging.LogErrorInfo{}
	}
	favorites := favoriteCreators(ctx, prefs.FavoriteCreators)
	if len(favorites) == 0 {
		logAction.AppendResult("skipped", "there are no favorite creators")
		return result, logging.LogErrorInfo{}
	}

	setsByRank := favoriteSetsByRank(ctx, logAction, favorites)
	queued := queuedItems(ctx)

	selected := selectedTypes(prefs.DownloadDefaults)
	for _, item := range items {
		if item.TMDB_ID == "" || len(item.DBSavedSets) > 0 || item.IgnoredInDB || queued[item.TMDB_ID+"|"+item.LibraryTitle+"|"+item.Edition] {
			continue
		}
		result.Checked++

		set, _, found := bestSet(item, setsByRank, nil)
		if !found {
			result.NoSet++
			continue
		}
		Err := addToQueue(ctx, models.DBSavedItem{
			MediaItem: item,
			PosterSets: []models.DBPosterSetDetail{{
				PosterSet:     set.PosterSet,
				SelectedTypes: selected,
				AutoDownload:  true,
			}},
			Source: source,
		})
		if Err.Message != "" {
			logAction.AppendWarning(fmt.Sprintf("%s_queue", item.Title), Err.Message)
			continue
		}
		result.Queued++
		logAction.AppendResult(item.Title, fmt.Sprintf("%s by %s", set.Title, set.UserCreated))
	}

	logAction.AppendResult("checked", result.Checked)
	logAction.AppendResult("queued", result.Queued)
	logAction.AppendResult("no_set", result.NoSet)
	return result, logging.LogErrorInfo{}
}

// FavoriteCreators is the favorite creators from the User Preferences, then followed creators, without hidden ones
func FavoriteCreators(ctx context.Context) []string {
	return favoriteCreators(ctx, config.Current.UserPreferences.FavoriteCreators)
}

// favoriteCreators is the ranked favorites, then followed creators not in the list, without hidden ones
func favoriteCreators(ctx context.Context, ranked []string) []string {
	favorites := slices.Clone(ranked)
	users, Err := getFollowing(ctx)
	if Err.Message != "" {
		return favorites
	}
	hidden := map[string]bool{}
	for _, user := range users {
		if user.Hide {
			hidden[strings.ToLower(user.Username)] = true
		}
	}
	for _, user := range users {
		if user.Follow && !user.Hide && !slices.ContainsFunc(favorites, func(name string) bool { return strings.EqualFold(name, user.Username) }) {
			favorites = append(favorites, user.Username)
		}
	}
	return slices.DeleteFunc(favorites, func(name string) bool { return hidden[strings.ToLower(name)] })
}

// favoriteSetsByRank is each favorite creator's sets, by the TMDB ID of the items, in rank order
func favoriteSetsByRank(ctx context.Context, logAction *logging.LogAction, favorites []string) []map[string][]models.SetRef {
	setsByRank := make([]map[string][]models.SetRef, 0, len(favorites))
	for _, username := range favorites {
		sets, Err := getCreatorSets(ctx, username)
		if Err.Message != "" {
			logAction.AppendWarning(fmt.Sprintf("%s_sets", username), Err.Message)
			setsByRank = append(setsByRank, nil)
			continue
		}
		byItem := map[string][]models.SetRef{}
		for _, set := range append(slices.Clone(sets.MovieSets), sets.ShowSets...) {
			for _, id := range set.ItemIDs {
				byItem[id] = append(byItem[id], set)
			}
		}
		setsByRank = append(setsByRank, byItem)
	}
	return setsByRank
}

// queuedItems is the items waiting in the Download Queue
func queuedItems(ctx context.Context) map[string]bool {
	queued := map[string]bool{}
	jobs, _ := getQueueJobs(ctx)
	for _, job := range jobs {
		if job.Status == "pending" || job.Status == "processing" {
			queued[job.TMDB_ID+"|"+job.LibraryTitle+"|"+job.Edition] = true
		}
	}
	return queued
}

// bestSet returns the best ranked creator's most popular set for the item, unranked sets last, and the creator's rank from 0.
// has, when given, keeps only the sets it returns true for.
func bestSet(item models.MediaItem, setsByRank []map[string][]models.SetRef, has func(set models.SetRef) bool) (models.SetRef, int, bool) {
	for rank, byItem := range setsByRank {
		candidates := []models.SetRef{}
		for _, set := range byItem[item.TMDB_ID] {
			if set.Type == item.Type && (has == nil || has(set)) {
				candidates = append(candidates, set)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		slices.SortStableFunc(candidates, func(a, b models.SetRef) int {
			if byRank := comparePopularity(a.PopularityGlobal, b.PopularityGlobal); byRank != 0 {
				return byRank
			}
			return b.DateUpdated.Compare(a.DateUpdated)
		})
		return candidates[0], rank, true
	}
	return models.SetRef{}, 0, false
}

// comparePopularity puts the most popular first and the sets that aren't ranked last
func comparePopularity(a, b int) int {
	switch {
	case a == b:
		return 0
	case a == 0:
		return 1
	case b == 0:
		return -1
	default:
		return a - b
	}
}

// DownloadDefaultTypes is the image types in the Download Defaults
func DownloadDefaultTypes() models.SelectedTypes {
	return selectedTypes(config.Current.UserPreferences.DownloadDefaults)
}

// selectedTypes is the image types to download, from the Download Defaults
func selectedTypes(downloadDefaults []string) models.SelectedTypes {
	return models.SelectedTypes{
		Poster:              slices.Contains(downloadDefaults, "poster"),
		Backdrop:            slices.Contains(downloadDefaults, "backdrop"),
		SeasonPoster:        slices.Contains(downloadDefaults, "season_poster"),
		SpecialSeasonPoster: slices.Contains(downloadDefaults, "special_season_poster"),
		Titlecard:           slices.Contains(downloadDefaults, "titlecard"),
	}
}
