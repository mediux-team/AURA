import { markOpenMediaItemSetSaved } from "@/helper/update-open-media-item";
import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { DBSavedItem } from "@/types/database/db-poster-set";

export interface AddItemToDownloadQueue_Request {
  item: DBSavedItem;
}
export interface AddItemToDownloadQueue_Response {
  result: string;
  job_id: number; // The new job, to follow it in the Download Queue
}

export const AddItemToDownloadQueue = async (
  dbItem: DBSavedItem
): Promise<APIResponse<AddItemToDownloadQueue_Response>> => {
  log(
    "INFO",
    "API - Media Server",
    "Add to Queue",
    `Adding '${dbItem.media_item.title}' (TMDB ID: ${dbItem.media_item.tmdb_id}) to the download queue`
  );
  for (const set of dbItem.poster_sets) {
    set.last_downloaded = new Date().toISOString();
  }
  try {
    const req: AddItemToDownloadQueue_Request = {
      item: dbItem,
    };
    const response = await apiClient.post<APIResponse<AddItemToDownloadQueue_Response>>(`/download/queue/item`, req);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while adding to download queue");
    } else {
      log(
        "INFO",
        "API - Media Server",
        "Add to Queue",
        `Added '${dbItem.media_item.title}' (TMDB ID: ${dbItem.media_item.tmdb_id}) to the download queue`,
        response.data
      );
    }

    // Show the sets as saved now, the queue saves them to the DB later
    for (const posterSet of dbItem.poster_sets) {
      markOpenMediaItemSetSaved({
        tmdbID: dbItem.media_item.tmdb_id,
        libraryTitle: dbItem.media_item.library_title,
        edition: dbItem.media_item.edition,
        setID: posterSet.id,
        setUser: posterSet.user_created,
        selectedTypes: posterSet.selected_types,
      });
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Media Server",
      "Add to Queue",
      `Failed to add '${dbItem.media_item.title}' (TMDB ID: ${dbItem.media_item.tmdb_id}) to the download queue: ${
        dbItem.media_item.rating_key
      }: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<AddItemToDownloadQueue_Response>(error);
  }
};
