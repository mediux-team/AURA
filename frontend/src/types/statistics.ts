// The statistics from aura, for the Statistics page

export interface ItemCounts {
  items: number;
  movies: number;
  shows: number;
  seasons: number;
  episodes: number;
}

export interface ItemRef {
  title: string;
  year: number;
  type: string;
  library: string;
  tmdb_id: string;
}

export interface MediaServerStats {
  id: string;
  name: string;
  type: string;
  main: boolean;
  counts: ItemCounts;
  matched: number; // Items in both media servers
  only_in_main_count: number;
  only_in_server_count: number;
  only_in_main: ItemRef[]; // The first of them, by title
  only_in_server: ItemRef[];
  unmapped_libraries: string[] | null; // Libraries that don't match a main media server library
}

export interface DownloadTotals {
  sets: number;
  images: number;
  redownloads: number;
  updates: number;
  updated_images: number;
  reapplies: number;
  saved_only: number;
  collection_images: number;
  images_failed: number;
  bytes: number;
  estimated_sets: number; // Counted from the saved sets, before downloads were recorded
}

export interface DownloadPoint {
  date: string;
  sets: number;
  images: number;
  updates: number;
  reapplies: number;
  images_failed: number;
  bytes: number;
}

export interface GroupCount {
  name: string;
  sets: number;
  images: number;
  favorite?: boolean;
}

export interface NameCount {
  name: string;
  count: number;
}

export interface Snapshot {
  date: string;
  counts: ItemCounts;
  items_with_sets: number;
  saved_sets: number;
}

export interface Statistics {
  generated_at: string;
  days: number; // How many days back the graphs go
  period: "day" | "week" | "month";
  library: {
    totals: ItemCounts;
    libraries: { title: string; type: string; counts: ItemCounts }[] | null;
    growth: { date: string; movies: number; shows: number }[] | null;
  };
  media_servers: MediaServerStats[];
  sets_applied: {
    items: number;
    items_with_sets: number;
    saved_sets: number;
    libraries: { title: string; items: number; items_with_sets: number }[] | null;
    image_types: { type: string; items: number; total: number }[] | null;
  };
  downloads: {
    all_time: DownloadTotals;
    in_range: DownloadTotals;
    timeline: DownloadPoint[];
    image_types: GroupCount[];
  };
  creators: {
    top: GroupCount[];
    favorite_sets: number;
    other_sets: number;
    favorite_count: number;
  };
  sources: GroupCount[];
  failures: {
    images_failed: number;
    failure_rate: number;
    open_failures: number;
    top_reasons: NameCount[];
  };
  data: {
    bytes_in_range: number;
    bytes_all_time: number;
    image_cache_bytes: number;
    image_cache_files: number;
  };
  collections: {
    collections: number;
    with_images: number;
    collection_images: number;
    saved_collection_sets: number;
  };
  history: Snapshot[];
}
