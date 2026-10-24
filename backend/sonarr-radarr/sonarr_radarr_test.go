package sonarr_radarr

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sync"
	"testing"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

// fakeSonarr is a Sonarr server with one series, TMDB ID 10, and the tags "keep" and "remove-me"
type fakeSonarr struct {
	mu      sync.Mutex
	tags    []SonarrRadarrTag
	putTags []int64
	putBody map[string]any
	apiKeys []string
}

// fakeSeries is a series as Sonarr sends it, with fields aura doesn't use
const fakeSeries = `{"id":5,"title":"Show","tags":[1,2],"monitorNewItems":"all","languageProfileId":1,
"alternateTitles":[{"title":"Show Alt","seasonNumber":1,"sceneSeasonNumber":1,"sceneOrigin":"tvdb"}],
"seasons":[{"seasonNumber":0,"monitored":false},{"seasonNumber":1,"monitored":true}]}`

func newFakeSonarr(t *testing.T) (*fakeSonarr, *httptest.Server) {
	t.Helper()
	f := &fakeSonarr{tags: []SonarrRadarrTag{{Label: "keep", ID: 1}, {Label: "remove-me", ID: 2}}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v3/system/status", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		_, _ = w.Write([]byte(`{"version":"4.0"}`))
	})
	mux.HandleFunc("GET /api/v3/tag", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(f.tags)
	})
	mux.HandleFunc("POST /api/v3/tag", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		var tag SonarrRadarrTag
		_ = json.NewDecoder(r.Body).Decode(&tag)
		f.mu.Lock()
		tag.ID = len(f.tags) + 1
		f.tags = append(f.tags, tag)
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(tag)
	})
	mux.HandleFunc("GET /api/v3/series/lookup", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		if r.URL.Query().Get("term") != "tmdb:10" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":5,"tmdbId":10}]`))
	})
	mux.HandleFunc("GET /api/v3/series/5", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		_, _ = w.Write([]byte(fakeSeries))
	})
	mux.HandleFunc("PUT /api/v3/series/5", func(w http.ResponseWriter, r *http.Request) {
		f.record(r)
		body, _ := io.ReadAll(r.Body)
		var item struct {
			Tags []int64 `json:"tags"`
		}
		var fields map[string]any
		_ = json.Unmarshal(body, &item)
		_ = json.Unmarshal(body, &fields)
		f.mu.Lock()
		f.putTags, f.putBody = item.Tags, fields
		f.mu.Unlock()
		_, _ = w.Write(body)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return f, server
}

func (f *fakeSonarr) record(r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.apiKeys = append(f.apiKeys, r.Header.Get("X-Api-Key"))
}

func TestNewSonarrRadarrInterface(t *testing.T) {
	app := config.Config_SonarrRadarrApp{Type: "Sonarr", Library: "Series", URL: "http://sonarr", ApiToken: "key"}
	if _, Err := NewSonarrRadarrInterface(testContext(), app); Err.Message != "" {
		t.Errorf("a complete Sonarr app should work: %s", Err.Message)
	}
	app.Type = "Lidarr"
	if _, Err := NewSonarrRadarrInterface(testContext(), app); Err.Message == "" {
		t.Error("an unsupported type should fail")
	}
	if _, Err := NewSonarrRadarrInterface(testContext(), config.Config_SonarrRadarrApp{Type: "Radarr"}); Err.Message == "" {
		t.Error("an app without its URL, token and library should fail")
	}
}

func TestSonarrRequests(t *testing.T) {
	fake, server := newFakeSonarr(t)
	app := config.Config_SonarrRadarrApp{Type: "Sonarr", Library: "Series", URL: server.URL, ApiToken: "key"}

	if valid, Err := TestConnection(testContext(), app); !valid || Err.Message != "" {
		t.Errorf("TestConnection = %v, %s", valid, Err.Message)
	}
	tags, Err := GetAllTags(testContext(), app)
	if Err.Message != "" || len(tags) != 2 {
		t.Errorf("GetAllTags = %v, %s", tags, Err.Message)
	}
	added, Err := AddNewTags(testContext(), app, []string{"new"})
	if Err.Message != "" || len(added) != 1 || added[0].Label != "new" || added[0].ID != 3 {
		t.Errorf("AddNewTags = %v, %s", added, Err.Message)
	}
	item, Err := GetItemInfoFromTMDBID(testContext(), app, 10)
	if series, ok := item.(SR_Item); Err.Message != "" || !ok || series.ID != 5 || !slices.Equal(series.Tags, []int64{1, 2}) {
		t.Errorf("GetItemInfoFromTMDBID = %+v, %s", item, Err.Message)
	}
	if _, Err := GetItemInfoFromTMDBID(testContext(), app, 99); Err.Message == "" {
		t.Error("a series that isn't in Sonarr should fail")
	}
	for _, key := range fake.apiKeys {
		if key != "key" {
			t.Fatalf("every request should send the API key, got %q", key)
		}
	}
}

func TestHandleTags(t *testing.T) {
	fake, server := newFakeSonarr(t)
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	config.Current.SonarrRadarr.Applications = []config.Config_SonarrRadarrApp{
		{Type: "Sonarr", Library: "Series", URL: server.URL, ApiToken: "key"},
	}
	config.Current.LabelsAndTags.Applications = []config.Config_LabelsAndTagsProvider{
		{Application: "Sonarr", Enabled: true, Add: []string{"aura"}, Remove: []string{"remove-me"}, AddLabelTagForSelectedTypes: true},
	}

	item := models.MediaItem{Type: "show", TMDB_ID: "10", LibraryTitle: "Series", Title: "Show"}
	if Err := HandleTags(testContext(), item, models.SelectedTypes{Poster: true}); Err.Message != "" {
		t.Fatalf("HandleTags: %s", Err.Message)
	}

	// "keep" stays, "remove-me" is removed, and the new "aura" and "aura-poster" tags are created and added
	slices.Sort(fake.putTags)
	if !slices.Equal(fake.putTags, []int64{1, 3, 4}) {
		t.Errorf("tags sent to Sonarr = %v, want [1 3 4]", fake.putTags)
	}
	// Every other field is sent back as Sonarr sent it
	var want map[string]any
	_ = json.Unmarshal([]byte(fakeSeries), &want)
	delete(want, "tags")
	delete(fake.putBody, "tags")
	if !reflect.DeepEqual(fake.putBody, want) {
		t.Errorf("series sent to Sonarr = %v, want %v", fake.putBody, want)
	}
	labels := []string{}
	for _, tag := range fake.tags {
		labels = append(labels, tag.Label)
	}
	if !slices.Equal(labels, []string{"keep", "remove-me", "aura", "aura-poster"}) {
		t.Errorf("Sonarr tags = %v", labels)
	}

	// Movies only go to Radarr, so Sonarr isn't changed
	fake.putTags = nil
	if Err := HandleTags(testContext(), models.MediaItem{Type: "movie", TMDB_ID: "10", LibraryTitle: "Series"}, models.SelectedTypes{}); Err.Message != "" || fake.putTags != nil {
		t.Errorf("a movie should not update Sonarr: %s", Err.Message)
	}
	if Err := HandleTags(testContext(), models.MediaItem{Type: "episode", TMDB_ID: "10", LibraryTitle: "Series"}, models.SelectedTypes{}); Err.Message == "" {
		t.Error("an episode should fail")
	}
}

func TestSRItemKeepsItsFields(t *testing.T) {
	item, err := parseSRItem([]byte(fakeSeries))
	if err != nil {
		t.Fatalf("parseSRItem: %v", err)
	}
	if item.ID != 5 || !slices.Equal(item.Tags, []int64{1, 2}) {
		t.Errorf("item = %d %v", item.ID, item.Tags)
	}

	// Removing every tag still sends an empty list
	item.Tags = nil
	body, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var sent map[string]json.RawMessage
	_ = json.Unmarshal(body, &sent)
	if string(sent["tags"]) != "[]" || string(sent["languageProfileId"]) != "1" {
		t.Errorf("sent = %s", body)
	}
}
