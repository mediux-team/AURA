"use client";

import {
  compareSeasonImages,
  formatDownloadProgressShortText,
  formatDownloadProgressText,
} from "@/helper/format-download-progress";
import { formatDownloadSize } from "@/helper/format-download-size";
import { upsertSavedSets } from "@/helper/media-item-update-saved-sets";
import { AddNewItemToDB } from "@/services/database/add";
import { downloadImageFileForMediaItem } from "@/services/downloads/download-image";
import { AddItemToDownloadQueue } from "@/services/downloads/queue-add";
import { GetMediaItemDetails } from "@/services/mediaserver/get-media-item-details";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  Check,
  CircleAlert,
  Database,
  DatabaseZap,
  Download,
  ListEnd,
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
import type { MediaItem, SelectedTypes } from "@/types/media-and-posters/media-item-and-library";
import type { BaseSetInfo, ImageFile } from "@/types/media-and-posters/sets";
import { DOWNLOAD_IMAGE_TYPE_OPTIONS, TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS } from "@/types/ui-options";

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

type DownloadTaskPayload = {
  kind: "download";
  itemKey: string;
  itemTitle: string;
  imageFile: ImageFile;
  fileName: string;
  mediaItem: MediaItem;
};

type AddToDBTaskPayload = {
  kind: "addToDB";
  itemKey: string;
  itemTitle: string;
  mediaItem: MediaItem;
  posterSet: DBPosterSetDetail;
  addToDBOnly: boolean;
};

type AddToQueueTaskPayload = {
  kind: "addToQueue";
  itemKey: string;
  itemTitle: string;
  dbItem: DBSavedItem;
};

// Non-Retryable “record only” task (e.g. fetch latest media item failed)
type NoteTaskPayload = {
  kind: "note";
  itemKey: string;
  itemTitle: string;
};

type TaskPayload = DownloadTaskPayload | AddToDBTaskPayload | AddToQueueTaskPayload | NoteTaskPayload;

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

// derive progress counts from tasks (uses totalPlanned so total doesn't grow as tasks are added)
const getOverallCounts = (state: DownloadProgress) => {
  const allTasks = Object.values(state.items).flatMap((i) => i.tasks || []);

  // Notes shouldn't affect progress totals
  const relevant = allTasks.filter((t) => t.payload.kind !== "note");

  const skipped = relevant.filter((t) => t.status === "skipped").length;
  const done = relevant.filter((t) => t.status === "completed" || t.status === "failed").length;

  // Remove skipped tasks from the effective total so progress doesn't jump early.
  const planned = state.totalPlanned || relevant.length || 0;
  const total = Math.max(0, planned - skipped);

  // If every relevant task is skipped, treat the run as fully resolved.
  if (total === 0 && relevant.length > 0) {
    return { done: 1, total: 1 };
  }

  if (total === 0) {
    return { done: 0, total: 1 };
  }

  return { done, total };
};

// derive progress (0..100) from tasks
const getOverallProgress = (state: DownloadProgress) => {
  const { done, total } = getOverallCounts(state);
  return Math.round((done / total) * 100);
};

// derive “errors per item” for the accordion
const getErrorsByItem = (state: DownloadProgress) =>
  Object.values(state.items)
    .map((i) => ({
      itemKey: i.itemKey,
      title: i.title,
      errors: i.tasks.filter((t) => t.status === "failed"),
    }))
    .filter((x) => x.errors.length > 0);

const DOWNLOAD_BATCH_SIZE = 5;

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
  })
  .refine(
    (data) =>
      Object.values(data.selectedOptionsByItem).some((item) => Array.isArray(item.types) && item.types.length > 0),
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
}
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

  // State - Add to Queue Only (defaults to the user preference)
  const defaultAddToQueue = useUserPreferencesStore((state) => state.defaultAddToQueue);
  const [addToQueueOnly, setAddToQueueOnly] = useState(defaultAddToQueue);
  const defaultAddToDBOnly = useUserPreferencesStore((state) => state.defaultAddToDBOnly);
  const autoUpgradeSets = useUserPreferencesStore((state) => state.autoUpgradeSets);

  // State - Add New Collection Items
