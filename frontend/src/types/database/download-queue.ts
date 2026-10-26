import type { DBSavedItem } from "@/types/database/db-poster-set";
import type { SelectedTypes } from "@/types/media-and-posters/media-item-and-library";

export type DownloadQueueJobStatus = "pending" | "processing" | "success" | "warning" | "error";

export interface DownloadQueueJob {
  id: number;
  tmdb_id: string;
  library_title: string;
  edition: string;
  media_item_title: string;
  item: DBSavedItem;
  status: DownloadQueueJobStatus;
  result_message: string;
  result_errors: string[];
  result_warnings: string[];
  created_at: string;
  started_at?: string;
  finished_at?: string;
}

export type DownloadQueueEventType =
  | "snapshot"
  | "job_added"
  | "job_started"
  | "job_progress"
  | "job_finished"
  | "job_removed"
  | "queue_cleared"
  | "queue_reordered";

export interface DownloadQueueProgress {
  job_id: number;
  media_item_title: string;
  image_type: string;
  season_number?: number;
  episode_number?: number;
  done: number; // Images of the job already downloaded
  failed: number; // Images of the job that couldn't be downloaded
  total: number; // Images the job downloads
}

export interface DownloadQueueEvent {
  type: DownloadQueueEventType;
  job?: DownloadQueueJob;
  jobs?: DownloadQueueJob[];
  progress?: DownloadQueueProgress;
}

export interface ImageDownloadResult {
  image_type: string;
  season_number?: number;
  episode_number?: number;
  success: boolean;
  failure_reason?: string;
}

export type DownloadHistoryStatus = "success" | "warning" | "error";

export interface DownloadHistoryEntry {
  id: number;
  tmdb_id: string;
  library_title: string;
  edition: string;
  media_item_title: string;
  media_item_year: number;
  set_id: string;
  set_title: string;
  set_creator: string;
  selected_types: SelectedTypes;
  warning_count: number;
  images_succeeded: number;
  images_failed: number;
  status: DownloadHistoryStatus;
  failed_images: ImageDownloadResult[];
  retry_count: number; // Times the set was retried, automatically or with Retry
  next_retry_at: string | null; // When it's retried automatically, null when it isn't
  created_at: string;
  rating_key?: string;
}
