"use client";

import {
  ArrowDownAZ,
  ArrowDownZA,
  BookmarkCheck,
  Check,
  ClockArrowDown,
  ClockArrowUp,
  Filter,
  RotateCcw,
  SortDescIcon,
  Undo2,
} from "lucide-react";

import { useMemo, useState } from "react";

import { SelectItemsPerPage } from "@/components/shared/select-items-per-page";
import { SortControl } from "@/components/shared/select-sort";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { ToggleGroup } from "@/components/ui/toggle-group";

import { cn } from "@/lib/cn";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";
import { useHomePageStore } from "@/lib/stores/page-store-home";

import { extractInfoFromSearchQuery } from "@/hooks/search-query";

import type { LibrarySection } from "@/types/media-and-posters/media-item-and-library";
import type {
  TYPE_FILTER_IGNORED_OPTIONS,
  TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS,
  TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS,
} from "@/types/ui-options";
import { FILTER_IGNORED_OPTIONS, HOME_PAGE_FILTER_IN_DB_OPTIONS } from "@/types/ui-options";

type HomeFilterProps = {
  // Filtering
  librarySections: LibrarySection[];
  filteredLibraries: string[];
  setFilteredLibraries: (libs: string[]) => void;
  filterInDB: TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS;
  setFilterInDB: (filter: TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS) => void;
  filterIgnored: TYPE_FILTER_IGNORED_OPTIONS;
  setFilterIgnored: (ignored: TYPE_FILTER_IGNORED_OPTIONS) => void;
  hasSetsAvailableFilter: TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS;
  setHasSetsAvailableFilter: (filter: TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS) => void;

  // Sorting
  hasUpdatedAt: boolean;
  hasEpisodeAddedAt: boolean;
  sortOption: string;
  setSortOption: (option: string) => void;
  sortOrder: "asc" | "desc";
  setSortOrder: (order: "asc" | "desc") => void;

  // Items Per Page
  setCurrentPage: (page: number) => void;
  itemsPerPage: number;
  setItemsPerPage: (num: number) => void;
};

