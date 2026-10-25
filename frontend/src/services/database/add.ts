import { markOpenMediaItemSetSaved } from "@/helper/update-open-media-item";
import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { DBPosterSetDetail, DBSavedItem } from "@/types/database/db-poster-set";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";

export interface AddNewItemToDB_Request {
  complete: boolean;
  media_item: MediaItem;
  poster_set: DBPosterSetDetail;
  add_to_db_only: boolean;
  auto_add_new_collection_items: boolean;
  downloaded_image_ids?: string[]; // The images that were downloaded, for the statistics
  failed_images?: number; // How many images couldn't be downloaded
}

// The images the download window downloaded for the set
export interface DownloadResult {
  downloadedImageIds: string[];
  failedImages: number;
}

export interface AddNewItemToDB_Response {
  saved_item: DBSavedItem;
}

export const AddNewItemToDB = async (
  mediaItem: MediaItem,
  posterSet: DBPosterSetDetail,
  addToDBOnly = false,
  auto_add_new_collection_items = false,
  downloadResult?: DownloadResult
): Promise<APIResponse<AddNewItemToDB_Response>> => {
  let complete = true;
  const media_data_size = JSON.stringify(mediaItem).length / 1024 / 1024;
  const poster_set_data_size = JSON.stringify(posterSet).length / 1024 / 1024;
  const data_size = media_data_size + poster_set_data_size;
  if (data_size > 10) {
    posterSet.images = [];
    mediaItem.series = undefined;
    complete = false;
  }
  posterSet.last_downloaded = new Date().toISOString();

  log(
    "INFO",
    "API - DB",
    "Add",
    `Adding '${mediaItem.title} (${mediaItem.tmdb_id} | ${mediaItem.library_title})' to DB`,
    {
      complete: complete,
      data_size_mb: data_size,
      media_data_size_mb: media_data_size,
      poster_set_data_size_mb: poster_set_data_size,
      add_to_db_only: addToDBOnly,
      auto_add_new_collection_items: auto_add_new_collection_items,
    }
  );
  try {
    const req: AddNewItemToDB_Request = {
      media_item: mediaItem,
      poster_set: posterSet,
      complete: complete,
      add_to_db_only: addToDBOnly,
      auto_add_new_collection_items: auto_add_new_collection_items,
      downloaded_image_ids: downloadResult?.downloadedImageIds,
      failed_images: downloadResult?.failedImages,
    };
    const response = await apiClient.post<APIResponse<AddNewItemToDB_Response>>(`/db`, req);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error adding item to DB");
    } else {
      log(
        "INFO",
        "API - DB",
        "Add",
        `Added '${mediaItem.title} (${mediaItem.tmdb_id} | ${mediaItem.library_title})' to DB successfully`,
        response.data
      );
    }

    if (response.data.data?.saved_item.media_item) {
      markOpenMediaItemSetSaved({
        tmdbID: response.data.data.saved_item.media_item.tmdb_id,
        libraryTitle: response.data.data.saved_item.media_item.library_title,
        edition: response.data.data.saved_item.media_item.edition,
        setID: posterSet.id,
        setUser: posterSet.user_created,
        selectedTypes: posterSet.selected_types,
      });
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - DB",
      "Add",
      `Failed to add item to DB: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<AddNewItemToDB_Response>(error);
  }
};
