package config

import "aura/models"

var (
	// Config State Variables
	Current    Config
	ConfigPath string = ""

	// Flags to track config loading and validation status
	Loaded           bool   = false
	Valid            bool   = false
	MediaServerValid bool   = true
	MediaServerName  string = ""
	MediuxValid      bool   = true
	AppFullyLoaded   bool   = false
	AppLoadingStep   string = ""

	// App Details
	AppName    string = ""
	AppAuthor  string = ""
	AppLicense string = ""
	AppPort    int    = 0
	AppVersion string = ""
	// The commit, build number and build date of the image, empty for local builds
	AppCommit      string = ""
	AppBuildNumber string = ""
	AppBuildDate   string = ""
)

// Adding a new config field:
//  1. Add the field to its section's struct in config.go, with json and yaml tags.
//     The yaml name is the key stored in the database, so choose it carefully.
//     Use json:"-" only for values the frontend must never see, like APIKeyHash.
//
//  2. If the default isn't 0, "" or false, set it in DefaultConfig() in defaults.go.
//     Existing installs use this default until the setting is saved.
//
//  3. Optional: validate it in that section's Validate function in validate.go.
//
//  4. Optional: if something must happen when it changes, like restarting a job, compare it in that
//     section's checkConfigDifferences function and act on it in applyConfigChanges in routing/config/update.go.
//
//  5. If it's a secret, mask it in SanitizeConfig() in sanitize.go.
//
//  6. Update the schema snapshot: go test ./config -run TestConfigSchemaSnapshot -update
//
//  7. Frontend: add the field to types/config/config.ts and types/config/config-default-app.ts,
//     add a control to components/settings-onboarding/ConfigSection<Section>.tsx, and add a search entry
//     in helper/config/settings-search.ts. If it should only save with Test & Save, add it to
//     CONFIG_GROUP_KEYS in hooks/use-config-autosave.ts.
//
//  8. Document it in docs/config.md.
//
// Renaming, moving or changing the type of a field: add a config migration in database/sqlite_config_migrations.go
// Removing a field needs no migration, the stored value is ignored.

type Config struct {
	Auth            Config_Auth              `json:"auth" yaml:"Auth,omitempty"`                        // Authentication settings.
	Logging         Config_Logging           `json:"logging" yaml:"Logging,omitempty"`                  // Logging configuration settings.
	MediaServers    []Config_MediaServer     `json:"media_servers" yaml:"MediaServers"`                 // Every media server. One of them is the main server.
	MainMediaServer string                   `json:"main_media_server" yaml:"MainMediaServer"`          // ID of the main media server.
	MediaServer     Config_MediaServer       `json:"media_server" yaml:"-"`                             // The main media server, kept in sync with its entry in MediaServers. Not stored.
	Mediux          Config_Mediux            `json:"mediux" yaml:"Mediux,omitempty"`                    // MediUX integration settings.
	Jobs            Config_Jobs              `json:"jobs" yaml:"Jobs,omitempty"`                        // Background jobs scheduling configuration.
	Backups         Config_Backups           `json:"backups" yaml:"Backups,omitempty"`                  // Database backup settings.
	Images          Config_Images            `json:"images" yaml:"Images,omitempty"`                    // Image settings.
	TMDB            Config_TMDB              `json:"tmdb" yaml:"TMDB,omitempty"`                        // TMDB (The Movie Database) integration settings.
	LabelsAndTags   Config_LabelsAndTags     `json:"labels_and_tags" yaml:"LabelsAndTags,omitempty"`    // Labels and tags settings.
	Notifications   Config_Notifications     `json:"notifications" yaml:"Notifications,omitempty"`      // Notification settings.
	SonarrRadarr    Config_SonarrRadarr_Apps `json:"sonarr_radarr" yaml:"SonarrRadarr,omitempty"`       // List of Sonarr/Radarr instances to integrate with.
	UserPreferences Config_UserPreferences   `json:"user_preferences" yaml:"UserPreferences,omitempty"` // User preferences, shared between sessions and devices.
}

