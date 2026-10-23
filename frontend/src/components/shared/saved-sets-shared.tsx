import { setRefsToFormItems } from "@/helper/download-modal/set-to-form-item";
import { DeleteItemFromDB } from "@/services/database/delete";
import { UpdateItemInDB } from "@/services/database/update";
import { UpgradeSavedSets } from "@/services/downloads/favorite-creators";
import { GetMediaItemDetails } from "@/services/mediaserver/get-media-item-details";
import { GetSetByID } from "@/services/mediux/get-set-by-id";
import { User } from "lucide-react";
import { toast } from "sonner";

import React from "react";

import Link from "next/link";

import DownloadModal from "@/components/shared/download-modal";
import { ErrorMessage } from "@/components/shared/error-message";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

import { cn } from "@/lib/cn";
import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { DBPosterSetDetail, DBSavedItem } from "@/types/database/db-poster-set";
import type { SelectedTypes } from "@/types/media-and-posters/media-item-and-library";
import type { IncludedItem, SetRef } from "@/types/media-and-posters/sets";

export const refreshPosterSet = async ({
  editSets,
  setEditSets,
  savedSet,
  setIsRefreshing,
  setUpdateError,
  setRefreshedSets,
  setRefreshedIncludedItems,
}: {
  editSets: EditSet[];
  setEditSets: React.Dispatch<React.SetStateAction<EditSet[]>>;
  savedSet: DBSavedItem;
  setIsRefreshing: (v: boolean) => void;
  setUpdateError: (v: APIResponse<unknown> | null) => void;
  setRefreshedSets: React.Dispatch<React.SetStateAction<SetRef[]>>;
  setRefreshedIncludedItems: React.Dispatch<React.SetStateAction<{ [tmdb_id: string]: IncludedItem }>>;
}) => {
  try {
    setIsRefreshing(true);
    setRefreshedSets([]);
    setRefreshedIncludedItems({});

    // Track if any requests failed
    let hasError = false;
    let errorResponse: APIResponse<unknown> | null = null;

    await Promise.all(
      editSets.map(async (set) => {
        if (set.id === "ignore") {
          return;
        }

        // Update the media item in the backend store by calling fetchMediaServerItemContent
        const resp = await GetMediaItemDetails(
          savedSet.media_item.title,
          savedSet.media_item.rating_key,
          savedSet.media_item.library_title
        );

        if (!resp || resp.status === "error") {
          log("ERROR", "Saved Sets Shared", "refreshPosterSet", resp?.error?.message || "Unknown error");
          // Store the first error we encounter
          if (!hasError) {
            hasError = true;
            errorResponse = resp;
          }
          return;
        }

        const response = await GetSetByID(
          savedSet.media_item.library_title,
          savedSet.media_item.tmdb_id,
          set.id,
          set.type,
          savedSet.media_item.edition
        );

        if (!response || response.status === "error") {
          log("ERROR", "Saved Sets Shared", "refreshPosterSet", response?.error?.message || "Unknown error");
          // Store the first error we encounter
          if (!hasError) {
            hasError = true;
            errorResponse = response;
          }
          return;
        }
        if (!response.data) {
          log("ERROR", "Saved Sets Shared", "refreshPosterSet", "No PosterSet found in response:", response);
          return;
        }

        const data = response.data;

        // Update refreshed sets and included items
        setRefreshedSets((prev) => {
          // Avoid duplicates
          if (prev.find((s) => s.id === data.set.id)) {
            return prev;
          }
          return [...prev, data.set];
        });
        setRefreshedIncludedItems((prev) => ({
          // Avoid duplicates
          ...prev,
          ...data.included_items,
        }));

        setEditSets((prev) =>
          prev.map((item) =>
            item.id === set.id
              ? {
                  ...item,
                  date_updated: data.set.date_updated,
                }
              : item
          )
        );
      })
    );

    // Update error state based on API calls results
    if (hasError) {
      setUpdateError(errorResponse);
    } else {
      setUpdateError(null);
    }
  } catch {
    // Handle unexpected errors
    setUpdateError({
      status: "error",
      error: {
        message: "Unexpected error refreshing poster sets",
        help: "Please try again later",
        function: "refreshPosterSet",
        line_number: 0,
      },
    });
  } finally {
    setIsRefreshing(false);
  }
};

