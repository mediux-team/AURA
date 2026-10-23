package database

import (
	"aura/config"
	"aura/logging"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	return logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
}

// openTestConfigDB opens a test database with the config tables, and uses it as the config store
func openTestConfigDB(t *testing.T) *SQliteDB {
	t.Helper()
	db := &SQliteDB{conn: openTestDB(t)}
	if Err := db.CreateConfigTables(testContext()); Err.Message != "" {
		t.Fatalf("CreateConfigTables: %s", Err.Message)
	}

	previousClient := Client
	Client = db
	config.SetStore(ConfigStore())
	t.Cleanup(func() {
		Client = previousClient
		config.SetStore(nil)
	})
	return db
}

func configUpdatedAt(t *testing.T, conn *sql.DB, key string) string {
	t.Helper()
	var updatedAt string
	if err := conn.QueryRow(`SELECT updated_at FROM Config WHERE key = ?`, key).Scan(&updatedAt); err != nil {
		t.Fatalf("updated_at for %s: %v", key, err)
	}
	return updatedAt
}

func TestSaveConfigValuesOnlyWritesChangedValues(t *testing.T) {
	db := openTestConfigDB(t)
	ctx := testContext()

	if Err := db.SaveConfigValues(ctx, map[string]string{"A": `"1"`, "B": `"2"`, "C": `"3"`}, false); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if _, err := db.conn.Exec(`UPDATE Config SET updated_at = '2000-01-01 00:00:00'`); err != nil {
		t.Fatal(err)
	}
	oldUpdatedAt := configUpdatedAt(t, db.conn, "B")

	// A changes, B is unchanged, C is not included
	if Err := db.SaveConfigValues(ctx, map[string]string{"A": `"changed"`, "B": `"2"`}, false); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if got := configUpdatedAt(t, db.conn, "A"); got == oldUpdatedAt {
		t.Error("changed value A should have a new updated_at")
	}
	if got := configUpdatedAt(t, db.conn, "B"); got != oldUpdatedAt {
		t.Errorf("unchanged value B updated_at = %s, want it unchanged", got)
	}

	values, Err := db.GetAllConfigValues(ctx)
	if Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if values["A"] != `"changed"` || values["C"] != `"3"` {
		t.Errorf("values = %v", values)
	}

	// Replace removes values that are not included
	if Err := db.SaveConfigValues(ctx, map[string]string{"A": `"replaced"`}, true); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	values, _ = db.GetAllConfigValues(ctx)
	if len(values) != 1 || values["A"] != `"replaced"` {
		t.Errorf("values after replace = %v", values)
	}
}

