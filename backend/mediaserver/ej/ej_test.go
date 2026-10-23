package ej

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

func num(n int) *int { return &n }

// fakeJellyfin is a Jellyfin server that keeps the images uploaded to it, with a MediUX server for the images
type fakeJellyfin struct {
	mu        sync.Mutex
	requests  []string
	uploads   map[string][]byte // Path to the decoded image
	types     map[string]string // Path to the Content-Type
	backdrops []EmbyJellyItemImagesResponse
	indexErr  string // Moving a backdrop fails with this, like Emby without access to the media folder
}

func newFakeJellyfin(t *testing.T) (*fakeJellyfin, *EJ) {
	t.Helper()
	f := &fakeJellyfin{uploads: map[string][]byte{}, types: map[string]string{}, backdrops: []EmbyJellyItemImagesResponse{{ImageType: "Backdrop", ImageIndex: 0, ImageTag: "old"}}}

	mediuxServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("image " + r.URL.Path))
	}))
	t.Cleanup(mediuxServer.Close)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /System/Info", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		_, _ = w.Write([]byte(`{"Version":"10.9","ServerName":"Home Jellyfin"}`))
	})
	mux.HandleFunc("GET /Items/{key}/Images", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		writeJSON(w, f.backdrops)
	})
	mux.HandleFunc("POST /Items/{key}/Images/{type}", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		body, _ := io.ReadAll(r.Body)
		decoded, _ := base64.StdEncoding.DecodeString(string(body))
		f.mu.Lock()
		defer f.mu.Unlock()
		f.uploads[r.URL.Path] = decoded
		f.types[r.URL.Path] = r.Header.Get("Content-Type")
		// The uploaded backdrop is added after the ones already there
		if r.PathValue("type") == "Backdrop" {
			f.backdrops = append(f.backdrops, EmbyJellyItemImagesResponse{ImageType: "Backdrop", ImageIndex: len(f.backdrops), ImageTag: "new"})
		}
	})
	mux.HandleFunc("POST /Items/{key}/Images/Backdrop/{index}/Index", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		if f.indexErr != "" {
			http.Error(w, f.indexErr, http.StatusInternalServerError)
		}
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	previousConfig, previousMediux := config.Current, mediux.MediuxApiURL
	t.Cleanup(func() { config.Current, mediux.MediuxApiURL = previousConfig, previousMediux })
	config.Current.MainMediaServer = "main"
	config.Current.Images.CacheImages.Enabled = false
	mediux.MediuxApiURL = mediuxServer.URL

	return f, &EJ{Config: config.Config_MediaServer{ID: "other", Type: "Jellyfin", URL: server.URL, ApiToken: "jf-token"}}
}

