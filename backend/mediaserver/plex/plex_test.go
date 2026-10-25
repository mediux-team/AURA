package plex

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

func num(n int) *int { return &n }

// fakePlex is a Plex server with a Movies library. It records the requests that change something.
type fakePlex struct {
	mu      sync.Mutex
	changes []string
	tokens  []string
}

func newFakePlex(t *testing.T) (*fakePlex, *Plex) {
	t.Helper()
	f := &fakePlex{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		f.record(r, false)
		_, _ = w.Write([]byte(`{"MediaContainer":{"version":"1.40","friendlyName":"Home Plex"}}`))
	})
	mux.HandleFunc("GET /library/sections/all", func(w http.ResponseWriter, r *http.Request) {
		f.record(r, false)
		_, _ = w.Write([]byte(`{"MediaContainer":{"Directory":[
			{"key":"1","type":"movie","title":"Movies","Location":[{"path":"/data/movies"}]},
			{"key":"2","type":"artist","title":"Music"}]}}`))
	})
	mux.HandleFunc("GET /library/sections/1/all", func(w http.ResponseWriter, r *http.Request) {
		f.record(r, false)
		_, _ = w.Write([]byte(`{"MediaContainer":{"title1":"Movies","totalSize":2,"Metadata":[
			{"ratingKey":"100","type":"movie","title":"Movie","year":2020,"guid":"plex://movie/1",
			 "Guid":[{"id":"imdb://tt1"},{"id":"tmdb://603"}],"Media":[{"Part":[{"file":"/data/movies/Movie.mkv","size":10}]}]},
			{"ratingKey":"101","type":"movie","title":"No TMDB","Media":[{"Part":[{"file":"/data/movies/Other.mkv"}]}]}]}}`))
	})
	mux.HandleFunc("POST /library/metadata/{key}/{type}", func(w http.ResponseWriter, r *http.Request) {
		f.record(r, true)
	})
	mux.HandleFunc("PUT /:/rate", func(w http.ResponseWriter, r *http.Request) {
		f.record(r, true)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// Not the main media server, so nothing is saved locally or in the main cache
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	config.Current.MainMediaServer = "main"
	return f, &Plex{Config: config.Config_MediaServer{ID: "other", Type: "Plex", URL: server.URL, ApiToken: "plex-token"}}
}

func (f *fakePlex) record(r *http.Request, change bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens = append(f.tokens, r.Header.Get("X-Plex-Token"))
	if change {
		f.changes = append(f.changes, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
	}
}

func TestPlexConnectionAndLibraries(t *testing.T) {
	fake, p := newFakePlex(t)

	ok, name, version, Err := p.TestConnection(testContext(), p.Config)
	if !ok || name != "Home Plex" || version != "1.40" || Err.Message != "" {
		t.Errorf("TestConnection = %v, %q, %q, %s", ok, name, version, Err.Message)
	}

	sections, Err := p.GetLibrarySections(testContext(), p.Config)
	if Err.Message != "" || len(sections) != 1 || sections[0].Title != "Movies" || sections[0].ID != "1" || sections[0].Paths[0] != "/data/movies" {
		t.Errorf("GetLibrarySections = %+v, %s (only movie and show libraries)", sections, Err.Message)
	}

	items, total, fetched, Err := p.GetLibrarySectionItems(testContext(), sections[0], "0", "")
	if Err.Message != "" || len(items) != 1 || total != 2 || fetched != 2 {
		t.Fatalf("GetLibrarySectionItems = %d items, total %d, fetched %d, %s (the item without a TMDB ID is skipped, but still counted)", len(items), total, fetched, Err.Message)
	}
	if item := items[0]; item.TMDB_ID != "603" || item.RatingKey != "100" || item.LibraryTitle != "Movies" || item.Movie == nil || item.Movie.File.Path != "/data/movies/Movie.mkv" {
		t.Errorf("item = %+v", item)
	}

	for _, token := range fake.tokens {
		if token != "plex-token" {
			t.Fatalf("every request should send the Plex token, got %q", token)
		}
	}
}

func TestPlexApplyImageAndRate(t *testing.T) {
	fake, p := newFakePlex(t)
	show := &models.MediaItem{Type: "show", Title: "Show", RatingKey: "200", Series: &models.MediaItemSeries{Seasons: []models.MediaItemSeason{
		{SeasonNumber: 1, RatingKey: "201", Episodes: []models.MediaItemEpisode{{SeasonNumber: 1, EpisodeNumber: 2, RatingKey: "203"}}},
	}}}

	images := []models.ImageFile{
		{ID: "poster", Type: "poster"},
		{ID: "backdrop", Type: "backdrop"},
		{ID: "season", Type: "season_poster", SeasonNumber: num(1)},
		{ID: "card", Type: "titlecard", SeasonNumber: num(1), EpisodeNumber: num(2)},
	}
	for _, image := range images {
		if Err := p.DownloadApplyImageToMediaItem(testContext(), show, image); Err.Message != "" {
			t.Fatalf("applying the %s: %s", image.Type, Err.Message)
		}
	}
	// A season the show doesn't have can't be applied
	if Err := p.DownloadApplyImageToMediaItem(testContext(), show, models.ImageFile{ID: "x", Type: "season_poster", SeasonNumber: num(9)}); Err.Message == "" {
		t.Error("an image for a missing season should fail")
	}

	if Err := p.RateMediaItem(testContext(), &models.MediaItem{RatingKey: "100"}, 8); Err.Message != "" {
		t.Fatalf("RateMediaItem: %s", Err.Message)
	}

	want := []string{
		"POST /library/metadata/200/posters?url=",
		"POST /library/metadata/200/arts?url=",
		"POST /library/metadata/201/posters?url=",
		"POST /library/metadata/203/posters?url=",
		"PUT /:/rate?identifier=com.plexapp.plugins.library&key=100&rating=8.0",
	}
	if len(fake.changes) != len(want) {
		t.Fatalf("changes = %v, want %v", fake.changes, want)
	}
	for i, change := range fake.changes {
		if !strings.HasPrefix(change, want[i]) {
			t.Errorf("change %d = %q, want it to start with %q", i, change, want[i])
		}
	}
	// The image is applied from its MediUX URL
	if !strings.Contains(fake.changes[0], "url=https%3A%2F%2Fimages.mediux.io%2Fassets%2Fposter") {
		t.Errorf("the poster should be applied from MediUX, got %q", fake.changes[0])
	}
}

func TestAddPlexHeaders(t *testing.T) {
	headers := AddPlexHeaders(config.Config_MediaServer{ApiToken: "abc"}, map[string]string{})
	if headers["X-Plex-Token"] != "abc" || headers["Accept"] != "application/json" || headers["X-Plex-Product"] != "aura" {
		t.Errorf("headers = %v", headers)
	}
}

func TestPlexEpisodesWithoutAFile(t *testing.T) {
	// An episode without media is skipped instead of failing the whole show
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"e1","parentIndex":1,"index":1,"Media":[{"Part":[{"file":"/tv/Show/S01E01.mkv","size":5,"duration":7}]}]},
			{"ratingKey":"e2","parentIndex":1,"index":2},
			{"ratingKey":"e3","parentIndex":1,"index":3,"Media":[{"Part":[]}]}
		]}}`))
	}))
	t.Cleanup(server.Close)

	item := models.MediaItem{RatingKey: "show", Title: "Show"}
	if Err := fetchSeasonsAndEpisodesForShow(testContext(), config.Config_MediaServer{Type: "Plex", URL: server.URL}, &item); Err.Message != "" {
		t.Fatalf("fetchSeasonsAndEpisodesForShow: %s", Err.Message)
	}
	if len(item.Series.Seasons) != 1 || len(item.Series.Seasons[0].Episodes) != 1 {
		t.Fatalf("seasons = %+v, want only episode e1", item.Series.Seasons)
	}
	if file := item.Series.Seasons[0].Episodes[0].File; file != (models.MediaItemFile{Path: "/tv/Show/S01E01.mkv", Size: 5, Duration: 7}) {
		t.Errorf("file = %+v", file)
	}

	// A movie without media has an empty file
	if file := firstFile(nil); file != (models.MediaItemFile{}) {
		t.Errorf("firstFile(nil) = %+v", file)
	}
}

func TestPlexShowCounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"MediaContainer":{"title1":"TV Shows","totalSize":1,"Metadata":[
			{"ratingKey":"200","type":"show","title":"Show","year":2020,"childCount":3,"leafCount":24,"guid":"plex://show/1","Guid":[{"id":"tmdb://1399"}]}]}}`))
	}))
	t.Cleanup(server.Close)
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	config.Current.MainMediaServer = "main"
	p := &Plex{Config: config.Config_MediaServer{ID: "other", Type: "Plex", URL: server.URL}}

	// The library listing has each show's seasons and episodes
	section := models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{ID: "2", Title: "TV Shows", Type: "show"}}
	items, _, _, Err := p.GetLibrarySectionItems(testContext(), section, "0", "")
	if Err.Message != "" || len(items) != 1 || items[0].SeasonCount != 3 || items[0].EpisodeCount != 24 || items[0].Series != nil {
		t.Fatalf("items = %+v, %s", items, Err.Message)
	}
}