func TestConfigSchemaMigrations(t *testing.T) {
	db := openTestConfigDB(t)
	ctx := testContext()

	var version int
	if err := db.conn.QueryRow(`SELECT version FROM ConfigSchemaVersion`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != LATEST_CONFIG_SCHEMA_VERSION {
		t.Fatalf("new config schema version = %d, want %d", version, LATEST_CONFIG_SCHEMA_VERSION)
	}

	// Add a migration that renames a key, as if a field was renamed in a newer version
	previousMigrations, previousLatest := configMigrations, LATEST_CONFIG_SCHEMA_VERSION
	t.Cleanup(func() { configMigrations, LATEST_CONFIG_SCHEMA_VERSION = previousMigrations, previousLatest })
	configMigrations = append(append([]configMigration{}, previousMigrations...), configMigration{
		Description: "Rename MediaServer.Token to MediaServer.ApiToken",
		Migrate:     renameConfigKey("MediaServer.Token", "MediaServer.ApiToken"),
	})
	LATEST_CONFIG_SCHEMA_VERSION = len(configMigrations) + 1

	if Err := db.SaveConfigValues(ctx, map[string]string{"MediaServer.Token": `"12345"`}, false); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if Err := db.CreateConfigTables(ctx); Err.Message != "" {
		t.Fatalf("CreateConfigTables: %s", Err.Message)
	}

	values, _ := db.GetAllConfigValues(ctx)
	if _, found := values["MediaServer.Token"]; found || values["MediaServer.ApiToken"] != `"12345"` {
		t.Errorf("values after migration = %v", values)
	}
	if err := db.conn.QueryRow(`SELECT version FROM ConfigSchemaVersion`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != LATEST_CONFIG_SCHEMA_VERSION {
		t.Errorf("config schema version after migration = %d, want %d", version, LATEST_CONFIG_SCHEMA_VERSION)
	}
}

func TestMoveMediaServerToMediaServers(t *testing.T) {
	db := openTestConfigDB(t)
	ctx := testContext()

	// A config saved before there were multiple media servers
	if _, err := db.conn.Exec(`UPDATE ConfigSchemaVersion SET version = 1;`); err != nil {
		t.Fatal(err)
	}
	if Err := db.SaveConfigValues(ctx, map[string]string{
		"MediaServer.Type":                    `"Plex"`,
		"MediaServer.URL":                     `"http://plex:32400"`,
		"MediaServer.ApiToken":                `"12345"`,
		"MediaServer.Libraries":               `[{"Title":"Movies"},{"Title":"4K Movies"}]`,
		"MediaServer.EnablePlexEventListener": `true`,
		"Mediux.ApiToken":                     `"mediux-token"`,
	}, false); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if Err := db.CreateConfigTables(ctx); Err.Message != "" {
		t.Fatalf("CreateConfigTables: %s", Err.Message)
	}

	values, _ := db.GetAllConfigValues(ctx)
	for key := range values {
		if strings.HasPrefix(key, "MediaServer.") {
			t.Errorf("%s should be moved to MediaServers", key)
		}
	}
	cfg, decodeErrors, unknownKeys := config.ConfigFromValues(values)
	if len(decodeErrors) > 0 || len(unknownKeys) > 0 {
		t.Fatalf("decodeErrors = %v, unknownKeys = %v", decodeErrors, unknownKeys)
	}
	cfg.LoadMainMediaServer()
	if len(cfg.MediaServers) != 1 || cfg.MainMediaServer == "" || cfg.MediaServers[0].ID != cfg.MainMediaServer {
		t.Fatalf("MediaServers = %+v, MainMediaServer = %q", cfg.MediaServers, cfg.MainMediaServer)
	}
	main := cfg.MediaServer
	if main.Name != "Plex" || !main.Enabled || main.Type != "Plex" || main.URL != "http://plex:32400" || main.ApiToken != "12345" ||
		len(main.Libraries) != 2 || !main.EnablePlexEventListener {
		t.Errorf("main media server = %+v", main)
	}
	if cfg.Mediux.ApiToken != "mediux-token" {
		t.Error("other values should not change")
	}
}

func TestMoveMediaServerToMediaServersNotSetUp(t *testing.T) {
	db := openTestConfigDB(t)
	ctx := testContext()

	if _, err := db.conn.Exec(`UPDATE ConfigSchemaVersion SET version = 1;`); err != nil {
		t.Fatal(err)
	}
	if Err := db.SaveConfigValues(ctx, map[string]string{"MediaServer.Type": `""`, "MediaServer.URL": `""`}, false); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if Err := db.CreateConfigTables(ctx); Err.Message != "" {
		t.Fatalf("CreateConfigTables: %s", Err.Message)
	}

	values, _ := db.GetAllConfigValues(ctx)
	if values["MediaServers"] != "null" || values["MainMediaServer"] != `""` {
		t.Errorf("MediaServers = %s, MainMediaServer = %s, want no media servers", values["MediaServers"], values["MainMediaServer"])
	}
}

func TestAddCollectionImagesToDownloadDefaults(t *testing.T) {
	db := openTestConfigDB(t)
	ctx := testContext()

	// Download defaults saved before the collection image types
	if _, err := db.conn.Exec(`UPDATE ConfigSchemaVersion SET version = 3;`); err != nil {
		t.Fatal(err)
	}
	if Err := db.SaveConfigValues(ctx, map[string]string{
		"UserPreferences.DownloadDefaults": `["poster","titlecard"]`,
		"Mediux.ApiToken":                  `"mediux-token"`,
	}, false); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if Err := db.CreateConfigTables(ctx); Err.Message != "" {
		t.Fatalf("CreateConfigTables: %s", Err.Message)
	}

	values, _ := db.GetAllConfigValues(ctx)
	if got := values["UserPreferences.DownloadDefaults"]; got != `["poster","titlecard","collection_poster","collection_backdrop"]` {
		t.Errorf("DownloadDefaults = %s", got)
	}
	if values["Mediux.ApiToken"] != `"mediux-token"` {
		t.Error("other values should not change")
	}
}

func TestMoveRedirectURLToRedirectURLs(t *testing.T) {
	db := openTestConfigDB(t)
	ctx := testContext()

	// A config saved with one OIDC Redirect URL
	if _, err := db.conn.Exec(`UPDATE ConfigSchemaVersion SET version = 4;`); err != nil {
		t.Fatal(err)
	}
	if Err := db.SaveConfigValues(ctx, map[string]string{
		"Auth.OIDC.Enabled":     `true`,
		"Auth.OIDC.RedirectURL": `"https://aura.example.com/api/auth/oidc/callback"`,
	}, false); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	if Err := db.CreateConfigTables(ctx); Err.Message != "" {
		t.Fatalf("CreateConfigTables: %s", Err.Message)
	}

	values, _ := db.GetAllConfigValues(ctx)
	if _, found := values["Auth.OIDC.RedirectURL"]; found {
		t.Error("Auth.OIDC.RedirectURL should be moved")
	}
	if got := values["Auth.OIDC.RedirectURLs"]; got != `["https://aura.example.com/api/auth/oidc/callback"]` {
		t.Errorf("RedirectURLs = %s", got)
	}
	if values["Auth.OIDC.Enabled"] != `true` {
		t.Error("other values should not change")
	}
}

func writeConfigYAML(t *testing.T, url string) string {
	t.Helper()
	yamlFile := filepath.Join(config.ConfigPath, "config.yaml")
	data := []byte(`
Auth:
  Enabled: false
MediaServer:
  Type: Plex
  URL: ` + url + `
  ApiToken: "12345"
Mediux:
  ApiToken: mediux-token
  DownloadQuality: original
Database:
  Type: sqlite3
  Path: AURA.db
`)
	if err := os.WriteFile(yamlFile, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return yamlFile
}

func TestLoadConfig(t *testing.T) {
	db := openTestConfigDB(t)
	ctx := testContext()

	// First startup with a config.yaml: it is imported and renamed
	yamlFile := writeConfigYAML(t, "http://plex:32400")
	config.Load(ctx)
	if !config.Loaded {
		t.Fatal("config not loaded")
	}
	if config.Current.MediaServer.URL != "http://plex:32400" || config.Current.Mediux.DownloadQuality != "original" {
		t.Errorf("imported config = %+v", config.Current)
	}
	// Older config files have no User Preferences, so the defaults are used
	if len(config.Current.UserPreferences.DownloadDefaults) != len(config.DownloadImageTypes) {
		t.Errorf("UserPreferences.DownloadDefaults = %v, want the defaults", config.Current.UserPreferences.DownloadDefaults)
	}
	if _, err := os.Stat(yamlFile); !os.IsNotExist(err) {
		t.Error("config.yaml should be renamed after import")
	}
	if _, err := os.Stat(yamlFile + ".imported"); err != nil {
		t.Errorf("config.yaml.imported: %v", err)
	}
	values, _ := db.GetAllConfigValues(ctx)
	if !strings.Contains(values["MediaServers"], `"URL":"http://plex:32400"`) || values["MainMediaServer"] == `""` {
		t.Errorf("stored MediaServers = %s, MainMediaServer = %s", values["MediaServers"], values["MainMediaServer"])
	}

	// Next startup: the config loads from the database
	config.Current = config.Config{}
	config.Load(ctx)
	if config.Current.MediaServer.URL != "http://plex:32400" {
		t.Errorf("loaded MediaServer.URL = %q", config.Current.MediaServer.URL)
	}

	// A new config.yaml is ignored, as the config is already in the database
	writeConfigYAML(t, "http://new-plex:32400")
	config.Load(ctx)
	if config.Current.MediaServer.URL != "http://plex:32400" {
		t.Errorf("MediaServer.URL = %q, config.yaml should be ignored", config.Current.MediaServer.URL)
	}
	if _, err := os.Stat(yamlFile); err != nil {
		t.Error("an ignored config.yaml should not be renamed")
	}

	// With AURA_IMPORT_CONFIG=true, it replaces the config in the database
	t.Setenv(config.ImportConfigEnv, "true")
	config.Load(ctx)
	if config.Current.MediaServer.URL != "http://new-plex:32400" {
		t.Errorf("MediaServer.URL = %q, config.yaml should be imported", config.Current.MediaServer.URL)
	}
	if _, err := os.Stat(yamlFile); !os.IsNotExist(err) {
		t.Error("config.yaml should be renamed after import")
	}
	// The first imported file is kept
	matches, _ := filepath.Glob(yamlFile + ".imported.*")
	if len(matches) != 1 {
		t.Errorf("expected a second, timestamped imported file, got %v", matches)
	}
}

func TestLoadConfigSavesDefaults(t *testing.T) {
	db := openTestConfigDB(t)
	ctx := testContext()

	config.Load(ctx)
	if !config.Loaded {
		t.Fatal("config not loaded")
	}
	if config.Current.Logging.Level != config.DefaultConfig().Logging.Level {
		t.Errorf("Logging.Level = %q, want the default", config.Current.Logging.Level)
	}
	values, _ := db.GetAllConfigValues(ctx)
	if len(values) == 0 {
		t.Error("default config should be saved to the database")
	}
}