export const renderTypeBadges = (savedSet: DBSavedItem) => {
  // A Download Queue entry with only collection images has no sets
  const selected = (savedSet.poster_sets ?? []).reduce(
    (acc, ps) => {
      acc.poster ||= ps.selected_types?.poster === true;
      acc.backdrop ||= ps.selected_types?.backdrop === true;
      acc.season_poster ||= ps.selected_types?.season_poster === true;
      acc.special_season_poster ||= ps.selected_types?.special_season_poster === true;
      acc.titlecard ||= ps.selected_types?.titlecard === true;
      return acc;
    },
    {
      poster: false,
      backdrop: false,
      season_poster: false,
      special_season_poster: false,
      titlecard: false,
    }
  );

  const badges: React.ReactNode[] = [];
  if (selected.poster) {
    badges.push(
      <Badge key="poster" className="bg-primary text-primary-foreground">
        Poster
      </Badge>
    );
  }
  if (selected.backdrop) {
    badges.push(
      <Badge key="backdrop" className="bg-primary text-primary-foreground">
        Backdrop
      </Badge>
    );
  }
  if (selected.season_poster) {
    badges.push(
      <Badge key="season_poster" className="bg-primary text-primary-foreground">
        Season Posters
      </Badge>
    );
  }
  if (selected.special_season_poster) {
    badges.push(
      <Badge key="special_season_poster" className="bg-primary text-primary-foreground">
        Special Poster
      </Badge>
    );
  }
  if (selected.titlecard) {
    badges.push(
      <Badge key="titlecard" className="bg-primary text-primary-foreground">
        Title Card
      </Badge>
    );
  }

  // A Download Queue entry's collection images
  for (const image of savedSet.collection_images?.images ?? []) {
    badges.push(
      <Badge key={image.type} className="bg-primary text-primary-foreground">
        {image.type === "collection_poster" ? "Collection Poster" : "Collection Backdrop"}
      </Badge>
    );
  }
  return badges;
};

export const handleStopIgnoring = async (
  savedSet: DBSavedItem,
  onUpdate: () => void,
  unignoreLoading: boolean,
  setUnignoreLoading: (v: boolean) => void,
  setUpdateError: (v: APIResponse<unknown> | null) => void
) => {
  if (unignoreLoading) return;
  setUnignoreLoading(true);
  const resp = await DeleteItemFromDB(savedSet);
  if (!resp || resp.status === "error") {
    log("ERROR", "Saved Sets Shared", "handleStopIgnoring", resp?.error?.message || "Unknown error");
    setUpdateError(resp);
    setUnignoreLoading(false);
    return;
  }
  setUpdateError(null);
  onUpdate();
  setUnignoreLoading(false);
};

export const onCloseSavedSetsEditDeleteModals = ({
  setIsEditModalOpen,
  setIsDeleteModalOpen,
  setUpdateError,
  setIsMounted,
}: {
  setIsEditModalOpen: (v: boolean) => void;
  setIsDeleteModalOpen: (v: boolean) => void;
  setUpdateError: (v: APIResponse<unknown> | null) => void;
  setIsMounted: (v: boolean) => void;
}) => {
  setIsEditModalOpen(false);
  setIsDeleteModalOpen(false);
  setUpdateError(null);
  setIsMounted(false);
};

