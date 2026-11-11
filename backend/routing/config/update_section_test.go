package routes_config

import (
	"aura/config"
	"aura/logging"
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// memoryConfigStore is an in-memory config.Store for tests
type memoryConfigStore struct {
	values map[string]string
	saves  int
}

func (s *memoryConfigStore) GetAllConfigValues(ctx context.Context) (map[string]string, logging.LogErrorInfo) {
	return maps.Clone(s.values), logging.LogErrorInfo{}
}

func (s *memoryConfigStore) SaveConfigValues(ctx context.Context, values map[string]string, replace bool) logging.LogErrorInfo {
	if replace {
		s.values = map[string]string{}
	}
	maps.Copy(s.values, values)
	s.saves++
	return logging.LogErrorInfo{}
}

// setupConfigSectionTest sets a known current config, saved to an in-memory store
func setupConfigSectionTest(t *testing.T) *memoryConfigStore {
	t.Helper()
	previousConfig, previousValid, previousFullyLoaded := config.Current, config.Valid, config.AppFullyLoaded
	t.Cleanup(func() {
		config.Current, config.Valid, config.AppFullyLoaded = previousConfig, previousValid, previousFullyLoaded
		config.SetStore(nil)
	})

	cfg := config.DefaultConfig()
	cfg.Auth.APIKeyHash = "$argon2id$hash"
	cfg.Auth.Password = "$argon2id$v=19$m=65536,t=1,p=4$password-hash"
	cfg.Auth.OIDC = config.Config_Auth_OIDC{Enabled: false, IssuerURL: "https://id.example.com", ClientID: "aura", ClientSecret: "client-secret"}
	cfg.MediaServer.UserID = "admin-user-id"
	cfg.Notifications.Providers = []config.Config_Notification_Provider{
		{Provider: "Discord", Enabled: true, Discord: &config.Config_Notification_Discord{Webhook: "https://discord.example.com/hook"}},
	}

	store := &memoryConfigStore{values: map[string]string{}}
	config.SetStore(store)
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))
	if Err := cfg.Save(ctx); Err.Message != "" {
		t.Fatal(Err.Message)
	}
	store.saves = 0
	config.Current = cfg
	config.Valid = true
	config.AppFullyLoaded = true
	return store
}

