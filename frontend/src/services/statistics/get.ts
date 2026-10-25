import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { Statistics } from "@/types/statistics";

export const GetStatistics = async (days: number): Promise<APIResponse<Statistics>> => {
  try {
    log("INFO", "API - Statistics", "Fetch", `Fetching the statistics for the last ${days} days`);
    const response = await apiClient.get<APIResponse<Statistics>>(`/statistics`, { params: { days } });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error fetching the statistics");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Statistics", "Fetch", "Error fetching the statistics", { error });
    return ReturnErrorMessage<Statistics>(error);
  }
};