type Config_Dev struct {
	Enabled   bool   `json:"enabled" yaml:"Enabled,omitempty"`      // Whether to enable development mode.
	LocalPath string `json:"local_path" yaml:"LocalPath,omitempty"` // Local path for development mode.
}
type Config_Auth struct {
	Enabled         bool   `json:"enabled" yaml:"Enabled"`                  // Whether to enable authentication.
	PasswordEnabled bool   `json:"password_enabled" yaml:"PasswordEnabled"` // Whether logging in with the password is allowed. Turn it off to only log in with Single Sign-On.
	Password        string `json:"password" yaml:"Password,omitempty"`      // Password for authentication.

	// APIKeyHash is the Argon2id hash of the single global API key used for programmatic/integration
	// access (e.g. Sonarr/Radarr webhooks, scripts, other apps).
	APIKeyHash string `json:"-" yaml:"APIKeyHash,omitempty"`

	OIDC Config_Auth_OIDC `json:"oidc" yaml:"OIDC,omitempty"` // OIDC (Single Sign-On) login settings.

	// SessionCookieSecure controls the Secure attribute of the browser session cookie.
	SessionCookieSecure string `json:"session_cookie_secure,omitempty" yaml:"SessionCookieSecure,omitempty"`

	// TrustProxyForCookieSecure, when true, also treats X-Forwarded-Proto: https as a secure transport
	// signal (for reverse-proxy TLS termination setups). Defaults to false since that header is
	// attacker-controllable unless the proxy is guaranteed to strip/overwrite it.
	TrustProxyForCookieSecure bool `json:"trust_proxy_for_cookie_secure,omitempty" yaml:"TrustProxyForCookieSecure,omitempty"`

	// AllowedOrigins is an optional list of extra origins allowed to make credentialed
	// cross-origin requests (cookies/API key headers). The standard deployment (Next.js server
	// proxying /api/* to the Go backend) never needs this - the browser only ever talks to one
	// origin. Only set this if you're calling the API directly from a different origin.
	AllowedOrigins []string `json:"allowed_origins,omitempty" yaml:"AllowedOrigins,omitempty"`
}

type Config_Auth_OIDC struct {
	Enabled        bool     `json:"enabled" yaml:"Enabled"`                                    // Whether OIDC (Single Sign-On) login is enabled.
	IssuerURL      string   `json:"issuer_url,omitempty" yaml:"IssuerURL,omitempty"`           // OIDC issuer URL (used for discovery).
	ClientID       string   `json:"client_id,omitempty" yaml:"ClientID,omitempty"`             // OIDC client ID.
	ClientSecret   string   `json:"client_secret,omitempty" yaml:"ClientSecret,omitempty"`     // OIDC client secret.
	RedirectURLs   []string `json:"redirect_urls,omitempty" yaml:"RedirectURLs,omitempty"`     // Callback URLs registered with the IdP, one for each address aura is opened at, e.g. https://aura.example.com/api/auth/oidc/callback.
	AllowedEmails  []string `json:"allowed_emails,omitempty" yaml:"AllowedEmails,omitempty"`   // Optional allowlist of exact emails permitted to log in via OIDC. Empty = any authenticated IdP user is allowed.
	AllowedDomains []string `json:"allowed_domains,omitempty" yaml:"AllowedDomains,omitempty"` // Optional allowlist of email domains (e.g. "example.com") permitted to log in via OIDC. Empty = any domain allowed.
}

type Config_Logging struct {
	Level     string `json:"level" yaml:"Level"`          // Logging level (e.g., TRACE, DEBUG, INFO, WARN, ERROR).
	File      string `json:"file" yaml:"File,omitempty"`  // File path for logging output.
	KeepDays  int    `json:"keep_days" yaml:"KeepDays"`   // Number of days to keep old log files before they are deleted.
	KeepFiles int    `json:"keep_files" yaml:"KeepFiles"` // Number of old log files to keep before the oldest are deleted.
}

