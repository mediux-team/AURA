"use client";

import { formatImageTypeLabel } from "@/helper/format-image-type";
import { RemoveDownloadHistoryEntry } from "@/services/downloads/history-remove";
import { RetryDownloadHistoryEntry } from "@/services/downloads/history-retry";
import { RemoveItemFromQueue } from "@/services/downloads/queue-remove";
import { RefreshCcw, RefreshCwOff, RotateCw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import React from "react";

import Link from "next/link";

import { AssetImage } from "@/components/shared/asset-image";
import { CreatorAvatar } from "@/components/shared/creator-avatar";
import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import { renderTypeBadges } from "@/components/shared/saved-sets-shared";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { H4 } from "@/components/ui/typography";

import { cn } from "@/lib/cn";
import { useMediaStore } from "@/lib/stores/global-store-media-store";

import type { DBPosterSetDetail, DBSavedItem } from "@/types/database/db-poster-set";
import type { DownloadHistoryEntry, DownloadQueueJob } from "@/types/database/download-queue";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";

const statusStyles: Record<DownloadHistoryEntry["status"], string> = {
  success: "border-green-400 text-green-500",
  warning: "border-yellow-400 text-yellow-500",
  error: "border-red-400 text-red-500",
};

type DownloadEntryCardProps =
  | {
      mode: "queue";
      job: DownloadQueueJob;
      fetchQueueEntries?: () => Promise<void>;
    }
  | {
      mode: "history";
      entry: DownloadHistoryEntry;
      maxRetries: number;
      fetchHistory?: () => Promise<void>;
    };

// countSelectedImages is how many of the sets' images are of a selected type
const countSelectedImages = (posterSets: DBPosterSetDetail[]) =>
  posterSets.reduce((total, set) => {
    const selected = set.selected_types;
    return (
      total +
      (set.images ?? []).filter((image) => {
        switch (image.type) {
          case "poster":
            return selected.poster;
          case "backdrop":
            return selected.backdrop;
          case "season_poster":
            return image.season_number === 0 ? selected.special_season_poster : selected.season_poster;
          case "titlecard":
            return selected.titlecard;
          default:
            return false;
        }
      }).length
    );
  }, 0);

// SetDetails is the set's name, then its ID and creator
const SetDetails: React.FC<{ title: string; id: string; creator: string }> = ({ title, id, creator }) => (
  <div className="w-full">
    <span className="text-sm text-muted-foreground">Set: {title}</span>
    <div className="flex flex-row items-center justify-between gap-2 rounded-sm hover:bg-muted/50">
      <span className="text-primary text-sm shrink-0">{id}</span>
      {creator && (
        <div className="flex items-center gap-1">
          <CreatorAvatar username={creator} className="rounded-lg mr-1 w-4 h-4" iconClassName="w-4 h-4" />
          <Link href={`/user/${creator}`} className="text-primary hover:underline text-xs text-right">
            {creator}
          </Link>
        </div>
      )}
    </div>
  </div>
);

// retryStatus says how many times a failure was retried, and when it's retried next
const retryStatus = (entry: DownloadHistoryEntry, maxRetries: number) => {
  if (!entry.set_id) return "";
  if (entry.next_retry_at) {
    const next = new Date(entry.next_retry_at);
    const time = next.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit" });
    const day =
      next.toDateString() === new Date().toDateString() ? time : `${next.toLocaleDateString("en-US")} at ${time}`;
    return `Retry ${entry.retry_count + 1} of ${maxRetries} at ${day}`;
  }
  if (entry.retry_count >= maxRetries) {
    return `Retried ${entry.retry_count} ${entry.retry_count === 1 ? "time" : "times"}, no more automatic retries`;
  }
  if (entry.retry_count > 0) return `Retried ${entry.retry_count} of ${maxRetries} times`;
  return "";
};

const hasSelectedTypes = (selected: DBPosterSetDetail["selected_types"] | undefined) =>
  !!selected &&
  (selected.poster ||
    selected.backdrop ||
    selected.season_poster ||
    selected.special_season_poster ||
    selected.titlecard);

const DownloadEntryCard: React.FC<DownloadEntryCardProps> = (props) => {
  const { setMediaItem } = useMediaStore();
  const [retrying, setRetrying] = React.useState(false);

  const isQueue = props.mode === "queue";

  const mediaItem: MediaItem | undefined = isQueue ? props.job.item.media_item : undefined;
  const posterSets = isQueue ? (Array.isArray(props.job.item.poster_sets) ? props.job.item.poster_sets : []) : [];

  const collectionImages = isQueue ? props.job.item.collection_images : undefined;
  // An entry with only collection images is shown as its collection
  const collectionOnly = posterSets.length === 0 && !!collectionImages;
  const collectionPoster = collectionImages?.images.find((img) => img.type === "collection_poster");

  const title = collectionOnly
    ? collectionImages!.collection.title
    : isQueue
      ? mediaItem!.title
      : props.entry.media_item_title;
  const year = isQueue ? mediaItem!.year : props.entry.media_item_year;
  const libraryTitle = collectionOnly
    ? collectionImages!.collection.library_title
    : isQueue
      ? mediaItem!.library_title
      : props.entry.library_title;
  const edition = isQueue ? mediaItem!.edition : props.entry.edition;
  const ratingKey = isQueue ? mediaItem!.rating_key : props.entry.rating_key;
  const imageCount = isQueue ? countSelectedImages(posterSets) + (collectionImages?.images.length ?? 0) : 0;

  const onDeleteConfirm = async () => {
    if (isQueue) {
      try {
        const response = await RemoveItemFromQueue(props.job.id);
        if (response.status === "error") {
          toast.error(
            `Error deleting from queue: ${response.error?.message || "Unknown error occurred trying to delete."}`
          );
        } else {
          toast.success(response.data?.result || "Successfully deleted from queue.");
        }
      } catch (error) {
        toast.error(
          `Error deleting from queue: ${error instanceof Error ? error.message : "Unknown error occurred trying to delete."}`
        );
      }
      if (props.fetchQueueEntries) {
        await props.fetchQueueEntries();
      }
    } else {
      try {
        const response = await RemoveDownloadHistoryEntry(props.entry.id);
        if (response.status === "error") {
          toast.error(`Error deleting history entry: ${response.error?.message || "Unknown error occurred."}`);
        } else {
          toast.success(response.data?.result || "Successfully deleted history entry.");
        }
      } catch (error) {
        toast.error(
          `Error deleting history entry: ${error instanceof Error ? error.message : "Unknown error occurred."}`
        );
      }
      if (props.fetchHistory) {
        await props.fetchHistory();
      }
    }
  };

  // onRetry adds the failed set back to the Download Queue
  const onRetry = async () => {
    if (isQueue) return;
    setRetrying(true);
    try {
      const response = await RetryDownloadHistoryEntry(props.entry.id);
      if (response.status === "error") {
        toast.error(`Error retrying download: ${response.error?.message || "Unknown error occurred."}`);
      } else {
        toast.success(response.data?.result || "Added to the Download Queue.");
        if (props.fetchHistory) await props.fetchHistory();
      }
    } catch (error) {
      toast.error(`Error retrying download: ${error instanceof Error ? error.message : "Unknown error occurred."}`);
    }
    setRetrying(false);
  };

  return (
    <Card className="relative w-full max-w-md mx-auto">
      <CardHeader>
        {/* Top Left: Auto Download Icon or the Download Result */}
        <div className="absolute top-2 left-2 flex items-center gap-2">
          {collectionOnly ? null : isQueue ? (
            posterSets.some((set) => set.auto_download) ? (
              <Popover>
                <PopoverTrigger asChild>
                  <RefreshCcw className="text-green-500 cursor-help" size={24} />
                </PopoverTrigger>
                <PopoverContent className="p-2 max-w-xs">
                  <p className="text-sm">
                    Auto Download is enabled for this item. Once it's downloaded, it will be periodically checked for
                    new updates based on your Auto Download settings.
                  </p>
                </PopoverContent>
              </Popover>
            ) : (
              <Popover>
                <PopoverTrigger asChild>
                  <RefreshCwOff className="text-red-500 cursor-help" size={24} />
                </PopoverTrigger>
                <PopoverContent className="p-2 max-w-xs">
                  <p className="text-sm">Auto Download is disabled for this item.</p>
                </PopoverContent>
              </Popover>
            )
          ) : (
            <Badge variant="outline" className={cn("text-sm", statusStyles[props.entry.status])}>
              {props.entry.status.toUpperCase()}
            </Badge>
          )}
        </div>

        {/* Top Right: Retry and Delete */}
        <div className="absolute top-2 right-2 flex items-center gap-1">
          {/* Retry, only a failure with a set can be retried */}
          {!isQueue && props.entry.set_id && (
            <Button
              variant="ghost"
              className="p-1 h-auto shadow-none hover:bg-muted/50 hover:text-primary cursor-pointer"
              disabled={retrying}
              onClick={onRetry}
              title="Retry"
              aria-label="Retry"
            >
              <RotateCw className={cn("w-5 h-5", retrying && "animate-spin")} />
            </Button>
          )}

          <ConfirmDestructiveDialogActionButton
            variant="ghost"
            className="text-destructive p-1 shadow-none hover:bg-muted/50 hover:text-red-500 cursor-pointer"
            confirmText={isQueue ? "Remove" : "Delete Entry"}
            title={isQueue ? "Remove from the Download Queue?" : "Delete History Entry?"}
            description={
              isQueue
                ? "Are you sure you want to remove this item from the download queue? This action cannot be undone."
                : "Are you sure you want to delete this download history entry? This action cannot be undone."
            }
            onConfirm={onDeleteConfirm}
          >
            <Trash2 className="w-5 h-5" />
          </ConfirmDestructiveDialogActionButton>
        </div>
      </CardHeader>

      {/* Middle: Image, the collection poster for collection images */}
      {collectionOnly && collectionPoster && (
        <div className="flex justify-center">
          <AssetImage
            image={collectionPoster}
            imageType="mediux"
            aspect="poster"
            className="w-[90%] h-auto transition-transform hover:scale-105"
          />
        </div>
      )}
      {!collectionOnly && ratingKey && (
        <div className="flex justify-center">
          <AssetImage
            image={isQueue ? mediaItem! : ({ rating_key: ratingKey } as MediaItem)}
            imageType="item"
            className="w-[90%] h-auto transition-transform hover:scale-105"
          />
        </div>
      )}

      {/* Content */}
      <CardContent className="p-0 ml-2 mr-2">
        {/* Title */}
        <H4>
          {isQueue && !collectionOnly ? (
            <Link
              href={"/media-item/"}
              className="text-primary hover:underline text-md"
              onClick={() => {
                setMediaItem(mediaItem!);
              }}
            >
              {title}
            </Link>
          ) : (
            <span className="text-md">{title}</span>
          )}
        </H4>

        <div className="flex flex-col gap-1 mb-2">
          <span className="text-xs sm:text-sm text-muted-foreground inline-block">
            {collectionOnly ? "Collection" : year} · {libraryTitle}
            {!collectionOnly && edition ? ` · ${edition}` : ""}
          </span>

          {isQueue && (
            <span className="text-xs sm:text-sm text-muted-foreground inline-block">
              Number of Images:
              <br />
              {imageCount}
            </span>
          )}
        </div>

        {/* Set */}
        {isQueue ? (
          <>
            {posterSets.map((set) => (
              <SetDetails key={set.id} title={set.title} id={set.id} creator={set.user_created} />
            ))}
            {collectionImages && !collectionOnly && (
              <span className="w-full text-sm text-muted-foreground">
                Collection: {collectionImages.collection.title}
              </span>
            )}
          </>
        ) : (
          props.entry.set_id && (
            <SetDetails title={props.entry.set_title} id={props.entry.set_id} creator={props.entry.set_creator} />
          )
        )}

        <Separator className="my-4" />

        {isQueue ? (
          collectionImages ||
          posterSets.some(
            (set) =>
              set.selected_types.poster ||
              set.selected_types.backdrop ||
              set.selected_types.season_poster ||
              set.selected_types.titlecard
          ) ? (
            <div className="flex flex-wrap gap-2">{renderTypeBadges(props.job.item)}</div>
          ) : (
            <div className="flex flex-wrap gap-2">
              <Badge key={"no-types"} variant="outline" className="text-sm bg-red-500">
                No Selected Types
              </Badge>
            </div>
          )
        ) : (
          <>
            {hasSelectedTypes(props.entry.selected_types) && (
              <>
                <div className="flex flex-wrap gap-2">
                  {renderTypeBadges({
                    poster_sets: [{ selected_types: props.entry.selected_types }],
                  } as unknown as DBSavedItem)}
                </div>
                <Separator className="my-4" />
              </>
            )}

            <div className="flex flex-wrap gap-2">
              <Badge variant="outline" className="text-sm border-green-400 text-green-500">
                {props.entry.images_succeeded} Succeeded
              </Badge>
              {props.entry.warning_count > 0 && (
                <Badge variant="outline" className="text-sm border-yellow-400 text-yellow-500">
                  {props.entry.warning_count} {props.entry.warning_count === 1 ? "Warning" : "Warnings"}
                </Badge>
              )}
              {props.entry.images_failed > 0 && (
                <Badge variant="outline" className="text-sm border-red-400 text-red-500">
                  {props.entry.images_failed} Failed
                </Badge>
              )}
            </div>

            {retryStatus(props.entry, props.maxRetries) && (
              <p className="flex items-center gap-1 text-xs text-muted-foreground mt-2">
                <RotateCw className="h-3 w-3 shrink-0" />
                {retryStatus(props.entry, props.maxRetries)}
              </p>
            )}

            {props.entry.failed_images.length > 0 && (
              <Accordion type="single" collapsible className="mt-3">
                <AccordionItem value="failed_images">
                  <AccordionTrigger className="cursor-pointer text-sm">Failed Images</AccordionTrigger>
                  <AccordionContent>
                    <ul className="text-xs space-y-1">
                      {props.entry.failed_images.map((image, idx) => {
                        const label = formatImageTypeLabel(image);
                        return (
                          <li key={idx} className="text-red-500">
                            {label && <b>{label}</b>}
                            {image.failure_reason ? `${label ? ": " : ""}${image.failure_reason}` : ""}
                          </li>
                        );
                      })}
                    </ul>
                  </AccordionContent>
                </AccordionItem>
              </Accordion>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
};

export default DownloadEntryCard;
