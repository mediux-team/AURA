"use client";

import { makePlural } from "@/helper/make_plural";
import { ReturnErrorMessage } from "@/services/api-error-return";
import type { AutodownloadResult } from "@/services/database/autodownload-force-check";
import { AutoDownloadForceCheck } from "@/services/database/autodownload-force-check";
import { DeleteItemFromDB } from "@/services/database/delete";
import { getAllItemsFromDB } from "@/services/database/get-all";
import { UpdateItemInDB } from "@/services/database/update";
import { UpgradeSavedSets, upgradeSetsMessage } from "@/services/downloads/favorite-creators";
import { AddItemToDownloadQueue } from "@/services/downloads/queue-add";
import { ApplyLabelsAndTagsToItem } from "@/services/labels-tags/apply";
import { GetLibrarySummaries } from "@/services/mediaserver/library-items";
import {
  ArrowBigUpDash,
  ArrowDownAZ,
  ArrowDownZA,
  BookmarkIcon,
  ClockArrowDown,
  ClockArrowUp,
  CloudCheck,
  CloudDownload,
  CloudOff,
  RefreshCcw as RefreshIcon,
  Tag,
  Trash2,
  XCircle,
} from "lucide-react";
import { toast } from "sonner";

import React, { useCallback, useEffect, useRef, useState } from "react";

import { CustomPagination } from "@/components/shared/custom-pagination";
import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import { ErrorMessage } from "@/components/shared/error-message";
import { FilterSavedSets } from "@/components/shared/filter-saved-sets";
import Loader from "@/components/shared/loader";
import { RefreshButton } from "@/components/shared/refresh-button";
import { RefreshProgress } from "@/components/shared/refresh-progress";
import { ResponsiveGrid } from "@/components/shared/responsive-grid";
import SavedSetsCard from "@/components/shared/saved-sets-cards";
import SavedSetsTableRow from "@/components/shared/saved-sets-table";
import { ViewControl } from "@/components/shared/select-view";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableHead, TableHeader, TableRow } from "@/components/ui/table";

import { cn } from "@/lib/cn";
import { log } from "@/lib/logger";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useSavedSetsPageStore } from "@/lib/stores/page-store-saved-sets";

import { extractInfoFromSearchQuery } from "@/hooks/search-query";
import { useRefreshing } from "@/hooks/use-refreshing";

import type { APIResponse } from "@/types/api/api-response";
import type { DBSavedItem } from "@/types/database/db-poster-set";

