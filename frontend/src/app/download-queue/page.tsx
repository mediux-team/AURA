"use client";

import { GetDownloadHistory } from "@/services/downloads/history-get";
import { RemoveAllDownloadHistory } from "@/services/downloads/history-remove";
import { GetAllDownloadQueueItems } from "@/services/downloads/queue-get";
import { RemoveAllFromQueue } from "@/services/downloads/queue-remove";
import { subscribeToDownloadQueue } from "@/services/downloads/queue-stream";
import { toast } from "sonner";

import React, { useCallback, useEffect, useState } from "react";

import Link from "next/link";

import { CustomPagination } from "@/components/shared/custom-pagination";
import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import DownloadEntryCard from "@/components/shared/download-entry";
import { DownloadQueueProgressPanel } from "@/components/shared/download-queue-progress";
import { ErrorMessage } from "@/components/shared/error-message";
import Loader from "@/components/shared/loader";
import { ResponsiveGrid } from "@/components/shared/responsive-grid";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { H2 } from "@/components/ui/typography";

import { useFailedDownloadsStore } from "@/lib/stores/global-store-failed-downloads";
import { useDownloadQueuePageStore } from "@/lib/stores/page-store-download-queue";

import type { APIResponse } from "@/types/api/api-response";
import type { DownloadHistoryEntry, DownloadQueueJob, DownloadQueueProgress } from "@/types/database/download-queue";
import { ITEMS_PER_PAGE_OPTIONS } from "@/types/ui-options";

