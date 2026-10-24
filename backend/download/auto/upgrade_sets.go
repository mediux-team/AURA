package autodownload

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
	"slices"
	"strings"
)

// Set in tests
var getSavedSets = database.GetAllSavedSets

// UpgradeSavedSetsResult is what a run did
type UpgradeSavedSetsResult struct {
	Checked  int `json:"checked"`  // Saved items that can be upgraded
	Upgraded int `json:"upgraded"` // Items a better set was added to the Download Queue for
}

// UpgradeItem is a saved item to upgrade
type UpgradeItem struct {
	TMDB_ID      string `json:"tmdb_id"`
	LibraryTitle string `json:"library_title"`
	Edition      string `json:"edition"`
}

// UpgradeSavedSets upgrades every saved set, while Automatically Upgrade Sets is on
func UpgradeSavedSets(ctx context.Context) (result UpgradeSavedSetsResult, Err logging.LogErrorInfo) {
	if !config.Current.UserPreferences.AutoUpgradeSets {
		_, logAction := logging.AddSubActionToContext(ctx, "Upgrading Saved Sets", logging.LevelInfo)
		logAction.AppendResult("skipped", "Automatically Upgrade Sets is off")
		logAction.Complete()
		return result, logging.LogErrorInfo{}
	}
	return UpgradeSets(ctx, nil)
}

// UpgradeSets moves the image types of saved sets to a set from a higher ranked favorite creator, for the items given or every saved item.
// Any favorite creator ranks above a creator who isn't one. Each image type moves on its own, to the best ranked creator with a set
// that has it, so the types a creator doesn't have stay with the saved set. A set with Auto Upgrade off keeps its types.
// The Download Defaults the item doesn't have yet are added too. The new sets have Auto Download on.
// Saving them clears the types on the other sets, and a set with no types left is removed.
func UpgradeSets(ctx context.Context, only []UpgradeItem) (result UpgradeSavedSetsResult, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Upgrading Saved Sets", logging.LevelInfo)
	defer logAction.Complete()

	favoriteCreatorsMu.Lock()
	defer favoriteCreatorsMu.Unlock()

	prefs := config.Current.UserPreferences
	favorites := favoriteCreators(ctx, prefs.FavoriteCreators)
	if len(favorites) == 0 {
		logAction.AppendResult("skipped", "there are no favorite creators")
		return result, logging.LogErrorInfo{}
	}

	saved, Err := getSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
	if Err.Message != "" {
		return result, Err
	}
	setsByRank := favoriteSetsByRank(ctx, logAction, favorites)
	queued := queuedItems(ctx)

	// rankOf is the creator's rank from 0, a creator who isn't a favorite ranks last
	rankOf := func(username string) int {
		for rank, favorite := range favorites {
			if strings.EqualFold(favorite, username) {
				return rank
			}
		}
		return len(favorites)
	}

	wanted := map[string]bool{}
	for _, item := range only {
		wanted[item.TMDB_ID+"|"+item.LibraryTitle+"|"+item.Edition] = true
	}

	defaults := DownloadDefaultTypes()
	for _, item := range saved.Items {
		if len(only) > 0 && !wanted[item.MediaItem.TMDB_ID+"|"+item.MediaItem.LibraryTitle+"|"+item.MediaItem.Edition] {
			continue
		}
		if len(item.PosterSets) == 0 || queued[item.MediaItem.TMDB_ID+"|"+item.MediaItem.LibraryTitle+"|"+item.MediaItem.Edition] {
			continue
		}
		result.Checked++

		upgrades := upgradeItemTypes(item, defaults.ForSetType(item.MediaItem.Type), setsByRank, rankOf)
		if len(upgrades) == 0 {
			continue
		}

		Err := addToQueue(ctx, models.DBSavedItem{
			MediaItem:  item.MediaItem,
			PosterSets: upgrades,
			Source:     models.SourceAutoUpgrade,
		})
		if Err.Message != "" {
			logAction.AppendWarning(fmt.Sprintf("%s_queue", item.MediaItem.Title), Err.Message)
			continue
		}
		result.Upgraded++
		described := []string{}
		for _, set := range upgrades {
			described = append(described, fmt.Sprintf("%s by %s for %s", set.Title, set.UserCreated, strings.Join(typeNames(set.SelectedTypes), ", ")))
		}
		logAction.AppendResult(item.MediaItem.Title, strings.Join(described, "; "))
	}

	logAction.AppendResult("checked", result.Checked)
	logAction.AppendResult("upgraded", result.Upgraded)
	return result, logging.LogErrorInfo{}
}

