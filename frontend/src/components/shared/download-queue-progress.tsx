"use client";

import { formatDownloadProgressShortText, formatDownloadProgressText } from "@/helper/format-download-progress";
import { formatImageTypeLabel } from "@/helper/format-image-type";
import { Check, Circle, Loader2, X } from "lucide-react";

import React from "react";

import { DownloadProgressBar } from "@/components/shared/download-progress-bar";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";

import type { DownloadQueueProgress, DownloadQueueProgressImage } from "@/types/database/download-queue";

interface DownloadQueueProgressPanelProps {
  progress: DownloadQueueProgress | null; // The job's images being downloaded
  processingTitle?: string; // Title of the job being downloaded, shown before its first image starts
  waiting?: number; // Jobs waiting, shown on the first job
}

const plural = (count: number, one: string, many: string) => `${count.toLocaleString()} ${count === 1 ? one : many}`;

// ImageStatusIcon shows where an image of the job is
const ImageStatusIcon: React.FC<{ status: DownloadQueueProgressImage["status"] }> = ({ status }) => {
  switch (status) {
    case "downloading":
      return <Loader2 className="h-3 w-3 shrink-0 animate-spin" />;
    case "done":
      return <Check className="h-3 w-3 shrink-0 text-green-500" strokeWidth={3} />;
    case "failed":
      return <X className="h-3 w-3 shrink-0 text-destructive" strokeWidth={3} />;
    default:
      return <Circle className="h-3 w-3 shrink-0 text-muted-foreground/50" />;
  }
};

// Shows the progress of a job being downloaded, with its images in an accordion
export const DownloadQueueProgressPanel: React.FC<DownloadQueueProgressPanelProps> = ({
  progress,
  processingTitle,
  waiting = 0,
}) => {
  const title = progress?.media_item_title || processingTitle;
  if (!title) return null;

  const total = progress?.total ?? 0;
  const done = progress?.done ?? 0;
  const failed = progress?.failed ?? 0;
  const active = progress?.active ?? [];
  const images = progress?.images ?? [];
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

  const summary = (
    <span>
      {total > 0
        ? `${done.toLocaleString()} of ${plural(total, "image", "images")} · ${(total - done).toLocaleString()} remaining`
        : "Getting ready..."}
      {failed > 0 && <span className="text-amber-500"> · {plural(failed, "image", "images")} failed</span>}
    </span>
  );

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

      <DownloadProgressBar value={percent} label={label} shortLabel={shortLabel} hasErrors={failed > 0} />

      {images.length > 0 ? (
        <Accordion type="single" collapsible>
          <AccordionItem value="images" className="border-b-0">
            <AccordionTrigger className="py-1 text-xs font-normal text-muted-foreground hover:no-underline cursor-pointer">
              {summary}
            </AccordionTrigger>
            <AccordionContent className="pb-0">
              <ul className="grid grid-cols-1 gap-x-4 gap-y-1 text-xs sm:grid-cols-2 xl:grid-cols-3">
                {images.map((image) => (
                  <li key={`${image.image_type}-${image.id}`} className="flex min-w-0 items-center gap-1.5">
                    <ImageStatusIcon status={image.status} />
                    <span className={image.status === "waiting" ? "truncate text-muted-foreground" : "truncate"}>
                      {formatImageTypeLabel(image) || "Image"}
                    </span>
                  </li>
                ))}
              </ul>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      ) : (
        <p className="text-xs text-muted-foreground">{summary}</p>
      )}
    </div>
  );
};