type Config_MediaServer struct {
	ID                           string                  `json:"id" yaml:"ID"`                                                          // Unique ID of the media server.
	Name                         string                  `json:"name" yaml:"Name"`                                                      // Name of the media server shown in aura.
	Enabled                      bool                    `json:"enabled" yaml:"Enabled"`                                                // Whether aura uses this media server. The main server is always enabled.
	Type                         string                  `json:"type" yaml:"Type"`                                                      // Type of media server (e.g., plex, emby, jellyfin).
	URL                          string                  `json:"url" yaml:"URL"`                                                        // Base URL of the media server. This is either the IP:Port or the domain name (e.g., plex.domain.com).
	ApiToken                     string                  `json:"api_token" yaml:"ApiToken"`                                             // Authentication token for accessing the media server.
	Libraries                    []models.LibrarySection `json:"libraries,omitempty" yaml:"Libraries,omitempty"`                        // List of media server libraries to manage.
	LibraryMap                   map[string]string       `json:"library_map,omitempty" yaml:"LibraryMap,omitempty"`                     // Which main server library each library matches, by title. Not used for the main server.
	UserID                       string                  `json:"user_id,omitempty" yaml:"-"`                                            // User ID for accessing the media server. This is used for Emby and Jellyfin servers. For Silo servers, this is the Profile ID.
	EnableSortByEpisodeAddedDate bool                    `json:"enable_sort_by_episode_added_date" yaml:"EnableSortByEpisodeAddedDate"` // Whether to check episodes for added date when getting Media Items. This is only for Plex servers.
	EnablePlexEventListener      bool                    `json:"enable_plex_event_listener" yaml:"EnablePlexEventListener"`             // Whether to enable the Plex event listener for reapplying images on refresh. Plex exclusive feature.
}

type Config_Mediux struct {
	ApiToken              string `json:"api_token" yaml:"ApiToken"`                           // Authentication token for accessing MediUX services.
	DownloadQuality       string `json:"download_quality" yaml:"DownloadQuality"`             // Quality of the media to download from MediUX (Options: "original", "optimized") Defaults to "optimized".
	SimultaneousDownloads int    `json:"simultaneous_downloads" yaml:"SimultaneousDownloads"` // How many images each download gets at once, and how many Download Queue jobs run at once. The settings page offers 1 to 5. Defaults to 3.
}

const DefaultSimultaneousDownloads = 3

// SimultaneousDownloads is how many images each download gets at once, and how many Download Queue jobs run at once
func SimultaneousDownloads() int {
	n := Current.Mediux.SimultaneousDownloads
	if n < 1 {
		return DefaultSimultaneousDownloads
	}
	return n
}

type Config_Jobs struct {
	AutoDownload                    Config_JobSetting `json:"auto_download" yaml:"AutoDownload,omitempty"`                                          // Schedule for auto-downloads.
	RefreshMediaItemsAndCollections Config_JobSetting `json:"refresh_media_items_and_collections" yaml:"RefreshMediaItemsAndCollections,omitempty"` // Schedule for refreshing media items and collections from media server.
	CheckForMediaItemChanges        Config_JobSetting `json:"check_for_media_item_changes" yaml:"CheckForMediaItemChanges,omitempty"`               // Schedule for checking rating keys and metadata changes.
	CheckForNewMediaItems           Config_JobSetting `json:"check_for_new_media_items" yaml:"CheckForNewMediaItems,omitempty"`                     // Schedule for adding the media server's newest items.
	HandleTempIgnoredItems          Config_JobSetting `json:"handle_temp_ignored_items" yaml:"HandleTempIgnoredItems,omitempty"`                    // Schedule for checking if temporarily ignored items now have sets on MediUX.
	RefreshMediuxUsers              Config_JobSetting `json:"refresh_mediux_users" yaml:"RefreshMediuxUsers,omitempty"`                             // Schedule for refreshing tracked MediUX users/creators.
	CheckMediuxSiteLink             Config_JobSetting `json:"check_mediux_site_link" yaml:"CheckMediuxSiteLink,omitempty"`                          // Schedule for checking MediUX site link availability.
	BackupDatabase                  Config_JobSetting `json:"backup_database" yaml:"BackupDatabase,omitempty"`                                      // Schedule for backing up the database.
	UpgradeSavedSets                Config_JobSetting `json:"upgrade_saved_sets" yaml:"UpgradeSavedSets,omitempty"`                                 // Schedule for upgrading saved sets to higher ranked favorite creators.
}

type Config_Backups struct {
	Keep int `json:"keep" yaml:"Keep"` // Number of database backups to keep before the oldest are deleted.
}