// upgradeItemTypes returns the sets to save for the image types that a higher ranked favorite creator has.
// A set that's already saved keeps its other types.
func upgradeItemTypes(item models.DBSavedItem, defaults models.SelectedTypes, setsByRank []map[string][]models.SetRef,
	rankOf func(username string) int) []models.DBPosterSetDetail {
	// The creator rank of each type the item has, and the types of sets with Auto Upgrade off
	heldRank := map[string]int{}
	kept := map[string]bool{}
	for _, set := range item.PosterSets {
		for _, imageType := range typeNames(set.SelectedTypes) {
			heldRank[imageType] = rankOf(set.UserCreated)
			kept[imageType] = kept[imageType] || set.NoAutoUpgrade
		}
	}

	// The Download Defaults the item doesn't have yet rank last, so any favorite creator's set adds them
	for _, imageType := range typeNames(defaults) {
		if _, held := heldRank[imageType]; !held {
			heldRank[imageType] = rankOf("")
		}
	}

	upgrades := []models.DBPosterSetDetail{}
	for _, imageType := range allImageTypes {
		rank, held := heldRank[imageType]
		if !held || kept[imageType] {
			continue
		}
		best, bestRank, found := bestSet(item.MediaItem, setsByRank, func(set models.SetRef) bool {
			return slices.Contains(typeNames(setImageTypes(item.MediaItem, set.PosterSet)), imageType)
		})
		if !found || bestRank >= rank {
			continue
		}

		i := slices.IndexFunc(upgrades, func(set models.DBPosterSetDetail) bool { return set.ID == best.ID })
		if i == -1 {
			upgrade := models.DBPosterSetDetail{AutoDownload: true}
			// A set that's already saved keeps its types and settings
			if savedIndex := slices.IndexFunc(item.PosterSets, func(set models.DBPosterSetDetail) bool { return set.ID == best.ID }); savedIndex != -1 {
				upgrade = item.PosterSets[savedIndex]
			}
			upgrade.PosterSet = best.PosterSet
			upgrades = append(upgrades, upgrade)
			i = len(upgrades) - 1
		}
		upgrades[i].SelectedTypes = selectedTypes(append(typeNames(upgrades[i].SelectedTypes), imageType))
	}
	return upgrades
}

// allImageTypes is every image type a set can have, in the order they're applied
var allImageTypes = []string{"poster", "backdrop", "season_poster", "special_season_poster", "titlecard"}

// typeNames is the names of the selected image types
func typeNames(types models.SelectedTypes) []string {
	selected := map[string]bool{
		"poster":                types.Poster,
		"backdrop":              types.Backdrop,
		"season_poster":         types.SeasonPoster,
		"special_season_poster": types.SpecialSeasonPoster,
		"titlecard":             types.Titlecard,
	}
	names := []string{}
	for _, imageType := range allImageTypes {
		if selected[imageType] {
			names = append(names, imageType)
		}
	}
	return names
}

// setImageTypes is the image types a set has for the item
func setImageTypes(item models.MediaItem, set models.PosterSet) (types models.SelectedTypes) {
	for _, image := range set.Images {
		if image.ItemTMDB_ID != "" && image.ItemTMDB_ID != item.TMDB_ID {
			continue
		}
		switch image.Type {
		case "poster":
			types.Poster = true
		case "backdrop":
			types.Backdrop = true
		case "season_poster":
			if image.SeasonNumber != nil && *image.SeasonNumber == 0 {
				types.SpecialSeasonPoster = true
			} else {
				types.SeasonPoster = true
			}
		case "titlecard":
			types.Titlecard = true
		}
	}
	return types.ForSetType(set.Type)
}
