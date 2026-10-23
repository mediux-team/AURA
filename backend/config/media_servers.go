package config

import (
	"aura/logging"
	"aura/models"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// NewMediaServerID returns a random ID for a media server
func NewMediaServerID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// IsMain reports whether this is the main media server
func (server Config_MediaServer) IsMain() bool {
	return server.ID == Current.MainMediaServer
}

// SavedMediaServerToken returns the saved API token of a media server, for a token sent back masked.
// An empty ID is the main media server. A new media server has no saved token.
func SavedMediaServerToken(serverID string) string {
	if serverID == "" {
		return Current.MediaServer.ApiToken
	}
	for _, server := range Current.MediaServers {
		if server.ID == serverID {
			return server.ApiToken
		}
	}
	return ""
}

// SetMediaServerUserID sets the User ID of a media server in the current config. The User ID isn't saved.
func SetMediaServerUserID(serverID, userID string) {
	UpdateLock.Lock()
	defer UpdateLock.Unlock()
	for i := range Current.MediaServers {
		if Current.MediaServers[i].ID == serverID {
			Current.MediaServers[i].UserID = userID
		}
	}
}

// clone returns a deep copy of the media server
func (server Config_MediaServer) clone() Config_MediaServer {
	var c Config_MediaServer
	deepCopyValue(reflect.ValueOf(&c).Elem(), reflect.ValueOf(server))
	return c
}

// mainMediaServerIndex returns the index of the main media server in MediaServers, or -1
func (config *Config) mainMediaServerIndex() int {
	for i, server := range config.MediaServers {
		if server.ID != "" && server.ID == config.MainMediaServer {
			return i
		}
	}
	return -1
}

// LoadMainMediaServer copies the main media server from MediaServers into MediaServer
func (config *Config) LoadMainMediaServer() {
	if i := config.mainMediaServerIndex(); i >= 0 {
		config.MediaServer = config.MediaServers[i].clone()
		return
	}
	config.MediaServer = Config_MediaServer{}
}

// SyncMediaServers copies MediaServer into its entry in MediaServers, and fills in missing values
func (config *Config) SyncMediaServers() {
	// Copy the main media server into its entry
	if i := config.mainMediaServerIndex(); i >= 0 {
		current := config.MediaServers[i]
		server := config.MediaServer.clone()
		server.ID = current.ID
		if server.Name == "" {
			server.Name = current.Name
		}
		// A name that is just the type follows the type
		if server.Name == current.Type {
			server.Name = server.Type
		}
		config.MediaServers[i] = server
	} else if config.MainMediaServer == "" && config.MediaServer.Type != "" {
		// No main media server yet, so add it first
		server := config.MediaServer.clone()
		server.ID = NewMediaServerID()
		if server.Name == "" {
			server.Name = server.Type
		}
		config.MediaServers = append([]Config_MediaServer{server}, config.MediaServers...)
		config.MainMediaServer = server.ID
	}

	// The main media server is always enabled and has no library map
	mainLibraries := map[string]bool{}
	if i := config.mainMediaServerIndex(); i >= 0 {
		main := &config.MediaServers[i]
		main.Enabled = true
		main.LibraryMap = nil
		for _, library := range main.Libraries {
			mainLibraries[library.Title] = true
		}
	}

	for i := range config.MediaServers {
		server := &config.MediaServers[i]
		if server.ID == "" {
			server.ID = NewMediaServerID()
		}
		if server.Name == "" {
			server.Name = server.Type
		}
		if server.ID == config.MainMediaServer {
			continue
		}
		server.LibraryMap = libraryMapWithDefaults(server.Libraries, server.LibraryMap, mainLibraries)
	}

	// Keep the main media server's ID and name in MediaServer too
	if i := config.mainMediaServerIndex(); i >= 0 {
		config.MediaServer.ID = config.MediaServers[i].ID
		config.MediaServer.Name = config.MediaServers[i].Name
		config.MediaServer.Enabled = true
		config.MediaServer.LibraryMap = nil
	}
}

// libraryMapWithDefaults matches libraries to main server libraries with the same title, unless set by the user.
// An empty value means the library doesn't match any main server library.
func libraryMapWithDefaults(libraries []models.LibrarySection, libraryMap map[string]string, mainLibraries map[string]bool) map[string]string {
	result := map[string]string{}
	for _, library := range libraries {
		mainLibrary, set := libraryMap[library.Title]
		switch {
		case !set && mainLibraries[library.Title]:
			mainLibrary = library.Title
		case set && mainLibrary != "" && !mainLibraries[mainLibrary]:
			// The main server library was removed
			mainLibrary = ""
		case !set:
			continue
		}
		result[library.Title] = mainLibrary
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func ValidateMediaServers(ctx context.Context, servers []Config_MediaServer, mainMediaServer string) bool {
	_, logAction := logging.AddSubActionToContext(ctx, "Validating MediaServers Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// Check that IDs are unique
	ids := map[string]bool{}
	for _, server := range servers {
		if ids[server.ID] {
			logAction.SetError(fmt.Sprintf("MediaServers: '%s' is used by more than one media server", server.ID), "Each media server needs its own ID", nil)
			isValid = false
		}
		ids[server.ID] = true
	}

	// Check that MainMediaServer is one of the media servers
	if len(servers) > 0 && !ids[mainMediaServer] {
		logAction.SetError("MainMediaServer is not one of the media servers", "Choose which media server is the main one", nil)
		isValid = false
	}

	// Check the other media servers the same way as the main one, which is checked on its own
	for i := range servers {
		server := &servers[i]
		if server.ID == mainMediaServer || !server.Enabled {
			continue
		}
		name := server.Name
		if name == "" {
			name = server.ID
		}
		if !validateMediaServer(ctx, server, name) {
			isValid = false
		}
	}

	return isValid
}

// ParseConfigYAML reads a config file. An older MediaServer becomes the main media server.
func ParseConfigYAML(data []byte) (cfg Config, err error) {
	// Older config files don't have User Preferences, so start from the defaults
	cfg = Config{UserPreferences: DefaultUserPreferences()}
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	// Older config files name title cards with EpisodeNamingConvention, static keeps only the episode number
	var episodeNaming struct {
		Images struct {
			SaveImagesLocally struct {
				EpisodeNamingConvention string `yaml:"EpisodeNamingConvention"`
			} `yaml:"SaveImagesLocally"`
		} `yaml:"Images"`
	}
	if yaml.Unmarshal(data, &episodeNaming) == nil && cfg.Images.SaveImagesLocally.NameTemplates.Titlecard == "" &&
		strings.EqualFold(episodeNaming.Images.SaveImagesLocally.EpisodeNamingConvention, "static") {
		cfg.Images.SaveImagesLocally.NameTemplates.Titlecard = "S{{SeasonNumber}}E{{EpisodeNumber}}"
	}
	// Older config files have one OIDC RedirectURL, and always allow the password
	var olderAuth struct {
		Auth struct {
			PasswordEnabled *bool `yaml:"PasswordEnabled"`
			OIDC            struct {
				RedirectURL string `yaml:"RedirectURL"`
			} `yaml:"OIDC"`
		} `yaml:"Auth"`
	}
	if yaml.Unmarshal(data, &olderAuth) == nil {
		if olderAuth.Auth.PasswordEnabled == nil {
			cfg.Auth.PasswordEnabled = true
		}
		if len(cfg.Auth.OIDC.RedirectURLs) == 0 && olderAuth.Auth.OIDC.RedirectURL != "" {
			cfg.Auth.OIDC.RedirectURLs = []string{olderAuth.Auth.OIDC.RedirectURL}
		}
	}

	if len(cfg.MediaServers) > 0 {
		cfg.LoadMainMediaServer()
		cfg.SyncMediaServers()
		return cfg, nil
	}

	var older struct {
		MediaServer Config_MediaServer `yaml:"MediaServer"`
	}
	if err = yaml.Unmarshal(data, &older); err != nil {
		return cfg, err
	}
	cfg.MediaServer = older.MediaServer
	cfg.SyncMediaServers()
	return cfg, nil
}

// RestoreMediaServerTokens keeps the current API token of each media server sent back masked
func RestoreMediaServerTokens(current, updated []Config_MediaServer) {
	tokens := map[string]string{}
	for _, server := range current {
		tokens[server.ID] = server.ApiToken
	}
	for i := range updated {
		token, ok := tokens[updated[i].ID]
		if ok && updated[i].ApiToken != token && strings.HasPrefix(updated[i].ApiToken, "***") {
			updated[i].ApiToken = token
		}
	}
}

// SwitchMainMediaServer makes another media server the main one, and converts the library maps.
// It returns which library on the new main media server each library on the old one matches.
func SwitchMainMediaServer(config *Config, newMainID string) (libraryChanges map[string]string, err error) {
	newMain := -1
	for i, server := range config.MediaServers {
		if server.ID == newMainID {
			newMain = i
		}
	}
	switch {
	case newMain < 0:
		return nil, fmt.Errorf("media server '%s' doesn't exist", newMainID)
	case newMainID == config.MainMediaServer:
		return nil, fmt.Errorf("'%s' is already the main media server", config.MediaServers[newMain].Name)
	case !config.MediaServers[newMain].Enabled:
		return nil, fmt.Errorf("'%s' is turned off", config.MediaServers[newMain].Name)
	}

	// Each old main library matches the first library of the new main media server mapped to it
	libraryChanges = map[string]string{}
	for _, library := range mappedLibraryTitles(config.MediaServers[newMain]) {
		oldLibrary := config.MediaServers[newMain].LibraryMap[library]
		if _, found := libraryChanges[oldLibrary]; !found {
			libraryChanges[oldLibrary] = library
		}
	}

	oldMainID := config.MainMediaServer
	for i := range config.MediaServers {
		server := &config.MediaServers[i]
		switch server.ID {
		case newMainID:
			server.LibraryMap = nil
		case oldMainID:
			// The old main media server matches the new main media server's libraries
			server.LibraryMap = map[string]string{}
			for _, library := range server.Libraries {
				server.LibraryMap[library.Title] = libraryChanges[library.Title]
			}
		default:
			for library, oldLibrary := range server.LibraryMap {
				server.LibraryMap[library] = libraryChanges[oldLibrary]
			}
		}
	}
	for i := range config.SonarrRadarr.Applications {
		app := &config.SonarrRadarr.Applications[i]
		if newLibrary, found := libraryChanges[app.Library]; found {
			app.Library = newLibrary
		}
	}

	config.MainMediaServer = newMainID
	config.LoadMainMediaServer()
	return libraryChanges, nil
}

// mappedLibraryTitles returns the libraries of a media server that match a main server library, sorted by title
func mappedLibraryTitles(server Config_MediaServer) []string {
	var libraries []string
	for library, mainLibrary := range server.LibraryMap {
		if mainLibrary != "" {
			libraries = append(libraries, library)
		}
	}
	sort.Strings(libraries)
	return libraries
}
