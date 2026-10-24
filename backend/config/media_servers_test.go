package config

import (
	"aura/logging"
	"aura/models"
	"context"
	"reflect"
	"testing"
)

func libraries(titles ...string) []models.LibrarySection {
	var sections []models.LibrarySection
	for _, title := range titles {
		sections = append(sections, models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: title}})
	}
	return sections
}

func TestSyncMediaServersAddsTheMainServer(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MediaServer = Config_MediaServer{Type: "Plex", URL: "http://plex:32400", ApiToken: "12345", UserID: "user"}
	cfg.SyncMediaServers()

	if len(cfg.MediaServers) != 1 || cfg.MainMediaServer == "" || cfg.MediaServers[0].ID != cfg.MainMediaServer {
		t.Fatalf("MediaServers = %+v, MainMediaServer = %q", cfg.MediaServers, cfg.MainMediaServer)
	}
	main := cfg.MediaServers[0]
	if main.Name != "Plex" || !main.Enabled || main.URL != "http://plex:32400" || main.ApiToken != "12345" {
		t.Errorf("main media server = %+v", main)
	}
	if cfg.MediaServer.ID != main.ID || cfg.MediaServer.Name != "Plex" {
		t.Errorf("MediaServer should have the main server's ID and name: %+v", cfg.MediaServer)
	}

	// Nothing to add before a media server is set up
	empty := DefaultConfig()
	empty.SyncMediaServers()
	if empty.MediaServers != nil || empty.MainMediaServer != "" {
		t.Errorf("MediaServers = %+v, MainMediaServer = %q, want none", empty.MediaServers, empty.MainMediaServer)
	}
}

func TestSyncMediaServersUpdatesTheMainServer(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MediaServers = []Config_MediaServer{
		{ID: "main", Name: "Living Room", Enabled: true, Type: "Plex", URL: "http://old:32400"},
		{ID: "second", Name: "Silo", Enabled: false, Type: "Silo", URL: "http://silo:8090"},
	}
	cfg.MainMediaServer = "main"
	cfg.LoadMainMediaServer()

	cfg.MediaServer.URL = "http://new:32400"
	cfg.MediaServer.Libraries = libraries("Movies")
	cfg.SyncMediaServers()

	main := cfg.MediaServers[0]
	if main.URL != "http://new:32400" || len(main.Libraries) != 1 || main.Name != "Living Room" || main.ID != "main" {
		t.Errorf("main media server = %+v", main)
	}
	if cfg.MediaServers[1].URL != "http://silo:8090" || cfg.MediaServers[1].Enabled {
		t.Errorf("other media servers should not change: %+v", cfg.MediaServers[1])
	}

	// The copy in MediaServer is separate from the entry in MediaServers
	cfg.MediaServer.Libraries[0].Title = "Changed"
	if cfg.MediaServers[0].Libraries[0].Title != "Movies" {
		t.Error("changing MediaServer should not change MediaServers until it is synced")
	}
}

func TestSyncMediaServersLibraryMap(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MediaServers = []Config_MediaServer{
		{ID: "main", Type: "Plex", Libraries: libraries("Movies", "Kids")},
		{
			ID:         "second",
			Type:       "Jellyfin",
			Libraries:  libraries("Movies", "Films", "Kids", "Home Videos", "Old"),
			LibraryMap: map[string]string{"Films": "Movies", "Kids": "", "Old": "Removed", "Not Selected": "Movies"},
		},
	}
	cfg.MainMediaServer = "main"
	cfg.LoadMainMediaServer()
	cfg.SyncMediaServers()

	want := map[string]string{
		"Movies": "Movies", // Same title as a main server library
		"Films":  "Movies", // Set by the user
		"Kids":   "",       // Set to not match anything, so it isn't matched by title
		"Old":    "",       // The main server library it matched was removed
	}
	if got := cfg.MediaServers[1].LibraryMap; !reflect.DeepEqual(got, want) {
		t.Errorf("LibraryMap = %v, want %v", got, want)
	}
	if cfg.MediaServers[0].LibraryMap != nil {
		t.Error("the main media server should not have a library map")
	}
}