function FilterHomeContent({
  librarySections,
  filteredLibraries,
  setFilteredLibraries,
  filterInDB,
  setFilterInDB,
  filterIgnored,
  setFilterIgnored,
  hasSetsAvailableFilter,
  setHasSetsAvailableFilter,

  hasUpdatedAt,
  hasEpisodeAddedAt,
  sortOption,
  setSortOption,
  sortOrder,
  setSortOrder,

  setCurrentPage,
  itemsPerPage,
  setItemsPerPage,
}: HomeFilterProps) {
  const { searchQuery, setSearchQuery } = useSearchQueryStore();
  const { searchTMDBID, searchLibrary, searchYear, searchTitle } = extractInfoFromSearchQuery(searchQuery);

  return (
    <div className="flex-grow space-y-4 overflow-y-auto px-4">
      {/* Sort Header */}
      <div className="flex items-center justify-center mb-0 mt-0">
        <Label className="text-lg font-semibold">Sort</Label>
      </div>
      <Separator className="my-2 w-full" />
      {/* Sort Control */}
      <SortControl
        options={[
          {
            value: "dateAdded",
            label: "Date Added",
            ascIcon: <ClockArrowUp />,
            descIcon: <ClockArrowDown />,
            type: "date",
          },
          ...(hasUpdatedAt
            ? [
                {
                  value: "dateUpdated",
                  label: "Date Updated",
                  ascIcon: <ClockArrowUp />,
                  descIcon: <ClockArrowDown />,
                  type: "date" as const,
                },
              ]
            : []),
          ...(hasEpisodeAddedAt
            ? [
                {
                  value: "newEpisodeAdded",
                  label: "New Episode Added",
                  ascIcon: <ClockArrowUp />,
                  descIcon: <ClockArrowDown />,
                  type: "date" as const,
                },
              ]
            : []),
          {
            value: "dateReleased",
            label: "Date Released",
            ascIcon: <ClockArrowUp />,
            descIcon: <ClockArrowDown />,
            type: "date",
          },
          {
            value: "title",
            label: "Title",
            ascIcon: <ArrowDownAZ />,
            descIcon: <ArrowDownZA />,
            type: "string",
          },
        ]}
        sortOption={sortOption}
        sortOrder={sortOrder}
        setSortOption={(value) => {
          setSortOption(value as "title" | "dateUpdated" | "dateAdded" | "dateReleased" | "newEpisodeAdded");
          if (value === "title") setSortOrder("asc");
          else if (value === "dateUpdated") setSortOrder("desc");
          else if (value === "dateAdded") setSortOrder("desc");
          else if (value === "dateReleased") setSortOrder("desc");
          else if (value === "newEpisodeAdded") setSortOrder("desc");
        }}
        setSortOrder={setSortOrder}
      />
      {/* Items Per Page Selection */}
      <div className="flex items-center mb-4">
        <SelectItemsPerPage
          setCurrentPage={setCurrentPage}
          itemsPerPage={itemsPerPage}
          setItemsPerPage={setItemsPerPage}
        />
      </div>

      <Separator className="my-2 w-full" />
      <Separator className="my-1 w-full" />

      {/* Filters Header */}
      <div className="flex items-center justify-center mb-0">
        <Label className="text-lg font-semibold">Filters</Label>
      </div>
      <Separator className="my-2 w-full" />

      {/* Search Info */}
      {(searchTitle || searchYear || searchTMDBID || searchLibrary) && (
        <div className="p-2 bg-secondary rounded-md">
          <Label className="text-md font-semibold mb-1 block">Current Search</Label>
          <div className="flex flex-col gap-1">
            {searchTitle && (
              <div className="text-sm">
                <span className="font-semibold">Search:</span> {searchTitle}
              </div>
            )}
            {typeof searchYear === "number" && searchYear > 0 && (
              <div className="text-sm">
                <span className="font-semibold">Year:</span> {searchYear}
              </div>
            )}
            {searchTMDBID && (
              <div className="text-sm">
                <span className="font-semibold">ID:</span> {searchTMDBID}
              </div>
            )}
            {searchLibrary && (
              <div className="text-sm">
                <span className="font-semibold">Library:</span> {searchLibrary}
              </div>
            )}
          </div>
          <Button
            variant={"destructive"}
            className="mt-2"
            onClick={() => {
              setSearchQuery("");
            }}
          >
            Clear Search
          </Button>
        </div>
      )}
      {/* Library Sections Filter */}
      <div className="flex flex-col">
        <>
          <Label className="text-md font-semibold mb-1">Library Sections</Label>
          <ToggleGroup
            type="multiple"
            className="flex flex-wrap gap-2 ml-2"
            value={filteredLibraries}
            onValueChange={setFilteredLibraries}
          >
            {librarySections.map((section) => (
              <Badge
                key={section.title}
                className="cursor-pointer text-sm active:scale-95 hover:brightness-120"
                variant={filteredLibraries.includes(section.title) ? "default" : "outline"}
                onClick={() => {
                  if (filteredLibraries.includes(section.title)) {
                    setFilteredLibraries(filteredLibraries.filter((lib) => lib !== section.title));
                  } else {
                    setFilteredLibraries([...filteredLibraries, section.title]);
                  }
                  setCurrentPage(1);
                }}
              >
                {section.title}
              </Badge>
            ))}
          </ToggleGroup>
          <Separator className="my-4 w-full" />
        </>

        {/* In-Database Filter */}
        <div className="flex flex-col">
          <Label className="text-md font-semibold mb-1">In-Database Filter</Label>
          <ToggleGroup
            type="single"
            className="flex flex-wrap gap-2 ml-2"
            value={filterInDB}
            onValueChange={(value) => {
              if (value) {
                setFilterInDB(value as TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS);
              }
            }}
          >
            {HOME_PAGE_FILTER_IN_DB_OPTIONS.map((option) => (
              <Badge
                key={option.value}
                className={cn(
                  "cursor-pointer text-sm active:scale-95 hover:brightness-120",
                  filterInDB === option.value && option.value === "inDB" && "bg-green-500 text-primary-foreground",
                  filterInDB === option.value && option.value === "notInDB" && "bg-red-500 text-primary-foreground"
                )}
                variant={filterInDB === option.value ? "default" : "outline"}
                onClick={() => {
                  if (filterInDB === option.value) {
                    setFilterInDB("");
                  } else {
                    setFilterInDB(option.value as TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS);
                    if (option.value === "inDB") {
                      if (
                        filterIgnored === "ignored" ||
                        filterIgnored === "always" ||
                        filterIgnored === "until-set-available" ||
                        filterIgnored === "until-new-set-available"
                      ) {
                        setFilterIgnored("");
                      }
                    }
                  }
                  setCurrentPage(1);
                }}
              >
                {option.label}
              </Badge>
            ))}
          </ToggleGroup>
          <Separator className="my-4 w-full" />
        </div>

        {/* Ignored Filter */}
        <div className="flex flex-col">
          <Label className="text-md font-semibold mb-1">Ignored Filter</Label>
          <ToggleGroup
            type="single"
            className="flex flex-wrap gap-2 ml-2"
            value={filterIgnored}
            onValueChange={(value) => {
              if (value) {
                setFilterIgnored(value as TYPE_FILTER_IGNORED_OPTIONS);
              }
            }}
          >
            {FILTER_IGNORED_OPTIONS.map((option) => (
              <Badge
                key={option.value}
                className={cn(
                  "cursor-pointer text-sm active:scale-95 hover:brightness-120",
                  filterIgnored === option.value && option.value === "always" && "bg-red-500 text-primary-foreground",
                  filterIgnored === option.value &&
                    option.value === "until-set-available" &&
                    "bg-orange-500 text-primary-foreground",
                  filterIgnored === option.value &&
                    option.value === "until-new-set-available" &&
                    "bg-yellow-500 text-primary-foreground",
                  filterIgnored === option.value &&
                    option.value === "ignored" &&
                    "bg-orange-500 text-primary-foreground",
                  filterIgnored === option.value &&
                    option.value === "not_ignored" &&
                    "bg-green-500 text-primary-foreground"
                )}
                variant={filterIgnored === option.value ? "default" : "outline"}
                onClick={() => {
                  if (filterIgnored === option.value) {
                    setFilterIgnored("");
                  } else {
                    setFilterIgnored(option.value as TYPE_FILTER_IGNORED_OPTIONS);
                    if (
                      option.value === "ignored" ||
                      option.value === "always" ||
                      option.value === "until-set-available" ||
                      option.value === "until-new-set-available"
                    ) {
                      if (filterInDB === "inDB") {
                        setFilterInDB("");
                      }
                    }
                  }
                  setCurrentPage(1);
                }}
              >
                {option.label}
              </Badge>
            ))}
          </ToggleGroup>

          <Separator className="my-4 w-full" />

          {/* Has Sets Available Filter */}
          <Label className="text-md font-semibold mb-1">Has Sets Available Filter</Label>
          <ToggleGroup
            type="single"
            className="flex flex-wrap gap-2 ml-2"
            value={hasSetsAvailableFilter}
            onValueChange={(value) => {
              if (value) {
                setHasSetsAvailableFilter(value as TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS);
              }
            }}
          >
            {[
              { value: "hasSetsAvailable", label: "Has Sets Available" },
              { value: "noSetsAvailable", label: "No Sets Available" },
            ].map((option) => (
              <Badge
                key={option.value}
                className={cn(
                  "cursor-pointer text-sm active:scale-95 hover:brightness-120",
                  hasSetsAvailableFilter === option.value &&
                    option.value === "hasSetsAvailable" &&
                    "bg-green-500 text-primary-foreground",
                  hasSetsAvailableFilter === option.value &&
                    option.value === "noSetsAvailable" &&
                    "bg-red-500 text-primary-foreground"
                )}
                variant={hasSetsAvailableFilter === option.value ? "default" : "outline"}
                onClick={() => {
                  if (hasSetsAvailableFilter === option.value) {
                    setHasSetsAvailableFilter("");
                  } else {
                    setHasSetsAvailableFilter(option.value as TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS);
                  }
                  setCurrentPage(1);
                }}
              >
                {option.label}
              </Badge>
            ))}
          </ToggleGroup>
        </div>
      </div>
    </div>
  );
}

