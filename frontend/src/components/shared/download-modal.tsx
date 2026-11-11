"use client";

import type { CollectionItem } from "@/app/collections/page";
import { formatDownloadProgressShortText, formatDownloadProgressText } from "@/helper/format-download-progress";
import { formatDownloadSize } from "@/helper/format-download-size";
import { upsertSavedSets } from "@/helper/media-item-update-saved-sets";
import { AddNewItemToDB } from "@/services/database/add";
import type { DownloadResult } from "@/services/database/add";
import { AddItemToDownloadQueue } from "@/services/downloads/queue-add";
import { subscribeToDownloadQueue } from "@/services/downloads/queue-stream";
import { GetMediaItemDetails } from "@/services/mediaserver/get-media-item-details";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  Check,
  CircleAlert,
  Database,
  DatabaseZap,
  Download,
  Loader,
  OctagonMinus,
  RefreshCcw,
  TriangleAlert,
  User,
  X,
} from "lucide-react";
import { z } from "zod";

import { Fragment, useEffect, useRef, useState } from "react";
import React from "react";
import type { ControllerRenderProps } from "react-hook-form";
import { useForm, useWatch } from "react-hook-form";

import Link from "next/link";
import { useRouter } from "next/navigation";

import { AssetImage } from "@/components/shared/asset-image";
import DownloadModalPopover from "@/components/shared/download-modal-popover";
import { DownloadProgressBar } from "@/components/shared/download-progress-bar";
import { PopoverHelp } from "@/components/shared/popover-help";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Form, FormControl, FormField, FormItem, FormLabel } from "@/components/ui/form";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Lead } from "@/components/ui/typography";

import { cn } from "@/lib/cn";
import { log } from "@/lib/logger";
import { useMediaStore } from "@/lib/stores/global-store-media-store";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import type { DBPosterSetDetail, DBSavedItem, PosterSet } from "@/types/database/db-poster-set";
import type { DownloadQueueJob, DownloadQueueProgress } from "@/types/database/download-queue";
import type { MediaItem, SelectedTypes } from "@/types/media-and-posters/media-item-and-library";
import type { BaseSetInfo, CollectionItemImageFile, ImageFile, SetSummary } from "@/types/media-and-posters/sets";
import {
  DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS,
  DOWNLOAD_IMAGE_TYPE_OPTIONS,
  TYPE_DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS,
  TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS,
} from "@/types/ui-options";

export interface FormItemDisplay {
  MediaItem: MediaItem;
  Set: PosterSet;
}

type SourceType = "show" | "movie" | "collection";

interface AssetTypeFormValues {
  types: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[];
  autodownload?: boolean;
  autoupgrade?: boolean;
  addToDBOnly?: boolean;
  source?: SourceType;
}

type TaskStatus = "pending" | "in-progress" | "completed" | "failed" | "skipped";

type AddToDBTaskPayload = {
  kind: "addToDB";
  itemKey: string;
  itemTitle: string;
  mediaItem: MediaItem;
  posterSet: DBPosterSetDetail;
  addToDBOnly: boolean;
  downloadResult?: DownloadResult; // The images downloaded for the set, for the statistics
};

// Downloads the item's images, or the collection's own images, in the Download Queue
type AddToQueueTaskPayload = {
  kind: "addToQueue";
  itemKey: string;
  itemTitle: string;
  dbItem: DBSavedItem;
  imageCount: number; // The images the job downloads, until the queue says
  formItem?: FormItemDisplay; // Marked as saved on the page when the job finishes
  jobId?: number; // The job in the Download Queue, once it's added
};

// Non-Retryable “record only” task (e.g. fetch latest media item failed)
type NoteTaskPayload = {
  kind: "note";
  itemKey: string;
  itemTitle: string;
};

type TaskPayload = AddToDBTaskPayload | AddToQueueTaskPayload | NoteTaskPayload;

type Task = {
  id: string;
  status: TaskStatus;
  label: string;
  attempts: number;
  payload: TaskPayload;
  error?: string;
};

type ItemProgress = {
  itemKey: string;
  title: string;
  tasks: Task[];
};

type DownloadProgress = {
  currentText: string;
  currentShortText?: string; // currentText for narrow screens
  totalPlanned: number;
  items: Record<string, ItemProgress>;
};

