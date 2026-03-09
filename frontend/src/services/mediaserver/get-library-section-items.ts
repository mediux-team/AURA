import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { LibrarySection } from "@/types/media-and-posters/media-item-and-library";

export interface GetLibrarySectionItems_Response {
  library_section: LibrarySection;
}

export const GetLibrarySectionItems = async (
  librarySection: LibrarySection,
  sectionStartIndex: number,
  enableSortByNewEpisode: boolean = true
): Promise<APIResponse<GetLibrarySectionItems_Response>> => {
  const logMessage =
    sectionStartIndex === 0
      ? `Fetching items for '${librarySection.title}'...`
      : `Fetching items for '${librarySection.title}' (index: ${sectionStartIndex})`;
  log("INFO", "API - Media Server", "Fetch Section Items", logMessage);
  try {
    const params = {
      section_id: librarySection.id,
      section_title: librarySection.title,
      section_type: librarySection.type,
      section_start_index: sectionStartIndex,
      enable_sort_by_new_episode: enableSortByNewEpisode,
    };
    const response = await apiClient.get<APIResponse<GetLibrarySectionItems_Response>>(`/mediaserver/library/items`, {
      params,
    });
    const resp = response.data;
    if (resp.status === "error") {
      throw new Error(resp.error?.message || `Unknown error fetching items for section '${librarySection.title}'`);
    } else {
      log(
        "INFO",
        "API - Media Server",
        "Fetch Section Items",
        `Fetched ${resp.data?.library_section.media_items.length ?? 0} items for '${librarySection.title}'`
      );
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Media Server",
      "Fetch Section Items",
      `Failed to fetch items for '${librarySection.title}': ${error instanceof Error ? error.message : "Unknown error"}`
    );
    return ReturnErrorMessage<GetLibrarySectionItems_Response>(error);
  }
};