func (f *fakeJellyfin) record(r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestEJTestConnection(t *testing.T) {
	_, e := newFakeJellyfin(t)
	ok, name, version, Err := e.TestConnection(testContext(), e.Config)
	if !ok || name != "Home Jellyfin" || version != "10.9" || Err.Message != "" {
		t.Errorf("TestConnection = %v, %q, %q, %s", ok, name, version, Err.Message)
	}
}

func TestEJDownloadApplyImage(t *testing.T) {
	fake, e := newFakeJellyfin(t)
	show := &models.MediaItem{Type: "show", Title: "Show", RatingKey: "200", Series: &models.MediaItemSeries{Seasons: []models.MediaItemSeason{
		{SeasonNumber: 1, RatingKey: "201", Episodes: []models.MediaItemEpisode{{SeasonNumber: 1, EpisodeNumber: 2, RatingKey: "203"}}},
	}}}

	// The poster, season poster and titlecard are uploaded to their own items
	for _, image := range []models.ImageFile{
		{ID: "poster", Type: "poster"},
		{ID: "season", Type: "season_poster", SeasonNumber: num(1)},
		{ID: "card", Type: "titlecard", SeasonNumber: num(1), EpisodeNumber: num(2)},
	} {
		if Err := e.DownloadApplyImageToMediaItem(testContext(), show, image); Err.Message != "" {
			t.Fatalf("applying the %s: %s", image.Type, Err.Message)
		}
	}
	for path, want := range map[string]string{
		"/Items/200/Images/Primary": "image /assets/poster",
		"/Items/201/Images/Primary": "image /assets/season",
		"/Items/203/Images/Primary": "image /assets/card",
	} {
		if string(fake.uploads[path]) != want || fake.types[path] != "image/png" {
			t.Errorf("%s got %q as %q, want %q as image/png", path, fake.uploads[path], fake.types[path], want)
		}
	}

	// A new backdrop is moved to the front
	if Err := e.DownloadApplyImageToMediaItem(testContext(), show, models.ImageFile{ID: "backdrop", Type: "backdrop"}); Err.Message != "" {
		t.Fatalf("applying the backdrop: %s", Err.Message)
	}
	if !slices.Contains(fake.requests, "POST /Items/200/Images/Backdrop/1/Index?newIndex=0") {
		t.Errorf("the new backdrop should be moved to index 0, requests = %v", fake.requests)
	}
}

func TestEJBackdropIndexFails(t *testing.T) {
	fake, e := newFakeJellyfin(t)
	fake.indexErr = "Access to the path '/media/Movie (2025)/backdrop.jpg' is denied."
	movie := &models.MediaItem{Type: "movie", Title: "Movie", RatingKey: "100"}

	// The error from moving the backdrop is returned, not a panic
	Err := e.DownloadApplyImageToMediaItem(testContext(), movie, models.ImageFile{ID: "backdrop", Type: "backdrop"})
	if Err.Message == "" {
		t.Fatal("expected an error when the backdrop can't be moved")
	}
}

func TestEJHelpers(t *testing.T) {
	jellyfin := AddEJAuthHeaders(config.Config_MediaServer{Type: "Jellyfin", ApiToken: "t"}, map[string]string{})
	if jellyfin["X-Emby-Token"] != "t" || jellyfin["Authorization"] != `MediaBrowser Token="t"` {
		t.Errorf("Jellyfin headers = %v", jellyfin)
	}
	if emby := AddEJAuthHeaders(config.Config_MediaServer{Type: "Emby", ApiToken: "t"}, map[string]string{}); emby["Authorization"] != "" {
		t.Errorf("Emby should only send X-Emby-Token, got %v", emby)
	}

	current := []EmbyJellyItemImagesResponse{{ImageType: "Primary", ImageTag: "p"}, {ImageType: "Backdrop", ImageIndex: 0, ImageTag: "a"}}
	updated := append(current, EmbyJellyItemImagesResponse{ImageType: "Backdrop", ImageIndex: 1, ImageTag: "b"})
	if got := findNewImage(testContext(), current, updated); got.ImageTag != "b" {
		t.Errorf("findNewImage = %+v, want the new backdrop", got)
	}
	if got := findNewImage(testContext(), current, current); got.ImageType != "" {
		t.Errorf("findNewImage with nothing new = %+v", got)
	}

	movie := models.MediaItem{Type: "movie", RatingKey: "1"}
	if got := getItemRatingKeyFromImageFile(movie, models.ImageFile{Type: "poster"}); got != "1" {
		t.Errorf("movie poster key = %q", got)
	}
}

func TestEJRefreshAddsNewItemsToTheMainCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Users/user/Items" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"TotalRecordCount":1,"Items":[{"Id":"300","Name":"New Movie","Type":"Movie","ProductionYear":2025,"ProviderIds":{"Tmdb":"603"},"DateCreated":"2025-03-04T05:06:07Z"}]}`))
	}))
	t.Cleanup(server.Close)

	previous := config.Current
	t.Cleanup(func() {
		config.Current = previous
		cache.LibraryStore.ClearAllSections()
	})
	config.Current.MainMediaServer = "main"
	cache.LibraryStore.UpdateSection(&models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{ID: "1", Title: "Movies", Type: "movie"}})
	section := models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{ID: "1", Title: "Movies", Type: "movie"}}

	// Another media server's items stay out of the main cache
	other := &EJ{Config: config.Config_MediaServer{ID: "other", Type: "Jellyfin", URL: server.URL, UserID: "user"}}
	if items, _, _, Err := other.GetLibrarySectionItems(testContext(), section, "0", ""); Err.Message != "" || len(items) != 1 {
		t.Fatalf("GetLibrarySectionItems on another media server = %d items, %s", len(items), Err.Message)
	}
	if _, found := cache.LibraryStore.GetMediaItemByRatingKey("300"); found {
		t.Error("another media server's item should not be added to the main cache")
	}

	// The main media server adds items found by a refresh, so their images and pages load
	main := &EJ{Config: config.Config_MediaServer{ID: "main", Type: "Jellyfin", URL: server.URL, UserID: "user"}}
	if _, _, _, Err := main.GetLibrarySectionItems(testContext(), section, "0", ""); Err.Message != "" {
		t.Fatalf("GetLibrarySectionItems on the main media server: %s", Err.Message)
	}
	if item, found := cache.LibraryStore.GetMediaItemByRatingKey("300"); !found || item.TMDB_ID != "603" || item.AddedAt != 1741064767 {
		t.Errorf("the refreshed item should be in the main cache, got %+v, %v", item, found)
	}
}

