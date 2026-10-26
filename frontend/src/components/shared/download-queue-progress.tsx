"use client";

import { formatDownloadProgressShortText, formatDownloadProgressText } from "@/helper/format-download-progress";
import { formatImageTypeLabel } from "@/helper/format-image-type";
import { Loader2 } from "lucide-react";

import React from "react";

import { DownloadProgressBar } from "@/components/shared/download-progress-bar";

import type { DownloadQueueProgress } from "@/types/database/download-queue";

interface DownloadQueueProgressPanelProps {
  progress: DownloadQueueProgress | null; // The job's images being downloaded
  processingTitle?: string; // Title of the job being downloaded, shown before its first image starts
  waiting?: number; // Jobs waiting, shown on the first job
}

const plural = (count: number, one: string, many: string) => `${count.toLocaleString()} ${count === 1 ? one : many}`;

// Shows the progress of a job being downloaded, like the progress in the Download Modal
export const DownloadQueueProgressPanel: React.FC<DownloadQueueProgressPanelProps> = ({
  progress,
  processingTitle,
  waiting = 0,
}) => {
  const title = progress?.media_item_title || processingTitle;
  if (!title) return null;

  const total = progress?.total ?? 0;
  const done = progress?.done ?? 0;
  const active = progress?.active ?? [];
  const percent = total > 0 ? Math.round((done / total) * 100) : 0;

  // One image says which it is, more say how many
  let label: string | undefined;
  let shortLabel: string | undefined;
  if (total > 0 && active.length === 1) {
    label = formatDownloadProgressText(active[0], title, Math.min(done + 1, total), total);
    shortLabel = formatDownloadProgressShortText(active[0]);
  } else if (total > 0 && active.length > 1) {
    label = `Downloading ${active.length} images for '${title}' (${done}/${total})`;
    shortLabel = `Downloading ${active.length} images`;
  }

  return (
    <div className="w-full space-y-2 rounded-lg border p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="flex min-w-0 items-center gap-2 text-sm font-medium">
          <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
          <span className="truncate">Downloading {title}</span>
        </p>
        {waiting > 0 && (
          <span className="shrink-0 text-xs text-muted-foreground">{plural(waiting, "job", "jobs")} waiting</span>
        )}
      </div>

      <DownloadProgressBar
        value={percent}
        label={label}
        shortLabel={shortLabel}
        hasErrors={!!progress && progress.failed > 0}
      />

      {active.length > 1 && (
        <ul className="space-y-0.5 text-xs text-muted-foreground">
          {active.map((image) => (
            <li key={image.id} className="flex min-w-0 items-center gap-1.5">
              <Loader2 className="h-3 w-3 shrink-0 animate-spin" />
              <span className="truncate">{formatImageTypeLabel(image) || "Image"}</span>
            </li>
          ))}
        </ul>
      )}

      <p className="text-xs text-muted-foreground">
        {total > 0 ? `${done.toLocaleString()} of ${plural(total, "image", "images")}` : "Getting ready..."}
        {progress && progress.failed > 0 && (
          <span className="text-amber-500"> · {plural(progress.failed, "image", "images")} failed</span>
        )}
      </p>
    </div>
  );
};
