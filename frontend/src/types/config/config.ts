import type { TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS } from "@/types/ui-options";

export interface AppConfig {
  auth: AppConfigAuth;
  logging: AppConfigLogging; // Logging configuration settings
  media_server: AppConfigMediaServer; // The main media server
  media_servers?: AppConfigMediaServer[]; // Every media server, including the main one
  main_media_server?: string; // ID of the main media server
  mediux: AppConfigMediux; // MediUX integration settings
  jobs: AppConfigJobs; // Background jobs scheduling configuration
  backups: AppConfigBackups; // Database backup settings
  images: AppConfigImages;
  tmdb: AppConfigTMDB; // TMDB (The Movie Database) integration settings
  labels_and_tags: AppConfigLabelsAndTags; // Labels and tags management settings
  notifications: AppConfigNotifications; // Notification settings
  sonarr_radarr: AppConfigSonarrRadarrApps; // List of Sonarr/Radarr instances to integrate with
  user_preferences: AppConfigUserPreferences; // User preferences, shared between sessions and devices
}

export interface AppConfigAuth {
  enabled: boolean; // Whether authentication is enabled
  password: string; // Hashed password for authentication
  oidc: AppConfigAuthOIDC;
  session_cookie_secure?: string; // "auto" | "always" | "never"
  trust_proxy_for_cookie_secure?: boolean;
  allowed_origins?: string[];
}

export interface AppConfigAuthOIDC {
  enabled: boolean;
  issuer_url?: string;
  client_id?: string;
  client_secret?: string; // Masked (e.g. "***ab12") once a value is set - only sent back if changed.
  redirect_url?: string;
  allowed_emails?: string[];
  allowed_domains?: string[];
}

export interface AppConfigLogging {
  level: string; // Logging level (e.g., DEBUG, INFO, WARN, ERROR)
  file?: string; // Log file path
  keep_days?: number; // Number of days to keep old log files before they are deleted
  keep_files?: number; // Number of old log files to keep before the oldest are deleted
}

export interface AppConfigMediaServer {
  id?: string; // Unique ID of the media server
  name?: string; // Name of the media server shown in aura
  enabled?: boolean; // Whether aura uses this media server. The main server is always enabled
  type: string; // Type of media server (e.g., plex, emby, jellyfin)
  url: string; // Base URL of the media server
  api_token: string; // Authentication token for accessing the media server
  libraries: AppConfigMediaServerLibrary[]; // List of media server libraries to manage
  user_id?: string; // User ID for accessing the media server (optional for Emby/Jellyfin)
  enable_sort_by_episode_added_date?: boolean; // Whether to enable sorting shows by latest episode added date (Plex only)
  enable_plex_event_listener?: boolean; // Whether to enable the Plex event listener for reapplying images on refresh (Plex only)
  library_map?: Record<string, string>; // Which main server library each library matches, by title. Not used for the main server
}

export interface AppConfigMediaServerLibrary {
  title: string; // Name of the library
  id: string; // Unique identifier for the library section
  type: string; // Type of the library (e.g., movie, show)
  path?: string; // Filesystem path to the library
}

export interface AppConfigMediux {
  api_token: string; // Authentication token for accessing MediUX services
  download_quality: string; // Preferred download quality (e.g., "original", "optimized")
}

export interface AppConfigJobs {
  auto_download: AppConfigJobSetting; // Schedule for auto-downloads
  refresh_media_items_and_collections: AppConfigJobSetting; // Schedule for refreshing media items and collections from media server
  check_for_media_item_changes: AppConfigJobSetting; // Schedule for checking rating keys and metadata changes
  handle_temp_ignored_items: AppConfigJobSetting; // Schedule for checking if temporarily ignored items now have sets on MediUX
  refresh_mediux_users: AppConfigJobSetting; // Schedule for refreshing tracked MediUX users/creators
  check_mediux_site_link: AppConfigJobSetting; // Schedule for checking MediUX site link availability
  backup_database: AppConfigJobSetting; // Schedule for backing up the database
  upgrade_saved_sets: AppConfigJobSetting; // Schedule for upgrading saved sets to higher ranked favorite creators
}

export interface AppConfigBackups {
  keep: number; // Number of database backups to keep before the oldest are deleted
}

export interface AppConfigJobSetting {
  enabled?: boolean; // Whether the job is enabled for automatic scheduling.
  cron: string; // Cron expression for the job schedule.
}

export interface AppConfigImages {
  cache_images: AppConfigCacheImages;
  save_images_locally: AppConfigSaveImagesLocally;
}

export interface AppConfigCacheImages {
  enabled: boolean; // Whether to enable caching of images.
}

