// Searchable settings, used by the navbar search and for setting names in the undo toast.
// label must match the setting's label in its card, so the Settings page can highlight it.
// configPath is the setting's path in the config section, if it has one.

export interface SettingsSearchEntry {
  sectionId: string; // Settings page section, from SETTINGS_SECTIONS
  sectionLabel: string;
  label: string;
  description: string;
  keywords?: string[];
  configPath?: string;
}

export const SETTINGS_SEARCH_ENTRIES: SettingsSearchEntry[] = [
  // MediUX
  {
    sectionId: "mediux",
    sectionLabel: "MediUX",
    label: "Token",
    description: "Your MediUX API token",
    keywords: ["api", "key", "mediux token"],
    configPath: "api_token",
  },
  {
    sectionId: "mediux",
    sectionLabel: "MediUX",
    label: "Download Quality",
    description: "Download original or optimized images",
    keywords: ["original", "optimized", "resolution"],
    configPath: "download_quality",
  },

  // Media Server
  {
    sectionId: "media-server",
    sectionLabel: "Media Servers",
    label: "Name",
    description: "The name shown for a media server",
    keywords: ["server name", "rename"],
    configPath: "name",
  },
  {
    sectionId: "media-server",
    sectionLabel: "Media Servers",
    label: "Other Media Servers",
    description: "Apply images to more Plex, Emby, Jellyfin or Silo servers",
    keywords: ["multiple", "second", "add server", "another server", "secondary"],
  },
  {
    sectionId: "media-server",
    sectionLabel: "Media Servers",
    label: "Library Mapping",
    description: "Which main server library each library of another media server matches",
    keywords: ["map", "match", "libraries", "other server"],
  },
  {
    sectionId: "media-server",
    sectionLabel: "Media Servers",
    label: "Type",
    description: "Plex, Emby, Jellyfin or Silo",
    keywords: ["plex", "emby", "jellyfin", "silo", "server"],
    configPath: "type",
  },
  {
    sectionId: "media-server",
    sectionLabel: "Media Servers",
    label: "URL",
    description: "The address of your media server",
    keywords: ["address", "host", "port", "server url"],
    configPath: "url",
  },
  {
    sectionId: "media-server",
    sectionLabel: "Media Servers",
    label: "Token",
    description: "The media server API token or key",
    keywords: ["api", "key", "plex token"],
    configPath: "api_token",
  },
  {
    sectionId: "media-server",
    sectionLabel: "Media Servers",
    label: "Libraries",
    description: "The libraries aura manages",
    keywords: ["movies", "shows", "library", "sections"],
    configPath: "libraries",
  },
  {
    sectionId: "media-server",
    sectionLabel: "Media Servers",
    label: 'Enable "Sort by Episode Added Date"',
    description: "Sort shows by the date their latest episode was added (Plex)",
    keywords: ["sort", "episode", "added"],
    configPath: "enable_sort_by_episode_added_date",
  },
  {
    sectionId: "media-server",
    sectionLabel: "Media Servers",
    label: "Enable Plex Websocket Listener",
    description: "Reapply images when Plex refreshes an item",
    keywords: ["websocket", "event", "listener", "reapply", "refresh"],
    configPath: "enable_plex_event_listener",
  },

  // Sonarr & Radarr
  {
    sectionId: "sonarr-radarr",
    sectionLabel: "Sonarr & Radarr",
    label: "Sonarr & Radarr",
    description: "Sonarr and Radarr instances",
    keywords: ["sonarr", "radarr", "arr", "webhook"],
    configPath: "applications",
  },

  // Authentication
  {
    sectionId: "authentication",
    sectionLabel: "Authentication",
    label: "Enabled",
    description: "Require a login to use aura",
    keywords: ["auth", "login", "password", "security"],
    configPath: "enabled",
  },
  {
    sectionId: "authentication",
    sectionLabel: "Authentication",
    label: "Password Enabled",
    description: "Allow logging in with the password",
    keywords: ["password", "login", "sso only"],
    configPath: "password_enabled",
  },
  {
    sectionId: "authentication",
    sectionLabel: "Authentication",
    label: "Argon2id Password Hash",
    description: "The password used to log in",
    keywords: ["password", "hash", "login"],
    configPath: "password",
  },
  {
    sectionId: "authentication",
    sectionLabel: "Authentication",
    label: "Single Sign-On (OIDC)",
    description: "Log in with an identity provider",
    keywords: ["oidc", "sso", "authentik", "authelia", "openid", "redirect url", "callback"],
    configPath: "oidc",
  },
  {
    sectionId: "authentication",
    sectionLabel: "Authentication",
    label: "Session Cookie Secure",
    description: "When the login cookie is only sent over HTTPS (Advanced)",
    keywords: ["cookie", "https", "secure", "session", "advanced"],
    configPath: "session_cookie_secure",
  },
  {
    sectionId: "authentication",
    sectionLabel: "Authentication",
    label: "Trust Reverse Proxy for HTTPS",
    description: "Treat requests as HTTPS when the reverse proxy says so (Advanced)",
    keywords: ["proxy", "reverse proxy", "x-forwarded-proto", "https", "advanced"],
    configPath: "trust_proxy_for_cookie_secure",
  },
  {
    sectionId: "authentication",
    sectionLabel: "Authentication",
    label: "Allowed Origins",
    description: "Other websites allowed to call the aura API (Advanced)",
    keywords: ["cors", "origins", "api", "advanced"],
    configPath: "allowed_origins",
  },
  {
    sectionId: "authentication",
    sectionLabel: "Authentication",
    label: "API Key",
    description: "The API key for scripts and integrations",
    keywords: ["api", "key", "integration", "webhook"],
  },

  // Images
  {
    sectionId: "images",
    sectionLabel: "Images",
    label: "Cache Images",
    description: "Cache images to load pages faster",
    keywords: ["cache", "performance"],
    configPath: "cache_images.enabled",
  },
  {
    sectionId: "images",
    sectionLabel: "Images",
    label: "Save Images Locally",
    description: "Save a copy of applied images next to your media",
    keywords: ["save", "local", "poster.jpg", "files"],
    configPath: "save_images_locally.enabled",
  },
  {
    sectionId: "images",
    sectionLabel: "Images",
    label: "Path",
    description: "A different folder to save images to",
    keywords: ["folder", "directory", "save path"],
    configPath: "save_images_locally.path",
  },
  {
    sectionId: "images",
    sectionLabel: "Images",
    label: "File Names",
    description: "How saved images are named, like poster, backdrop and title cards",
    keywords: ["file names", "naming", "episode", "titlecard", "poster", "agregarr"],
    configPath: "save_images_locally.name_templates",
  },

  {
    sectionId: "images",
    sectionLabel: "Images",
    label: "Running on Windows",
    description: "Convert Windows paths from the media server",
    keywords: ["windows", "paths", "drive"],
    configPath: "save_images_locally.running_on_windows",
  },

  // Labels & Tags
  {
    sectionId: "labels-and-tags",
    sectionLabel: "Labels & Tags",
    label: "Labels & Tags",
    description: "Labels and tags added or removed when images are applied",
    keywords: ["label", "tag", "kometa", "overlay"],
    configPath: "applications",
  },
  {
    sectionId: "labels-and-tags",
    sectionLabel: "Labels & Tags",
    label: 'Remove "Overlay" label only on poster download',
    description: "Only remove the Overlay label when a poster is downloaded",
    keywords: ["overlay", "kometa"],
    configPath: "remove_overlay_label_only_on_poster_download",
  },

  // Notifications
  {
    sectionId: "notifications",
    sectionLabel: "Notifications",
    label: "Notifications",
    description: "Discord, Pushover, Gotify and webhook notifications",
    keywords: ["discord", "pushover", "gotify", "webhook", "alerts", "providers"],
    configPath: "providers",
  },
  {
    sectionId: "notifications",
    sectionLabel: "Notifications",
    label: "Custom Notification Templates",
    description: "The title and message of each notification",
    keywords: ["template", "message", "title"],
    configPath: "templates",
  },

  // Logging
  {
    sectionId: "logging",
    sectionLabel: "Logging",
    label: "Level",
    description: "How much aura logs",
    keywords: ["log level", "debug", "trace", "verbose"],
    configPath: "level",
  },
  {
    sectionId: "logging",
    sectionLabel: "Logging",
    label: "Log File",
    description: "View or clear the log files",
    keywords: ["logs", "view logs", "clear logs"],
  },
  {
    sectionId: "logging",
    sectionLabel: "Logging",
    label: "Keep Old Logs (days)",
    description: "How many days old log files are kept",
    keywords: ["log rotation", "delete old logs", "keep logs", "retention"],
    configPath: "keep_days",
  },
  {
    sectionId: "logging",
    sectionLabel: "Logging",
    label: "Keep Old Logs (files)",
    description: "How many old log files are kept",
    keywords: ["log rotation", "delete old logs", "keep logs", "retention", "backups"],
    configPath: "keep_files",
  },
  {
    sectionId: "backups",
    sectionLabel: "Backups",
    label: "Keep Backups",
    description: "How many database backups are kept",
    keywords: ["backup", "database", "restore", "retention"],
    configPath: "keep",
  },
  {
    sectionId: "backups",
    sectionLabel: "Backups",
    label: "Back Up Now",
    description: "Back up the database now",
    keywords: ["backup", "database", "restore", "copy"],
  },

  // User Preferences
  {
    sectionId: "user-preferences",
    sectionLabel: "User Preferences",
    label: "View Density",
    description: "How compact item listings are",
    keywords: ["density", "compact", "grid", "size"],
  },
  {
    sectionId: "user-preferences",
    sectionLabel: "User Preferences",
    label: "Download Defaults",
    description: "Image types selected by default when downloading",
    keywords: ["download", "defaults", "poster", "backdrop", "titlecard", "collection poster", "collection backdrop"],
    configPath: "download_defaults",
  },
  {
    sectionId: "user-preferences",
    sectionLabel: "User Preferences",
    label: "Only Show Download Defaults",
    description: "Only show the default image types",
    keywords: ["download", "defaults", "filter"],
    configPath: "show_only_download_defaults",
  },
  {
    sectionId: "user-preferences",
    sectionLabel: "User Preferences",
    label: "Add to Queue by Default",
    description: 'Check "Add to Queue" when downloading',
    keywords: ["queue", "download"],
    configPath: "default_add_to_queue",
  },
  {
    sectionId: "user-preferences",
    sectionLabel: "User Preferences",
    label: "Add to Database Only by Default",
    description: 'Check "Add to Database Only" when downloading',
    keywords: ["database", "save", "download"],
    configPath: "default_add_to_db_only",
  },
  {
    sectionId: "user-preferences",
    sectionLabel: "User Preferences",
    label: "Auto Add New Collection Items by Default",
    description: 'Check "Auto Add New Collection Items" when downloading a collection set',
    keywords: ["collection", "auto add", "new movies", "download"],
    configPath: "default_auto_add_new_collection_items",
  },
  {
    sectionId: "user-preferences",
    sectionLabel: "User Preferences",
    label: "Show Date Modified",
    description: "Show when poster sets were last modified",
    keywords: ["date", "modified", "updated"],
    configPath: "show_date_modified",
  },
  {
    sectionId: "user-preferences",
    sectionLabel: "User Preferences",
    label: "Automatically Download Images for New Items",
    description: "Automatically download images for new items",
    keywords: ["download", "new items", "auto download"],
    configPath: "auto_download_new_items",
  },
  {
    sectionId: "user-preferences",
    sectionLabel: "User Preferences",
    label: "Automatically Upgrade Sets",
    description: "Automatically upgrade sets to a better version when available",
    keywords: ["upgrade", "better version", "auto upgrade"],
    configPath: "auto_upgrade_sets",
  },
  {
    sectionId: "user-preferences",
    sectionLabel: "User Preferences",
    label: "Your Favorite Creators",
    description: "List of your favorite MediUX creators",
    keywords: ["favorite", "creators", "mediux"],
    configPath: "favorite_creators",
  },

  // Advanced
  {
    sectionId: "advanced",
    sectionLabel: "Advanced",
    label: "Export Configuration",
    description: "Download your configuration as a backup, or a redacted one to share",
    keywords: ["export", "backup", "download config", "yaml", "redacted", "share", "support"],
  },
  {
    sectionId: "advanced",
    sectionLabel: "Advanced",
    label: "Import Configuration",
    description: "Restore your configuration from a file",
    keywords: ["import", "restore", "upload config", "yaml"],
  },
  {
    sectionId: "advanced",
    sectionLabel: "Advanced",
    label: "Debug Mode",
    description: "Show extra details for troubleshooting",
    keywords: ["debug", "troubleshoot"],
  },
  {
    sectionId: "advanced",
    sectionLabel: "Advanced",
    label: "App Cache",
    description: "Clear the data this browser stores for aura",
    keywords: ["cache", "clear", "reset", "storage"],
  },
];