func TestEJDatesAreInSeconds(t *testing.T) {
	// The same as Plex and Silo, so sorting by date works
	created := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	if got := unixSeconds(created); got != created.Unix() {
		t.Errorf("unixSeconds = %d, want %d", got, created.Unix())
	}
	if got := unixSeconds(time.Time{}); got != 0 {
		t.Errorf("a date that isn't set should be 0, got %d", got)
	}
}

func TestEJSpecialsStayInTheSpecialsSeason(t *testing.T) {
	// With "Display specials within seasons", Season 2 lists S00E02 as well as S02E02
	episode := func(id string, season, number int, file string) map[string]any {
		return map[string]any{"Id": id, "ParentIndexNumber": season, "IndexNumber": number, "Path": file, "MediaSources": []map[string]any{{"Size": 1}}}
	}
	special := episode("s0e2", 0, 2, "/tv/Show/Specials/S00E02.mkv")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /Shows/show/Seasons", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"Items": []map[string]any{{"Id": "season0", "IndexNumber": 0}, {"Id": "season2", "IndexNumber": 2}}})
	})
	mux.HandleFunc("GET /Shows/show/Episodes", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("SeasonId") == "season0" {
			writeJSON(w, map[string]any{"Items": []map[string]any{special}})
			return
		}
		// An episode without media sources is kept, without a size
		noSources := map[string]any{"Id": "s2e3", "ParentIndexNumber": 2, "IndexNumber": 3, "Path": "/tv/Show/Season 02/S02E03.mkv"}
		writeJSON(w, map[string]any{"Items": []map[string]any{episode("s2e1", 2, 1, "/tv/Show/Season 02/S02E01.mkv"), special, episode("s2e2", 2, 2, "/tv/Show/Season 02/S02E02.mkv"), noSources}})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	item := models.MediaItem{RatingKey: "show", Title: "Show"}
	if Err := fetchSeasonsForShow(testContext(), config.Config_MediaServer{Type: "Jellyfin", URL: server.URL}, &item); Err.Message != "" {
		t.Fatalf("fetchSeasonsForShow: %s", Err.Message)
	}
	got := map[int][]string{}
	for _, season := range item.Series.Seasons {
		for _, ep := range season.Episodes {
			if ep.SeasonNumber != season.SeasonNumber {
				t.Errorf("season %d has episode %s from season %d", season.SeasonNumber, ep.RatingKey, ep.SeasonNumber)
			}
			got[season.SeasonNumber] = append(got[season.SeasonNumber], ep.RatingKey)
		}
	}
	if !slices.Equal(got[0], []string{"s0e2"}) || !slices.Equal(got[2], []string{"s2e1", "s2e2", "s2e3"}) {
		t.Errorf("episodes by season = %v", got)
	}
}