const SavedSetsPage: React.FC = () => {
  const [savedSets, setSavedSets] = useState<DBSavedItem[]>([]);
  const [loading, setLoading] = useState(true);
  // A refresh keeps the page showing, with its progress at the top
  const [refreshing, handleRefresh] = useRefreshing(() => fetchSavedSets());
  const [error, setError] = useState<APIResponse<unknown> | null>(null);
  const isFetchingRef = useRef(false);
  const { searchQuery, setSearchQuery } = useSearchQueryStore();
  const [recheckStatus, setRecheckStatus] = useState<Record<string, AutodownloadResult>>({});

  const [bulkEditMode, setBulkEditMode] = useState(false);
  const [bulkEditActionGroups] = useState<{ group: string; actions: { value: string; label: string }[] }[]>([
    {
      group: "Autodownload",
      actions: [
        { value: "force-recheck", label: "Force Autodownload Recheck" },
        { value: "enable-autodownload", label: "Enable Autodownload" },
        { value: "disable-autodownload", label: "Disable Autodownload" },
      ],
    },
    {
      group: "Downloads",
      actions: [
        { value: "force-redownload", label: "Force Redownload" },
        { value: "upgrade-sets", label: "Upgrade to Favorite Creators" },
      ],
    },
    {
      group: "Metadata",
      actions: [{ value: "apply-label-tag", label: "Apply Labels/Tags" }],
    },
    {
      group: "Danger Zone",
      actions: [{ value: "delete-selected", label: "Delete Selected Sets" }],
    },
  ]);
  const [bulkEditSelectedAction, setBulkEditSelectedAction] = useState<string>("");
  const [bulkEditSelectedItems, setBulkEditSelectedItems] = useState<Set<string>>(new Set());

  const {
    currentPage,
    setCurrentPage,
    itemsPerPage,
    setItemsPerPage,
    sortOption,
    setSortOption,
    sortOrder,
    setSortOrder,
    viewOption,
    setViewOption,
    filteredLibraries,
    setFilteredLibraries,
    filterAutoDownload,
    setFilterAutoDownload,
    filteredUsers,
    setFilteredUsers,
    filteredTypes,
    setFilteredTypes,
    filterMultiSetOnly,
    setFilterMultiSetOnly,
    filterMediaItemOnServer,
    setFilterMediaItemOnServer,
  } = useSavedSetsPageStore();

  // The library options, from aura
  const [librarySummaries, setLibrarySummaries] = useState<{ title: string; type: string }[]>([]);
  const [librarySectionsLoaded, setLibrarySectionsLoaded] = useState(false);
  useEffect(() => {
    void GetLibrarySummaries().then((libraries) => {
      setLibrarySummaries(libraries);
      setLibrarySectionsLoaded(true);
    });
  }, []);
  const getSectionSummaries = useCallback(() => librarySummaries, [librarySummaries]);

  // User Filter Options: all unique users from the savedSets
  const [filterUserOptions, setFilterUserOptions] = useState<string[]>([]);

  // Selected Types Options
  const typeOptions = [
    { label: "Poster", value: "poster" },
    { label: "Backdrop", value: "backdrop" },
    { label: "Season Posters", value: "season_poster" },
    { label: "Title Cards", value: "titlecard" },
    { label: "No Selected Types", value: "none" },
  ]; // Total Items: Total items matching filters in DB (for pagination)
  const [totalItems, setTotalItems] = useState(0);
  // Is Wide Screen: for showing/hiding the ViewControl
  const [isWideScreen, setIsWideScreen] = useState(false);

  // Set sortOption to "date_downloaded" if its not title, date_downloaded, year, or library
  useEffect(() => {
    if (
      sortOption !== "title" &&
      sortOption !== "date_downloaded" &&
      sortOption !== "year" &&
      sortOption !== "library"
    ) {
      setSortOption("date_downloaded");
      setSortOrder("desc");
    }
  }, [sortOption, setSortOption, setSortOrder]);

  const { searchTMDBID, searchLibrary, searchYear, searchTitle } = extractInfoFromSearchQuery(searchQuery);

  // Fetch saved sets with filters from store
  const fetchSavedSets = useCallback(async () => {
    if (isFetchingRef.current) return;
    isFetchingRef.current = true;
    try {
      setLoading(true);

      const response = await getAllItemsFromDB(
        searchTMDBID,
        searchLibrary,
        searchYear,
        searchTitle,
        filteredLibraries,
        filteredTypes,
        filterAutoDownload,
        filterMultiSetOnly,
        filteredUsers,
        filterMediaItemOnServer,
        itemsPerPage,
        currentPage,
        sortOption,
        sortOrder
      );

      if (response.status === "error") {
        setError(response);
        setSavedSets([]);
        setTotalItems(0);
        setFilterUserOptions([]);
        return;
      }

      if (!response.data) {
        // Nothing saved yet is not an error
        setError(null);
        setSavedSets([]);
        setTotalItems(0);
        setFilterUserOptions([]);
        return;
      }

      setSavedSets(response.data.items);
      setTotalItems(response.data.total_items || 0);
      setFilterUserOptions(response.data.unique_users || []);

      setError(null);
    } catch (error) {
      setError(ReturnErrorMessage<unknown>(error));
      setSavedSets([]);
      setTotalItems(0);
      setFilterUserOptions([]);
    } finally {
      setLoading(false);
      isFetchingRef.current = false;
    }
  }, [
    searchTMDBID,
    searchLibrary,
    searchYear,
    searchTitle,
    filteredLibraries,
    filteredTypes,
    filterAutoDownload,
    filterMultiSetOnly,
    filteredUsers,
    filterMediaItemOnServer,
    itemsPerPage,
    currentPage,
    sortOption,
    sortOrder,
  ]);

  // Load values from store first, then fetch data
  useEffect(() => {
    // Fetch after store values are loaded
    fetchSavedSets();
  }, [fetchSavedSets]);

  // Change to Card View if on mobile
  useEffect(() => {
    const handleResize = () => {
      if (window.innerWidth < 1300) {
        setViewOption("card");
        setIsWideScreen(false);
      } else {
        setIsWideScreen(true);
      }
    };
    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, [setViewOption]);

  // Calculate total pages
  const totalPages = Math.ceil(totalItems / itemsPerPage);

  // Whether a filter or search is applied
  const hasActiveFilters =
    !!searchQuery ||
    filteredLibraries.length > 0 ||
    !!filterAutoDownload ||
    filteredUsers.length > 0 ||
    filteredTypes.length > 0 ||
    !!filterMediaItemOnServer ||
    !!filterMultiSetOnly;

  // Nothing saved yet, so only show the empty state
  const nothingSavedYet =
    (!savedSets || savedSets.length === 0) && !hasActiveFilters && !Object.keys(recheckStatus).length;

  if (loading && !refreshing) {
    return <Loader className="mt-10" message="Loading saved sets..." />;
  }

  if (error) {
    return (
      <div className="flex flex-col items-center p-6 gap-4">
        <ErrorMessage error={error} />
      </div>
    );
  }

  const handleRecheckItem = async (title: string, item: DBSavedItem): Promise<void> => {
    try {
      const response = await AutoDownloadForceCheck(item);
      if (response.status === "error") {
        toast.error(response.error?.message || "Failed to recheck item");
        return;
      }
      setRecheckStatus((prev) => ({
        ...prev,
        [title]: response.data?.result as AutodownloadResult,
      }));
    } catch (error) {
      const errorResponse = ReturnErrorMessage<unknown>(error);
      toast.error(errorResponse.error?.message || "An unexpected error occurred");
    }
    fetchSavedSets();
  };

  const handleRedownloadItem = async (item: DBSavedItem): Promise<void> => {
    try {
      // The queue gets the sets' latest images
      const response = await AddItemToDownloadQueue({ ...item, source: "redownload" });
      if (response.status === "error") {
        toast.error(response.error?.message || "Failed to add item to download queue");
        return;
      }
    } catch (error) {
      const errorResponse = ReturnErrorMessage<unknown>(error);
      toast.error(errorResponse.error?.message || "An unexpected error occurred");
    }
  };

  const handleApplyLabelsTagsToItem = async (item: DBSavedItem): Promise<void> => {
    try {
      const response = await ApplyLabelsAndTagsToItem(item);
      if (response.status === "error") {
        toast.error(response.error?.message || "Failed to apply labels/tags");
        return;
      }
    } catch (error) {
      const errorResponse = ReturnErrorMessage<unknown>(error);
      toast.error(errorResponse.error?.message || "An unexpected error occurred");
    }
  };

  const handleDeleteItemFromDB = async (item: DBSavedItem): Promise<void> => {
    try {
      const response = await DeleteItemFromDB(item);
      if (response.status === "error") {
        toast.error(response.error?.message || "Failed to delete item");
        return;
      }
    } catch (error) {
      const errorResponse = ReturnErrorMessage<unknown>(error);
      toast.error(errorResponse.error?.message || "An unexpected error occurred");
    }
  };

  const actionForceRedownload = async (itemsToRedownload: DBSavedItem[]) => {
    if (isFetchingRef.current) return;
    isFetchingRef.current = true;
    setRecheckStatus({}); // Reset recheck status

    log("INFO", "Saved Sets Page", "Force Redownload", `Forcing redownload for ${itemsToRedownload.length} sets`, {
      itemsToRedownload,
    });

    // Show loading toast
    toast.loading(`Adding ${itemsToRedownload.length} items to download queue...`, {
      id: "force-redownload",
      duration: 0, // Keep it open until we manually close it
    });

    for (const [index, item] of itemsToRedownload.entries()) {
      toast.loading(`Adding ${index + 1} of ${itemsToRedownload.length} - ${item.media_item.title} to download queue`, {
        id: "force-redownload",
        duration: 0, // Keep it open until we manually close it
      });
      await handleRedownloadItem(item);
    }

    // Close loading toast
    toast.success("Add to download queue completed", {
      id: "force-redownload",
      duration: 2000,
    });

    isFetchingRef.current = false;
    fetchSavedSets();
  };

  const actionForceAutoDownloadRecheck = async (setsToRecheck: DBSavedItem[]) => {
    if (isFetchingRef.current) return;
    isFetchingRef.current = true;
    setRecheckStatus({}); // Reset recheck status

    log("INFO", "Saved Sets Page", "Force Recheck", `Forcing recheck for ${setsToRecheck.length} sets`, {
      setsToRecheck,
    });

    // Show loading toast
    toast.loading(`Rechecking ${setsToRecheck.length} sets...`, {
      id: "force-recheck",
      duration: 0, // Keep it open until we manually close it
    });

    for (const [index, set] of setsToRecheck.entries()) {
      toast.loading(`Rechecking ${index + 1} of ${setsToRecheck.length} - ${set.media_item.title}`, {
        id: "force-recheck",
        duration: 0, // Keep it open until we manually close it
      });
      await handleRecheckItem(set.media_item.title, set);
    }

    // Close loading toast
    toast.success("Recheck completed", {
      id: "force-recheck",
      duration: 2000,
    });

    isFetchingRef.current = false;
    fetchSavedSets();
  };

  const actionBulkSetAutodownload = async (setsToUpdate: DBSavedItem[], autoDownload: boolean) => {
    const actionLabel = autoDownload ? "Enable Autodownload" : "Disable Autodownload";
    log(
      "INFO",
      "Saved Sets Page",
      actionLabel,
      `Setting autodownload to ${autoDownload} for ${setsToUpdate.length} sets`,
      { setsToUpdate }
    );

    toast.loading(`Updating autodownload for ${setsToUpdate.length} sets...`, {
      id: "bulk-set-autodownload",
      duration: 0, // Keep it open until we manually close it
    });

    for (const [index, set] of setsToUpdate.entries()) {
      toast.loading(`Updating ${index + 1} of ${setsToUpdate.length} - ${set.media_item.title}`, {
        id: "bulk-set-autodownload",
        duration: 0, // Keep it open until we manually close it
      });

      const updatedSet: DBSavedItem = {
        ...set,
        poster_sets: set.poster_sets.map((posterSet) => ({
          ...posterSet,
          auto_download: autoDownload,
        })),
      };

      const response = await UpdateItemInDB(updatedSet);
      if (response.status === "error") {
        toast.error(response.error?.message || `Failed to update ${set.media_item.title}`);
      }
    }

    toast.success(`Autodownload ${autoDownload ? "enabled" : "disabled"} successfully`, {
      id: "bulk-set-autodownload",
      duration: 2000,
    });

    setBulkEditSelectedItems(new Set());
    fetchSavedSets();
  };

  const actionApplyLabelsTags = async (setsToApply: DBSavedItem[]) => {
    log("INFO", "Saved Sets Page", "Apply Labels/Tags", `Applying labels/tags to ${setsToApply.length} sets`, {
      setsToApply,
    });

    // Show loading toast
    toast.loading(`Applying labels/tags to ${setsToApply.length} sets...`, {
      id: "apply-labels-tags",
      duration: 0, // Keep it open until we manually close it
    });

    for (const [index, set] of setsToApply.entries()) {
      toast.loading(`Applying labels/tags to ${index + 1} of ${setsToApply.length} - ${set.media_item.title}`, {
        id: "apply-labels-tags",
        duration: 0, // Keep it open until we manually close it
      });
      await handleApplyLabelsTagsToItem(set);
    }

    // Close loading toast
    toast.success("Labels/Tags applied successfully", {
      id: "apply-labels-tags",
      duration: 2000,
    });
  };

  // actionUpgradeSets replaces the selected sets with a set from a higher ranked favorite creator
  const actionUpgradeSets = async (setsToUpgrade: DBSavedItem[]) => {
    toast.loading(`Checking ${setsToUpgrade.length} ${setsToUpgrade.length === 1 ? "item" : "items"} for upgrades...`, {
      id: "upgrade-sets",
    });
    const response = await UpgradeSavedSets(
      setsToUpgrade.map((set) => ({
        tmdb_id: set.media_item.tmdb_id,
        library_title: set.media_item.library_title,
        edition: set.media_item.edition ?? "",
      }))
    );
    if (response.status === "error" || !response.data) {
      toast.error(response.error?.message || "Could not upgrade the selected sets", { id: "upgrade-sets" });
      return;
    }
    toast.success(upgradeSetsMessage(response.data), { id: "upgrade-sets", duration: 5000 });
  };

  const actionDeleteSelectedSets = async (setsToDelete: DBSavedItem[]) => {
    log("INFO", "Saved Sets Page", "Delete Selected Sets", `Deleting ${setsToDelete.length} sets`, {
      setsToDelete,
    });

    // Show loading toast
    toast.loading(`Deleting ${setsToDelete.length} sets...`, {
      id: "delete-sets",
      duration: 0, // Keep it open until we manually close it
    });

    for (const [index, set] of setsToDelete.entries()) {
      toast.loading(`Deleting ${index + 1} of ${setsToDelete.length} - ${set.media_item.title}`, {
        id: "delete-sets",
        duration: 0, // Keep it open until we manually close it
      });
      await handleDeleteItemFromDB(set);
    }

    // Close loading toast
    toast.success("Sets deleted successfully", {
      id: "delete-sets",
      duration: 2000,
    });
    setBulkEditSelectedItems(new Set());
    fetchSavedSets();
  };

  const handleBulkEditButtonToggleClick = () => {
    setBulkEditSelectedItems(new Set());
    setBulkEditSelectedAction("");
    setBulkEditMode(!bulkEditMode);
  };

  const handleBulkEditButtonRunClick = () => {
    if (!bulkEditSelectedAction) return;
    if (bulkEditSelectedItems.size === 0) return;
    if (!bulkEditMode) return;

    // Get the Selected Keys from the Bulk Edit Selected Items (key: `TMDB ID|||Library Title`)
    const selectedKeys = Array.from(bulkEditSelectedItems);

    // Find matching savedSets for each selected key
    const selectedSavedItems = selectedKeys
      .map((key) => {
        const [tmdbId, libraryTitle] = key.split("|||");
        return savedSets.find(
          (set) => String(set.media_item.tmdb_id) === tmdbId && set.media_item.library_title === libraryTitle
        );
      })
      .filter(Boolean);

    if (selectedSavedItems.length === 0) {
      toast.warning("No matching saved sets found for the selected items", {
        id: "force-recheck",
        duration: 2000,
      });
      return;
    }

    if (bulkEditSelectedAction === "force-recheck") {
      actionForceAutoDownloadRecheck(selectedSavedItems as DBSavedItem[]);
    } else if (bulkEditSelectedAction === "enable-autodownload") {
      actionBulkSetAutodownload(selectedSavedItems as DBSavedItem[], true);
    } else if (bulkEditSelectedAction === "disable-autodownload") {
      actionBulkSetAutodownload(selectedSavedItems as DBSavedItem[], false);
    } else if (bulkEditSelectedAction === "apply-label-tag") {
      actionApplyLabelsTags(selectedSavedItems as DBSavedItem[]);
    } else if (bulkEditSelectedAction === "delete-selected") {
      actionDeleteSelectedSets(selectedSavedItems as DBSavedItem[]);
    } else if (bulkEditSelectedAction === "force-redownload") {
      actionForceRedownload(selectedSavedItems as DBSavedItem[]);
    } else if (bulkEditSelectedAction === "upgrade-sets") {
      actionUpgradeSets(selectedSavedItems as DBSavedItem[]);
    }

    // Set the Selected Action back to empty
    setBulkEditSelectedAction("");
  };

  return (
    <div className="container mx-auto p-4 min-h-screen flex flex-col items-center">
      {refreshing && <RefreshProgress title="Loading the saved sets from the database" />}

      {/* Row 1: Filter Icon (left), Autodownload (right) */}
      {!nothingSavedYet && (
        <div className="w-full flex items-center justify-between mb-2">
          <div>
            <FilterSavedSets
              getSectionSummaries={getSectionSummaries}
              librarySectionsLoaded={librarySectionsLoaded}
              filteredLibraries={filteredLibraries}
              setFilteredLibraries={setFilteredLibraries}
              typeOptions={typeOptions}
              filteredTypes={filteredTypes}
              setFilteredTypes={setFilteredTypes}
              filterAutoDownload={filterAutoDownload}
              setFilterAutoDownload={setFilterAutoDownload}
              filterUserOptions={filterUserOptions}
              filteredUsers={filteredUsers}
              setFilteredUsers={setFilteredUsers}
              filterMultiSetOnly={filterMultiSetOnly}
              setFilterMultiSetOnly={setFilterMultiSetOnly}
              searchTitle={searchTitle}
              searchYear={searchYear}
              searchTMDBID={searchTMDBID}
              searchLibrary={searchLibrary}
              sortOption={sortOption}
              sortOrder={sortOrder}
              setSortOption={setSortOption}
              setSortOrder={setSortOrder}
              setCurrentPage={setCurrentPage}
              itemsPerPage={itemsPerPage}
              setItemsPerPage={setItemsPerPage}
              filterMediaItemOnServer={filterMediaItemOnServer}
              setFilterMediaItemOnServer={setFilterMediaItemOnServer}
            />
          </div>

          {/* Bulk Edit Mode Button, hidden when there are no sets */}
          <div className={cn(!bulkEditMode && (!savedSets || savedSets.length === 0) && "hidden")}>
            <Button
              variant={bulkEditMode ? "destructive" : "secondary"}
              onClick={handleBulkEditButtonToggleClick}
              className={cn(
                "flex items-center gap-1 text-xs sm:text-sm cursor-pointer",
                bulkEditMode
                  ? "border-red-600 bg-red-600/10 hover:bg-red-600/20"
                  : "border border-1 border-yellow-500 hover:bg-yellow-800"
              )}
            >
              {bulkEditMode ? "Cancel Bulk Edit" : "Bulk Edit"}
            </Button>
          </div>
        </div>
      )}

      {/* Row 2: ViewControl (right) */}
      {!nothingSavedYet && (
        <div className="w-full flex items-center justify-end mb-0">
          {isWideScreen && (
            <ViewControl
              options={[
                { value: "card", label: "Card View" },
                { value: "table", label: "Table View" },
              ]}
              viewOption={viewOption}
              setViewOption={setViewOption}
              label="View:"
              showLabel={true}
            />
          )}
        </div>
      )}

      {!nothingSavedYet && <Separator className="mt-0 my-4 w-full" />}

      {Object.keys(recheckStatus).length > 0 && (
        <div className="w-full">
          <div className="flex items-center justify-between mb-2">
            <h3 className="text-lg font-semibold mb-2 ml-1">Recheck Status:</h3>
            <span className="text-xs text-muted-foreground">
              {Object.keys(recheckStatus).length} {makePlural(Object.keys(recheckStatus).length, "item")}
            </span>
            <Badge className="mb-4 bg-blue-100 text-blue-800 px-2 py-1" onClick={() => setRecheckStatus({})}>
              Clear All
            </Badge>
          </div>
          <div className="grid grid-cols-1 gap-2">
            {Object.entries(recheckStatus)
              .sort(([, a], [, b]) => a.item.localeCompare(b.item))
              .map(([title, result]) => (
                <div key={title} className="rounded-md border p-3 flex flex-col gap-2 bg-background">
                  <div className="flex justify-between items-center">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold">{result.item}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      <Badge
                        className={cn("inline-flex items-center rounded-full px-2 py-0.5 text-sm font-medium", {
                          "bg-green-300 text-green-900": result.overall_result === "success",
                          "bg-yellow-300 text-yellow-900": result.overall_result === "warn",
                          "bg-red-300 text-red-900": result.overall_result === "error",
                          "bg-gray-300 text-gray-900": result.overall_result === "skipped",
                        })}
                      >
                        {["success", "warn", "error", "skipped"].includes(result.overall_result)
                          ? result.overall_result
                          : "unknown"}
                      </Badge>
                    </div>
                  </div>
                  <p className="text-sm text-muted-foreground">{result.overall_message}</p>
                  {result.sets &&
                    result.sets.map((set, index) => {
                      const setStatus = ["success", "warn", "error", "skipped"].includes(set.result)
                        ? set.result
                        : "other";

                      return (
                        <div key={`${set.id}-${index}`} className="flex items-start gap-2 text-xs pl-2">
                          <span className="shrink-0 text-muted-foreground mt-1">
                            Set {set.id} (created by {set.user_created}):
                          </span>

                          <Badge
                            className={cn(
                              "shrink-0 rounded-full px-2",
                              setStatus === "success" && "bg-green-500/15 text-green-600",
                              setStatus === "error" && "bg-red-500/15 text-red-600",
                              setStatus === "skipped" && "bg-muted text-muted-foreground",
                              setStatus === "other" && "bg-muted text-muted-foreground"
                            )}
                          >
                            {setStatus}
                          </Badge>

                          <span className="min-w-0 text-muted-foreground break-words mt-1">{set.reason}</span>
                        </div>
                      );
                    })}
                  <div className="flex justify-end gap-2 mt-2">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={async () => {
                        const item = savedSets.find((set) => set.media_item.title === title);
                        if (!item) return;
                        setRecheckStatus((prev) => {
                          const newStatus = { ...prev };
                          delete newStatus[title];
                          return newStatus;
                        });
                        await handleRecheckItem(title, item);
                      }}
                      className="h-6 w-6 text-muted-foreground hover:text-yellow-500"
                    >
                      <RefreshIcon className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => {
                        setRecheckStatus((prev) => {
                          const newStatus = { ...prev };
                          delete newStatus[title];
                          return newStatus;
                        });
                      }}
                      className="h-6 w-6 text-muted-foreground hover:text-red-500"
                    >
                      <XCircle className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              ))}
          </div>
          <Separator className="my-4 w-full" />
        </div>
      )}

      {/* If nothing has been saved yet, explain how to save a set */}
      {nothingSavedYet && (
        <div className="mx-auto mt-10 flex max-w-md flex-col items-center gap-2 text-center">
          <BookmarkIcon className="h-10 w-10 text-muted-foreground" />
          <p className="text-lg font-medium">No saved sets yet</p>
          <p className="text-sm text-muted-foreground">Once you have saved a set, it will appear here.</p>
        </div>
      )}

      {/* If no saved sets match the filters or search */}
      {(!savedSets || savedSets.length === 0) &&
        !loading &&
        !error &&
        !Object.keys(recheckStatus).length &&
        hasActiveFilters && (
          <div className="w-full">
            <ErrorMessage
              error={ReturnErrorMessage<string>(
                [
                  `No ${filterMultiSetOnly ? "Multi-Poster" : ""} Sets found`,
                  filteredLibraries.length > 0 ? `in ${filteredLibraries.map((lib) => `"${lib}"`).join(", ")}` : null,
                  filterAutoDownload === "on"
                    ? "that are set to Auto Download"
                    : filterAutoDownload === "off"
                      ? "that are not set to Auto Download"
                      : null,
                  filteredTypes.length > 0
                    ? `with ${makePlural(filteredTypes.length, "type")} ${filteredTypes
                        .map((t) => `"${typeOptions.find((opt) => opt.value === t)?.label || t}"`)
                        .join(", ")}`
                    : null,
                  filteredUsers.length > 0
                    ? `for ${makePlural(filteredUsers.length, "user")} ${filteredUsers
                        .map((u) => (u === "|||no-user|||" ? '"No User"' : `"${u}"`))
                        .join(", ")}`
                    : null,
                  searchQuery ? `matching "${searchQuery}"` : null,
                ]
                  .filter(Boolean)
                  .join("\n")
              )}
            />
            <div className="text-center text-muted-foreground mt-4">
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setSearchQuery("");
                  setFilteredLibraries([]);
                  if (setFilterAutoDownload) setFilterAutoDownload("");
                  setFilteredUsers([]);
                  setFilteredTypes([]);
                  setFilterMediaItemOnServer("");
                  setCurrentPage(1);
                  if (filterMultiSetOnly) setFilterMultiSetOnly(false);
                }}
                className="text-sm"
              >
                <XCircle className="inline mr-1" />
                Clear All Filters & Search Query
              </Button>
            </div>
          </div>
        )}

      {/* Table View (only available for larger screens) */}
      {viewOption === "table" && savedSets && savedSets.length > 0 && (
        <Table className="border-separate border-spacing-x-0 border-spacing-y-2">
          <TableHeader className="[&_th]:border-b">
            <TableRow>
              <TableHead className="w-[20px]">
                {bulkEditMode && (
                  <Checkbox
                    checked={bulkEditSelectedItems.size === savedSets.length && savedSets.length > 0}
                    onCheckedChange={(checked) => {
                      if (checked) {
                        // Select all
                        const allKeys = new Set(
                          savedSets.map(
                            (set) => `${set.media_item.tmdb_id}|||${set.media_item.library_title}` // Key format
                          )
                        );
                        setBulkEditSelectedItems(allKeys);
                      } else {
                        // Deselect all
                        setBulkEditSelectedItems(new Set());
                      }
                    }}
                    aria-label="Select All Saved Sets"
                    className="cursor-pointer border-1 border-primary"
                  />
                )}
              </TableHead>
              <TableHead className="w-[20px]"></TableHead>
              <TableHead className="w-[70px]"></TableHead>
              <TableHead
                className="w-[300px] group cursor-pointer select-none"
                onClick={() => {
                  if (sortOption === "title") {
                    // Toggle sort order
                    setSortOrder(sortOrder === "asc" ? "desc" : "asc");
                  } else {
                    setSortOption("title");
                    setSortOrder("asc");
                  }
                }}
              >
                <span className="inline-flex items-center gap-1">
                  Title
                  <span className="opacity-0 group-hover:opacity-100 transition-opacity duration-150 flex items-center">
                    {sortOption === "title" ? (
                      sortOrder === "asc" ? (
                        <ArrowDownAZ className="h-4 w-4 ml-1" />
                      ) : (
                        <ArrowDownZA className="h-4 w-4 ml-1" />
                      )
                    ) : (
                      <ArrowDownAZ className="h-4 w-4 ml-1" />
                    )}
                  </span>
                </span>
              </TableHead>
              <TableHead
                className="w-[75px] group cursor-pointer select-none"
                onClick={() => {
                  if (sortOption === "year") {
                    // Toggle sort order
                    setSortOrder(sortOrder === "asc" ? "desc" : "asc");
                  } else {
                    setSortOption("year");
                    setSortOrder("desc");
                  }
                }}
              >
                <span className="inline-flex items-center gap-1">
                  Year
                  <span className="opacity-0 group-hover:opacity-100 transition-opacity duration-150 flex items-center">
                    {sortOption === "year" ? (
                      sortOrder === "asc" ? (
                        <ClockArrowUp className="h-4 w-4 ml-1" />
                      ) : (
                        <ClockArrowDown className="h-4 w-4 ml-1" />
                      )
                    ) : (
                      <ClockArrowDown className="h-4 w-4 ml-1" />
                    )}
                  </span>
                </span>
              </TableHead>
              <TableHead
                className="group cursor-pointer select-none"
                onClick={() => {
                  if (sortOption === "library") {
                    setSortOrder(sortOrder === "asc" ? "desc" : "asc");
                  } else {
                    setSortOption("library");
                    setSortOrder("asc");
                  }
                }}
              >
                <span className="inline-flex items-center gap-1">
                  Library
                  <span className="opacity-0 group-hover:opacity-100 transition-opacity duration-150 flex items-center">
                    {sortOption === "library" ? (
                      sortOrder === "asc" ? (
                        <ArrowDownAZ className="h-4 w-4 ml-1" />
                      ) : (
                        <ArrowDownZA className="h-4 w-4 ml-1" />
                      )
                    ) : (
                      <ArrowDownAZ className="h-4 w-4 ml-1" />
                    )}
                  </span>
                </span>
              </TableHead>
              <TableHead
                className="w-[150px] group cursor-pointer select-none"
                onClick={() => {
                  if (sortOption === "date_downloaded") {
                    setSortOrder(sortOrder === "asc" ? "desc" : "asc");
                  } else {
                    setSortOption("date_downloaded");
                    setSortOrder("desc");
                  }
                }}
              >
                <span className="inline-flex items-center gap-1">
                  Last Downloaded
                  <span className="opacity-0 group-hover:opacity-100 transition-opacity duration-150 flex items-center">
                    {sortOption === "date_downloaded" ? (
                      sortOrder === "asc" ? (
                        <ClockArrowUp className="h-4 w-4 ml-1" />
                      ) : (
                        <ClockArrowDown className="h-4 w-4 ml-1" />
                      )
                    ) : (
                      <ClockArrowDown className="h-4 w-4 ml-1" />
                    )}
                  </span>
                </span>
              </TableHead>
              <TableHead>Sets</TableHead>
              <TableHead>Types</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {savedSets &&
              savedSets.length > 0 &&
              savedSets.map((savedSet) => (
                <SavedSetsTableRow
                  key={`${savedSet.media_item.library_title}-${savedSet.media_item.rating_key}`}
                  savedSet={savedSet}
                  onUpdate={fetchSavedSets}
                  handleRecheckItem={handleRecheckItem}
                  handleRedownloadItem={handleRedownloadItem}
                  bulkEditMode={bulkEditMode}
                  bulkEditSelectedItems={bulkEditSelectedItems}
                  setBulkEditSelectedItems={setBulkEditSelectedItems}
                />
              ))}
          </TableBody>
        </Table>
      )}

      {/* Card View */}
      {viewOption === "card" && (
        <>
          {/* Add a Select All CheckBox for when in Card View  */}
          {bulkEditMode && viewOption === "card" && (
            <div className="flex items-center">
              <Checkbox
                className="mr-2 mb-4"
                checked={savedSets.length > 0 && bulkEditSelectedItems.size === savedSets.length}
                onCheckedChange={(checked) => {
                  if (checked) {
                    // Select all items
                    const allKeys = savedSets.map(
                      (set) => `${set.media_item.tmdb_id}|||${set.media_item.library_title}`
                    );
                    setBulkEditSelectedItems(new Set(allKeys));
                  } else {
                    // Deselect all items
                    setBulkEditSelectedItems(new Set());
                  }
                }}
              />
              <Label className="mb-4 ml-1">Select All on Page</Label>
            </div>
          )}

          <ResponsiveGrid size="larger">
            {savedSets &&
              savedSets.length > 0 &&
              savedSets.map((savedSet) => (
                <SavedSetsCard
                  key={`${savedSet.media_item.library_title}-${savedSet.media_item.rating_key}`}
                  savedSet={savedSet}
                  onUpdate={fetchSavedSets}
                  handleRecheckItem={handleRecheckItem}
                  handleRedownloadItem={handleRedownloadItem}
                  bulkEditMode={bulkEditMode}
                  bulkEditSelectedItems={bulkEditSelectedItems}
                  setBulkEditSelectedItems={setBulkEditSelectedItems}
                />
              ))}
          </ResponsiveGrid>
        </>
      )}

      {/* Bulk Edit Mode Options */}
      {bulkEditMode && (
        <div className="sticky bottom-4 z-101 flex justify-center w-full pointer-events-none mt-2">
          <div className="pointer-events-auto flex items-center gap-2 rounded-full border bg-background/95 backdrop-blur-md shadow-lg px-2 py-1.5">
            <Badge variant="secondary" className="rounded-full px-2.5 py-1 text-xs font-medium tabular-nums shrink-0">
              {bulkEditSelectedItems.size} selected
            </Badge>

            <Separator orientation="vertical" className="h-5" />

            <Select value={bulkEditSelectedAction} onValueChange={(value) => setBulkEditSelectedAction(value)}>
              <SelectTrigger className="h-8 w-[190px] rounded-full border-0 bg-muted/60 text-xs shadow-none focus-visible:ring-1">
                <SelectValue placeholder="Choose an action" />
              </SelectTrigger>
              <SelectContent className="z-102">
                {bulkEditActionGroups.map((group, index) => (
                  <React.Fragment key={group.group}>
                    {index > 0 && <SelectSeparator />}
                    <SelectGroup>
                      <SelectLabel>{group.group}</SelectLabel>
                      {group.actions.map((action) => (
                        <SelectItem key={action.value} value={action.value}>
                          {action.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </React.Fragment>
                ))}
              </SelectContent>
            </Select>

            {bulkEditSelectedAction && bulkEditSelectedAction !== "delete-selected" && (
              <Button
                size="icon"
                className="h-8 w-8 rounded-full shrink-0"
                disabled={bulkEditSelectedItems.size === 0 || !bulkEditSelectedAction}
                onClick={handleBulkEditButtonRunClick}
              >
                {bulkEditSelectedAction === "apply-label-tag" && <Tag className="h-4 w-4" />}
                {bulkEditSelectedAction === "force-recheck" && <RefreshIcon className="h-4 w-4" />}
                {bulkEditSelectedAction === "force-redownload" && <CloudDownload className="h-4 w-4" />}
                {bulkEditSelectedAction === "upgrade-sets" && <ArrowBigUpDash className="h-4 w-4" />}
                {bulkEditSelectedAction === "enable-autodownload" && <CloudCheck className="h-4 w-4" />}
                {bulkEditSelectedAction === "disable-autodownload" && <CloudOff className="h-4 w-4" />}
              </Button>
            )}

            {bulkEditSelectedAction === "delete-selected" && (
              <ConfirmDestructiveDialogActionButton
                onConfirm={async () => {
                  handleBulkEditButtonRunClick();
                }}
                title="Confirm Bulk Delete"
                description={`You are about to delete ${bulkEditSelectedItems.size} ${bulkEditSelectedItems.size === 1 ? "set" : "sets"} from the database. This action cannot be undone.`}
                confirmText={`Yes, delete ${bulkEditSelectedItems.size} ${bulkEditSelectedItems.size === 1 ? "set" : "sets"}`}
                cancelText="Cancel"
                variant="destructive"
                disabled={bulkEditSelectedItems.size === 0}
                className="h-8 w-8 rounded-full shrink-0 p-0 inline-flex items-center justify-center bg-destructive text-white hover:bg-destructive/90"
              >
                <Trash2 className="h-4 w-4" />
              </ConfirmDestructiveDialogActionButton>
            )}
          </div>
        </div>
      )}

      {/* Pagination */}
      {itemsPerPage && !nothingSavedYet && (
        <CustomPagination
          currentPage={currentPage}
          totalPages={totalPages}
          setCurrentPage={setCurrentPage}
          scrollToTop={true}
          filterItemsLength={totalItems}
          itemsPerPage={itemsPerPage}
        />
      )}

      {/* Refresh Button */}
      <RefreshButton onClick={() => void handleRefresh()} refreshing={refreshing} />
    </div>
  );
};

export default SavedSetsPage;
