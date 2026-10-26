import { log } from "@/lib/logger";

import type { DownloadQueueEvent } from "@/types/database/download-queue";

export interface DownloadQueueStreamHandlers {
  onSnapshot: (jobs: DownloadQueueEvent["jobs"]) => void;
  onJobAdded: (job: NonNullable<DownloadQueueEvent["job"]>) => void;
  onJobStarted: (job: NonNullable<DownloadQueueEvent["job"]>) => void;
  onJobProgress: (progress: NonNullable<DownloadQueueEvent["progress"]>) => void;
  onJobFinished: (job: NonNullable<DownloadQueueEvent["job"]>) => void;
  onJobRemoved: (job: NonNullable<DownloadQueueEvent["job"]>) => void;
  onQueueCleared: () => void;
  onQueueReordered: (jobs: DownloadQueueEvent["jobs"]) => void;
}

// Subscribes to live download queue updates over Server-Sent Events.
export const subscribeToDownloadQueue = (handlers: Partial<DownloadQueueStreamHandlers>): (() => void) => {
  const es = new EventSource(`/api/download/queue/stream`, { withCredentials: true });

  es.onmessage = (event: MessageEvent<string>) => {
    try {
      const evt: DownloadQueueEvent = JSON.parse(event.data);
      switch (evt.type) {
        case "snapshot":
          handlers.onSnapshot?.(evt.jobs ?? []);
          break;
        case "job_added":
          if (evt.job) handlers.onJobAdded?.(evt.job);
          break;
        case "job_started":
          if (evt.job) handlers.onJobStarted?.(evt.job);
          break;
        case "job_progress":
          if (evt.progress) handlers.onJobProgress?.(evt.progress);
          break;
        case "job_finished":
          if (evt.job) handlers.onJobFinished?.(evt.job);
          break;
        case "job_removed":
          if (evt.job) handlers.onJobRemoved?.(evt.job);
          break;
        case "queue_cleared":
          handlers.onQueueCleared?.();
          break;
        case "queue_reordered":
          handlers.onQueueReordered?.(evt.jobs ?? []);
          break;
      }
    } catch (error) {
      log("ERROR", "API - Download Queue", "Stream", "Failed to parse download queue event", { error });
    }
  };

  es.onerror = () => {
    // EventSource auto-reconnects with backoff; nothing to do here besides log.
    log("ERROR", "API - Download Queue", "Stream", "Download queue stream connection error, will auto-reconnect");
  };

  return () => es.close();
};
