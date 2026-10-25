import type { CollectionItem } from "@/app/collections/page";
import { collectionItemInfo } from "@/helper/item-info";
import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { CollectionItemImageFile, SetSummary } from "@/types/media-and-posters/sets";
import { TYPE_DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS } from "@/types/ui-options";

export interface DownloadCollectionImage_Request {
  collection_item: CollectionItem;
  image_file: CollectionItemImageFile;
  set?: SetSummary; // The set the image is from, for the statistics
}

export interface DownloadCollectionImage_Response {
  result: string;
}

export const DownloadImageFileForCollectionItem = async (
  imageType: TYPE_DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS,
  collectionItem: CollectionItem,
  imageFile: CollectionItemImageFile,
  set?: SetSummary
): Promise<APIResponse<DownloadCollectionImage_Response>> => {
  log(
    "INFO",
    "API - Media Server",
    "Download and Update Collection Image",
    `Downloading ${imageType} image and updating ${collectionItemInfo(collectionItem)}`
  );
  try {
    const req: DownloadCollectionImage_Request = {
      collection_item: collectionItem,
      image_file: imageFile,
      set,
    };
    const response = await apiClient.post<APIResponse<DownloadCollectionImage_Response>>(
      `/download/image/collection`,
      req
    );
    if (response.data.status === "error") {
      throw new Error(
        response.data.error?.message || `Unknown error downloading ${imageType} image and updating media server`
      );
    } else {
      log(
        "INFO",
        "API - Media Server",
        "Download and Update Collection Image",
        `Downloaded ${imageType} image and updated ${collectionItemInfo(collectionItem)}`,
        response.data
      );
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Media Server",
      "Download and Update Collection Image",
      `Failed to download ${imageType} image and update ${collectionItemInfo(collectionItem)}
			${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<DownloadCollectionImage_Response>(error);
  }
};
