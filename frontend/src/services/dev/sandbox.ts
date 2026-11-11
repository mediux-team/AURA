import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { DBSavedItem } from "@/types/database/db-poster-set";

// The test sandbox, only in dev builds

export interface SandboxFailureRule {
  tmdb_id: string;
  library_title: string;
  edition: string;
  title: string;
  types: string[]; // The image types that fail, every type when empty
  remaining: number; // How many more downloads fail, until it's cleared when 0
  reason: string;
}

export interface SandboxState {
  enabled: boolean;
  delay_seconds: number; // The wait before each image downloads
  failures: SandboxFailureRule[];
}

export type SandboxChangeType =
  "image_updated" | "image_new" | "movie_file_changed" | "episode_file_changed" | "episode_added" | "season_added";

export interface SandboxChange {
  tmdb_id: string;
  library_title: string;
  edition: string;
  change: SandboxChangeType;
  image_id?: string;
  image_type?: string;
  season_number?: number;
  episode_number?: number;
}

export interface SandboxRecheckResult {
  item: string;
  sets: { id: string; title: string; user_created: string; result: string; reason: string }[] | null;
  overall_result: string;
  overall_message: string;
}

// sandboxRequest sends a request to the sandbox, and logs it
const sandboxRequest = async <T>(
  action: string,
  request: () => Promise<{ data: APIResponse<T> }>
): Promise<APIResponse<T>> => {
  try {
    const response = await request();
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || `Unknown error: ${action}`);
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Sandbox", action, `Failed: ${action}`, { error });
    return ReturnErrorMessage<T>(error);
  }
};

export const getSandboxState = () =>
  sandboxRequest<SandboxState>("Get State", () => apiClient.get<APIResponse<SandboxState>>(`/dev/sandbox`));

export const setSandboxDelay = (seconds: number) =>
  sandboxRequest<SandboxState>("Set Download Wait", () =>
    apiClient.put<APIResponse<SandboxState>>(`/dev/sandbox/delay`, { seconds })
  );

export const addSandboxFailure = (rule: SandboxFailureRule) =>
  sandboxRequest<SandboxState>("Make Downloads Fail", () =>
    apiClient.post<APIResponse<SandboxState>>(`/dev/sandbox/failures`, rule)
  );

export const clearSandboxFailures = () =>
  sandboxRequest<SandboxState>("Clear Failures", () =>
    apiClient.delete<APIResponse<SandboxState>>(`/dev/sandbox/failures`)
  );

export const simulateSandboxChange = (change: SandboxChange) =>
  sandboxRequest<{ result: string }>("Simulate a Change", () =>
    apiClient.post<APIResponse<{ result: string }>>(`/dev/sandbox/simulate`, change)
  );

export const recheckSandboxItem = (item: { tmdb_id: string; library_title: string; edition: string }) =>
  sandboxRequest<SandboxRecheckResult>("Run the Auto Download Check", () =>
    apiClient.post<APIResponse<SandboxRecheckResult>>(`/dev/sandbox/recheck`, item)
  );

// findSavedItems finds saved items by title, or one item by its TMDB ID and library
export const findSavedItems = (filter: { title?: string; tmdbID?: string; library?: string }) =>
  sandboxRequest<{ items: DBSavedItem[] }>("Find Saved Items", () =>
    apiClient.get<APIResponse<{ items: DBSavedItem[] }>>(`/db`, {
      params: {
        item_title: filter.title ?? "",
        item_tmdb_id: filter.tmdbID ?? "",
        item_library_title: filter.library ?? "",
        items_per_page: 20,
        page_number: 1,
      },
    })
  );