// helper for stable ids
const newId = () => globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`;

// derive progress counts from tasks, with the images of the Download Queue jobs (uses totalPlanned so total doesn't grow as tasks are added)
const getOverallCounts = (state: DownloadProgress, jobProgress: Record<number, DownloadQueueProgress>) => {
  let done = 0;
  let total = 0;
  for (const task of Object.values(state.items).flatMap((i) => i.tasks || [])) {
    // Notes and skipped tasks shouldn't affect progress totals
    if (task.payload.kind === "note" || task.status === "skipped") continue;
    const finished = task.status === "completed" || task.status === "failed";
    if (task.payload.kind === "addToQueue") {
      const job = task.payload.jobId ? jobProgress[task.payload.jobId] : undefined;
      const images = job?.total || task.payload.imageCount || 1;
      total += images;
      done += finished ? images : (job?.done ?? 0);
    } else {
      total += 1;
      if (finished) done += 1;
    }
  }

  total = Math.max(total, state.totalPlanned);
  if (total === 0) return { done: 0, total: 1 };
  return { done: Math.min(done, total), total };
};

// derive progress (0..100) from tasks
const getOverallProgress = (state: DownloadProgress, jobProgress: Record<number, DownloadQueueProgress>) => {
  const { done, total } = getOverallCounts(state, jobProgress);
  return Math.round((done / total) * 100);
};

const getErrorsByItem = (state: DownloadProgress) =>
  Object.values(state.items)
    .map((i) => ({
      itemKey: i.itemKey,
      title: i.title,
      errors: i.tasks.filter((t) => t.status === "failed"),
    }))
    .filter((x) => x.errors.length > 0);

// queuedImageCount is how many of the set's images a Download Queue job downloads for the item, like the queue counts them
const queuedImageCount = (images: ImageFile[], types: string[], mediaItem: MediaItem) =>
  images.filter((img) => {
    const seasons = mediaItem.series?.seasons ?? [];
    switch (img.type) {
      case "poster":
        return types.includes("poster");
      case "backdrop":
        return types.includes("backdrop");
      case "season_poster":
        return (
          types.includes(img.season_number === 0 ? "special_season_poster" : "season_poster") &&
          seasons.some((season) => season.season_number === img.season_number)
        );
      case "titlecard":
        return (
          types.includes("titlecard") &&
          seasons.some(
            (season) =>
              season.season_number === img.season_number &&
              season.episodes.some((ep) => ep.episode_number === img.episode_number)
          )
        );
      default:
        return false;
    }
  }).length;

// Unique key for a media item in the form
// Some media servers (e.g. Silo) use the same rating key for an item in every library it is in,
// so the library title is part of the key, like it is for the media items in the database
const getItemFormKey = (mediaItem: MediaItem) =>
  `${mediaItem.rating_key}_${(mediaItem.library_title || "").replace(/[^a-zA-Z0-9]/g, "_")}`;

const formSchema = z
  .object({
    selectedOptionsByItem: z.record(
      z.string(),
      z.object({
        types: z.array(z.enum(DOWNLOAD_IMAGE_TYPE_OPTIONS.map((opt) => opt.value as TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS))),
        autodownload: z.boolean().optional(),
        autoupgrade: z.boolean().optional(),
        source: z.enum(["movie", "collection"]).optional(),
        addToDBOnly: z.boolean().optional(),
      })
    ),
    collectionTypes: z.array(z.enum(["collection_poster", "collection_backdrop"])).optional(),
  })
  .refine(
    (data) =>
      Object.values(data.selectedOptionsByItem).some((item) => Array.isArray(item.types) && item.types.length > 0) ||
      (data.collectionTypes?.length ?? 0) > 0,
    {
      message: "Please select at least one image type to download.",
      path: ["selectedOptionsByItem.message"],
    }
  );

export interface DownloadModalProps {
  baseSetInfo: BaseSetInfo;
  formItems: FormItemDisplay[];
  autoDownloadDefault?: boolean;
  onDownloadComplete?: (item: MediaItem) => void;
  // Libraries that share the same media server item (e.g. Silo), so the images apply to all of them
  sharedLibraryTitles?: string[];
  // A collection set's own poster and backdrop, and the media server collection they apply to
  collectionImages?: { collection: CollectionItem; images: CollectionItemImageFile[] };
}

// The key of the collection images in the download progress
const COLLECTION_PROGRESS_KEY = "collection";

// The error of a job that was removed from the Download Queue before it finished
const STOPPED_ERROR = "Removed from the Download Queue before it finished";

interface DuplicateMap {
  [mediaItemKey: string]: {
    options: Array<{
      id: string;
      type: "movie" | "collection";
      image?: ImageFile;
    }>;
    selectedType?: "movie" | "collection" | "";
  };
}

const findDuplicateMediaItems = (items: FormItemDisplay[]): DuplicateMap => {
  return items.reduce((acc: DuplicateMap, item) => {
    if (!acc[getItemFormKey(item.MediaItem)]) {
      acc[getItemFormKey(item.MediaItem)] = {
        options: [
          {
            id: item.Set.id,
            type: item.Set.type as "movie" | "collection",
            image:
              item.Set.images.find((img) => img.type === "poster") ||
              item.Set.images.find((img) => img.type === "backdrop"),
          },
        ],
      };
    } else {
      acc[getItemFormKey(item.MediaItem)].options.push({
        id: item.Set.id,
        type: item.Set.type as "movie" | "collection",
        image:
          item.Set.images.find((img) => img.type === "poster") ||
          item.Set.images.find((img) => img.type === "backdrop"),
      });
    }
    return acc;
  }, {});
};

const DownloadModal: React.FC<DownloadModalProps> = ({
  baseSetInfo,
  formItems,
  autoDownloadDefault = true, // Default to true if not provided
  onDownloadComplete,
  sharedLibraryTitles = [],
  collectionImages,
}) => {
  const router = useRouter();
  const [isMounted, setIsMounted] = useState(false);

  // True when formItems spans more than one library (e.g. the same title also downloading
  // to another library), so each item's card can be labeled with which library it belongs to.
  const showLibraryNames = new Set(formItems.map((item) => item.MediaItem.library_title)).size > 1;

  // Download Progress
  const [progress, setProgress] = useState<DownloadProgress>({
    currentText: "",
    totalPlanned: 0,
    items: {},
  });

  // State - Modal Button Texts
  const [buttonTexts, setButtonTexts] = useState({
    cancel: "Cancel",
    download: "Download",
  });

  // State - Selected Types - Files Size and Download Size
  const [selectedSizes, setSelectedSizes] = useState({
    fileCount: 0,
    downloadSize: 0,
    poster: 0,
    backdrop: 0,
    season_poster: 0,
    special_season_poster: 0,
    titlecard: 0,
  });

  // State - Duplicate Media Items
  const [duplicates, setDuplicates] = useState<DuplicateMap>({});

  // The set, sent with collection images for the statistics
  const setSummary: SetSummary = {
    id: baseSetInfo.id,
    title: baseSetInfo.title,
    type: baseSetInfo.type,
    user_created: baseSetInfo.user_created,
  };

  // The window's jobs in the Download Queue, their progress, and the task each job is for
  const [jobProgress, setJobProgress] = useState<Record<number, DownloadQueueProgress>>({});
  const queueJobsRef = useRef<Record<number, AddToQueueTaskPayload & { taskId: string }>>({});
  const [waitingJobs, setWaitingJobs] = useState(0);
  const [followingQueue, setFollowingQueue] = useState(false);
  const [queuingDone, setQueuingDone] = useState(false);
  const defaultAddToDBOnly = useUserPreferencesStore((state) => state.defaultAddToDBOnly);
  const autoUpgradeSets = useUserPreferencesStore((state) => state.autoUpgradeSets);
  const defaultAutoUpgrade = useUserPreferencesStore((state) => state.defaultAutoUpgrade);

  // State - Add New Collection Items
  const defaultAutoAddNewCollectionItems = useUserPreferencesStore((state) => state.defaultAutoAddNewCollectionItems);
  const [autoAddNewCollectionItems, setAutoAddNewCollectionItems] = useState(defaultAutoAddNewCollectionItems);

  // User Preferences
  const { downloadDefaults } = useUserPreferencesStore();

  // The collection image types this set has, checked to start when they're in the Download Defaults
  const collectionImageTypes = DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS.map((opt) => opt.value).filter((type) =>
    collectionImages?.images.some((img) => img.type === type)
  );
  const defaultCollectionTypes = collectionImageTypes.filter((type) => downloadDefaults.includes(type));

  // Media Store
  const { setMediaItem } = useMediaStore();

  // Search Query Store
  const { setSearchQuery } = useSearchQueryStore();

  // Cancel Ref
  const cancelRef = useRef(false);

  // Function - Reset Progress Values
  const resetProgress = () => {
    setProgress({
      currentText: "",
      totalPlanned: 0,
      items: {},
    });
  };

  // Function - Close Modal
  const handleClose = () => {
    cancelRef.current = true;
    setIsMounted(false);
    resetProgress();
    // The jobs keep going in the Download Queue
    queueJobsRef.current = {};
    setJobProgress({});
    setWaitingJobs(0);
    setFollowingQueue(false);
    setQueuingDone(false);
    setButtonTexts({
      cancel: "Cancel",
      download: "Download",
    });
    form.reset();
  };

  // Function - Handle Link Click
  const getMediuxBaseUrl = () => {
    if (baseSetInfo.type === "boxset") {
      return `https://mediux.io/boxset/${baseSetInfo.id}`;
    } else {
      return `https://mediux.io/${baseSetInfo.type}-set/${baseSetInfo.id}`;
    }
  };

  // Compute Asset Types based on what the form item has
  const computeAssetTypes = (item: FormItemDisplay): TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[] => {
    if (!item.Set) return [];

    const setHasPoster = item.Set.images.some((img) => img.type === "poster");
    const setHasBackdrop = item.Set.images.some((img) => img.type === "backdrop");
    const setHasSeasonPosters = item.Set.images.some((img) => img.type === "season_poster" && img.season_number !== 0);
    const mediaItemHasSpecialSeason =
      item.MediaItem.series?.seasons?.some((season) => season.season_number === 0) ?? false;
    const setHasSpecialSeasonPosters =
      mediaItemHasSpecialSeason &&
      item.Set.images.some((img) => img.type === "season_poster" && img.season_number === 0);
    const setHasTitleCards = item.Set.images.some((img) => img.type === "titlecard");
    const types: (TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS | null)[] = [
      setHasPoster ? "poster" : null,
      setHasBackdrop ? "backdrop" : null,
      setHasSeasonPosters ? "season_poster" : null,
      setHasSpecialSeasonPosters ? "special_season_poster" : null,
      setHasTitleCards ? "titlecard" : null,
    ];

    return types.filter((type): type is TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS => type !== null);
  };

  const getPossibleFutureAssetTypes = (item: FormItemDisplay): TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[] => {
    const supportedTypes: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[] =
      item.Set.type === "show"
        ? ["poster", "backdrop", "season_poster", "special_season_poster", "titlecard"]
        : ["poster", "backdrop"];

    const existingTypes = new Set(computeAssetTypes(item));
    return supportedTypes.filter((type) => !existingTypes.has(type));
  };

  // Define Form
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    mode: "onChange",
    defaultValues: {
      selectedOptionsByItem: formItems.reduce(
        (acc, item) => {
          acc[getItemFormKey(item.MediaItem)] = {
            types: computeAssetTypes(item).filter((type) => downloadDefaults.includes(type)),
            autodownload: autoDownloadDefault,
            autoupgrade: defaultAutoUpgrade,
            addToDBOnly: defaultAddToDBOnly,
            source: item.Set.type === "movie" || item.Set.type === "collection" ? item.Set.type : undefined,
          };
          return acc;
        },
        {} as z.infer<typeof formSchema>["selectedOptionsByItem"]
      ),
      collectionTypes: defaultCollectionTypes,
    },
  });

  const watchSelectedOptions = useWatch({
    control: form.control,
    name: "selectedOptionsByItem",
  });
  const watchCollectionTypes = useWatch({
    control: form.control,
    name: "collectionTypes",
  });

  // Reset form on mount
  useEffect(() => {
    form.reset({
      selectedOptionsByItem: formItems.reduce(
        (acc, item) => {
          acc[getItemFormKey(item.MediaItem)] = {
            types: computeAssetTypes(item).filter((type) => downloadDefaults.includes(type)),
            autodownload: autoDownloadDefault,
            autoupgrade: defaultAutoUpgrade,
            addToDBOnly: defaultAddToDBOnly,
            source: item.Set.type === "movie" || item.Set.type === "collection" ? item.Set.type : undefined,
          };
          return acc;
        },
        {} as z.infer<typeof formSchema>["selectedOptionsByItem"]
      ),
      collectionTypes: defaultCollectionTypes,
    });
    // defaultCollectionTypes comes from collectionImages and downloadDefaults
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    formItems,
    form,
    autoDownloadDefault,
    downloadDefaults,
    defaultAddToDBOnly,
    defaultAutoUpgrade,
    collectionImages,
  ]);

  useEffect(() => {
    // If all the form items are set to "Add to Database Only", change button text
    if (Object.values(watchSelectedOptions).every((option) => option.addToDBOnly || option.types?.length === 0)) {
      setButtonTexts((prev) => ({
        ...prev,
        download: "Add to Database",
      }));
    } else {
      setButtonTexts((prev) => ({
        ...prev,
        download: "Download",
      }));
    }
  }, [watchSelectedOptions]);

  useEffect(() => {
    const dups = findDuplicateMediaItems(formItems);
    setDuplicates(
      Object.fromEntries(
        Object.entries(dups)
          .filter(([, value]) => value.options.length > 1)
          .map(([key, value]) => {
            // Set initial selection to "movie" if available
            const hasMovie = value.options.some((opt) => opt.type === "movie");
            return [
              key,
              {
                ...value,
                selectedType: hasMovie ? "movie" : "collection",
              },
            ];
          })
      )
    );

    // Update form values for duplicates
    Object.entries(dups)
      .filter(([, value]) => value.options.length > 1)
      .forEach(([key, value]) => {
        const hasMovie = value.options.some((opt) => opt.type === "movie");

        // Uncheck collection if movie is available
        if (hasMovie) {
          form.setValue(`selectedOptionsByItem.${key}`, {
            ...form.getValues(`selectedOptionsByItem.${key}`),
            source: "movie",
          });
        }
      });
  }, [formItems, form]);

  useEffect(() => {
    const calculateSizes = () => {
      let totalFiles = 0;
      let totalSize = 0;
      let numOfPosters = 0;
      let numOfBackdrops = 0;
      let numOfSeasonPosters = 0;
      let numOfSpecialSeasonPosters = 0;
      let numOfTitleCards = 0;

      // Iterate through each selected item
      Object.entries(watchSelectedOptions).forEach(([itemKey, selection]) => {
        // Find the corresponding form item to access PosterFile sizes
        const formItem = formItems.find((item) => getItemFormKey(item.MediaItem) === itemKey);

        if (!formItem || !selection.types || selection.addToDBOnly) return;

        // Count files and sum sizes for each selected type
        selection.types.forEach((type) => {
          switch (type) {
            case "poster": {
              const posterImage = formItem.Set.images.find((img) => img.type === "poster" && img.file_size);
              if (posterImage && posterImage.file_size) {
                totalSize += posterImage.file_size;
                totalFiles += 1;
                numOfPosters += 1;
              }
              break;
            }
            case "backdrop": {
              const backdropImage = formItem.Set.images.find((img) => img.type === "backdrop" && img.file_size);
              if (backdropImage && backdropImage.file_size) {
                totalSize += backdropImage.file_size;
                totalFiles += 1;
                numOfBackdrops += 1;
              }
              break;
            }
            case "season_poster":
              formItem.Set.images.forEach((sp) => {
                if (sp.type === "season_poster" && sp.season_number !== 0 && sp.file_size) {
                  if (formItem.MediaItem.series && formItem.MediaItem.series.seasons) {
                    // Check if season exists in MediaItem
                    const seasonExists = formItem.MediaItem.series.seasons.some(
                      (season) => season.season_number === sp.season_number
                    );
                    if (seasonExists) {
                      totalSize += sp.file_size;
                      totalFiles += 1;
                      numOfSeasonPosters += 1;
                    }
                  } else {
                    totalSize += sp.file_size;
                    totalFiles += 1;
                    numOfSeasonPosters += 1;
                  }
                }
              });
              break;
            case "special_season_poster":
              formItem.Set.images.forEach((sp) => {
                if (sp.type === "season_poster" && sp.season_number === 0 && sp.file_size) {
                  if (formItem.MediaItem.series && formItem.MediaItem.series.seasons) {
                    // Check if special season exists in MediaItem
                    const specialSeasonExists = formItem.MediaItem.series.seasons.some(
                      (season) => season.season_number === 0
                    );
                    if (specialSeasonExists) {
                      totalSize += sp.file_size;
                      totalFiles += 1;
                      numOfSpecialSeasonPosters += 1;
                    }
                  } else {
                    totalSize += sp.file_size;
                    totalFiles += 1;
                    numOfSpecialSeasonPosters += 1;
                  }
                }
              });
              break;
            case "titlecard":
              formItem.Set.images.forEach((tc) => {
                if (tc.type === "titlecard" && tc.file_size) {
                  if (formItem.MediaItem.series && formItem.MediaItem.series.seasons) {
                    // Check if episode exists in MediaItem
                    const episodeExists = formItem.MediaItem.series.seasons.some((season) =>
                      season.episodes.some(
                        (ep) => ep.season_number === tc.season_number && ep.episode_number === tc.episode_number
                      )
                    );
                    if (episodeExists) {
                      totalSize += tc.file_size;
                      totalFiles += 1;
                      numOfTitleCards += 1;
                    }
                  } else {
                    totalSize += tc.file_size;
                    totalFiles += 1;
                    numOfTitleCards += 1;
                  }
                }
              });
              break;
          }
        });
      });

      setSelectedSizes({
        fileCount: totalFiles,
        downloadSize: totalSize,
        poster: numOfPosters,
        backdrop: numOfBackdrops,
        season_poster: numOfSeasonPosters,
        special_season_poster: numOfSpecialSeasonPosters,
        titlecard: numOfTitleCards,
      });
    };

    calculateSizes();
  }, [formItems, watchSelectedOptions]);

  const LOG_VALUES = () => {
    log("INFO", "Download Modal", "Debug Info", "Logging props values:", {
      baseSetInfo,
      autoDownloadDefault,
    });
    log("INFO", "Download Modal", "Debug Info", "Logging form items:", formItems);
    log("INFO", "Download Modal", "Debug Info", "Logging form values:", form);
    log("INFO", "Download Modal", "Debug Info", "Logging watch selected types:", watchSelectedOptions);
    log("INFO", "Download Modal", "Debug Info", "Logging progress:", progress);
    log("INFO", "Download Modal", "Debug Info", "Logging duplicates:", duplicates);
  };

  // --- Progress/Task Helpers ---
  const setCurrentText = (text: string, shortText?: string) => {
    setProgress((prev) => ({
      ...prev,
      currentText: text,
      currentShortText: shortText,
    }));
  };

  const upsertItem = (itemKey: string, title: string) => {
    setProgress((prev) => {
      if (prev.items[itemKey]) return prev;
      return {
        ...prev,
        items: {
          ...prev.items,
          [itemKey]: { itemKey, title, tasks: [] },
        },
      };
    });
  };

  const addTask = (itemKey: string, title: string, task: Task) => {
    setProgress((prev) => {
      const existing = prev.items[itemKey] ?? { itemKey, title, tasks: [] as Task[] };
      return {
        ...prev,
        items: {
          ...prev.items,
          [itemKey]: {
            ...existing,
            title: existing.title || title,
            tasks: [...existing.tasks, task],
          },
        },
      };
    });
  };

  const updateTask = (taskId: string, updater: (t: Task) => Task) => {
    setProgress((prev) => {
      let changed = false;
      const nextItems: Record<string, ItemProgress> = {};

      for (const [itemKey, item] of Object.entries(prev.items)) {
        const idx = item.tasks.findIndex((t) => t.id === taskId);
        if (idx === -1) {
          nextItems[itemKey] = item;
          continue;
        }

        const nextTasks = [...item.tasks];
        nextTasks[idx] = updater(nextTasks[idx]);
        nextItems[itemKey] = { ...item, tasks: nextTasks };
        changed = true;
      }

      return changed ? { ...prev, items: nextItems } : prev;
    });
  };

  const findTask = (state: DownloadProgress, taskId: string): Task | undefined => {
    for (const item of Object.values(state.items)) {
      const t = item.tasks.find((x) => x.id === taskId);
      if (t) return t;
    }
    return undefined;
  };

  const getAssetStatus = (itemKey: string, assetType: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS): TaskStatus | undefined => {
    const tasks = progress.items[itemKey]?.tasks ?? [];
    const relevant = tasks.filter(
      (t) => t.payload.kind === "addToQueue" && t.payload.dbItem.poster_sets[0]?.selected_types?.[assetType]
    );
    if (relevant.length === 0) return undefined;

    // Aggregate status so multi-file types don't "lose" completion due to a final skipped task.
    if (relevant.some((t) => t.status === "failed")) return "failed";
    if (relevant.some((t) => t.status === "in-progress")) return "in-progress";
    if (relevant.some((t) => t.status === "pending")) return "pending";
    if (relevant.some((t) => t.status === "completed")) return "completed";
    return "skipped"; // all skipped
  };

  // Mark the item as saved on the page
  const markItemSaved = (latestMediaItem: MediaItem, item: FormItemDisplay, selectedTypes: SelectedTypes) => {
    onDownloadComplete?.(upsertSavedSets(latestMediaItem, item.Set.id, item.Set.user_created, selectedTypes));
  };

  const runAddToDBTask = async (taskId: string, payload: AddToDBTaskPayload): Promise<boolean> => {
    updateTask(taskId, (t) => ({
      ...t,
      status: "in-progress",
      attempts: (t.attempts ?? 0) + 1,
      error: undefined,
    }));
    setCurrentText(`Adding "${payload.itemTitle}" to DB`, "Adding to DB");

    try {
      const resp = await AddNewItemToDB(
        payload.mediaItem,
        payload.posterSet,
        payload.addToDBOnly,
        autoAddNewCollectionItems,
        payload.downloadResult
      );
      if (resp.status === "error") {
        throw new Error(resp.error?.message || (typeof resp.error === "string" ? resp.error : "Unknown error"));
      }
      updateTask(taskId, (t) => ({ ...t, status: "completed" }));
      return true;
    } catch (error) {
      updateTask(taskId, (t) => ({
        ...t,
        status: "failed",
        error: `"${payload.itemTitle}" - Add to DB failed - ${error instanceof Error ? error.message : "Unknown error"}`,
      }));
      return false;
    }
  };

  const runAddToQueueTask = async (taskId: string, payload: AddToQueueTaskPayload): Promise<boolean> => {
    updateTask(taskId, (t) => ({
      ...t,
      status: "in-progress",
      attempts: (t.attempts ?? 0) + 1,
      error: undefined,
    }));
    setCurrentText(`Adding "${payload.itemTitle}" to the Download Queue`, "Adding to the queue");

    try {
      const resp = await AddItemToDownloadQueue(payload.dbItem);
      if (resp.status === "error" || !resp.data?.job_id) {
        throw new Error(resp.error?.message || (typeof resp.error === "string" ? resp.error : "Unknown error"));
      }
      // It stays in progress until the job finishes in the Download Queue
      const jobId = resp.data.job_id;
      queueJobsRef.current[jobId] = { ...payload, jobId, taskId };
      setWaitingJobs((n) => n + 1);
      updateTask(taskId, (t) => ({ ...t, payload: { ...payload, jobId } }));
      return true;
    } catch (error) {
      updateTask(taskId, (t) => ({
        ...t,
        status: "failed",
        error: `"${payload.itemTitle}" - Add to queue failed - ${error instanceof Error ? error.message : "Unknown error"}`,
      }));
      return false;
    }
  };

  const retryTask = async (taskId: string) => {
    const t = findTask(progress, taskId);
    if (!t) return;

    // NOTE: "note" tasks are not retryable
    if (t.payload.kind === "note") return;

    if (t.payload.kind === "addToDB") {
      await runAddToDBTask(taskId, t.payload);
    } else if (t.payload.kind === "addToQueue") {
      setFollowingQueue(true);
      setQueuingDone(true);
      await runAddToQueueTask(taskId, t.payload);
    }

    setButtonTexts((prev) => ({ ...prev, download: "Download Again" }));
  };

  // finishQueueJob records how the window's job ended in the Download Queue
  const finishQueueJob = (jobId: number, status: "completed" | "failed", error?: string) => {
    const job = queueJobsRef.current[jobId];
    if (!job) return;
    delete queueJobsRef.current[jobId];
    updateTask(job.taskId, (t) => ({ ...t, status, error }));
    // The set is saved when its job finishes, unless the job was stopped
    if (job.formItem && error !== STOPPED_ERROR) {
      markItemSaved(job.dbItem.media_item, job.formItem, job.dbItem.poster_sets[0].selected_types);
    }
    setWaitingJobs((n) => Math.max(0, n - 1));
  };

  // Follows the window's jobs in the Download Queue while it's open
  useEffect(() => {
    if (!followingQueue) return;
    return subscribeToDownloadQueue({
      onSnapshot: (jobs) => {
        // A job that isn't in the queue anymore already finished
        const inQueue = new Set((jobs ?? []).map((job) => job.id));
        for (const jobId of Object.keys(queueJobsRef.current).map(Number)) {
          if (!inQueue.has(jobId)) finishQueueJob(jobId, "completed");
        }
      },
      onJobStarted: (job) => {
        if (queueJobsRef.current[job.id]) setCurrentText(`Downloading ${job.media_item_title}`, "Downloading");
      },
      onJobProgress: (jobProgressUpdate) => {
        if (!queueJobsRef.current[jobProgressUpdate.job_id]) return;
        setJobProgress((prev) => ({ ...prev, [jobProgressUpdate.job_id]: jobProgressUpdate }));
        const image = jobProgressUpdate.active[0];
        if (image) {
          setCurrentText(
            formatDownloadProgressText(
              image,
              jobProgressUpdate.media_item_title,
              Math.min(jobProgressUpdate.done + 1, jobProgressUpdate.total),
              jobProgressUpdate.total
            ),
            formatDownloadProgressShortText(image)
          );
        }
      },
      onJobFinished: (job: DownloadQueueJob) => {
        const errors = job.result_errors?.length ? job.result_errors.join("\n") : undefined;
        finishQueueJob(job.id, job.status === "error" ? "failed" : "completed", errors);
      },
      onJobRemoved: (job) => finishQueueJob(job.id, "failed", STOPPED_ERROR),
      onQueueCleared: () => {
        for (const jobId of Object.keys(queueJobsRef.current).map(Number))
          finishQueueJob(jobId, "failed", STOPPED_ERROR);
      },
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [followingQueue]);

  // Done once every job the window added has finished
  useEffect(() => {
    if (!queuingDone || waitingJobs > 0) return;
    setQueuingDone(false);
    setFollowingQueue(false);
    setCurrentText("Completed!");
    setButtonTexts({ cancel: "Close", download: "Download Again" });
    setIsMounted(false);
  }, [queuingDone, waitingJobs]);

  const renderFormItemAssetType = (
    field: ControllerRenderProps<
      { selectedOptionsByItem: Record<string, AssetTypeFormValues> },
      `selectedOptionsByItem.${string}`
    >,
    assetType: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS,
    item: FormItemDisplay
  ) => {
    const types = field.value?.types || [];
    const isDuplicate = duplicates[getItemFormKey(item.MediaItem)];

    // Calculate checked state
    const isChecked = types.includes(assetType) && (!isDuplicate || isDuplicate.selectedType === item.Set.type);

    // Calculate disabled state
    const isDisabled = Boolean(isDuplicate && isDuplicate.selectedType && isDuplicate.selectedType !== item.Set.type);

    // Check download status from task state
    const status = getAssetStatus(getItemFormKey(item.MediaItem), assetType);
    const isDownloaded = status === "completed";
    const isFailed = status === "failed";
    const isLoading = status === "in-progress";
    const isSkipped = status === "skipped";

    // Check if *this assetType* is already downloaded in another set (using DBSavedSets)
    const downloadedSetForType = item.MediaItem?.db_saved_sets?.find(
      (set) => set.id !== item.Set.id && set.selected_types?.[assetType]
    );
    const isDownloadedInAnotherSet = Boolean(downloadedSetForType);

    const itemTmdbId = item.MediaItem.tmdb_id;
    const itemImages = item.Set.images.filter((img) => {
      if (typeof img.item_tmdb_id !== "string" || !img.item_tmdb_id.trim()) return true;
      return typeof itemTmdbId === "string" && img.item_tmdb_id === itemTmdbId;
    });

    const numberOfAssetType = (() => {
      switch (assetType) {
        case "poster":
          return itemImages.filter((img) => img.type === "poster").length;
        case "backdrop":
          return itemImages.filter((img) => img.type === "backdrop").length;
        case "season_poster":
          return itemImages.filter((img) => img.type === "season_poster" && img.season_number !== 0).length;
        case "special_season_poster":
          return itemImages.filter((img) => img.type === "season_poster" && img.season_number === 0).length;
        case "titlecard":
          return itemImages.filter((img) => img.type === "titlecard").length;
      }
    })();

    return (
      <FormItem key={`${field.name}-${assetType}`} className="flex flex-row items-start space-x-2">
        <FormControl className="mt-1">
          <Checkbox
            checked={isChecked}
            disabled={isDisabled}
            onCheckedChange={(checked) => {
              const newTypes = checked ? [...types, assetType] : types.filter((type: string) => type !== assetType);

              // Update duplicates tracking
              if (checked && isDuplicate) {
                setDuplicates((prev) => ({
                  ...prev,
                  [getItemFormKey(item.MediaItem)]: {
                    ...prev[getItemFormKey(item.MediaItem)],
                    selectedType: item.Set.type as "movie" | "collection",
                  },
                }));
              } else if (!checked && isDuplicate) {
                setDuplicates((prev) => ({
                  ...prev,
                  [getItemFormKey(item.MediaItem)]: {
                    ...prev[getItemFormKey(item.MediaItem)],
                    selectedType: "",
                  },
                }));
              }

              field.onChange({
                ...(field.value ?? {}),
                types: newTypes,
                autodownload: newTypes.length === 0 ? false : field.value?.autodownload,
                addToDBOnly: newTypes.length === 0 ? false : field.value?.addToDBOnly,
                source: item.Set.type === "movie" || item.Set.type === "collection" ? item.Set.type : undefined,
              });
            }}
            className="h-5 w-5 sm:h-4 sm:w-4 cursor-pointer"
          />
        </FormControl>
        <FormLabel className={cn("text-md font-normal cursor-pointer", isLoading ? "animate-pulse text-primary" : "")}>
          {DOWNLOAD_IMAGE_TYPE_OPTIONS.find((opt) => opt.value === assetType)?.label}
          {numberOfAssetType > 1 && `s`}
          {numberOfAssetType > 1 && <span className="text-xs text-muted-foreground">({numberOfAssetType})</span>}
        </FormLabel>
        {isDownloaded ? (
          <Check className="h-4 w-4 text-green-500 mt-1" strokeWidth={3} />
        ) : isFailed ? (
          <X className="h-4 w-4 text-destructive mt-1" strokeWidth={3} />
        ) : isLoading ? (
          <Loader className="h-4 w-4 mt-1 animate-spin" />
        ) : isSkipped ? (
          <Check className="h-4 w-4 text-yellow-500 mt-1" strokeWidth={3} />
        ) : isDownloadedInAnotherSet ? (
          <PopoverHelp
            ariaLabel="Type already downloaded in another set"
            side="bottom"
            className="max-w-xs"
            trigger={<TriangleAlert className="h-4 w-4 mt-1 text-yellow-500 cursor-help" />}
          >
            <div className="flex items-center">
              <CircleAlert className="h-5 w-5 text-yellow-500 mr-2" />
              <span className="text-xs">
                {DOWNLOAD_IMAGE_TYPE_OPTIONS.find((opt) => opt.value === assetType)?.label}{" "}
                {DOWNLOAD_IMAGE_TYPE_OPTIONS.find((opt) => opt.value === assetType)?.label?.endsWith("s")
                  ? "have"
                  : "has"}{" "}
                already been downloaded in set {downloadedSetForType?.id} by user {downloadedSetForType?.user_created}.
              </span>
            </div>
          </PopoverHelp>
        ) : null}
      </FormItem>
    );
  };

  // The collection's own poster and backdrop, applied to the media server collection and not saved
  const renderCollectionImages = (collection: CollectionItem) => {
    const tasks = progress.items[COLLECTION_PROGRESS_KEY]?.tasks ?? [];
    const statusOf = (type: TYPE_DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS) =>
      tasks.find(
        (t) =>
          t.payload.kind === "addToQueue" && t.payload.dbItem.collection_images?.images.some((img) => img.type === type)
      )?.status;

    return (
      <FormField
        key={COLLECTION_PROGRESS_KEY}
        control={form.control}
        name="collectionTypes"
        render={({ field }) => (
          <div
            className={cn("rounded-md border p-4 rounded-lg mb-4", {
              "border-green-500": tasks.length > 0 && tasks.every((t) => t.status === "completed"),
              "border-destructive": tasks.some((t) => t.status === "failed"),
            })}
          >
            <FormLabel className="text-md font-normal mb-4">Collection: {collection.title}</FormLabel>
            <div className="flex flex-wrap gap-4">
              {collectionImageTypes.map((type) => {
                const status = statusOf(type);
                return (
                  <FormItem key={type} className="flex flex-row items-start space-x-2">
                    <FormControl className="mt-1">
                      <Checkbox
                        checked={field.value?.includes(type) ?? false}
                        onCheckedChange={(checked) =>
                          field.onChange(
                            checked ? [...(field.value ?? []), type] : (field.value ?? []).filter((t) => t !== type)
                          )
                        }
                        className="h-5 w-5 sm:h-4 sm:w-4 cursor-pointer"
                      />
                    </FormControl>
                    <FormLabel
                      className={cn(
                        "text-md font-normal cursor-pointer",
                        status === "in-progress" ? "animate-pulse text-primary" : ""
                      )}
                    >
                      {DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS.find((opt) => opt.value === type)?.label}
                    </FormLabel>
                    {status === "completed" ? (
                      <Check className="h-4 w-4 text-green-500 mt-1" strokeWidth={3} />
                    ) : status === "failed" ? (
                      <X className="h-4 w-4 text-destructive mt-1" strokeWidth={3} />
                    ) : status === "in-progress" ? (
                      <Loader className="h-4 w-4 mt-1 animate-spin" />
                    ) : null}
                  </FormItem>
                );
              })}
            </div>
            <p className="text-xs text-muted-foreground mt-2">
              Applied to the collection on your media server when the Download Queue gets to them. Collection images
              aren't saved, so Auto Download doesn't update them.
            </p>
          </div>
        )}
      />
    );
  };

  const renderFormItem = (item: FormItemDisplay) => {
    const isDuplicate = duplicates[getItemFormKey(item.MediaItem)];
    const existingAssetTypes = computeAssetTypes(item);
    const possibleFutureAssetTypes = getPossibleFutureAssetTypes(item);
    // Calculate disabled state
    const isDisabled = Boolean(isDuplicate && isDuplicate.selectedType && isDuplicate.selectedType !== item.Set.type);

    // Calculate whether the item is already in the database
    const isInDatabase =
      (item.MediaItem && item.MediaItem.db_saved_sets && item.MediaItem.db_saved_sets.length > 0) || false;

    // Calculate whether the item is already in the database and has this set saved
    const isInDatabaseWithSet =
      isInDatabase &&
      item.MediaItem.db_saved_sets &&
      item.MediaItem.db_saved_sets.some((set) => set.id === item.Set.id);

    // Calculate whether the item has any error tasks
    const itemProgress = progress.items[getItemFormKey(item.MediaItem)];
    const hasErrorTasks = itemProgress ? itemProgress.tasks.some((t) => t.status === "failed") : false;
    const allSuccessful = itemProgress
      ? itemProgress.tasks.length > 0 &&
        itemProgress.tasks.every((t) => t.status === "completed" || t.status === "skipped")
      : false;

    return (
      <FormField
        key={`${getItemFormKey(item.MediaItem)}-${item.Set.type}`}
        control={form.control}
        name={`selectedOptionsByItem.${getItemFormKey(item.MediaItem)}`}
        render={({ field }) => (
          <div
            className={cn("rounded-md border p-4 rounded-lg mb-4", {
              "border-green-500": isInDatabaseWithSet || allSuccessful,
              "border-yellow-500": isInDatabase && !isInDatabaseWithSet,
              "border-destructive": hasErrorTasks,
            })}
          >
            <FormLabel
              className="text-md font-normal mb-4"
              onDoubleClick={() => {
                setMediaItem(item.MediaItem);
              }}
            >
              <span className="flex items-center justify-between w-full">
                {item.MediaItem.title} {/\(\d{4}\)$/.test(item.MediaItem.title) ? "" : `(${item.MediaItem.year})`}
                {item.MediaItem && item.MediaItem.db_saved_sets && item.MediaItem.db_saved_sets.length > 0 && (
                  <Popover modal={true}>
                    <PopoverTrigger>
                      <Database
                        className={cn(
                          "h-4 w-4 ml-2 cursor-pointer",
                          isInDatabaseWithSet ? "text-green-500" : "text-yellow-500"
                        )}
                      />
                    </PopoverTrigger>
                    <PopoverContent
                      className={cn(
                        "max-w-[400px] rounded-lg shadow-lg border-2 p-2 flex flex-col items-center justify-center",
                        isInDatabaseWithSet ? "border-green-800" : "border-yellow-800"
                      )}
                    >
                      <div className="flex items-center mb-2">
                        <CircleAlert className="h-5 w-5 text-yellow-500 mr-2" />
                        <span className="text-sm text-muted-foreground">
                          This media item already exists in your database
                        </span>
                      </div>
                      <div className="text-xs text-muted-foreground mb-2">
                        You have previously saved it in the following sets
                      </div>
                      <ul className="space-y-2">
                        {item.MediaItem.db_saved_sets.map((set) => (
                          <li key={set.id} className="flex items-center rounded-md px-2 py-1 shadow-sm">
                            <Button
                              variant="outline"
                              className={cn(
                                "flex items-center transition-colors rounded-md px-2 py-1 cursor-pointer text-sm",
                                set.id.toString() === item.Set.id.toString()
                                  ? "text-green-600  hover:bg-green-100  hover:text-green-600"
                                  : "text-yellow-600 hover:bg-yellow-100 hover:text-yellow-700"
                              )}
                              aria-label={`View saved set ${set.id} ${set.user_created ? `by ${set.user_created}` : ""}`}
                              onClick={(e) => {
                                e.stopPropagation();
                                setSearchQuery(
                                  `${item.MediaItem.title} Y:${item.MediaItem.year}: ID:${item.MediaItem.tmdb_id}: L:${item.MediaItem.library_title}:`
                                );
                                router.push("/saved-sets");
                              }}
                            >
                              Set ID: {set.id}
                              {set.user_created ? ` by ${set.user_created}` : ""}
                            </Button>
                          </li>
                        ))}
                      </ul>
                    </PopoverContent>
                  </Popover>
                )}
                {baseSetInfo.type === "boxset" &&
                  isDuplicate &&
                  isDuplicate.selectedType !== "" &&
                  isDuplicate.selectedType !== item.Set.type && (
                    <Popover modal={true}>
                      <PopoverTrigger>
                        <TriangleAlert className="h-4 w-4 text-yellow-500 cursor-help" />
                      </PopoverTrigger>
                      <PopoverContent className="max-w-[400px] w-60">
                        <div className="text-sm text-yellow-500">
                          This item is selected in the{" "}
                          {isDuplicate.selectedType === "movie" ? "Movie Set" : "Collection Set"}.
                        </div>
                        {(() => {
                          const img = isDuplicate.options.find((o) => o.type === isDuplicate.selectedType)?.image;
                          return (
                            img && <AssetImage className="mt-2" image={img} imageType="mediux" matchedToItem={false} />
                          );
                        })()}
                      </PopoverContent>
                    </Popover>
                  )}
              </span>
            </FormLabel>

            {showLibraryNames && (
              <p className="text-xs text-muted-foreground -mt-3 mb-3">{item.MediaItem.library_title}</p>
            )}

            <div className="space-y-2">
              {existingAssetTypes.map((assetType) => (
                <div key={assetType}>
                  {renderFormItemAssetType(
                    field as ControllerRenderProps<
                      {
                        selectedOptionsByItem: Record<string, AssetTypeFormValues>;
                      },
                      `selectedOptionsByItem.${string}`
                    >,
                    assetType,
                    item
                  )}
                </div>
              ))}

              {possibleFutureAssetTypes.length > 0 && (
                <>
                  <div className="flex items-center space-x-2">
                    <FormLabel className={`text-md font-normal` + (isDisabled ? " text-gray-500" : "")}>
                      Possible Future Types
                    </FormLabel>
                    <DownloadModalPopover type="possible-future-types" />
                  </div>
                  {possibleFutureAssetTypes.map((assetType) => (
                    <div key={`future-${assetType}`}>
                      {renderFormItemAssetType(
                        field as ControllerRenderProps<
                          {
                            selectedOptionsByItem: Record<string, AssetTypeFormValues>;
                          },
                          `selectedOptionsByItem.${string}`
                        >,
                        assetType,
                        item
                      )}
                    </div>
                  ))}
                </>
              )}

              <FormLabel className={`text-md font-normal` + (isDisabled ? " text-gray-500" : "")}>
                Download Options
              </FormLabel>
              <FormItem className="flex items-center space-x-2">
                <FormControl>
                  <Checkbox
                    checked={isDisabled || field.value?.types?.length === 0 ? false : field.value?.addToDBOnly || false}
                    disabled={isDisabled || field.value?.types?.length === 0}
                    onCheckedChange={(checked) => {
                      field.onChange({
                        ...(field.value ?? {}),
                        addToDBOnly: checked,
                      });
                    }}
                    className="h-5 w-5 sm:h-4 sm:w-4 cursor-pointer"
                  />
                </FormControl>
                <FormLabel className="text-md font-normal cursor-pointer">Add to Database Only</FormLabel>

                <DownloadModalPopover type="add-to-db-only" />
              </FormItem>
              <FormItem className="flex items-center space-x-2">
                <FormControl>
                  <Checkbox
                    checked={
                      isDisabled || field.value?.types?.length === 0 ? false : field.value?.autodownload || false
                    }
                    disabled={isDisabled || field.value?.types?.length === 0}
                    onCheckedChange={(checked) => {
                      field.onChange({
                        ...(field.value ?? {}),
                        autodownload: checked,
                      });
                    }}
                    className="h-5 w-5 sm:h-4 sm:w-4 cursor-pointer"
                  />
                </FormControl>
                <FormLabel className="text-md font-normal cursor-pointer">Auto Download</FormLabel>
                <DownloadModalPopover type="autodownload" />
              </FormItem>
              {/* Only does something while Automatically Upgrade Sets is on */}
              {autoUpgradeSets && (
                <FormItem className="flex items-center space-x-2">
                  <FormControl>
                    <Checkbox
                      checked={
                        isDisabled || field.value?.types?.length === 0 ? false : field.value?.autoupgrade !== false
                      }
                      disabled={isDisabled || field.value?.types?.length === 0}
                      onCheckedChange={(checked) => {
                        field.onChange({
                          ...(field.value ?? {}),
                          autoupgrade: checked === true,
                        });
                      }}
                      className="h-5 w-5 sm:h-4 sm:w-4 cursor-pointer"
                    />
                  </FormControl>
                  <FormLabel className="text-md font-normal cursor-pointer">Auto Upgrade</FormLabel>
                  <DownloadModalPopover type="autoupgrade" />
                </FormItem>
              )}
            </div>
          </div>
        )}
      />
    );
  };

  const createDBItem = (
    item: FormItemDisplay,
    options: z.infer<typeof formSchema>["selectedOptionsByItem"][string],
    latestMediaItem: MediaItem
  ): {
    dbItem: DBSavedItem;
  } => {
    return {
      dbItem: {
        media_item: latestMediaItem,
        poster_sets: [
          {
            id: item.Set.id,
            type: item.Set.type,
            title: item.Set.title,
            user_created: item.Set.user_created,
            date_created: item.Set.date_created,
            date_updated: item.Set.date_updated,
            popularity: item.Set.popularity,
            popularity_global: item.Set.popularity_global,
            images: item.Set.images,
            selected_types: {
              poster: options.types?.includes("poster") || false,
              backdrop: options.types?.includes("backdrop") || false,
              season_poster: options.types?.includes("season_poster") || false,
              special_season_poster: options.types?.includes("special_season_poster") || false,
              titlecard: options.types?.includes("titlecard") || false,
            },
            auto_download: options.autodownload || false,
            auto_add_new_collection_items: autoAddNewCollectionItems,
            no_auto_upgrade: options.autoupgrade === false,
            last_downloaded: "",
            to_delete: false,
          },
        ],
      },
    };
  };

  // Compute how many tasks we *expect* to run before starting
  const computePlannedTotal = (data: z.infer<typeof formSchema>) => {
    let total = 0;

    for (const item of formItems) {
      const selected = data.selectedOptionsByItem[getItemFormKey(item.MediaItem)];
      if (!selected) continue;

      // Skip duplicates that aren't selected
      if (
        (item.Set.type === "movie" || item.Set.type === "collection") &&
        duplicates[getItemFormKey(item.MediaItem)]?.selectedType &&
        duplicates[getItemFormKey(item.MediaItem)].selectedType !== item.Set.type
      ) {
        continue;
      }

      // Nothing selected and not DB-only => no tasks
      if ((selected.types?.length ?? 0) === 0 && !selected.addToDBOnly) continue;

      // DB-only => just one task
      if (selected.addToDBOnly) {
        total += 1;
        continue;
      }

      // The images the item's job downloads
      total += queuedImageCount(item.Set.images, selected.types ?? [], item.MediaItem);
    }

    // The collection's own images
    if (collectionImages) {
      total += data.collectionTypes?.length ?? 0;
    }

    return total;
  };

  const onSubmit = async (data: z.infer<typeof formSchema>) => {
    if (isMounted) return;
    cancelRef.current = false;

    const startTime = Date.now();

    try {
      setIsMounted(true);
      setButtonTexts({
        cancel: "Cancel",
        download: "Starting...",
      });
      resetProgress();
      // Reset + set the planned total up front (fixed total)
      const plannedTotal = computePlannedTotal(data);
      setProgress({
        currentText: "Starting...",
        totalPlanned: plannedTotal,
        items: {},
      });

      log("INFO", "Download Modal", "Debug Info", "Form submitted with data:", data);
      log("INFO", "Download Modal", "Debug Info", "Progress:", progress);
      log("INFO", "Download Modal", "Debug Info", "Selected Types:", { watchSelectedOptions });
      log("INFO", "Download Modal", "Debug Info", "Auto Add New Collection Items:", autoAddNewCollectionItems);

      // Follow the jobs in the Download Queue from the start, so none of their updates are missed
      queueJobsRef.current = {};
      setJobProgress({});
      setWaitingJobs(0);
      setFollowingQueue(true);

      // The collection images get their own job
      const selectedCollectionImages = collectionImages
        ? (data.collectionTypes ?? [])
            .map((type) => collectionImages.images.find((img) => img.type === type))
            .filter((img): img is CollectionItemImageFile => img !== undefined)
        : [];
      const queuedCollectionImages =
        collectionImages && selectedCollectionImages.length > 0
          ? { collection: collectionImages.collection, images: selectedCollectionImages, set: setSummary }
          : undefined;

      // Sort formItems by MediaItemTitle for consistent order
      const sortedFormItems = formItems;
      log("INFO", "Download Modal", "Debug Info", "Sorted Form Items:", sortedFormItems);

      // Go through each item in the formItems
      for (let idx = 0; idx < sortedFormItems.length; idx++) {
        if (cancelRef.current) return; // Exit if cancelled
        const item = sortedFormItems[idx];
        log("INFO", "Download Modal", "Debug Info", "Processing Item:", item);

        const selectedOptions = data.selectedOptionsByItem[getItemFormKey(item.MediaItem)];
        log("INFO", "Download Modal", "Debug Info", "Selected Types for Item:", selectedOptions);

        // If no types are selected and not set to "Add to DB Only", skip this item
        if (selectedOptions.types?.length === 0 && !selectedOptions.addToDBOnly) {
          log("INFO", "Download Modal", "Debug Info", `Skipping ${item.MediaItem.title} - Nothing to do here.`);
          continue;
        }

        upsertItem(getItemFormKey(item.MediaItem), item.MediaItem.title);

        // If the item set is a movie or collection, check for duplicates
        // If this duplicate type is not selected, skip it
        if (
          (item.Set.type === "movie" || item.Set.type === "collection") &&
          duplicates[getItemFormKey(item.MediaItem)] &&
          duplicates[getItemFormKey(item.MediaItem)].selectedType &&
          duplicates[getItemFormKey(item.MediaItem)].selectedType !== item.Set.type
        ) {
          log(
            "INFO",
            "Download Modal",
            "Debug Info",
            `Skipping ${item.MediaItem.title} in ${item.Set.id} - Duplicate type selected: ${duplicates[getItemFormKey(item.MediaItem)].selectedType}`
          );
          continue;
        }

        // Get the latest media item from the server
        const latestMediaItemResp = await GetMediaItemDetails(
          item.MediaItem.title,
          item.MediaItem.rating_key,
          item.MediaItem.library_title,
          "item"
        );
        if (latestMediaItemResp.status === "error" || !latestMediaItemResp.data) {
          const noteId = newId();
          const msg =
            latestMediaItemResp.status === "error"
              ? latestMediaItemResp.error?.help ||
                latestMediaItemResp.error?.detail ||
                latestMediaItemResp.error?.message ||
                "Unknown error"
              : "No media item found.";

          addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
            id: noteId,
            status: "failed",
            label: "Fetch latest media item",
            attempts: 1,
            error: `Error fetching latest media item for ${item.MediaItem.title}: ${msg}`,
            payload: { kind: "note", itemKey: getItemFormKey(item.MediaItem), itemTitle: item.MediaItem.title },
          });
          continue;
        }

        const latestMediaItem = latestMediaItemResp.data.media_item;
        const createdSavedItem = createDBItem(item, selectedOptions, latestMediaItem);

        // Add to DB only
        if (selectedOptions.addToDBOnly) {
          const taskId = newId();
          addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
            id: taskId,
            status: "pending",
            label: `Add "${item.MediaItem.title}" to DB`,
            attempts: 0,
            payload: {
              kind: "addToDB",
              itemKey: getItemFormKey(item.MediaItem),
              itemTitle: item.MediaItem.title,
              mediaItem: createdSavedItem.dbItem.media_item,
              posterSet: createdSavedItem.dbItem.poster_sets[0],
              addToDBOnly: selectedOptions.addToDBOnly,
            },
          });

          const ok = await runAddToDBTask(taskId, {
            kind: "addToDB",
            itemKey: getItemFormKey(item.MediaItem),
            itemTitle: item.MediaItem.title,
            mediaItem: createdSavedItem.dbItem.media_item,
            posterSet: createdSavedItem.dbItem.poster_sets[0],
            addToDBOnly: selectedOptions.addToDBOnly,
          });

          if (ok) {
            markItemSaved(latestMediaItem, item, {
              poster: selectedOptions.types?.includes("poster"),
              backdrop: selectedOptions.types?.includes("backdrop"),
              season_poster: selectedOptions.types?.includes("season_poster"),
              special_season_poster: selectedOptions.types?.includes("special_season_poster"),
              titlecard: selectedOptions.types?.includes("titlecard"),
            });
          }
          continue;
        }

        const selectedTypes = selectedOptions.types.sort((a, b) => {
          const order = ["poster", "backdrop", "season_poster", "special_season_poster", "titlecard"];
          return order.indexOf(a) - order.indexOf(b);
        });

        // If no types are selected, skip this item
        if (selectedTypes.length === 0) {
          log("INFO", "Download Modal", "Debug Info", `Skipping ${item.MediaItem.title} - No types selected.`);
          continue;
        }

        log("INFO", "Download Modal", "Debug Info", `Selected Types for ${item.MediaItem.title}:`, selectedTypes);

        // The Download Queue downloads the item's images, before the jobs that run in the background
        const taskId = newId();
        const queuePayload: AddToQueueTaskPayload = {
          kind: "addToQueue",
          itemKey: getItemFormKey(item.MediaItem),
          itemTitle: item.MediaItem.title,
          dbItem: { ...createdSavedItem.dbItem, source: "download_window" },
          imageCount: queuedImageCount(item.Set.images, selectedTypes, latestMediaItem),
          formItem: item,
        };
        addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
          id: taskId,
          status: "pending",
          label: `Download "${item.MediaItem.title}" in the Download Queue`,
          attempts: 0,
          payload: queuePayload,
        });
        await runAddToQueueTask(taskId, queuePayload);
      }

      // The collection images get their own job, listed by the collection
      if (queuedCollectionImages && formItems.length > 0 && !cancelRef.current) {
        const item = formItems[0];
        const dbItem: DBSavedItem = {
          media_item: item.MediaItem,
          poster_sets: [],
          collection_images: queuedCollectionImages,
          source: "download_window",
        };
        const taskId = newId();
        const payload: AddToQueueTaskPayload = {
          kind: "addToQueue",
          itemKey: COLLECTION_PROGRESS_KEY,
          itemTitle: queuedCollectionImages.collection.title,
          dbItem,
          imageCount: queuedCollectionImages.images.length,
        };
        addTask(COLLECTION_PROGRESS_KEY, queuedCollectionImages.collection.title, {
          id: taskId,
          status: "pending",
          label: `Download the "${queuedCollectionImages.collection.title}" collection images in the Download Queue`,
          attempts: 0,
          payload,
        });
        await runAddToQueueTask(taskId, payload);
      }

      // The total is the queued images now, and the window is done when the queue finishes its jobs
      setProgress((prev) => ({ ...prev, totalPlanned: 0 }));
      setCurrentText("Waiting for the Download Queue", "Waiting for the queue");
      setButtonTexts({ cancel: "Close", download: "Downloading..." });
      setQueuingDone(true);

      const timeTaken = (Date.now() - startTime) / 1000;
      log("INFO", "Download Modal", "Debug Info", `Added to the Download Queue in ${timeTaken.toFixed(2)} seconds.`);
    } catch (error) {
      const taskId = newId();
      addTask("general", "General", {
        id: taskId,
        status: "failed",
        label: "Unexpected error",
        attempts: 1,
        error: error instanceof Error ? error.message : "An unknown error occurred",
        payload: { kind: "note", itemKey: "general", itemTitle: "General" },
      });

      setButtonTexts({
        cancel: "Close",
        download: "Retry Download",
      });
      setIsMounted(false);
    }
  };

  return (
    <Dialog
      onOpenChange={(open) => {
        if (open) {
          setAutoAddNewCollectionItems(defaultAutoAddNewCollectionItems);
        } else {
          handleClose();
        }
      }}
    >
      <DialogTrigger asChild>
        <Download className="mr-2 h-5 w-5 sm:h-7 sm:w-7 cursor-pointer active:scale-95 hover:text-primary" />
      </DialogTrigger>

      <DialogPortal>
        <DialogOverlay />
        <DialogContent
          className={cn("z-50", "max-h-[85vh] overflow-y-auto", "sm:max-w-[700px]", "border border-primary")}
        >
          <DialogHeader>
            <DialogTitle onClick={LOG_VALUES}>{baseSetInfo.title}</DialogTitle>
            <div className="flex items-center justify-center sm:justify-start">
              <Avatar className="rounded-lg mr-1 w-4 h-4">
                <AvatarImage
                  src={`/api/images/mediux/avatar?username=${baseSetInfo.user_created}`}
                  className="w-4 h-4"
                />
                <AvatarFallback className="">
                  <User className="w-4 h-4" />
                </AvatarFallback>
              </Avatar>
              <Link href={`/user/${baseSetInfo.user_created}`} className="hover:underline">
                {baseSetInfo.user_created}
              </Link>
            </div>
            <DialogDescription>
              <Link
                href={getMediuxBaseUrl()}
                className="hover:text-primary transition-colors text-sm text-muted-foreground"
                target="_blank"
                rel="noopener noreferrer"
              >
                {baseSetInfo.type === "boxset" ? "Boxset" : "Set"} ID: {baseSetInfo.id}
              </Link>
            </DialogDescription>
          </DialogHeader>

          {formItems.length > 0 ? (
            <Form {...form}>
              <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-2">
                {/* The collection's own poster and backdrop */}
                {collectionImages &&
                  collectionImageTypes.length > 0 &&
                  renderCollectionImages(collectionImages.collection)}

                {/* Form Items */}
                {formItems.map(renderFormItem)}

                {/* If the all items have no types selected, show a message */}
                {formItems.every(
                  (item) =>
                    !watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.types?.length &&
                    !watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.addToDBOnly
                ) &&
                  !watchCollectionTypes?.length && (
                    <div className="text-sm text-destructive">
                      No image types selected for download. Please select at least one image type to download.
                    </div>
                  )}

                {/* Auto Add New Collection Items 
									Only show this button if the set is a collection and all of the items are movies that have Autodownload enabled and not set to Add to DB Only
								*/}
                {baseSetInfo.type === "collection" &&
                  formItems.every(
                    (item) =>
                      item.MediaItem.type === "movie" &&
                      watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.autodownload &&
                      !watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.addToDBOnly
                  ) && (
                    <FormItem className="flex items-center space-x-2 mb-4">
                      <FormControl>
                        <Checkbox
                          checked={autoAddNewCollectionItems}
                          onCheckedChange={(checked) => setAutoAddNewCollectionItems(checked ? true : false)}
                        />
                      </FormControl>
                      <FormLabel className="text-md font-normal cursor-pointer">
                        Auto Add New Collection Items
                      </FormLabel>
                      <DownloadModalPopover type="auto-add-new-collection-items" />
                    </FormItem>
                  )}

                <div>
                  {/* Number of Images and Total Download Size (only if there are images to download) */}
                  {selectedSizes.fileCount > 0 &&
                    formItems.some((item) => !watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.addToDBOnly) && (
                      <>
                        <div className="text-sm text-muted-foreground">Number of Images: {selectedSizes.fileCount}</div>
                        <div className="text-sm text-muted-foreground">
                          Total Download Size: ~{formatDownloadSize(selectedSizes.downloadSize)}
                        </div>
                      </>
                    )}

                  {/* Silo uses one item for every library it is in, so the images apply to all of them */}
                  {sharedLibraryTitles.length > 1 && (
                    <div className="text-sm text-muted-foreground mt-1">
                      * This will apply to all versions of this item in all libraries ({sharedLibraryTitles.join(", ")})
                    </div>
                  )}

                  {/* Always show the database-only message if any item is set to Add to DB Only or has types selected */}
                  {formItems.some((item) => watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.addToDBOnly) && (
                    <div className="text-sm text-muted-foreground mt-1">
                      * Will add{" "}
                      {(() => {
                        const titles = formItems
                          .filter((item) => watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.addToDBOnly)
                          .map((item) =>
                            showLibraryNames
                              ? `${item.MediaItem.title} (${item.MediaItem.library_title})`
                              : item.MediaItem.title
                          );

                        if (titles.length === 0) return "";
                        if (titles.length === 1)
                          return <span className="font-medium text-yellow-500">{`'${titles[0]}' `}</span>;
                        if (titles.length === 2)
                          return (
                            <>
                              <span className="font-medium text-yellow-500">{`'${titles[0]}'`}</span>
                              {" and "}
                              <span className="font-medium text-yellow-500">{`'${titles[1]}' `}</span>
                            </>
                          );

                        return (
                          <>
                            {titles.slice(0, -1).map((title, idx) => (
                              <Fragment key={title}>
                                <span className="font-medium text-yellow-500">{`'${title}'`}</span>
                                {idx < titles.length - 2 ? ", " : ""}
                              </Fragment>
                            ))}
                            {" and "}
                            <span className="font-medium text-yellow-500">{`'${titles[titles.length - 1]}' `}</span>
                          </>
                        );
                      })()}
                      to database without downloading any images.
                    </div>
                  )}
                </div>

                {/* Progress Bar */}
                {Object.values(progress.items).some((i) => i.tasks.length > 0) &&
                  (() => {
                    const overall = getOverallProgress(progress, jobProgress);
                    const errors = getErrorsByItem(progress);
                    const hasErrors = errors.length > 0;

                    return (
                      <DownloadProgressBar
                        value={overall}
                        label={progress.currentText}
                        shortLabel={progress.currentShortText}
                        hasErrors={hasErrors}
                      />
                    );
                  })()}

                {/* Errors */}
                {getErrorsByItem(progress).length > 0 && (
                  <div className="my-2">
                    {(() => {
                      const grouped = getErrorsByItem(progress);
                      const errorCount = grouped.reduce((acc, g) => acc + g.errors.length, 0);

                      return (
                        <Accordion type="single" collapsible>
                          <AccordionItem value="errors">
                            <AccordionTrigger className={cn("text-destructive justify-start gap-2 [&>svg]:ml-0")}>
                              Errors ({errorCount})
                            </AccordionTrigger>
                            <AccordionContent>
                              <div className="flex flex-col space-y-2">
                                {grouped.map((g) => (
                                  <div key={g.itemKey} className="flex flex-col space-y-2">
                                    {g.errors.map((t) => (
                                      <div key={t.id} className="flex items-center text-destructive">
                                        {t.payload.kind !== "note" ? (
                                          <button
                                            type="button"
                                            className="mr-2 h-4 w-4 text-yellow-500 cursor-pointer"
                                            onClick={() => retryTask(t.id)}
                                            aria-label="Retry"
                                          >
                                            <RefreshCcw className="h-4 w-4" />
                                          </button>
                                        ) : (
                                          <span className="mr-1 h-4 w-4">
                                            <X className="h-4 w-4" />
                                          </span>
                                        )}
                                        <span>{t.error || t.label}</span>
                                      </div>
                                    ))}
                                  </div>
                                ))}
                              </div>
                            </AccordionContent>
                          </AccordionItem>
                        </Accordion>
                      );
                    })()}
                  </div>
                )}

                <DialogFooter>
                  <div className="flex space-x-4 justify-end w-full">
                    {/* Cancel button to close the modal */}
                    <DialogClose asChild>
                      <Button
                        className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
                        variant="ghost"
                        onClick={() => {
                          handleClose();
                        }}
                      >
                        {buttonTexts.cancel}
                      </Button>
                    </DialogClose>

                    {/* Download button to display download info */}
                    {
                      // Only show if at least one item has types selected or is set to "Add to DB Only"
                      formItems.some(
                        (item) =>
                          watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.types?.length > 0 ||
                          watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.addToDBOnly
                      ) && (
                        <Button
                          variant={"outline"}
                          className="cursor-pointer hover:text-primary hover:brightness-120 transition-colors"
                          disabled={
                            // Disable if no items are selected and nothing is set to "Add to DB Only"
                            (selectedSizes.fileCount === 0 &&
                              formItems.every(
                                (item) =>
                                  !watchSelectedOptions[getItemFormKey(item.MediaItem)].types.length &&
                                  !watchSelectedOptions[getItemFormKey(item.MediaItem)].addToDBOnly
                              )) ||
                            isMounted
                          }
                        >
                          {(() => {
                            const isBusy =
                              buttonTexts.download.startsWith("Starting") ||
                              buttonTexts.download.startsWith("Adding") ||
                              buttonTexts.download.startsWith("Downloading");

                            const { done, total } = getOverallCounts(progress, jobProgress);

                            if (isBusy) {
                              return (
                                <>
                                  <Loader className="h-4 w-4 animate-spin" />
                                  {progress.totalPlanned > 0 && done !== total && (
                                    <span className="ml-2 text-muted-foreground tabular-nums">
                                      {done}/{total}
                                    </span>
                                  )}
                                </>
                              );
                            }

                            return (
                              <>
                                {buttonTexts.download === "Download" ||
                                buttonTexts.download === "Download Again" ||
                                buttonTexts.download === "Retry Download" ? (
                                  <Download className="h-4 w-4" />
                                ) : buttonTexts.download === "Add to Database" ? (
                                  <DatabaseZap className="h-4 w-4" />
                                ) : null}
                                {buttonTexts.download}
                              </>
                            );
                          })()}
                        </Button>
                      )
                    }

                    {/* Reset Form button: If the all items have no types selected and nothing is set to "Add to DB Only" */}
                    {formItems.every(
                      (item) =>
                        !watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.types?.length &&
                        !watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.addToDBOnly
                    ) && (
                      <Button
                        className="cursor-pointer hover:text-primary hover:brightness-120 transition-colors"
                        variant="secondary"
                        onClick={() => {
                          form.reset();
                          setSelectedSizes({
                            fileCount: 0,
                            downloadSize: 0,
                            poster: 0,
                            backdrop: 0,
                            season_poster: 0,
                            special_season_poster: 0,
                            titlecard: 0,
                          });
                          resetProgress();
                          setDuplicates({});
                        }}
                      >
                        <OctagonMinus className="h-4 w-4" />
                        Reset Form
                      </Button>
                    )}
                  </div>
                </DialogFooter>
              </form>
            </Form>
          ) : (
            <div className="p-4 text-center">
              <Lead className="text-md text-muted-foreground mb-2">No items found in this set.</Lead>
              <p className="text-sm text-muted-foreground">
                Please ensure the set has items with images available for download.
              </p>
            </div>
          )}
        </DialogContent>
      </DialogPortal>
    </Dialog>
  );
};

export default DownloadModal;