func TestParseConfigYAML(t *testing.T) {
	// Config files from before multiple media servers
	older, err := ParseConfigYAML([]byte("MediaServer:\n  Type: Plex\n  URL: http://plex:32400\n  ApiToken: \"12345\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(older.MediaServers) != 1 || older.MediaServers[0].URL != "http://plex:32400" || older.MediaServer.URL != "http://plex:32400" {
		t.Errorf("MediaServers = %+v, MediaServer = %+v", older.MediaServers, older.MediaServer)
	}
	if len(older.UserPreferences.DownloadDefaults) != len(DownloadImageTypes) {
		t.Error("older config files should use the default User Preferences")
	}

	// Config files with MediaServers
	newer, err := ParseConfigYAML([]byte(`
MainMediaServer: second
MediaServers:
  - ID: first
    Type: Plex
    URL: http://plex:32400
    ApiToken: "12345"
  - ID: second
    Type: Silo
    URL: http://silo:8090
    ApiToken: "67890"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(newer.MediaServers) != 2 || newer.MediaServer.ID != "second" || newer.MediaServer.URL != "http://silo:8090" {
		t.Errorf("MediaServers = %+v, MediaServer = %+v", newer.MediaServers, newer.MediaServer)
	}
}

func TestRestoreMediaServerTokens(t *testing.T) {
	current := []Config_MediaServer{{ID: "a", ApiToken: "secret-a"}, {ID: "b", ApiToken: "secret-b"}}
	updated := []Config_MediaServer{
		{ID: "a", ApiToken: MaskToken("secret-a")}, // Sent back masked
		{ID: "b", ApiToken: "new-token"},           // Changed
		{ID: "c", ApiToken: "***1234"},             // New server
	}
	RestoreMediaServerTokens(current, updated)
	if updated[0].ApiToken != "secret-a" || updated[1].ApiToken != "new-token" || updated[2].ApiToken != "***1234" {
		t.Errorf("tokens = %q, %q, %q", updated[0].ApiToken, updated[1].ApiToken, updated[2].ApiToken)
	}
}

func TestValidateMediaServers(t *testing.T) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	valid := []Config_MediaServer{
		{ID: "main", Type: "Plex"},
		{ID: "second", Enabled: true, Type: "Silo", URL: "http://silo:8090", ApiToken: "12345", Libraries: libraries("Movies")},
		{ID: "paused", Enabled: false, Type: "Unknown"},
	}
	if !ValidateMediaServers(ctx, valid, "main") {
		t.Error("expected valid media servers")
	}
	if !ValidateMediaServers(ctx, nil, "") {
		t.Error("no media servers should be valid")
	}

	tests := map[string]struct {
		servers []Config_MediaServer
		main    string
	}{
		"duplicate ID":         {[]Config_MediaServer{{ID: "a", Type: "Plex"}, {ID: "a", Type: "Plex"}}, "a"},
		"main is not a server": {valid, "missing"},
		"invalid type":         {[]Config_MediaServer{{ID: "a"}, {ID: "b", Enabled: true, Type: "Kodi", URL: "http://x", ApiToken: "y", Libraries: libraries("Movies")}}, "a"},
		"missing URL":          {[]Config_MediaServer{{ID: "a"}, {ID: "b", Enabled: true, Type: "Plex", ApiToken: "y", Libraries: libraries("Movies")}}, "a"},
		"missing API token":    {[]Config_MediaServer{{ID: "a"}, {ID: "b", Enabled: true, Type: "Plex", URL: "http://x", Libraries: libraries("Movies")}}, "a"},
		"missing libraries":    {[]Config_MediaServer{{ID: "a"}, {ID: "b", Enabled: true, Type: "Plex", URL: "http://x", ApiToken: "y"}}, "a"},
		"URL without http":     {[]Config_MediaServer{{ID: "a"}, {ID: "b", Enabled: true, Type: "Plex", URL: "plex:32400", ApiToken: "y", Libraries: libraries("Movies")}}, "a"},
	}
	for name, test := range tests {
		if ValidateMediaServers(ctx, test.servers, test.main) {
			t.Errorf("%s: expected invalid media servers", name)
		}
	}
}

func TestSavedMediaServerToken(t *testing.T) {
	previous := Current
	t.Cleanup(func() { Current = previous })
	Current = DefaultConfig()
	Current.MediaServers = []Config_MediaServer{{ID: "main", ApiToken: "main-token"}, {ID: "silo", ApiToken: "silo-token"}}
	Current.MainMediaServer = "main"
	Current.LoadMainMediaServer()

	for id, want := range map[string]string{"": "main-token", "main": "main-token", "silo": "silo-token", "new": ""} {
		if got := SavedMediaServerToken(id); got != want {
			t.Errorf("SavedMediaServerToken(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestSwitchMainMediaServer(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MediaServers = []Config_MediaServer{
		{ID: "plex", Name: "Plex", Enabled: true, Type: "Plex", Libraries: libraries("Movies", "4K Movies", "Series", "Kids")},
		{ID: "silo", Name: "Silo", Enabled: true, Type: "Silo", Libraries: libraries("Films", "4K", "Shows", "Extra"),
			LibraryMap: map[string]string{"Films": "Movies", "4K": "4K Movies", "Shows": "Series", "Extra": ""}},
		{ID: "jellyfin", Name: "Jellyfin", Enabled: true, Type: "Jellyfin", Libraries: libraries("Movies", "TV", "Kids"),
			LibraryMap: map[string]string{"Movies": "Movies", "TV": "Series", "Kids": "Kids"}},
		{ID: "paused", Name: "Paused", Enabled: false, Type: "Plex"},
	}
	cfg.MainMediaServer = "plex"
	cfg.SonarrRadarr.Applications = []Config_SonarrRadarrApp{{Type: "Sonarr", Library: "Series"}, {Type: "Radarr", Library: "Kids"}}
	cfg.LoadMainMediaServer()

	changes, err := SwitchMainMediaServer(&cfg, "silo")
	if err != nil {
		t.Fatal(err)
	}
	wantChanges := map[string]string{"Movies": "Films", "4K Movies": "4K", "Series": "Shows"}
	if !reflect.DeepEqual(changes, wantChanges) {
		t.Errorf("library changes = %v, want %v", changes, wantChanges)
	}
	if cfg.MainMediaServer != "silo" || cfg.MediaServer.ID != "silo" || cfg.MediaServers[1].LibraryMap != nil {
		t.Errorf("MainMediaServer = %q, MediaServer.ID = %q, silo map = %v", cfg.MainMediaServer, cfg.MediaServer.ID, cfg.MediaServers[1].LibraryMap)
	}
	// The old main media server matches the new main's libraries; Kids isn't on Silo
	if want := map[string]string{"Movies": "Films", "4K Movies": "4K", "Series": "Shows", "Kids": ""}; !reflect.DeepEqual(cfg.MediaServers[0].LibraryMap, want) {
		t.Errorf("old main library map = %v, want %v", cfg.MediaServers[0].LibraryMap, want)
	}
	// Other media servers now match the new main's libraries
	if want := map[string]string{"Movies": "Films", "TV": "Shows", "Kids": ""}; !reflect.DeepEqual(cfg.MediaServers[2].LibraryMap, want) {
		t.Errorf("jellyfin library map = %v, want %v", cfg.MediaServers[2].LibraryMap, want)
	}
	// Sonarr/Radarr libraries follow, unless there is no matching library
	if cfg.SonarrRadarr.Applications[0].Library != "Shows" || cfg.SonarrRadarr.Applications[1].Library != "Kids" {
		t.Errorf("Sonarr/Radarr libraries = %q, %q", cfg.SonarrRadarr.Applications[0].Library, cfg.SonarrRadarr.Applications[1].Library)
	}

	for _, id := range []string{"silo", "paused", "missing"} {
		if _, err := SwitchMainMediaServer(&cfg, id); err == nil {
			t.Errorf("switching to %q should fail", id)
		}
	}
}

func TestSyncMediaServersDuringSetup(t *testing.T) {
	// Setup sends the other media servers, and the main one only in MediaServer
	cfg := DefaultConfig()
	cfg.MediaServer = Config_MediaServer{Name: "Living Room", Type: "Plex", URL: "http://plex:32400", Libraries: libraries("Movies")}
	cfg.MediaServers = []Config_MediaServer{{Enabled: true, Type: "Silo", URL: "http://silo:8090", Libraries: libraries("Movies", "Films")}}
	cfg.SyncMediaServers()

	if len(cfg.MediaServers) != 2 || cfg.MediaServers[0].Type != "Plex" || cfg.MainMediaServer != cfg.MediaServers[0].ID {
		t.Fatalf("MediaServers = %+v, MainMediaServer = %q, want the main media server first", cfg.MediaServers, cfg.MainMediaServer)
	}
	if cfg.MediaServers[0].Name != "Living Room" {
		t.Errorf("main name = %q, want the name entered during setup", cfg.MediaServers[0].Name)
	}
	silo := cfg.MediaServers[1]
	if silo.ID == "" || silo.ID == cfg.MainMediaServer || silo.Name != "Silo" || silo.LibraryMap["Movies"] != "Movies" {
		t.Errorf("other media server = %+v", silo)
	}
}

func TestSyncMediaServersWithAStaleMainServer(t *testing.T) {
	// The media servers were removed, but MainMediaServer still has the old ID
	cfg := DefaultConfig()
	cfg.MainMediaServer = "removed-server"
	cfg.MediaServer = Config_MediaServer{Type: "Silo", URL: "http://silo:8090", ApiToken: "token", Libraries: libraries("Movies")}
	cfg.SyncMediaServers()

	if len(cfg.MediaServers) != 1 || cfg.MediaServers[0].Type != "Silo" || cfg.MainMediaServer != cfg.MediaServers[0].ID || cfg.MainMediaServer == "removed-server" {
		t.Fatalf("MediaServers = %+v, MainMediaServer = %q, want Silo saved as the main media server", cfg.MediaServers, cfg.MainMediaServer)
	}

	// After saving and loading again, Silo is still the main media server
	cfg.MediaServer = Config_MediaServer{}
	cfg.LoadMainMediaServer()
	if cfg.MediaServer.Type != "Silo" || len(cfg.MediaServer.Libraries) != 1 {
		t.Errorf("loaded main media server = %+v", cfg.MediaServer)
	}
}
