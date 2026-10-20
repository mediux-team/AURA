import type { AppConfig } from "@/types/config/config";
import { DOWNLOAD_IMAGE_TYPE_OPTIONS } from "@/types/ui-options";

// Central default
export const defaultAppConfig = (): AppConfig =>
  ({
    auth: {
      enabled: false,
      password: "",
      oidc: {
        enabled: false,
        issuer_url: "",
        client_id: "",
        client_secret: "",
        redirect_url: "",
        allowed_emails: [],
        allowed_domains: [],
      },
      session_cookie_secure: "auto",
      trust_proxy_for_cookie_secure: false,
      allowed_origins: [],
    },
    logging: {
      level: "",
      file: "",
      keep_days: 7,
      keep_files: 7,
    },
    media_server: {
      type: "",
      url: "",
      api_token: "",
      libraries: [],
      user_id: "",
    },
    mediux: {
      api_token: "",
      download_quality: "",
    },
    jobs: {
      auto_download: { enabled: false, cron: "0 0 * * *" },
      refresh_media_items_and_collections: { enabled: true, cron: "0 4 * * *" },
      check_for_media_item_changes: { enabled: true, cron: "0 */6 * * *" },
      handle_temp_ignored_items: { enabled: true, cron: "0 */6 * * *" },
      refresh_mediux_users: { enabled: true, cron: "0 */12 * * *" },
      check_mediux_site_link: { enabled: true, cron: "0 */1 * * *" },
      backup_database: { enabled: true, cron: "0 3 * * *" },
    },
    backups: {
      keep: 7,
    },
    images: {
      cache_images: { enabled: false },
      save_images_locally: {
        enabled: false,
        path: "",
      },
    },
    tmdb: {
      api_token: "",
    },
    labels_and_tags: {
      applications: [],
      remove_overlay_label_only_on_poster_download: false,
    },
    notifications: {
      enabled: false,
      providers: [],
      // The built-in templates come from aura's backend
      templates: {},
    },
    sonarr_radarr: {
      applications: [{ type: "", library: "", url: "", api_token: "" }],
    },
    user_preferences: {
      download_defaults: DOWNLOAD_IMAGE_TYPE_OPTIONS.map((option) => option.value),
      show_only_download_defaults: false,
      show_date_modified: false,
      default_add_to_queue: false,
      default_add_to_db_only: false,
      auto_download_new_items: false,
      favorite_creators: [],
      default_sort: "dateAdded",
      default_sort_order: "desc",
      default_items_per_page: 20,
      default_libraries: [],
      default_filter_in_db: "",
      default_filter_ignored: "",
      default_has_sets_available_filter: "",
    },
  }) satisfies AppConfig;