type Config_JobSetting struct {
	Enabled *bool  `json:"enabled,omitempty" yaml:"Enabled,omitempty"`
	Cron    string `json:"cron,omitempty" yaml:"Cron,omitempty"` // Cron expression for the job schedule.
}

type JobSettingDefault struct {
	Enabled bool
	Cron    string
}

// Resolve returns the effective enabled-state and cron for a job setting,
// falling back to def for any field the user hasn't explicitly set.
func (j Config_JobSetting) Resolve(def JobSettingDefault) (enabled bool, cron string) {
	enabled = def.Enabled
	if j.Enabled != nil {
		enabled = *j.Enabled
	}
	cron = j.Cron
	if cron == "" {
		cron = def.Cron
	}
	return enabled, cron
}

type Config_Images struct {
	CacheImages       Config_CacheImages       `json:"cache_images" yaml:"CacheImages"`              // Settings for caching images.
	SaveImagesLocally Config_SaveImagesLocally `json:"save_images_locally" yaml:"SaveImagesLocally"` // Settings for saving images locally alongside content.
}

type Config_CacheImages struct {
	Enabled bool `json:"enabled" yaml:"Enabled"` // Whether to enable caching of images.
}

type Config_SaveImagesLocally struct {
	Enabled          bool   `json:"enabled" yaml:"Enabled"`                                         // Whether to save images next to their content.
	Path             string `json:"path,omitempty" yaml:"Path,omitempty"`                           // By default, this is set to alongside the content. If set, this will override that behavior and save all images to this path.
	RunningOnWindows bool   `json:"running_on_windows,omitempty" yaml:"RunningOnWindows,omitempty"` // Whether the application is running on Windows. This affects path formatting.

	NameTemplates Config_ImageNameTemplates `json:"name_templates" yaml:"NameTemplates,omitempty"` // File names for each image type, empty uses the standard names
}

// PageViewPages are the pages whose sort and filter options are saved in the User Preferences
var PageViewPages = []string{"home", "saved_sets", "collections", "logs", "media_item", "download_queue", "statistics"}

// maxPageViewSize is the most a page's saved options can take, as JSON
const maxPageViewSize = 16 << 10

// DefaultImageNameTemplates are the standard names, they work with every media server
func DefaultImageNameTemplates() Config_ImageNameTemplates {
	return Config_ImageNameTemplates{
		Poster:              "poster",
		Backdrop:            "backdrop",
		SeasonPoster:        "season{{SeasonNumber}}-poster",
		SpecialSeasonPoster: "season-specials-poster",
		Titlecard:           "{{EpisodeFileName}}-thumb",
	}
}

type Config_ImageNameTemplates struct {
	Poster              string `json:"poster" yaml:"Poster,omitempty"`
	Backdrop            string `json:"backdrop" yaml:"Backdrop,omitempty"`
	SeasonPoster        string `json:"season_poster" yaml:"SeasonPoster,omitempty"`
	SpecialSeasonPoster string `json:"special_season_poster" yaml:"SpecialSeasonPoster,omitempty"`
	Titlecard           string `json:"titlecard" yaml:"Titlecard,omitempty"`
}

var ImageNameTemplateVariables = []string{
	"{{Title}}", "{{Year}}", "{{TMDBID}}", "{{LibraryTitle}}", "{{LibraryID}}", "{{Edition}}",
	"{{SeasonNumber}}", "{{EpisodeNumber}}", "{{EpisodeFileName}}",
}

type Config_TMDB struct {
	ApiToken string `json:"-" yaml:"ApiToken"` // API token for accessing TMDB (The Movie Database) services.
}

type Config_LabelsAndTags struct {
	Applications                           []Config_LabelsAndTagsProvider `json:"applications,omitempty" yaml:"Applications,omitempty"`
	RemoveOverlayLabelOnlyOnPosterDownload bool                           `json:"remove_overlay_label_only_on_poster_download,omitempty" yaml:"RemoveOverlayLabelOnlyOnPosterDownload,omitempty"` // Whether to remove the "Overlay" label from media items after downloading a poster image. This is to allow Kometa to reprocess the image and apply the overlays. This is a Plex exclusive feature and should be set to true if you have "Overlay" in your "Remove" list for the Plex application under "LabelsAndTags".
}