// PerPageSelect picks how many cards each page shows
const PerPageSelect: React.FC<{ value: number; onChange: (value: number) => void }> = ({ value, onChange }) => (
  <div className="flex items-center gap-2">
    <Label className="text-sm text-muted-foreground whitespace-nowrap">Per page</Label>
    <Select value={String(value)} onValueChange={(v) => onChange(Number(v))}>
      <SelectTrigger className="h-8 w-20">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {ITEMS_PER_PAGE_OPTIONS.map((option) => (
          <SelectItem key={option} value={String(option)}>
            {option}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  </div>
);

// withoutJob is the progress without the job's
const withoutJob = (progress: Record<number, DownloadQueueProgress>, jobId: number) => {
  if (!(jobId in progress)) return progress;
  const rest = { ...progress };
  delete rest[jobId];
  return rest;
};

// pageOf returns one page of the list, and the number of pages
const pageOf = <T,>(list: T[], page: number, perPage: number) => ({
  items: list.slice((page - 1) * perPage, page * perPage),
  totalPages: Math.max(1, Math.ceil(list.length / perPage)),
});

const DownloadQueuePage: React.FC = () => {
  // States - Queue
  const [jobs, setJobs] = useState<DownloadQueueJob[]>([]);
  const [progress, setProgress] = useState<Record<number, DownloadQueueProgress>>({}); // By job ID
  const [queueLoading, setQueueLoading] = useState(true);
  const [queueError, setQueueError] = useState<APIResponse<unknown> | null>(null);

  // States - History
  const [historyEntries, setHistoryEntries] = useState<DownloadHistoryEntry[]>([]);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [historyError, setHistoryError] = useState<APIResponse<unknown> | null>(null);
  const [maxRetries, setMaxRetries] = useState(0);

  // States - Pages, each tab has its own
  const [queuePage, setQueuePage] = useState(1);
  const queuePerPage = useDownloadQueuePageStore((state) => state.queuePerPage);
  const setQueuePerPage = useDownloadQueuePageStore((state) => state.setQueuePerPage);
  const [historyPage, setHistoryPage] = useState(1);
  const historyPerPage = useDownloadQueuePageStore((state) => state.failedPerPage);
  const setHistoryPerPage = useDownloadQueuePageStore((state) => state.setFailedPerPage);

  const setFailedCount = useFailedDownloadsStore((state) => state.setFailedCount);

  // A quiet fetch updates the list without the loader, for changes while the page is open
  const fetchHistory = useCallback(
    async (quiet = false) => {
      try {
        if (!quiet) setHistoryLoading(true);
        const response = await GetDownloadHistory();
        if (response.status === "error") {
          setHistoryError(response);
          return;
        }
        setHistoryEntries(response.data?.entries || []);
        setMaxRetries(response.data?.max_retries ?? 0);
        setHistoryError(null);
        setFailedCount(response.data?.total ?? response.data?.entries?.length ?? 0);
      } finally {
        setHistoryLoading(false);
      }
    },
    [setFailedCount]
  );

  // Initial queue load (SSE also sends a snapshot on connect, but this avoids a loading flash on slow connections)
  const fetchQueueEntries = useCallback(async () => {
    try {
      setQueueLoading(true);
      const response = await GetAllDownloadQueueItems();
      if (response.status === "error") {
        setQueueError(response);
        return;
      }
      setJobs(response.data?.jobs || []);
      setQueueError(null);
    } finally {
      setQueueLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchQueueEntries();
    fetchHistory();
  }, [fetchQueueEntries, fetchHistory]);

  // Live updates over SSE, replacing the old 2s HTTP polling.
  useEffect(() => {
    const unsubscribe = subscribeToDownloadQueue({
      onSnapshot: (snapshotJobs) => {
        setJobs(snapshotJobs ?? []);
        setProgress({});
        setQueueLoading(false);
        setQueueError(null);
      },
      onJobAdded: (job) => {
        // The snapshot query can race a real add, so the job may already be
        // in state by the time this event arrives - don't append a duplicate.
        setJobs((prev) => (prev.some((j) => j.id === job.id) ? prev : [...prev, job]));
        // A retried failure counts its retry
        fetchHistory(true);
      },
      onJobStarted: (job) => {
        setProgress((prev) => withoutJob(prev, job.id));
        // The started job is processing, and is added if it was missing
        const started: DownloadQueueJob = { ...job, status: "processing" };
        setJobs((prev) =>
          prev.some((j) => j.id === job.id) ? prev.map((j) => (j.id === job.id ? started : j)) : [...prev, started]
        );
      },
      onJobProgress: (jobProgress) => {
        setProgress((prev) => ({ ...prev, [jobProgress.job_id]: jobProgress }));
      },
      onJobFinished: (job) => {
        // Server always clears the job on finish, so drop it here too.
        setProgress((prev) => withoutJob(prev, job.id));
        setJobs((prev) => prev.filter((j) => j.id !== job.id));
        // A job finishing means a new history entry may exist.
        fetchHistory(true);
      },
      onJobRemoved: (job) => {
        setProgress((prev) => withoutJob(prev, job.id));
        setJobs((prev) => prev.filter((j) => j.id !== job.id));
      },
      onQueueCleared: () => {
        setJobs([]);
        setProgress({});
      },
      onQueueReordered: (reorderedJobs) => {
        setJobs(reorderedJobs ?? []);
      },
    });

    return unsubscribe;
  }, [fetchHistory]);

  const onClearQueue = async () => {
    const response = await RemoveAllFromQueue();
    if (response.status === "error") {
      toast.error(`Error clearing queue: ${response.error?.message || "Unknown error occurred."}`);
    } else {
      toast.success(response.data?.result || "Cleared the download queue.");
    }
  };

  const onClearHistory = async () => {
    const response = await RemoveAllDownloadHistory();
    if (response.status === "error") {
      toast.error(`Error clearing history: ${response.error?.message || "Unknown error occurred."}`);
    } else {
      toast.success(response.data?.result || "Cleared the download history.");
    }
    await fetchHistory();
  };

  // Finished jobs get recorded to History, so the queue only shows pending/processing.
  const activeJobs = jobs
    .filter((j) => j.status === "pending" || j.status === "processing")
    .sort((a, b) => (a.status === b.status ? 0 : a.status === "processing" ? -1 : 1));

  const processingJobs = activeJobs.filter((j) => j.status === "processing");
  const firstWaitingJob = activeJobs.find((j) => j.status === "pending");

  const queuePageItems = pageOf(activeJobs, queuePage, queuePerPage);
  const historyPageItems = pageOf(historyEntries, historyPage, historyPerPage);

  // A page that's gone, after jobs finish or entries are removed, shows the last page
  useEffect(() => {
    if (queuePage > queuePageItems.totalPages) setQueuePage(queuePageItems.totalPages);
  }, [queuePage, queuePageItems.totalPages]);
  useEffect(() => {
    if (historyPage > historyPageItems.totalPages) setHistoryPage(historyPageItems.totalPages);
  }, [historyPage, historyPageItems.totalPages]);

  return (
    <div className="container mx-auto p-4 min-h-screen flex flex-col items-center">
      <H2 className="text-3xl font-bold mb-4">Download Queue</H2>

      <Tabs defaultValue="queue" className="w-full">
        <TabsList className="mx-auto">
          <TabsTrigger value="queue">Queue</TabsTrigger>
          <TabsTrigger value="history">Failed</TabsTrigger>
        </TabsList>

        <TabsContent value="queue" className="w-full">
          {queueLoading ? (
            <Loader className="mt-10" message="Loading download queue entries..." />
          ) : queueError ? (
            <div className="flex flex-col items-center p-6 gap-4">
              <ErrorMessage error={queueError} />
            </div>
          ) : (
            <>
              <div className="w-full flex items-center justify-end gap-2 mb-2">
                {activeJobs.length > 0 && (
                  <PerPageSelect
                    value={queuePerPage}
                    onChange={(value) => {
                      setQueuePerPage(value);
                      setQueuePage(1);
                    }}
                  />
                )}
                <ConfirmDestructiveDialogActionButton
                  variant="outline"
                  className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
                  confirmText="Clear Queue"
                  title="Clear Download Queue?"
                  description="Are you sure you want to remove all items from the download queue? Items downloading now are stopped once the images already downloading finish. This action cannot be undone."
                  onConfirm={onClearQueue}
                  disabled={jobs.length === 0}
                >
                  Clear Queue
                </ConfirmDestructiveDialogActionButton>
              </div>

              {processingJobs.length > 0 && (
                <div className="w-full space-y-3 mb-4">
                  {processingJobs.map((job, index) => (
                    <DownloadQueueProgressPanel
                      key={job.id}
                      progress={progress[job.id] ?? null}
                      processingTitle={job.media_item_title}
                      waiting={index === 0 ? activeJobs.length - processingJobs.length : 0}
                    />
                  ))}
                </div>
              )}

              {activeJobs.length === 0 ? (
                <p className="text-gray-500">
                  No download queue entries found. Add some items to the queue to get started.
                </p>
              ) : (
                <>
                  <ResponsiveGrid size="larger">
                    {queuePageItems.items.map((job) => (
                      <DownloadEntryCard
                        key={job.id}
                        mode="queue"
                        job={job}
                        fetchQueueEntries={fetchQueueEntries}
                        canMoveToTop={job.status === "pending" && job.id !== firstWaitingJob?.id}
                      />
                    ))}
                  </ResponsiveGrid>
                  <CustomPagination
                    currentPage={queuePage}
                    totalPages={queuePageItems.totalPages}
                    setCurrentPage={setQueuePage}
                    scrollToTop={true}
                    filterItemsLength={activeJobs.length}
                    itemsPerPage={queuePerPage}
                  />
                </>
              )}
            </>
          )}
        </TabsContent>

        <TabsContent value="history" className="w-full">
          {historyLoading ? (
            <Loader className="mt-10" message="Loading failed downloads..." />
          ) : historyError ? (
            <div className="flex flex-col items-center p-6 gap-4">
              <ErrorMessage error={historyError} />
            </div>
          ) : (
            <>
              <div className="w-full flex flex-wrap items-center justify-between gap-2 mb-2">
                <p className="text-sm text-muted-foreground">
                  Successful downloads show on the{" "}
                  <Link href="/saved-sets" className="text-primary hover:underline">
                    Saved Sets
                  </Link>{" "}
                  page.
                </p>
                <div className="flex items-center gap-2 ml-auto">
                  {historyEntries.length > 0 && (
                    <PerPageSelect
                      value={historyPerPage}
                      onChange={(value) => {
                        setHistoryPerPage(value);
                        setHistoryPage(1);
                      }}
                    />
                  )}
                  <ConfirmDestructiveDialogActionButton
                    variant="outline"
                    className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
                    confirmText="Clear Failed"
                    title="Clear Failed Downloads?"
                    description="Are you sure you want to remove all failed downloads from this list? This action cannot be undone."
                    onConfirm={onClearHistory}
                    disabled={historyEntries.length === 0}
                  >
                    Clear Failed
                  </ConfirmDestructiveDialogActionButton>
                </div>
              </div>

              {historyEntries.length === 0 ? (
                <p className="text-gray-500">No failed downloads.</p>
              ) : (
                <>
                  <ResponsiveGrid size="larger">
                    {historyPageItems.items.map((entry) => (
                      <DownloadEntryCard
                        key={entry.id}
                        mode="history"
                        entry={entry}
                        maxRetries={maxRetries}
                        fetchHistory={() => fetchHistory(true)}
                      />
                    ))}
                  </ResponsiveGrid>
                  <CustomPagination
                    currentPage={historyPage}
                    totalPages={historyPageItems.totalPages}
                    setCurrentPage={setHistoryPage}
                    scrollToTop={true}
                    filterItemsLength={historyEntries.length}
                    itemsPerPage={historyPerPage}
                  />
                </>
              )}
            </>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default DownloadQueuePage;