export interface AppConfigSaveImagesLocally {
  enabled: boolean; // Whether to save images locally.
  path: string; // Path to save images locally. If empty, images will be saved next to content.
  running_on_windows?: boolean; // Convert Windows paths from the media server like C:\Movies to /C/Movies
  name_templates?: AppConfigImageNameTemplates; // File names for each image type
}

export interface AppConfigImageNameTemplates {
  poster: string;
  backdrop: string;
  season_poster: string;
  special_season_poster: string;
  titlecard: string;
}

export interface AppConfigTMDB {
  api_token: string; // API key for accessing TMDB services
}

export interface AppConfigLabelsAndTags {
  applications: AppConfigLabelsAndTagsApplication[];
  remove_overlay_label_only_on_poster_download: boolean; // Whether to remove the "Overlay" label from media items after downloading a poster image. This is to allow Kometa to reprocess the image and apply the overlays. This is a Plex exclusive feature and should be set to true if you have "Overlay" in your "Remove" list for the Plex application under "LabelsAndTags".
}

export interface AppConfigLabelsAndTagsApplication {
  application: string; // Name of the application (e.g., Plex)
  enabled: boolean; // Whether label/tag management is enabled for this application
  add: string[]; // List of labels/tags to add
  remove: string[]; // List of labels/tags to remove
  add_label_tag_for_selected_types: boolean; // Whether to add labels/tags for selected media types (poster, backdrop, etc.)
}

export interface AppConfigNotifications {
  enabled: boolean;
  providers: AppConfigNotificationProviders[];
  templates: Partial<AppConfigNotificationTemplate>;
}

export interface AppConfigNotificationProviders {
  provider: string;
  enabled: boolean;
  discord?: AppConfigNotificationDiscord;
  pushover?: AppConfigNotificationPushover;
  gotify?: AppConfigNotificationGotify;
  webhook?: AppConfigNotificationWebhook;
}

export interface AppConfigNotificationDiscord {
  webhook: string;
}

export interface AppConfigNotificationPushover {
  user_key: string;
  api_token: string;
}

export interface AppConfigNotificationGotify {
  url: string;
  api_token: string;
}

export interface AppConfigNotificationWebhook {
  url: string;
  headers: { [key: string]: string }; // Key-value pairs for custom headers
}

export interface AppConfigNotificationTemplate {
  app_startup: AppConfigNotificationCustomNotification;
  test_notification: AppConfigNotificationCustomNotification;
  autodownload: AppConfigNotificationCustomNotification;
  download_queue: AppConfigNotificationCustomNotification;
  new_item_auto_download: AppConfigNotificationCustomNotification;
  new_sets_available_for_ignored_items: AppConfigNotificationCustomNotification;
  check_for_media_item_changes_job: AppConfigNotificationCustomNotification;
  sonarr_notification: AppConfigNotificationCustomNotification;
  silo_plugin: AppConfigNotificationCustomNotification;
}

export interface NotificationTemplateVariablesCatalog {
  template_variables: Record<string, string[]>;
  default_templates?: AppConfigNotificationTemplate;
}

export interface AppConfigNotificationCustomNotification {
  enabled: boolean;
  title: string;
  message: string;
  include_image: boolean;
}

export interface AppConfigSonarrRadarrApps {
  applications: AppConfigSonarrRadarrApp[];
}
export interface AppConfigSonarrRadarrApp {
  type: string; // Type of service (either "sonarr" or "radarr").
  library: string; // Name of the Media Server library associated with this Sonarr/Radarr instance.
  url: string; // Base URL of the Sonarr/Radarr server.
  api_token: string; // API key for accessing the Sonarr/Radarr server.
}

export interface AppConfigUserPreferences {
  download_defaults: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[]; // Image types selected by default in the Download Modal
  show_only_download_defaults: boolean; // Whether to only show the default image types in the Download Modal
  show_date_modified: boolean; // Whether to show the date modified on poster sets
  default_add_to_queue: boolean; // Whether "Add to Queue" is checked by default in the Download Modal
  default_auto_add_new_collection_items?: boolean; // Whether "Auto Add New Collection Items" is checked by default in the Download Modal
  default_add_to_db_only: boolean; // Whether "Add to Database Only" is checked by default in the Download Modal
  auto_download_new_items: boolean; // Whether new items get images from the favorite creators automatically
  auto_upgrade_sets: boolean; // Whether to automatically upgrade sets to a better version when available
  favorite_creators: string[]; // MediUX usernames ranked best first
  page_views?: Record<string, PageView>; // Each page's sort and filter options, so they're the same on every device
}

// PageView is a page's sort and filter options
export type PageView = Record<string, unknown>;