const MAX_RESULTS = 6;

// searchSettings returns the settings that match every word in the query, best matches first
export const searchSettings = (query: string): SettingsSearchEntry[] => {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (words.length === 0 || query.trim().length < 2) return [];

  const scored = SETTINGS_SEARCH_ENTRIES.map((entry) => {
    const label = entry.label.toLowerCase();
    const haystack = [entry.label, entry.sectionLabel, entry.description, ...(entry.keywords ?? [])]
      .join(" ")
      .toLowerCase();
    if (!words.every((word) => haystack.includes(word))) return null;
    // Prefer matches in the label, then at the start of the label
    let score = 0;
    for (const word of words) {
      if (label.includes(word)) score += 2;
      if (label.startsWith(word)) score += 1;
    }
    return { entry, score };
  }).filter((result): result is { entry: SettingsSearchEntry; score: number } => result !== null);

  return scored
    .sort((a, b) => b.score - a.score)
    .slice(0, MAX_RESULTS)
    .map((result) => result.entry);
};

// settingLabel returns the name of a setting from its config section and path, if it is known
export const settingLabel = (sectionId: string, configPath: string): string | undefined =>
  SETTINGS_SEARCH_ENTRIES.find((entry) => entry.sectionId === sectionId && entry.configPath === configPath)?.label;

