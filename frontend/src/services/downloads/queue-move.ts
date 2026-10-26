import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface MoveQueueItemToTop_Request {
  job_id: number;
}
export interface MoveQueueItemToTop_Response {
  result: string;
}

// MoveQueueItemToTop makes a waiting job the next one to download
export const MoveQueueItemToTop = async (jobId: number): Promise<APIResponse<MoveQueueItemToTop_Response>> => {
  log("INFO", "API - Download Queue", "Move to Top", `Moving job ${jobId} to the top of the download queue`);
  try {
    const req: MoveQueueItemToTop_Request = { job_id: jobId };
    const response = await apiClient.post<APIResponse<MoveQueueItemToTop_Response>>(`/download/queue/item/top`, req);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while moving the job to the top");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Download Queue",
      "Move to Top",
      `Failed to move job ${jobId} to the top of the download queue`,
      {
        error,
      }
    );
    return ReturnErrorMessage<MoveQueueItemToTop_Response>(error);
  }
};
