package autodownload

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"testing"
	"time"
)

// withImages gives a test set images of the types, for the item
func withImages(set models.SetRef, tmdbID string, imageTypes ...string) models.SetRef {
	for _, imageType := range imageTypes {
		image := models.ImageFile{ID: set.ID + "-" + imageType, Type: imageType, ItemTMDB_ID: tmdbID}
		if imageType == "season_poster" || imageType == "titlecard" {
			image.SeasonNumber = new(int)
			*image.SeasonNumber = 1
		}
		set.Images = append(set.Images, image)
	}
	return set
}

func setupUpgradeTest(t *testing.T, creatorSets map[string]models.CreatorSetsResponse, saved *[]models.DBSavedItem) *[]models.DBSavedItem {
	t.Helper()
	queued := []models.DBSavedItem{}
	previous := struct {
		sets   func(context.Context, string) (models.CreatorSetsResponse, logging.LogErrorInfo)
		follow func(context.Context) ([]models.MediuxUserInfo, logging.LogErrorInfo)
		jobs   func(context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo)
		add    func(context.Context, models.DBSavedItem) logging.LogErrorInfo
		saved  func(context.Context, models.DBFilter) (database.PagedSavedItems, logging.LogErrorInfo)
		prefs  config.Config_UserPreferences
	}{getCreatorSets, getFollowing, getQueueJobs, addToQueue, getSavedSets, config.Current.UserPreferences}
	t.Cleanup(func() {
		getCreatorSets, getFollowing, getQueueJobs, addToQueue, getSavedSets = previous.sets, previous.follow, previous.jobs, previous.add, previous.saved
		config.Current.UserPreferences = previous.prefs
	})
	getCreatorSets = func(ctx context.Context, username string) (models.CreatorSetsResponse, logging.LogErrorInfo) {
		return creatorSets[username], logging.LogErrorInfo{}
	}
	getFollowing = func(ctx context.Context) ([]models.MediuxUserInfo, logging.LogErrorInfo) {
		return nil, logging.LogErrorInfo{}
	}
	getQueueJobs = func(ctx context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo) {
		return []database.DownloadQueueJob{{TMDB_ID: "5", LibraryTitle: "Movies", Status: "pending"}}, logging.LogErrorInfo{}
	}
	addToQueue = func(ctx context.Context, item models.DBSavedItem) logging.LogErrorInfo {
		queued = append(queued, item)
		return logging.LogErrorInfo{}
	}
	getSavedSets = func(ctx context.Context, _ models.DBFilter) (database.PagedSavedItems, logging.LogErrorInfo) {
		return database.PagedSavedItems{Items: *saved}, logging.LogErrorInfo{}
	}
	config.Current.UserPreferences = config.Config_UserPreferences{
		AutoUpgradeSets:  true,
		FavoriteCreators: []string{"first", "second"},
		DownloadDefaults: []string{"poster", "backdrop", "titlecard"},
	}
	return &queued
}

func savedSet(id, user, setType string, keep bool, types models.SelectedTypes) models.DBPosterSetDetail {
	return models.DBPosterSetDetail{
		PosterSet:     models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: id, UserCreated: user, Type: setType}},
		SelectedTypes: types,
		NoAutoUpgrade: keep,
	}
}

func savedItem(tmdbID, itemType string, sets ...models.DBPosterSetDetail) models.DBSavedItem {
	library := "Movies"
	if itemType == "show" {
		library = "TV Shows"
	}
	return models.DBSavedItem{MediaItem: models.MediaItem{TMDB_ID: tmdbID, Type: itemType, Title: "Item " + tmdbID, LibraryTitle: library}, PosterSets: sets}
}

func queuedByID(queued []models.DBSavedItem) map[string]models.DBSavedItem {
	got := map[string]models.DBSavedItem{}
	for _, item := range queued {
		got[item.MediaItem.TMDB_ID] = item
	}
	return got
}