func patchConfigSection(t *testing.T, section, body string) (int, updateConfigSectionResponse, string) {
	t.Helper()
	router := chi.NewRouter()
	router.Patch("/api/config/{section}", UpdateAppConfigSection)

	request := httptest.NewRequest(http.MethodPatch, "/api/config/"+section, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var response struct {
		Status string                      `json:"status"`
		Data   updateConfigSectionResponse `json:"data"`
		Error  *logging.LogErrorInfo       `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decoding response: %v\n%s", err, recorder.Body.String())
	}
	errorMessage := ""
	if response.Error != nil {
		errorMessage = response.Error.Message
	}
	return recorder.Code, response.Data, errorMessage
}

func TestUpdateConfigSectionSavesOnlyTheChangedField(t *testing.T) {
	store := setupConfigSectionTest(t)
	before := config.Current.Clone()

	code, response, errorMessage := patchConfigSection(t, "user_preferences", `{"default_add_to_db_only": true}`)
	if code != http.StatusOK || errorMessage != "" {
		t.Fatalf("status %d, error %q", code, errorMessage)
	}
	if !response.Changed {
		t.Error("response.Changed = false, want true")
	}
	if !response.Status.CurrentSetup.UserPreferences.DefaultAddToDBOnly {
		t.Error("response should include the updated config")
	}
	// The frontend stores this status, so it must be complete
	if !response.Status.AppFullyLoaded {
		t.Error("response status should include app_fully_loaded")
	}

	if !config.Current.UserPreferences.DefaultAddToDBOnly {
		t.Error("config.Current not updated")
	}
	if store.values["UserPreferences.DefaultAddToDBOnly"] != "true" {
		t.Errorf("stored value = %s, want true", store.values["UserPreferences.DefaultAddToDBOnly"])
	}

	// Everything else is unchanged, including fields hidden from the frontend
	after := config.Current.Clone()
	after.UserPreferences.DefaultAddToDBOnly = false
	if !configEqual(before, after) {
		t.Error("other settings changed")
	}
	if config.Current.Auth.APIKeyHash != "$argon2id$hash" || config.Current.MediaServer.UserID != "admin-user-id" {
		t.Error("hidden fields (APIKeyHash, UserID) should be kept")
	}
}

func TestUpdateConfigSectionMergesNestedFields(t *testing.T) {
	setupConfigSectionTest(t)

	// Only the client ID changes; the rest of the OIDC settings are kept
	code, response, errorMessage := patchConfigSection(t, "auth", `{"oidc": {"client_id": "new-client"}}`)
	if code != http.StatusOK || errorMessage != "" {
		t.Fatalf("status %d, error %q", code, errorMessage)
	}
	if !response.Changed {
		t.Error("response.Changed = false, want true")
	}
	oidc := config.Current.Auth.OIDC
	if oidc.ClientID != "new-client" || oidc.IssuerURL != "https://id.example.com" || oidc.ClientSecret != "client-secret" {
		t.Errorf("OIDC = %+v", oidc)
	}
}

func TestUpdateConfigSectionKeepsMaskedSecrets(t *testing.T) {
	store := setupConfigSectionTest(t)

	// Sending back the masked secret must not replace the real one
	code, response, errorMessage := patchConfigSection(t, "auth", `{"oidc": {"client_secret": "***cret"}}`)
	if code != http.StatusOK || errorMessage != "" {
		t.Fatalf("status %d, error %q", code, errorMessage)
	}
	if response.Changed || store.saves != 0 {
		t.Errorf("a masked secret should not count as a change (changed %v, saves %d)", response.Changed, store.saves)
	}
	if config.Current.Auth.OIDC.ClientSecret != "client-secret" {
		t.Errorf("ClientSecret = %q, want it kept", config.Current.Auth.OIDC.ClientSecret)
	}

	// The same for the password hash, which is masked too
	code, response, errorMessage = patchConfigSection(t, "auth", `{"password": "***hash"}`)
	if code != http.StatusOK || errorMessage != "" || response.Changed {
		t.Errorf("masked password: status %d, error %q, changed %v", code, errorMessage, response.Changed)
	}
	if !strings.HasSuffix(config.Current.Auth.Password, "password-hash") {
		t.Errorf("Password = %q, want it kept", config.Current.Auth.Password)
	}
	if strings.Contains(response.Status.CurrentSetup.Auth.Password, "argon2id") {
		t.Error("the password hash should be masked in the response")
	}
}

func TestUpdateConfigSectionReplacesLists(t *testing.T) {
	setupConfigSectionTest(t)
	original := config.Current.Notifications.Providers

	code, _, errorMessage := patchConfigSection(t, "notifications", `{"providers": [{"provider": "Gotify", "enabled": false, "gotify": {"url": "http://gotify", "api_token": "token"}}]}`)
	if code != http.StatusOK || errorMessage != "" {
		t.Fatalf("status %d, error %q", code, errorMessage)
	}
	providers := config.Current.Notifications.Providers
	if len(providers) != 1 || providers[0].Provider != "Gotify" || providers[0].Discord != nil {
		t.Errorf("providers = %+v, want only the new Gotify provider", providers)
	}
	// The previous config is not changed by the update
	if original[0].Provider != "Discord" || original[0].Discord == nil {
		t.Error("the update changed the previous config's providers")
	}
}

func TestUpdateConfigSectionRejectsInvalidChanges(t *testing.T) {
	tests := []struct {
		name, section, body, wantError string
	}{
		{"unknown section", "not_a_section", `{}`, "Unknown config section"},
		{"unknown field", "logging", `{"lvl": "DEBUG"}`, "Invalid changes for config section"},
		{"not an object", "logging", `"DEBUG"`, "Invalid request body"},
		{"wrong type", "logging", `{"level": 5}`, "Invalid changes for config section"},
		{"invalid value", "logging", `{"level": "LOUD"}`, "Logging.Level is not a valid level"},
		{"hidden field", "auth", `{"APIKeyHash": "x"}`, "Invalid changes for config section"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := setupConfigSectionTest(t)
			before := config.Current.Clone()

			code, _, errorMessage := patchConfigSection(t, test.section, test.body)
			if code != http.StatusInternalServerError || !strings.Contains(errorMessage, test.wantError) {
				t.Errorf("status %d, error %q, want an error containing %q", code, errorMessage, test.wantError)
			}
			if store.saves != 0 || !configEqual(before, config.Current) {
				t.Error("an invalid change should not change or save the config")
			}
		})
	}
}

func TestUpdateConfigSectionNoChanges(t *testing.T) {
	store := setupConfigSectionTest(t)

	code, response, errorMessage := patchConfigSection(t, "logging", `{"level": "INFO"}`)
	if code != http.StatusOK || errorMessage != "" {
		t.Fatalf("status %d, error %q", code, errorMessage)
	}
	if response.Changed || store.saves != 0 {
		t.Errorf("unchanged value should not be saved (changed %v, saves %d)", response.Changed, store.saves)
	}
}

func configEqual(a, b config.Config) bool {
	aValues, _ := a.ToValues()
	bValues, _ := b.ToValues()
	return maps.Equal(aValues, bValues) && a.MediaServer.UserID == b.MediaServer.UserID
}

// A setting without its own check must still be saved
func TestUpdateConfigSectionSavesSettingsWithoutTheirOwnCheck(t *testing.T) {
	store := setupConfigSectionTest(t)

	code, response, errorMessage := patchConfigSection(t, "images", `{"save_images_locally": {"running_on_windows": true}}`)
	if code != http.StatusOK || errorMessage != "" {
		t.Fatalf("status %d, error %q", code, errorMessage)
	}
	if !response.Changed || store.values["Images.SaveImagesLocally.RunningOnWindows"] != "true" {
		t.Errorf("changed %v, stored %q, want the setting saved", response.Changed, store.values["Images.SaveImagesLocally.RunningOnWindows"])
	}

	code, response, errorMessage = patchConfigSection(t, "auth", `{"trust_proxy_for_cookie_secure": true, "allowed_origins": ["https://app.example.com"]}`)
	if code != http.StatusOK || errorMessage != "" || !response.Changed {
		t.Fatalf("auth: status %d, error %q, changed %v", code, errorMessage, response.Changed)
	}
	if !config.Current.Auth.TrustProxyForCookieSecure || len(config.Current.Auth.AllowedOrigins) != 1 {
		t.Errorf("auth settings not saved: %+v", config.Current.Auth)
	}
}