// URL of a setting on the Settings page
export const settingURL = (entry: SettingsSearchEntry) =>
  `/settings?section=${entry.sectionId}&setting=${encodeURIComponent(entry.label)}`;

// Settings page section of each config section, and its name
const CONFIG_SECTIONS: Record<string, { sectionId: string; label: string }> = {
  mediux: { sectionId: "mediux", label: "MediUX" },
  media_server: { sectionId: "media-server", label: "Main Media Server" },
  media_servers: { sectionId: "media-server", label: "Media Servers" },
  sonarr_radarr: { sectionId: "sonarr-radarr", label: "Sonarr & Radarr" },
  auth: { sectionId: "authentication", label: "Authentication" },
  images: { sectionId: "images", label: "Images" },
  labels_and_tags: { sectionId: "labels-and-tags", label: "Labels & Tags" },
  notifications: { sectionId: "notifications", label: "Notifications" },
  logging: { sectionId: "logging", label: "Logging" },
  user_preferences: { sectionId: "user-preferences", label: "User Preferences" },
  jobs: { sectionId: "jobs", label: "Jobs" },
  backups: { sectionId: "backups", label: "Backups" },
};

// configSectionLabel returns the name of a config section
export const configSectionLabel = (configSection: string): string =>
  CONFIG_SECTIONS[configSection]?.label ?? configSection;

// configSettingLabel returns the name of a setting
export const configSettingLabel = (configSection: string, configPath: string): string | undefined => {
  const section = CONFIG_SECTIONS[configSection];
  return section ? settingLabel(section.sectionId, configPath) : undefined;
};