type Config_LabelsAndTagsProvider struct {
	Application                 string   `json:"application,omitempty" yaml:"Application,omitempty"`
	Enabled                     bool     `json:"enabled,omitempty" yaml:"Enabled,omitempty"`
	Add                         []string `json:"add,omitempty" yaml:"Add,omitempty"`
	Remove                      []string `json:"remove,omitempty" yaml:"Remove,omitempty"`
	AddLabelTagForSelectedTypes bool     `json:"add_label_tag_for_selected_types,omitempty" yaml:"AddLabelTagForSelectedTypes,omitempty"`
}

type Config_Notifications struct {
	Enabled              bool                           `json:"enabled" yaml:"Enabled"`                                    // Whether this notification method is enabled
	Providers            []Config_Notification_Provider `json:"providers,omitempty" yaml:"Providers,omitempty"`            // List of notification providers
	NotificationTemplate Config_NotificationTemplate    `json:"templates,omitempty" yaml:"NotificationTemplate,omitempty"` // Custom notification templates for different events
}

type Config_Notification_Provider struct {
	Provider string                        `json:"provider,omitempty" yaml:"Provider,omitempty"` // Notification provider
	Enabled  bool                          `json:"enabled,omitempty" yaml:"Enabled,omitempty"`   // Whether this notification method is enabled
	Discord  *Config_Notification_Discord  `json:"discord,omitempty" yaml:"Discord,omitempty"`   // Discord notification settings
	Pushover *Config_Notification_Pushover `json:"pushover,omitempty" yaml:"Pushover,omitempty"` // Pushover notification settings
	Gotify   *Config_Notification_Gotify   `json:"gotify,omitempty" yaml:"Gotify,omitempty"`     // Gotify notification settings
	Webhook  *Config_Notification_Webhook  `json:"webhook,omitempty" yaml:"Webhook,omitempty"`   // Webhook notification settings
}

type Config_Notification_Discord struct {
	Webhook string `json:"webhook,omitempty" yaml:"Webhook,omitempty"` // Webhook URL for the Discord notification provider.
}

type Config_Notification_Pushover struct {
	ApiToken string `json:"api_token,omitempty" yaml:"ApiToken,omitempty"` // Token for the Pushover notification provider.
	UserKey  string `json:"user_key,omitempty" yaml:"UserKey,omitempty"`   // UserKey for the Pushover notification provider.
}

type Config_Notification_Gotify struct {
	URL      string `json:"url,omitempty" yaml:"URL,omitempty"`            // URL for the Gotify notification provider.
	ApiToken string `json:"api_token,omitempty" yaml:"ApiToken,omitempty"` // Token for the Gotify notification provider.
}

type Config_Notification_Webhook struct {
	URL     string            `json:"url,omitempty" yaml:"URL,omitempty"`         // URL for the Webhook notification provider.
	Headers map[string]string `json:"headers,omitempty" yaml:"Headers,omitempty"` // Headers for the Webhook notification provider.
}

type Config_NotificationTemplate struct {
	// Any additional custom notification templates should be added here. You will also need to update the following files to ensure the new template is fully integrated:
	// - backend/config/defaults.go
	// - backend/config/template_variables.go
	// - backend/config/validate.go
	// - backend/routing/config/update.go
	// - backend/routing/validation/notification.go
	// - backend/utils/variable_filler.go
	// - frontend/src/components/settings-onboarding/ConfigSectionNotifications.tsx
	// - frontend/src/types/config/config.ts

	AppStartup                      Config_CustomNotification `json:"app_startup" yaml:"AppStartup,omitempty"`                                               // Custom notification settings for application startup.
	TestNotification                Config_CustomNotification `json:"test_notification" yaml:"TestNotification,omitempty"`                                   // Custom notification settings for test notifications.
	Autodownload                    Config_CustomNotification `json:"autodownload" yaml:"AutoDownload,omitempty"`                                            // Custom notification settings for auto-download events.
	DownloadQueue                   Config_CustomNotification `json:"download_queue" yaml:"DownloadQueue,omitempty"`                                         // Custom notification settings for download queue events.
	NewItemAutoDownload             Config_CustomNotification `json:"new_item_auto_download" yaml:"NewItemAutoDownload,omitempty"`                           // Custom notification settings for sets downloaded automatically from the favorite creators.
	NewSetsAvailableForIgnoredItems Config_CustomNotification `json:"new_sets_available_for_ignored_items" yaml:"NewSetsAvailableForIgnoredItems,omitempty"` // Custom notification settings for when new sets become available for ignored items.
	CheckForMediaItemChangesJob     Config_CustomNotification `json:"check_for_media_item_changes_job" yaml:"CheckForMediaItemChangesJob,omitempty"`         // Custom notification settings for the media item changes job.
	SonarrNotification              Config_CustomNotification `json:"sonarr_notification" yaml:"SonarrNotification,omitempty"`                               // Custom notification settings for Sonarr events.
	SiloPlugin                      Config_CustomNotification `json:"silo_plugin" yaml:"SiloPlugin,omitempty"`                                               // Custom notification settings for sets downloaded when Silo uses an image from the aura Silo plugin.
}

