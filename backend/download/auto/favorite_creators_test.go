package autodownload

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"slices"
	"testing"
	"time"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

func testSet(id, user, setType string, popularity int, updated time.Time, items ...string) models.SetRef {
	return models.SetRef{
		PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: id, Title: id, Type: setType, UserCreated: user, PopularityGlobal: popularity, DateUpdated: updated}},
		ItemIDs:   items,
	}
}

func TestDownloadFromFavoriteCreators(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	creatorSets := map[string]models.CreatorSetsResponse{
		"first": {MovieSets: []models.SetRef{
			testSet("first-unranked-new", "first", "movie", 0, day(9), "1"),
			testSet("first-rank-50", "first", "movie", 50, day(1), "1"),
			testSet("first-rank-10", "first", "movie", 10, day(2), "1"),
		}},
		"second": {
			MovieSets: []models.SetRef{testSet("second-movie", "second", "movie", 1, day(1), "1", "2")},
			ShowSets:  []models.SetRef{testSet("second-show", "second", "show", 0, day(1), "3")},
		},
		"followed": {MovieSets: []models.SetRef{testSet("followed-movie", "followed", "movie", 0, day(1), "4")}},
		"hidden":   {MovieSets: []models.SetRef{testSet("hidden-movie", "hidden", "movie", 0, day(1), "5")}},
	}

	queued := []models.DBSavedItem{}
	previous := struct {
		sets   func(context.Context, string) (models.CreatorSetsResponse, logging.LogErrorInfo)
		follow func(context.Context) ([]models.MediuxUserInfo, logging.LogErrorInfo)
		jobs   func(context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo)
		add    func(context.Context, models.DBSavedItem) logging.LogErrorInfo
		prefs  config.Config_UserPreferences
	}{getCreatorSets, getFollowing, getQueueJobs, addToQueue, config.Current.UserPreferences}
	t.Cleanup(func() {
		getCreatorSets, getFollowing, getQueueJobs, addToQueue = previous.sets, previous.follow, previous.jobs, previous.add
		config.Current.UserPreferences = previous.prefs
	})
	getCreatorSets = func(ctx context.Context, username string) (models.CreatorSetsResponse, logging.LogErrorInfo) {
		return creatorSets[username], logging.LogErrorInfo{}
	}
	getFollowing = func(ctx context.Context) ([]models.MediuxUserInfo, logging.LogErrorInfo) {
		return []models.MediuxUserInfo{{Username: "followed", Follow: true}, {Username: "hidden", Hide: true}}, logging.LogErrorInfo{}
	}
	getQueueJobs = func(ctx context.Context) ([]database.DownloadQueueJob, logging.LogErrorInfo) {
		return []database.DownloadQueueJob{{TMDB_ID: "6", LibraryTitle: "Movies", Status: "pending"}}, logging.LogErrorInfo{}
	}
	addToQueue = func(ctx context.Context, item models.DBSavedItem) logging.LogErrorInfo {
		queued = append(queued, item)
		return logging.LogErrorInfo{}
	}
	config.Current.UserPreferences = config.Config_UserPreferences{
		AutoDownloadNewItems: true,
		FavoriteCreators:     []string{"first", "second", "hidden"},
		DownloadDefaults:     []string{"poster", "backdrop"},
	}

	items := []models.MediaItem{
		{TMDB_ID: "1", Type: "movie", Title: "Both creators", LibraryTitle: "Movies"},
		{TMDB_ID: "2", Type: "movie", Title: "Second only", LibraryTitle: "Movies"},
		{TMDB_ID: "3", Type: "show", Title: "Show", LibraryTitle: "Series"},
		{TMDB_ID: "4", Type: "movie", Title: "Followed only", LibraryTitle: "Movies"},
		{TMDB_ID: "5", Type: "movie", Title: "Hidden only", LibraryTitle: "Movies"},
		{TMDB_ID: "6", Type: "movie", Title: "Already queued", LibraryTitle: "Movies"},
		{TMDB_ID: "7", Type: "movie", Title: "Saved", LibraryTitle: "Movies", DBSavedSets: []models.DBSavedSet{{ID: "set"}}},
		{TMDB_ID: "8", Type: "movie", Title: "Nobody", LibraryTitle: "Movies"},
	}
	result, Err := DownloadFromFavoriteCreators(testContext(), items, models.SourceFavoriteCreatorsNewItem)
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}

	got := []string{}
	for _, item := range queued {
		got = append(got, item.MediaItem.Title+": "+item.PosterSets[0].ID)
	}
	want := []string{
		"Both creators: first-rank-10", // the best ranked creator, then their best ranked set
		"Second only: second-movie",
		"Show: second-show",
		"Followed only: followed-movie", // followed creators are favorites too
	}
	if !slices.Equal(got, want) {
		t.Errorf("queued = %q, want %q", got, want)
	}
	// Hidden and nobody have no set from a favorite, already queued and saved aren't checked
	if result != (FavoriteCreatorsResult{Checked: 6, Queued: 4, NoSet: 2}) {
		t.Errorf("result = %+v", result)
	}
	if queued[0].Source != models.SourceFavoriteCreatorsNewItem {
		t.Errorf("source = %q, want a new item from a favorite creator", queued[0].Source)
	}
	if selected := queued[0].PosterSets[0].SelectedTypes; !selected.Poster || !selected.Backdrop || selected.Titlecard {
		t.Errorf("selected types = %+v, want the Download Defaults", selected)
	}

	// Nothing happens while it's off
	queued = nil
	config.Current.UserPreferences.AutoDownloadNewItems = false
	if result, _ := DownloadFromFavoriteCreators(testContext(), items, models.SourceFavoriteCreatorsNewItem); result.Queued != 0 || len(queued) != 0 {
		t.Errorf("with the setting off %d were queued", len(queued))
	}
}

func TestBestSetUnrankedGoLastNewestFirst(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	byItem := map[string][]models.SetRef{"1": {
		testSet("old", "a", "movie", 0, day(1), "1"),
		testSet("new", "a", "movie", 0, day(5), "1"),
	}}
	set, _, found := bestSet(models.MediaItem{TMDB_ID: "1", Type: "movie"}, []map[string][]models.SetRef{nil, byItem}, nil)
	if !found || set.ID != "new" {
		t.Errorf("best set = %q, %v, want the newest of the unranked sets", set.ID, found)
	}
}