export const savedSetsConfirmEdit = async ({
  editSets,
  savedSet,
  onUpdate,
  isMounted,
  setIsMounted,
  setUpdateError,
  setIsEditModalOpen,
  setIsDeleteModalOpen,
  allToDelete,
}: {
  editSets: EditSet[];
  savedSet: DBSavedItem;
  onUpdate: () => void;
  isMounted: boolean;
  setIsMounted: (v: boolean) => void;
  setUpdateError: (v: APIResponse<unknown> | null) => void;
  setIsEditModalOpen: (v: boolean) => void;
  setIsDeleteModalOpen: (v: boolean) => void;
  allToDelete: boolean;
}) => {
  if (isMounted) return;
  setIsMounted(true);

  if (allToDelete) {
    setIsEditModalOpen(false);
    setIsDeleteModalOpen(true);
    setUpdateError(null);
    setIsMounted(false);
    return;
  }

  // Create a new DBSavedItem object with updated values
  const updatedSavedSet: DBSavedItem = {
    ...savedSet,

    poster_sets: editSets.map((editSet, _) => ({
      id: editSet.id,
      title: editSet.title,
      type: editSet.type,
      user_created: editSet.user_created,
      date_created: editSet.date_created,
      date_updated: editSet.date_updated,
      popularity: editSet.popularity,
      popularity_global: editSet.popularity_global,
      set: editSet,
      images: editSet.images,
      last_downloaded: new Date().toISOString(),
      selected_types: editSet.selected_types,
      auto_download: editSet.auto_download,
      auto_add_new_collection_items: editSet.auto_add_new_collection_items,
      no_auto_upgrade: editSet.no_auto_upgrade,
      to_delete: editSet.to_delete,
    })),
  };

  const response = await UpdateItemInDB(updatedSavedSet);
  if (!response || response.status === "error") {
    log("ERROR", "Saved Sets Shared", "savedSetsConfirmEdit", response?.error?.message || "Unknown error");
    setUpdateError(response);
    setIsMounted(false);
    return;
  }

  setUpdateError(null);
  setIsEditModalOpen(false);
  onUpdate();

  setIsMounted(false);
};

export const savedSetsConfirmDelete = async ({
  savedSet,
  onUpdate,
  isMounted,
  setIsMounted,
  setUpdateError,
  setIsDeleteModalOpen,
}: {
  savedSet: DBSavedItem;
  onUpdate: () => void;
  isMounted: boolean;
  setIsMounted: (v: boolean) => void;
  setUpdateError: (v: APIResponse<unknown> | null) => void;
  setIsDeleteModalOpen: (v: boolean) => void;
}) => {
  if (isMounted) return;
  setIsMounted(true);
  const resp = await DeleteItemFromDB(savedSet);
  if (!resp || resp.status === "error") {
    log("ERROR", "Saved Sets Shared", "savedSetsConfirmDelete", resp?.error?.message || "Unknown error");
    setUpdateError(resp);
    setIsMounted(false);
    return;
  }
  setUpdateError(null);
  setIsDeleteModalOpen(false);
  onUpdate();
  setIsMounted(false);
};

export interface EditSet extends DBPosterSetDetail {
  previousDateUpdated: string;
}

export interface SavedSetEditModalProps {
  open: boolean;
  onClose: () => void;
  editSets: EditSet[];
  setEditSets: React.Dispatch<React.SetStateAction<EditSet[]>>;
  savedSet: DBSavedItem;
  allToDelete: boolean;
  updateError: APIResponse<unknown> | null;
  confirmEdit: () => void;
  refreshedSets?: SetRef[];
  refreshedIncludedItems?: { [tmdb_id: string]: IncludedItem };
}