type Config_CustomNotification struct {
	Enabled      bool   `json:"enabled" yaml:"Enabled"`                                // Whether custom notifications are enabled.
	Title        string `json:"title,omitempty" yaml:"Title,omitempty"`                // Title for the custom notification.
	Message      string `json:"message,omitempty" yaml:"Message,omitempty"`            // Message for the custom notification.
	IncludeImage bool   `json:"include_image,omitempty" yaml:"IncludeImage,omitempty"` // Whether to include an image with the custom notification.
}

type Config_SonarrRadarr_Apps struct {
	Applications []Config_SonarrRadarrApp `json:"applications,omitempty" yaml:"Applications,omitempty"` // List of Sonarr/Radarr applications to integrate with.
}

type Config_SonarrRadarrApp struct {
	Type     string `json:"type,omitempty" yaml:"Type,omitempty"`          // Type of service (either "sonarr" or "radarr").
	Library  string `json:"library,omitempty" yaml:"Library,omitempty"`    // Name of the Media Server library associated with this Sonarr/Radarr instance.
	URL      string `json:"url,omitempty" yaml:"URL,omitempty"`            // Base URL of the Sonarr/Radarr server.
	ApiToken string `json:"api_token,omitempty" yaml:"ApiToken,omitempty"` // API key for accessing the Sonarr/Radarr server.
}

type Config_UserPreferences struct {
	DownloadDefaults                 []string                  `json:"download_defaults" yaml:"DownloadDefaults"`                                     // Image types selected by default in the Download Modal.
	ShowOnlyDownloadDefaults         bool                      `json:"show_only_download_defaults" yaml:"ShowOnlyDownloadDefaults"`                   // Whether to only show the default image types in the Download Modal.
	ShowDateModified                 bool                      `json:"show_date_modified" yaml:"ShowDateModified"`                                    // Whether to show the date modified on poster sets.
	DefaultAddToQueue                bool                      `json:"default_add_to_queue" yaml:"DefaultAddToQueue"`                                 // Whether "Add to Queue" is checked by default in the Download Modal.
	DefaultAddToDBOnly               bool                      `json:"default_add_to_db_only" yaml:"DefaultAddToDBOnly"`                              // Whether "Add to Database Only" is checked by default in the Download Modal.
	DefaultAutoAddNewCollectionItems bool                      `json:"default_auto_add_new_collection_items" yaml:"DefaultAutoAddNewCollectionItems"` // Whether "Auto Add New Collection Items" is checked by default in the Download Modal.
	DefaultAutoUpgrade               bool                      `json:"default_auto_upgrade" yaml:"DefaultAutoUpgrade"`                                // Whether "Auto Upgrade" is checked by default in the Download Modal.
	AutoDownloadNewItems             bool                      `json:"auto_download_new_items" yaml:"AutoDownloadNewItems"`                           // Whether images are downloaded automatically for new items, from the favorite creators.
	AutoUpgradeSets                  bool                      `json:"auto_upgrade_sets" yaml:"AutoUpgradeSets"`                                      // Whether to automatically upgrade sets when available.
	FavoriteCreators                 []string                  `json:"favorite_creators" yaml:"FavoriteCreators"`                                     // MediUX usernames ranked best first, the first is rank 1.
	PageViews                        map[string]map[string]any `json:"page_views" yaml:"PageViews,omitempty"`                                         // Each page's sort and filter options, by page, so they're the same on every device.
}