const defaultAutoAddNewCollectionItems = useUserPreferencesStore((state) => state.defaultAutoAddNewCollectionItems);
  const [autoAddNewCollectionItems, setAutoAddNewCollectionItems] = useState(false);

  // User Preferences
  const { downloadDefaults } = useUserPreferencesStore();

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
            autoupgrade: true,
            addToDBOnly: defaultAddToDBOnly,
            source: item.Set.type === "movie" || item.Set.type === "collection" ? item.Set.type : undefined,
          };
          return acc;
        },
        {} as z.infer<typeof formSchema>["selectedOptionsByItem"]
      ),
    },
  });

  const watchSelectedOptions = useWatch({
    control: form.control,
    name: "selectedOptionsByItem",
  });

  // Reset form on mount
  useEffect(() => {
    form.reset({
      selectedOptionsByItem: formItems.reduce(
        (acc, item) => {
          acc[getItemFormKey(item.MediaItem)] = {
            types: computeAssetTypes(item).filter((type) => downloadDefaults.includes(type)),
            autodownload: autoDownloadDefault,
            autoupgrade: true,
            addToDBOnly: defaultAddToDBOnly,
            source: item.Set.type === "movie" || item.Set.type === "collection" ? item.Set.type : undefined,
          };
          return acc;
        },
        {} as z.infer<typeof formSchema>["selectedOptionsByItem"]
      ),
    });
  }, [formItems, form, autoDownloadDefault, downloadDefaults, defaultAddToDBOnly]);

  useEffect(() => {
    // If all the form items are set to "Add to Database Only", change button text
    if (Object.values(watchSelectedOptions).every((option) => option.addToDBOnly || option.types?.length === 0)) {
      setButtonTexts((prev) => ({
        ...prev,
        download: "Add to Database",
      }));
    } else if (addToQueueOnly) {
      setButtonTexts((prev) => ({
        ...prev,
        download: "Add to Queue",
      }));
    } else {
      setButtonTexts((prev) => ({
        ...prev,
        download: "Download",
      }));
    }
  }, [addToQueueOnly, watchSelectedOptions]);

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
    const relevant = tasks.filter((t) => t.payload.kind === "download" && t.payload.imageFile.type === assetType);
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

  const runDownloadTask = async (taskId: string, payload: DownloadTaskPayload): Promise<boolean> => {
    updateTask(taskId, (t) => ({
      ...t,
      status: "in-progress",
      attempts: (t.attempts ?? 0) + 1,
      error: undefined,
    }));

    try {
      const response = await downloadImageFileForMediaItem(payload.imageFile, payload.mediaItem, payload.fileName);

      if (response.status === "error") {
        throw new Error(response.error?.message || "Unknown error");
      }

      updateTask(taskId, (t) => ({ ...t, status: "completed" }));
      return true;
    } catch (error) {
      const message = `"${payload.itemTitle}" - ${payload.fileName} - ${error instanceof Error ? error.message : "Unknown error"}`;
      updateTask(taskId, (t) => ({ ...t, status: "failed", error: message }));
      return false;
    }
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
        autoAddNewCollectionItems
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
    setCurrentText(`Adding "${payload.itemTitle}" to queue`, "Adding to queue");

    try {
      const resp = await AddItemToDownloadQueue(payload.dbItem);
      if (resp.status === "error") {
        throw new Error(resp.error?.message || (typeof resp.error === "string" ? resp.error : "Unknown error"));
      }
      updateTask(taskId, (t) => ({ ...t, status: "completed" }));
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

    if (t.payload.kind === "download") {
      await runDownloadTask(taskId, t.payload);
    } else if (t.payload.kind === "addToDB") {
      await runAddToDBTask(taskId, t.payload);
    } else if (t.payload.kind === "addToQueue") {
      await runAddToQueueTask(taskId, t.payload);
    }

    setButtonTexts((prev) => ({ ...prev, download: "Download Again" }));
  };

  const runInBatches = async <T,>(
    jobs: T[],
    batchSize: number,
    worker: (job: T) => Promise<boolean>
  ): Promise<boolean[]> => {
    const results: boolean[] = [];

    for (let i = 0; i < jobs.length; i += batchSize) {
      if (cancelRef.current) break;

      const batch = jobs.slice(i, i + batchSize);
      const settled = await Promise.allSettled(batch.map((job) => worker(job)));

      for (const entry of settled) {
        results.push(entry.status === "fulfilled" ? entry.value : false);
      }
    }

    return results;
  };

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

      // Add-to-queue-only => just one task (no downloads, no add-to-db)
      if (addToQueueOnly) {
        total += 1;
        continue;
      }

      // Download tasks
      for (const type of selected.types ?? []) {
        switch (type) {
          case "poster":
            if (item.Set.images.some((img) => img.type === "poster")) total += 1;
            break;
          case "backdrop":
            if (item.Set.images.some((img) => img.type === "backdrop")) total += 1;
            break;
          case "season_poster":
            total += item.Set.images?.filter((sp) => sp.type === "season_poster" && sp.season_number !== 0).length ?? 0;
            break;
          case "special_season_poster":
            total += item.Set.images?.filter((sp) => sp.type === "season_poster" && sp.season_number === 0).length ?? 0;
            break;
          case "titlecard":
            total +=
              item.Set.images?.filter(
                (tc) => tc.type === "titlecard" && tc.season_number != null && tc.episode_number != null
              ).length ?? 0;
            break;
        }
      }

      // After downloads, you *attempt* add-to-db (or mark it skipped) => count it as a planned task
      if ((selected.types?.length ?? 0) > 0) total += 1;
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
      log("INFO", "Download Modal", "Debug Info", "Add to Queue Only:", addToQueueOnly);
      log("INFO", "Download Modal", "Debug Info", "Auto Add New Collection Items:", autoAddNewCollectionItems);

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

        // Add to queue only (for items that actually have types selected)
        if (addToQueueOnly) {
          const taskId = newId();
          addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
            id: taskId,
            status: "pending",
            label: `Add "${item.MediaItem.title}" to queue`,
            attempts: 0,
            payload: {
              kind: "addToQueue",
              itemKey: getItemFormKey(item.MediaItem),
              itemTitle: item.MediaItem.title,
              dbItem: createdSavedItem.dbItem,
            },
          });

          const queued = await runAddToQueueTask(taskId, {
            kind: "addToQueue",
            itemKey: getItemFormKey(item.MediaItem),
            itemTitle: item.MediaItem.title,
            dbItem: createdSavedItem.dbItem,
          });
          if (queued) {
            markItemSaved(latestMediaItem, item, {
              poster: selectedTypes.includes("poster"),
              backdrop: selectedTypes.includes("backdrop"),
              season_poster: selectedTypes.includes("season_poster"),
              special_season_poster: selectedTypes.includes("special_season_poster"),
              titlecard: selectedTypes.includes("titlecard"),
            });
          }
          continue;
        }

        type DownloadJob = {
          taskId: string;
          payload: DownloadTaskPayload;
        };

        const downloadJobs: DownloadJob[] = [];

        for (const type of selectedTypes) {
          if (cancelRef.current) return; // Exit if cancelled
          switch (type) {
            case "poster": {
              const posterImg = item.Set.images.find((img) => img.type === "poster");
              if (!posterImg) break;

              const taskId = newId();
              const payload: DownloadTaskPayload = {
                kind: "download",
                itemKey: getItemFormKey(item.MediaItem),
                itemTitle: item.MediaItem.title,
                imageFile: posterImg,
                fileName: "Poster",
                mediaItem: latestMediaItem,
              };

              addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
                id: taskId,
                status: "pending",
                label: payload.fileName,
                attempts: 0,
                payload,
              });
              downloadJobs.push({ taskId, payload });
              break;
            }

            case "backdrop": {
              const backdropImg = item.Set.images.find((img) => img.type === "backdrop");
              if (!backdropImg) break;

              const taskId = newId();
              const payload: DownloadTaskPayload = {
                kind: "download",
                itemKey: getItemFormKey(item.MediaItem),
                itemTitle: item.MediaItem.title,
                imageFile: backdropImg,
                fileName: "Backdrop",
                mediaItem: latestMediaItem,
              };

              addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
                id: taskId,
                status: "pending",
                label: payload.fileName,
                attempts: 0,
                payload,
              });
              downloadJobs.push({ taskId, payload });
              break;
            }

            case "season_poster": {
              const seasonPosters = item.Set.images
                .filter((sp) => sp.type === "season_poster" && sp.season_number !== 0)
                .sort((a, b) => {
                  if (a.season_number! < b.season_number!) return -1;
                  if (a.season_number! > b.season_number!) return 1;
                  return 0;
                });

              for (const sp of seasonPosters) {
                const seasonNumber = sp.season_number!;
                const exists = latestMediaItem.series?.seasons.some((season) => season.season_number === seasonNumber);
                const taskId = newId();
                const payload: DownloadTaskPayload = {
                  kind: "download",
                  itemKey: getItemFormKey(item.MediaItem),
                  itemTitle: item.MediaItem.title,
                  imageFile: sp,
                  fileName: `Season ${seasonNumber} Poster`,
                  mediaItem: latestMediaItem,
                };

                addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
                  id: taskId,
                  status: "pending",
                  label: payload.fileName,
                  attempts: 0,
                  payload,
                });

                if (!exists) {
                  updateTask(taskId, (t) => ({ ...t, status: "skipped" }));
                  continue;
                }

                downloadJobs.push({ taskId, payload });
              }
              break;
            }

            case "special_season_poster": {
              const specialSeasonPosters = item.Set.images.filter(
                (sp) => sp.type === "season_poster" && sp.season_number === 0
              );

              for (const sp of specialSeasonPosters) {
                const taskId = newId();
                const payload: DownloadTaskPayload = {
                  kind: "download",
                  itemKey: getItemFormKey(item.MediaItem),
                  itemTitle: item.MediaItem.title,
                  imageFile: sp,
                  fileName: "Specials Season Poster",
                  mediaItem: latestMediaItem,
                };

                addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
                  id: taskId,
                  status: "pending",
                  label: payload.fileName,
                  attempts: 0,
                  payload,
                });
                downloadJobs.push({ taskId, payload });
              }
              break;
            }

            case "titlecard": {
              const titlecards = item.Set.images
                .filter((tc) => tc.type === "titlecard" && tc.season_number != null && tc.episode_number != null)
                .sort((a, b) => {
                  if (a.season_number! < b.season_number!) return -1;
                  if (a.season_number! > b.season_number!) return 1;
                  if (a.episode_number! < b.episode_number!) return -1;
                  if (a.episode_number! > b.episode_number!) return 1;
                  return 0;
                });

              for (const tc of titlecards) {
                const seasonNumber = tc.season_number!;
                const episodeNumber = tc.episode_number!;
                const exists = latestMediaItem.series?.seasons.some(
                  (season) =>
                    season.season_number === seasonNumber &&
                    season.episodes.some((ep) => ep.episode_number === episodeNumber)
                );
                const taskId = newId();
                const payload: DownloadTaskPayload = {
                  kind: "download",
                  itemKey: getItemFormKey(item.MediaItem),
                  itemTitle: item.MediaItem.title,
                  imageFile: tc,
                  fileName: `S${seasonNumber}E${episodeNumber} Titlecard`,
                  mediaItem: latestMediaItem,
                };

                addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
                  id: taskId,
                  status: "pending",
                  label: payload.fileName,
                  attempts: 0,
                  payload,
                });

                if (!exists) {
                  updateTask(taskId, (t) => ({ ...t, status: "skipped" }));
                  continue;
                }

                downloadJobs.push({ taskId, payload });
              }
              break;
            }
          }
        }

        const totalForItem = downloadJobs.length;

        // No matching images in this set for selected types (e.g. future types).
        // Still persist selected types to the DB so they can be auto-downloaded later.
        if (totalForItem === 0) {
          const addId = newId();
          addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
            id: addId,
            status: "pending",
            label: `Add "${item.MediaItem.title}" to DB`,
            attempts: 0,
            payload: {
              kind: "addToDB",
              itemKey: getItemFormKey(item.MediaItem),
              itemTitle: item.MediaItem.title,
              mediaItem: createdSavedItem.dbItem.media_item,
              posterSet: createdSavedItem.dbItem.poster_sets[0],
              addToDBOnly: selectedOptions.addToDBOnly || false,
            },
          });

          const ok = await runAddToDBTask(addId, {
            kind: "addToDB",
            itemKey: getItemFormKey(item.MediaItem),
            itemTitle: item.MediaItem.title,
            mediaItem: createdSavedItem.dbItem.media_item,
            posterSet: createdSavedItem.dbItem.poster_sets[0],
            addToDBOnly: selectedOptions.addToDBOnly || false,
          });

          if (ok) {
            markItemSaved(latestMediaItem, item, {
              poster: selectedOptions.types.includes("poster"),
              backdrop: selectedOptions.types.includes("backdrop"),
              season_poster: selectedOptions.types.includes("season_poster"),
              special_season_poster: selectedOptions.types.includes("special_season_poster"),
              titlecard: selectedOptions.types.includes("titlecard"),
            });
          }
          continue;
        }

        // Posters, then backdrops, then each season's poster and its titlecards, like the Download Queue
        const posterJobs = downloadJobs.filter((j) => j.payload.imageFile.type === "poster");
        const backdropJobs = downloadJobs.filter((j) => j.payload.imageFile.type === "backdrop");
        const otherJobs = downloadJobs
          .filter((j) => j.payload.imageFile.type !== "poster" && j.payload.imageFile.type !== "backdrop")
          .sort((a, b) => compareSeasonImages(a.payload.imageFile, b.payload.imageFile));

        // One count for all the images of the item, the same text as the Download Queue
        let started = 0;
        const downloadJob = async (job: (typeof downloadJobs)[number]) => {
          if (cancelRef.current) return false;
          started += 1;
          const image = job.payload.imageFile;
          const imageLabel = {
            image_type: image.type,
            season_number: image.season_number,
            episode_number: image.episode_number,
          };
          setCurrentText(
            formatDownloadProgressText(imageLabel, item.MediaItem.title, started, totalForItem),
            formatDownloadProgressShortText(imageLabel)
          );
          return runDownloadTask(job.taskId, job.payload);
        };

        let downloadResults: boolean[] = [];
        for (const jobs of [posterJobs, backdropJobs, otherJobs]) {
          if (jobs.length === 0) continue;
          downloadResults = downloadResults.concat(await runInBatches(jobs, DOWNLOAD_BATCH_SIZE, downloadJob));
        }

        const downloadedAtLeastOneForItem = downloadResults.some(Boolean);

        // Only add to DB if at least one download succeeded
        if (!downloadedAtLeastOneForItem) {
          const taskId = newId();
          addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
            id: taskId,
            status: "skipped",
            label: `Add "${item.MediaItem.title}" to DB (skipped: no successful downloads)`,
            attempts: 0,
            payload: {
              kind: "addToDB",
              itemKey: getItemFormKey(item.MediaItem),
              itemTitle: item.MediaItem.title,
              mediaItem: createdSavedItem.dbItem.media_item,
              posterSet: createdSavedItem.dbItem.poster_sets[0],
              addToDBOnly: selectedOptions.addToDBOnly || false,
            },
          });
          continue;
        }

        const addId = newId();
        addTask(getItemFormKey(item.MediaItem), item.MediaItem.title, {
          id: addId,
          status: "pending",
          label: `Add "${item.MediaItem.title}" to DB`,
          attempts: 0,
          payload: {
            kind: "addToDB",
            itemKey: getItemFormKey(item.MediaItem),
            itemTitle: item.MediaItem.title,
            mediaItem: createdSavedItem.dbItem.media_item,
            posterSet: createdSavedItem.dbItem.poster_sets[0],
            addToDBOnly: selectedOptions.addToDBOnly || false,
          },
        });

        const ok = await runAddToDBTask(addId, {
          kind: "addToDB",
          itemKey: getItemFormKey(item.MediaItem),
          itemTitle: item.MediaItem.title,
          mediaItem: createdSavedItem.dbItem.media_item,
          posterSet: createdSavedItem.dbItem.poster_sets[0],
          addToDBOnly: selectedOptions.addToDBOnly || false,
        });

        if (ok) {
          markItemSaved(latestMediaItem, item, {
            poster: selectedOptions.types.includes("poster"),
            backdrop: selectedOptions.types.includes("backdrop"),
            season_poster: selectedOptions.types.includes("season_poster"),
            special_season_poster: selectedOptions.types.includes("special_season_poster"),
            titlecard: selectedOptions.types.includes("titlecard"),
          });
        }
      }

      setCurrentText("Completed!");
      setButtonTexts({
        cancel: "Close",
        download: "Download Again",
      });
      setIsMounted(false);

      const timeTaken = (Date.now() - startTime) / 1000;
      log("INFO", "Download Modal", "Debug Info", `All tasks completed in ${timeTaken.toFixed(2)} seconds.`);
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
          setAddToQueueOnly(defaultAddToQueue);
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
          className={cn("z-50", "max-h-[80vh] overflow-y-auto", "sm:max-w-[700px]", "border border-primary")}
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
                {/* Form Items */}
                {formItems.map(renderFormItem)}

                {/* If the all items have no types selected, show a message */}
                {formItems.every(
                  (item) =>
                    !watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.types?.length &&
                    !watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.addToDBOnly
                ) && (
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

                {/* Add to Queue 
									Only show this button if at least one item has types selected for download and not set to Add to DB Only
								*/}
                {formItems.some(
                  (item) =>
                    watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.types?.length > 0 &&
                    !watchSelectedOptions?.[getItemFormKey(item.MediaItem)]?.addToDBOnly
                ) && (
                  <FormItem className="flex items-center space-x-2 mb-4">
                    <FormControl>
                      <Checkbox
                        checked={addToQueueOnly}
                        onCheckedChange={(checked) => setAddToQueueOnly(checked ? true : false)}
                      />
                    </FormControl>
                    <FormLabel className="text-md font-normal cursor-pointer">Add to Queue</FormLabel>
                    <DownloadModalPopover type="add-to-queue-only" />
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
                    const overall = getOverallProgress(progress);
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

                            const { done, total } = getOverallCounts(progress);

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
                                ) : buttonTexts.download === "Add to Queue" ? (
                                  <ListEnd className="h-4 w-4" />
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