export const SavedSetEditModal: React.FC<SavedSetEditModalProps> = ({
  open,
  onClose,
  editSets,
  setEditSets,
  allToDelete,
  updateError,
  confirmEdit,
  refreshedSets,
  refreshedIncludedItems,
}) => {
  // Replace the hard-coded array with dynamically generated list.
  const renderEditTypeBadges = (editSet: (typeof editSets)[number], index: number) => {
    const images = editSet.images ?? [];

    // Build available types in one pass
    let hasPoster = false;
    let hasBackdrop = false;
    let hasTitlecard = false;
    let hasSeason0Poster = false;
    let hasNonSeason0Poster = false;

    for (const img of images) {
      if (img.type === "poster") hasPoster = true;
      else if (img.type === "backdrop") hasBackdrop = true;
      else if (img.type === "titlecard") hasTitlecard = true;
      else if (img.type === "season_poster") {
        if (img.season_number === 0) hasSeason0Poster = true;
        else hasNonSeason0Poster = true;
      }
    }

    const availableTypes: string[] = [];
    if (hasPoster) availableTypes.push("poster");
    if (hasBackdrop) availableTypes.push("backdrop");
    if (hasSeason0Poster) availableTypes.push("special_season_poster");
    if (hasNonSeason0Poster) availableTypes.push("season_poster");
    if (hasTitlecard) availableTypes.push("titlecard");

    return availableTypes.map((type) => {
      const isSelected = editSet.selected_types?.[type as keyof SelectedTypes] === true;

      const isTypeDisabled =
        editSet.to_delete ||
        (!isSelected &&
          editSets.some((item, j) => j !== index && item.selected_types?.[type as keyof SelectedTypes] === true));

      const label =
        type === "poster"
          ? "Poster"
          : type === "backdrop"
            ? "Backdrop"
            : type === "season_poster"
              ? "Season Posters"
              : type === "special_season_poster"
                ? "Special Poster"
                : type === "titlecard"
                  ? "Title Card"
                  : type;

      return (
        <Badge
          key={type}
          className={`flex items-center gap-2 transition duration-200 hover:brightness-120 active:scale-95 ${
            isTypeDisabled
              ? "bg-secondary opacity-50 cursor-not-allowed"
              : isSelected
                ? "cursor-pointer bg-primary text-primary-foreground"
                : "cursor-pointer bg-secondary text-secondary-foreground"
          }`}
          onClick={() => {
            if (isTypeDisabled) return;

            setEditSets((prev) =>
              prev.map((item, i) => {
                if (i !== index) return item;

                const key = type as keyof SelectedTypes;
                return {
                  ...item,
                  selected_types: {
                    ...item.selected_types,
                    [key]: !(item.selected_types?.[key] === true),
                  },
                };
              })
            );
          }}
        >
          {label}
        </Badge>
      );
    });
  };

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent
        className={cn(
          "z-50",
          "max-h-[80vh] overflow-y-auto",
          "sm:max-w-[700px]",
          allToDelete ? "border border-red-500" : "border border-primary"
        )}
      >
        <DialogHeader>
          <DialogTitle>Edit Saved Set</DialogTitle>
          <DialogDescription>
            Edit each set individually. Toggle type badges to update selected types. Use the delete option to mark a set
            for deletion.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {editSets.map((editSet, index) => (
            <div key={editSet.id} className="border p-2 rounded-md">
              <div className="flex items-center justify-between">
                <span className="font-semibold">
                  <Link
                    href={`https://mediux.io/${editSet.type}-set/${editSet.id}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="hover:underline ml-1"
                  >
                    {editSet.title}
                  </Link>
                </span>
                <Button
                  variant={editSet.to_delete ? "ghost" : "outline"}
                  size="sm"
                  className={`cursor-pointer border-1 hover:brightness-120 hover:text-red-500 ${editSet.to_delete && "text-destructive"}`}
                  onClick={() => {
                    setEditSets((prev) =>
                      prev.map((item, i) =>
                        i === index
                          ? {
                              ...item,
                              to_delete: !item.to_delete,
                              selected_types: !item.to_delete ? ({} as SelectedTypes) : item.selected_types,
                            }
                          : item
                      )
                    );
                  }}
                >
                  {editSet.to_delete ? "Undo Delete" : "Delete Set"}
                </Button>
              </div>
              {editSet.user_created && (
                <div className="flex items-center gap-1">
                  <Avatar className="rounded-lg mr-1 w-4 h-4">
                    <AvatarImage
                      src={`/api/images/mediux/avatar?username=${editSet.user_created}`}
                      className="w-4 h-4"
                    />
                    <AvatarFallback className="">
                      <User className="w-4 h-4" />
                    </AvatarFallback>
                  </Avatar>
                  <Link href={`/user/${editSet.user_created}`} className="hover:underline">
                    {editSet.user_created}
                  </Link>
                </div>
              )}
              <DialogDescription className="ml-1">
                Set ID:{" "}
                <Link
                  href={`https://mediux.io/${editSet.type}-set/${editSet.id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:underline"
                >
                  {editSet.id}
                </Link>
              </DialogDescription>
              <div className="flex flex-wrap gap-2 mt-2">{renderEditTypeBadges(editSet, index)}</div>
              <div className="flex flex-wrap gap-2 mt-2">
                <Badge
                  className={`cursor-pointer transition duration-200 ${
                    editSet.auto_download
                      ? "bg-primary text-primary-foreground hover:bg-red-500"
                      : "bg-secondary text-secondary-foreground"
                  }`}
                  onClick={() => {
                    setEditSets((prev) =>
                      prev.map((item, i) =>
                        i === index
                          ? {
                              ...item,
                              auto_download: !item.auto_download,
                            }
                          : item
                      )
                    );
                  }}
                >
                  {editSet.auto_download ? "Autodownload" : "No Autodownload"}
                </Badge>
                <Badge
                  className={`cursor-pointer transition duration-200 ${
                    !editSet.no_auto_upgrade
                      ? "bg-primary text-primary-foreground hover:bg-red-500"
                      : "bg-secondary text-secondary-foreground"
                  }`}
                  onClick={() => {
                    setEditSets((prev) =>
                      prev.map((item, i) => (i === index ? { ...item, no_auto_upgrade: !item.no_auto_upgrade } : item))
                    );
                  }}
                >
                  {editSet.no_auto_upgrade ? "No Auto Upgrade" : "Auto Upgrade"}
                </Badge>
              </div>
              <div className="flex items-center justify-between">
                <div>
                  {editSet.previousDateUpdated &&
                    editSet.date_updated &&
                    editSet.previousDateUpdated !== editSet.date_updated &&
                    // Compare the two dates to check if the set has been updated since it was last fetched
                    new Date(editSet.date_updated) > new Date(editSet.previousDateUpdated) && (
                      <div className="text-green-600 text-xs mt-1">Set has updates</div>
                    )}
                </div>
                {refreshedSets && refreshedIncludedItems && (
                  <div className="flex items-center">
                    <span className="text-md text-muted-foreground mt-2 mr-2">Redownload</span>
                    <DownloadModal
                      baseSetInfo={editSet}
                      formItems={setRefsToFormItems(
                        refreshedSets.find((s) => s.id === editSet.id)
                          ? [refreshedSets.find((s) => s.id === editSet.id)!]
                          : [],
                        refreshedIncludedItems
                      )}
                    />
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
        {updateError && <ErrorMessage error={updateError} />}
        <DialogFooter>
          <Button
            variant="outline"
            className={cn("active:scale-95 hover:brightness-120", "hover:text-primary")}
            onClick={onClose}
          >
            Cancel
          </Button>
          <Button
            className={`cursor-pointer hover:brightness-120 border-1 ${allToDelete ? "text-destructive" : ""}`}
            variant={allToDelete ? "ghost" : "default"}
            onClick={confirmEdit}
          >
            {allToDelete ? (editSets.length === 1 ? "Delete Set" : "Delete All") : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export interface SavedSetDeleteModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  confirmDelete: () => void;
}

export const SavedSetDeleteModal: React.FC<SavedSetDeleteModalProps> = ({ open, onClose, title, confirmDelete }) => (
  <Dialog open={open} onOpenChange={onClose}>
    <DialogContent className={cn("z-50", "max-h-[80vh] overflow-y-auto", "sm:max-w-[700px]", "border border-red-500")}>
      <DialogHeader>
        <DialogTitle>Confirm Delete</DialogTitle>
        <DialogDescription>
          Are you sure you want to delete all sets for "{title}"? This action cannot be undone.
        </DialogDescription>
      </DialogHeader>
      <DialogFooter>
        <Button variant="outline" className="hover:text-primary active:scale-95 hover:brightness-120" onClick={onClose}>
          Cancel
        </Button>
        <Button
          variant="ghost"
          className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
          onClick={confirmDelete}
        >
          Delete
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
);

export interface SavedSetsListProps {
  savedSet: DBSavedItem;
  layout: "table" | "card";
}

export const SavedSetsList: React.FC<SavedSetsListProps> = ({ savedSet, layout }) => {
  const sets = savedSet.poster_sets;
  if (sets.length === 0) return null;
  const heading = sets.length > 1 ? "Sets:" : "Set:";

  if (layout === "card") {
    return (
      <div className="w-full">
        <span className="text-sm text-muted-foreground">{heading}</span>
        <ul className="flex flex-col gap-1">
          {sets.map((ps) => (
            <li key={ps.id} className="flex flex-row items-center justify-between rounded-sm hover:bg-muted/50">
              <span className="text-primary text-sm shrink-0">{ps.id}</span>

              <div className="flex items-center gap-1">
                <Avatar className="rounded-lg mr-1 w-4 h-4">
                  <AvatarImage src={`/api/images/mediux/avatar?username=${ps.user_created}`} className="w-4 h-4" />
                  <AvatarFallback className="">
                    <User className="w-4 h-4" />
                  </AvatarFallback>
                </Avatar>
                <Link href={`/user/${ps.user_created}`} className="text-primary hover:underline text-xs text-right">
                  {ps.user_created || ""}
                </Link>
              </div>
            </li>
          ))}
        </ul>
      </div>
    );
  }

  // layout === "table"
  return (
    <div className="w-full">
      <ul className="flex flex-col gap-0">
        {sets.map((ps) => (
          <li key={ps.id} className="flex flex-row gap-4 items-center rounded-sm px-2 py-1 hover:bg-muted/50">
            <span className="text-primary text-sm">{ps.id}</span>
            <span className="text-muted-foreground text-xs">—</span>
            <div className="flex items-center gap-1">
              <Avatar className="rounded-lg mr-1 w-4 h-4">
                <AvatarImage src={`/api/images/mediux/avatar?username=${ps.user_created}`} className="w-4 h-4" />
                <AvatarFallback className="">
                  <User className="w-4 h-4" />
                </AvatarFallback>
              </Avatar>
              <Link href={`/user/${ps.user_created}`} className="text-primary hover:underline text-xs text-right">
                {ps.user_created || ""}
              </Link>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
};

// upgradeSavedItem replaces the item's saved sets with a set from a higher ranked favorite creator
export const upgradeSavedItem = async (item: DBSavedItem) => {
  const title = item.media_item.title;
  const id = `upgrade-${item.media_item.tmdb_id}-${item.media_item.library_title}`;
  toast.loading(`Checking ${title} for an upgrade...`, { id });
  const response = await UpgradeSavedSets([
    {
      tmdb_id: item.media_item.tmdb_id,
      library_title: item.media_item.library_title,
      edition: item.media_item.edition ?? "",
    },
  ]);
  if (response.status === "error" || !response.data) {
    toast.error(response.error?.message || `Could not upgrade ${title}`, { id });
  } else if (response.data.upgraded > 0) {
    toast.success(`${title} was upgraded, the new set is in the Download Queue.`, { id });
  } else if (response.data.checked === 0) {
    toast.info(`Auto Upgrade is off for ${title}'s sets, so they're kept.`, { id });
  } else {
    toast.info(`None of your favorite creators ranked above this set's creator have a set for ${title}.`, { id });
  }
};