func TestUpgradeSavedSets(t *testing.T) {
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	posterAndBackdrop := models.SelectedTypes{Poster: true, Backdrop: true}
	creatorSets := map[string]models.CreatorSetsResponse{
		"first": {MovieSets: []models.SetRef{
			withImages(testSet("first-1", "first", "movie", 0, day, "1"), "1", "poster", "backdrop"),
			withImages(testSet("first-3", "first", "movie", 0, day, "3"), "3", "poster", "backdrop"),
			withImages(testSet("first-4", "first", "movie", 0, day, "4"), "4", "poster"),
		}},
		"second": {MovieSets: []models.SetRef{
			withImages(withImages(withImages(testSet("second-2", "second", "movie", 0, day, "2", "5", "6"), "2", "poster", "backdrop"), "5", "poster"), "6", "poster"),
		}},
	}
	saved := []models.DBSavedItem{
		savedItem("1", "movie", savedSet("old-1", "second", "movie", false, posterAndBackdrop)), // A higher ranked favorite has a set
		savedItem("2", "movie", // A favorite beats creators who aren't favorites, for each type
			savedSet("old-2", "someone", "movie", false, models.SelectedTypes{Poster: true}),
			savedSet("old-2b", "else", "movie", false, models.SelectedTypes{Backdrop: true})),
		savedItem("3", "movie", savedSet("first-3", "first", "movie", false, posterAndBackdrop)), // Already the best ranked creator
		savedItem("4", "movie", savedSet("old-4", "someone", "movie", true, posterAndBackdrop)),  // Auto Upgrade off
		savedItem("5", "movie", savedSet("old-5", "someone", "movie", false, posterAndBackdrop)), // Already in the Download Queue
		savedItem("6", "movie", savedSet("old-6", "first", "movie", false, posterAndBackdrop)),   // Saved from a higher ranked creator than the best available
	}
	queued := setupUpgradeTest(t, creatorSets, &saved)

	result, Err := UpgradeSavedSets(testContext())
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if result != (UpgradeSavedSetsResult{Checked: 5, Upgraded: 2}) {
		t.Errorf("result = %+v", result)
	}
	got := queuedByID(*queued)
	if len(got) != 2 || len(got["1"].PosterSets) != 1 || got["1"].PosterSets[0].ID != "first-1" || len(got["2"].PosterSets) != 1 || got["2"].PosterSets[0].ID != "second-2" {
		t.Fatalf("queued = %+v", *queued)
	}
	// The types the new set has, with Auto Download on. Saving it clears them on the old sets.
	if set := got["1"].PosterSets[0]; !set.AutoDownload || set.SelectedTypes != posterAndBackdrop || got["1"].Source != models.SourceAutoUpgrade || len(got["1"].ReplaceSetIDs) != 0 {
		t.Errorf("upgraded item = %+v", got["1"])
	}
	if got["2"].PosterSets[0].SelectedTypes != posterAndBackdrop {
		t.Errorf("both types move to the favorite, got %+v", got["2"].PosterSets[0].SelectedTypes)
	}

	// Nothing happens while Automatically Upgrade Sets is off
	config.Current.UserPreferences.AutoUpgradeSets = false
	*queued = nil
	if result, _ := UpgradeSavedSets(testContext()); result.Checked != 0 || len(*queued) != 0 {
		t.Errorf("with upgrades off: %+v, queued %d", result, len(*queued))
	}

	// Chosen items are upgraded even with the switch off, and only those
	result, Err = UpgradeSets(testContext(), []UpgradeItem{{TMDB_ID: "2", LibraryTitle: "Movies"}, {TMDB_ID: "4", LibraryTitle: "Movies"}})
	if Err.Message != "" || result != (UpgradeSavedSetsResult{Checked: 2, Upgraded: 1}) || len(*queued) != 1 || (*queued)[0].MediaItem.TMDB_ID != "2" {
		t.Errorf("chosen items: %+v %s, queued %+v", result, Err.Message, *queued)
	}
}

func TestUpgradeSavedSetsKeepsTypesTheFavoriteDoesntHave(t *testing.T) {
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	firstShow := withImages(testSet("first-10", "first", "show", 0, day, "10"), "10", "poster")
	creatorSets := map[string]models.CreatorSetsResponse{"first": {ShowSets: []models.SetRef{firstShow}}}
	// Titlecards saved from a creator who isn't a favorite
	saved := []models.DBSavedItem{savedItem("10", "show", savedSet("old-10", "someone", "show", false, models.SelectedTypes{Titlecard: true}))}
	queued := setupUpgradeTest(t, creatorSets, &saved)

	// The favorite only has a poster, so only the poster is added and the titlecards stay with the saved set
	if _, Err := UpgradeSavedSets(testContext()); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	got := queuedByID(*queued)
	if len(got["10"].PosterSets) != 1 || got["10"].PosterSets[0].ID != "first-10" || got["10"].PosterSets[0].SelectedTypes != (models.SelectedTypes{Poster: true}) {
		t.Fatalf("queued = %+v", *queued)
	}

	// Once the poster is saved nothing changes
	saved = []models.DBSavedItem{savedItem("10", "show",
		savedSet("old-10", "someone", "show", false, models.SelectedTypes{Titlecard: true}),
		savedSet("first-10", "first", "show", false, models.SelectedTypes{Poster: true}))}
	*queued = nil
	if result, _ := UpgradeSavedSets(testContext()); result.Upgraded != 0 {
		t.Errorf("nothing should change, queued %+v", *queued)
	}

	// When the favorite adds titlecards later, they move to the favorite's set, which keeps its poster
	creatorSets["first"] = models.CreatorSetsResponse{ShowSets: []models.SetRef{withImages(firstShow, "10", "titlecard")}}
	if _, Err := UpgradeSavedSets(testContext()); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	got = queuedByID(*queued)
	if len(got["10"].PosterSets) != 1 || got["10"].PosterSets[0].SelectedTypes != (models.SelectedTypes{Poster: true, Titlecard: true}) {
		t.Fatalf("queued = %+v", *queued)
	}
}