export function FilterHome({
  librarySections,
  filteredLibraries,
  setFilteredLibraries,
  filterInDB,
  setFilterInDB,
  filterIgnored,
  setFilterIgnored,
  hasSetsAvailableFilter,
  setHasSetsAvailableFilter,

  hasUpdatedAt,
  hasEpisodeAddedAt,
  sortOption,
  setSortOption,
  sortOrder,
  setSortOrder,

  setCurrentPage,
  itemsPerPage,
  setItemsPerPage,
}: HomeFilterProps) {
  // State - Open/Close Modal
  const [modalOpen, setModalOpen] = useState(false);
  const [savedDefault, setSavedDefault] = useState(false);
  const [resetDefaultsFeedback, setResetDefaultsFeedback] = useState(false);

  // Calculate number of active filters
  const numberOfActiveFilters = useMemo(() => {
    let count = 0;
    if (filteredLibraries.length > 0) count++;
    if (filterInDB !== "") count++;
    if (filterIgnored !== "") count++;
    if (hasSetsAvailableFilter !== "") count++;
    return count;
  }, [filteredLibraries.length, filterInDB, filterIgnored, hasSetsAvailableFilter]);

  const handleClearSession = () => {
    useHomePageStore.getState().resetToSavedDefaults();
    const prefs = useUserPreferencesStore.getState();
    const defaultSort = prefs.defaultSort || "dateAdded";
    const defaultSortOrder = (prefs.defaultSortOrder as "asc" | "desc") || "desc";
    const defaultItemsPerPage = prefs.defaultItemsPerPage || 20;
    const defaultLibraries = prefs.defaultLibraries || [];
    const defaultFilterInDB = (prefs.defaultFilterInDB as TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS) || "";
    const defaultFilterIgnored = (prefs.defaultFilterIgnored as TYPE_FILTER_IGNORED_OPTIONS) || "";
    const defaultHasSetsAvailableFilter =
      (prefs.defaultHasSetsAvailableFilter as TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS) || "";

    setSortOption(defaultSort);
    setSortOrder(defaultSortOrder);
    setItemsPerPage(defaultItemsPerPage);
    setFilteredLibraries(defaultLibraries);
    setFilterInDB(defaultFilterInDB);
    setFilterIgnored(defaultFilterIgnored);
    setHasSetsAvailableFilter(defaultHasSetsAvailableFilter);
    setCurrentPage(1);
  };

  const handleResetDefaults = () => {
    useUserPreferencesStore.getState().resetHomeDefaults();
    useHomePageStore.getState().resetToFactoryDefaults();
    setSortOption("dateAdded");
    setSortOrder("desc");
    setItemsPerPage(20);
    setFilteredLibraries([]);
    setFilterInDB("");
    setFilterIgnored("");
    setHasSetsAvailableFilter("");
    setCurrentPage(1);
    setResetDefaultsFeedback(true);
    setTimeout(() => {
      setResetDefaultsFeedback(false);
    }, 2000);
  };

  const handleSetAsDefault = () => {
    useUserPreferencesStore.getState().setHomeDefaults({
      sortOption,
      sortOrder,
      itemsPerPage,
      filteredLibraries,
      filterInDB,
      filterIgnored,
      hasSetsAvailableFilter,
    });
    useHomePageStore.setState({ sessionModified: false });
    setSavedDefault(true);
    setTimeout(() => {
      setSavedDefault(false);
    }, 2000);
  };

  return (
    <Dialog open={modalOpen} onOpenChange={setModalOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" className={cn(numberOfActiveFilters > 0 && "ring-1 ring-primary ring-offset-1")}>
          <SortDescIcon className="h-5 w-5" />
          Sort & Filter {numberOfActiveFilters > 0 && `(${numberOfActiveFilters})`}
          <Filter className="h-5 w-5" />
        </Button>
      </DialogTrigger>
      <DialogContent
        className={cn("z-50", "max-h-[80vh] overflow-y-auto", "sm:max-w-[700px]", "border border-primary")}
      >
        <DialogHeader>
          <DialogTitle>Sort & Filter</DialogTitle>
          <DialogDescription>Use the options below to sort and filter your media items.</DialogDescription>
        </DialogHeader>
        <Separator className="my-1 w-full" />
        <FilterHomeContent
          librarySections={librarySections}
          filteredLibraries={filteredLibraries}
          setFilteredLibraries={setFilteredLibraries}
          filterInDB={filterInDB}
          setFilterInDB={setFilterInDB}
          filterIgnored={filterIgnored}
          setFilterIgnored={setFilterIgnored}
          hasSetsAvailableFilter={hasSetsAvailableFilter}
          setHasSetsAvailableFilter={setHasSetsAvailableFilter}
          hasUpdatedAt={hasUpdatedAt}
          hasEpisodeAddedAt={hasEpisodeAddedAt}
          sortOption={sortOption}
          setSortOption={setSortOption}
          sortOrder={sortOrder}
          setSortOrder={setSortOrder}
          setCurrentPage={setCurrentPage}
          itemsPerPage={itemsPerPage}
          setItemsPerPage={setItemsPerPage}
        />
        <Separator className="my-1 w-full" />
        <DialogFooter className="flex flex-col sm:flex-row items-center justify-between gap-2 px-4 pt-2">
          <div className="w-full sm:w-auto flex justify-start">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleClearSession}
              className="text-muted-foreground hover:text-foreground cursor-pointer"
              title="Temporarily clear all active filters and sort options for this session"
            >
              <RotateCcw className="h-4 w-4 mr-1.5" />
              Clear Session
            </Button>
          </div>
          <div className="w-full sm:w-auto flex flex-row items-center justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleResetDefaults}
              className="text-muted-foreground hover:text-foreground cursor-pointer"
              title="Reset your saved server defaults back to original factory defaults"
            >
              {resetDefaultsFeedback ? (
                <>
                  <Check className="h-4 w-4 mr-1.5 text-green-400" />
                  Defaults Reset!
                </>
              ) : (
                <>
                  <Undo2 className="h-4 w-4 mr-1.5" />
                  Reset Defaults
                </>
              )}
            </Button>
            <Button
              type="button"
              variant="default"
              size="sm"
              onClick={handleSetAsDefault}
              className="cursor-pointer"
              title="Save current filters and sort options as your default"
            >
              {savedDefault ? (
                <>
                  <Check className="h-4 w-4 mr-1.5 text-green-400" />
                  Default Saved!
                </>
              ) : (
                <>
                  <BookmarkCheck className="h-4 w-4 mr-1.5" />
                  Set as Default
                </>
              )}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

