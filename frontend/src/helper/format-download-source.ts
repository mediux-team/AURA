// Where downloads came from
const SOURCE_LABELS: Record<string, string> = {
  download_window: "Download window",
  download_queue: "Add to Queue",
  favorite_creators_new_item: "Favorite creators, new items",
  favorite_creators_all_items: "Favorite creators, all items",
  auto_upgrade: "Upgrades",
  silo_plugin: "Silo plugin",
  auto_download: "Auto Download",
  sonarr_webhook: "Sonarr webhook",
  plex_refresh: "Plex refresh",
  retry: "Retry",
  backfill: "Saved sets, before tracking",
};

export const sourceLabel = (source: string) => SOURCE_LABELS[source] ?? source;

// downloadSource is what added a Download Queue job, the user's Add to Queue when it doesn't say
export const downloadSource = (source?: string) => source || "download_queue";
