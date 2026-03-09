"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetLibrarySectionItems } from "@/services/mediaserver/get-library-section-items";
import { GetLibrarySections } from "@/services/mediaserver/get-library-sections";
import { Loader } from "lucide-react";

import { useCallback, useEffect, useRef, useState } from "react";

import { CustomPagination } from "@/components/shared/custom-pagination";
import { ErrorMessage } from "@/components/shared/error-message";
import { FilterHome } from "@/components/shared/filter-home";
import HomeMediaItemCard from "@/components/shared/media-item-card";
import { HomeMediaItemCardSkeletonGrid } from "@/components/shared/media-item-card-skeleton";
import { RefreshButton } from "@/components/shared/refresh-button";
import { ResponsiveGrid } from "@/components/shared/responsive-grid";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";

import { cn } from "@/lib/cn";
import { log } from "@/lib/logger";
import { MAX_CACHE_DURATION, useLibrarySectionsStore } from "@/lib/stores/global-store-library-sections";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useHomePageStore } from "@/lib/stores/page-store-home";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import { searchItems } from "@/hooks/search-query";

import type { APIResponse } from "@/types/api/api-response";
import type { LibrarySection } from "@/types/media-and-posters/media-item-and-library";

export default function Home() {
  const isMounted = useRef(false);

  // -------------------------------
  // States
  // -------------------------------
  // Search
  const { searchQuery } = useSearchQueryStore();
  const prevSearchQuery = useRef(searchQuery);

  // Loading & Error
  const [error, setError] = useState<APIResponse<unknown> | null>(null);
  const [fullyLoaded, setFullyLoaded] = useState<boolean>(false);

  // Library sections & progress
  const [librarySections, setLibrarySections] = useState<LibrarySection[]>([]);
  const [sectionProgress, setSectionProgress] = useState<{
    [key: string]: { loaded: number; total: number };
  }>({});

  // State to track the HomePageStore values
  const {
    filteredLibraries,
    setFilteredLibraries,
    filterInDB,
    setFilterInDB,
    filterIgnored,
    setFilterIgnored,
    currentPage,
    setCurrentPage,
    itemsPerPage,
    setItemsPerPage,
    sortOption,
    setSortOption,
    sortOrder,
    setSortOrder,
    filteredAndSortedMediaItems,
    setFilteredAndSortedMediaItems,
  } = useHomePageStore();

  const { sections, setSections, timestamp } = useLibrarySectionsStore();
  const hasHydrated = useLibrarySectionsStore((state) => state.hasHydrated);
  const enableSortByNewEpisode = useUserPreferencesStore((state) => state.enableSortByNewEpisode);
  const prefsHasHydrated = useUserPreferencesStore((state) => state.hasHydrated);

  // -------------------------------
  // Derived values
  // -------------------------------
  const paginatedItems = filteredAndSortedMediaItems.slice(
    (currentPage - 1) * itemsPerPage,
    currentPage * itemsPerPage
  );
  const totalPages = Math.ceil(filteredAndSortedMediaItems.length / itemsPerPage);

  // Set sortOption to "dateAdded" if its not title or dateUpdated or dateAdded or dateReleased or newEpisodeAdded
  useEffect(() => {
    if (
      sortOption !== "title" &&
      sortOption !== "dateUpdated" &&
      sortOption !== "dateAdded" &&
      sortOption !== "dateReleased" &&
      sortOption !== "newEpisodeAdded"
    ) {
      setSortOption("dateAdded");
      setSortOrder("desc");
    }
    // If newEpisodeAdded is selected but the feature is disabled, fall back to dateAdded
    if (sortOption === "newEpisodeAdded" && !enableSortByNewEpisode) {
      setSortOption("dateAdded");
      setSortOrder("desc");
    }
  }, [sortOption, setSortOption, setSortOrder, enableSortByNewEpisode]);

  // Fetch data from cache or API
  const getMediaItems = useCallback(
    async (useCache: boolean) => {
      if (isMounted.current && useCache) return;
      setSectionProgress({});
      setLibrarySections([]);
      setError(null);
      setFullyLoaded(false);
      try {
        // Check if we want to use cache
        if (useCache) {
          const isCacheAgeValid = timestamp ? Date.now() - timestamp < MAX_CACHE_DURATION : false;
          const cacheContainsSectionsAndTimestamp = sections && timestamp && Object.keys(sections).length > 0;
          log("INFO", "Home Page", "Library Cache", "Attempting to load sections from cache", {
            "Current Time": Date.now(),
            "Cache Timestamp": timestamp,
            "Cache Age Max (ms)": MAX_CACHE_DURATION,
            "Cache Age (ms)": timestamp ? Date.now() - timestamp : "N/A",
            "Is Cache Age Valid": isCacheAgeValid,
            "Cache Contains Sections & Timestamp": cacheContainsSectionsAndTimestamp,
          });
          if (cacheContainsSectionsAndTimestamp) {
            if (isCacheAgeValid) {
              setLibrarySections(Object.values(sections));
              setFullyLoaded(true);
              log("INFO", "Home Page", "Library Cache", "Using cached sections", sections);
              return;
            } else {
              log("WARN", "Home Page", "Library Cache", "Cache expired, fetching fresh data");
            }
          } else {
            log("WARN", "Home Page", "Library Cache", "No valid cache found, fetching fresh data");
          }
        }

        // Fetch fresh data
        const response = await GetLibrarySections();
        if (response.status === "error") {
          setError(response);
          setFullyLoaded(true);
          return;
        }

        const fetchedSections = response.data?.sections || [];
        if (!fetchedSections || fetchedSections.length === 0) {
          setError(ReturnErrorMessage<unknown>(new Error("No sections found, please check the logs.")));
          return;
        }

        // Initialize each section's MediaItems to an empty array
        fetchedSections.forEach((section) => (section.media_items = []));
        setLibrarySections(fetchedSections.slice().sort((a, b) => a.title.localeCompare(b.title)));

        // Process each section concurrently
        await Promise.all(
          fetchedSections.map(async (section) => {
            let itemsFetched = 0;
            let totalSize = Infinity;
            let allItems: LibrarySection["media_items"] = [];

            while (itemsFetched < totalSize) {
              const itemsResponse = await GetLibrarySectionItems(section, itemsFetched, enableSortByNewEpisode);
              if (itemsResponse.status === "error") {
                setError(itemsResponse);
                return;
              }

              const data = itemsResponse.data?.library_section;
              const items = data?.media_items || [];

              allItems = allItems.concat(items);
              if (totalSize === Infinity) {
                totalSize = data?.total_size ?? 0;
              }
              itemsFetched += items.length;
              setSectionProgress((prev) => ({
                ...prev,
                [section.id]: {
                  loaded: itemsFetched,
                  total: totalSize,
                },
              }));
              if (items.length === 0) {
                break;
              }
            }
            section.media_items = allItems;
            section.total_size = totalSize;
          })
        );

        // Build the sections object for the store
        const sectionsObj = fetchedSections.reduce<Record<string, LibrarySection>>((acc, section) => {
          acc[section.title] = section;
          return acc;
        }, {});
        const librarySections = fetchedSections.slice().sort((a, b) => a.title.localeCompare(b.title));
        // Store in zustand and update timestamp
        setSections(sectionsObj, Date.now());
        setFullyLoaded(true);
        log("INFO", "Home Page", "", "Sections fetched successfully from server", {
          "Library Sections": librarySections,
          Sections: sectionsObj,
        });
        setLibrarySections(librarySections);
      } catch (error) {
        setError(ReturnErrorMessage<unknown>(error));
      } finally {
        isMounted.current = false;
      }
    },
    [sections, setSections, timestamp, enableSortByNewEpisode]
  );

  useEffect(() => {
    if (!hasHydrated || !prefsHasHydrated) return;
    getMediaItems(true);
    isMounted.current = true;
  }, [getMediaItems, hasHydrated, prefsHasHydrated]);

  useEffect(() => {
    if (searchQuery !== prevSearchQuery.current) {
      setCurrentPage(1);
      prevSearchQuery.current = searchQuery;
    }
  }, [searchQuery, setCurrentPage]);

  // Filter items based on the search query
  useEffect(() => {
    const filterAndSortItems = async () => {
      let items = librarySections.flatMap((section) => section.media_items || []);

      // Sort items by Title
      if (sortOption === "title") {
        if (sortOrder === "asc") {
          items.sort((a, b) => a.title.localeCompare(b.title));
        } else if (sortOrder === "desc") {
          items.sort((a, b) => b.title.localeCompare(a.title));
        }
      } else if (sortOption === "dateUpdated") {
        if (sortOrder === "asc") {
          items.sort((a, b) => (a.updated_at ?? 0) - (b.updated_at ?? 0));
        } else if (sortOrder === "desc") {
          items.sort((a, b) => (b.updated_at ?? 0) - (a.updated_at ?? 0));
        }
      } else if (sortOption === "dateAdded") {
        if (sortOrder === "asc") {
          items.sort((a, b) => (a.added_at ?? 0) - (b.added_at ?? 0));
        } else if (sortOrder === "desc") {
          items.sort((a, b) => (b.added_at ?? 0) - (a.added_at ?? 0));
        }
      } else if (sortOption === "dateReleased") {
        if (sortOrder === "asc") {
          items.sort((a, b) => (a.released_at ?? 0) - (b.released_at ?? 0));
        } else if (sortOrder === "desc") {
          items.sort((a, b) => (b.released_at ?? 0) - (a.released_at ?? 0));
        }
      } else if (sortOption === "newEpisodeAdded") {
        // Sort shows by the most recently added episode; movies fall back to their own added_at.
        const getLatestEpisodeAdded = (item: (typeof items)[number]) =>
          item.latest_episode_added_at ?? item.added_at ?? 0;
        if (sortOrder === "asc") {
          items.sort((a, b) => getLatestEpisodeAdded(a) - getLatestEpisodeAdded(b));
        } else {
          items.sort((a, b) => getLatestEpisodeAdded(b) - getLatestEpisodeAdded(a));
        }
      }

      // Filter by selected libraries
      if (filteredLibraries.length > 0) {
        items = items.filter((item) => filteredLibraries.includes(item.library_title));
      }

      // Filter out items already in the DB
      if (filterInDB === "notInDB") {
        items = items.filter((item) => !item.db_saved_sets || item.db_saved_sets.length === 0);
      } else if (filterInDB === "inDB") {
        items = items.filter((item) => item.db_saved_sets && item.db_saved_sets.length > 0);
      }

      // Filter out items that are ignored
      if (filterIgnored === "always") {
        items = items.filter((item) => item.ignored_in_db && item.ignored_mode === "always");
      } else if (filterIgnored === "temp") {
        items = items.filter((item) => item.ignored_in_db && item.ignored_mode === "temp");
      } else if (filterIgnored === "ignored") {
        items = items.filter((item) => item.ignored_in_db);
      } else if (filterIgnored === "not_ignored") {
        items = items.filter((item) => !item.ignored_in_db);
      }

      // Filter out items by search
      const filteredItems = searchItems(items, searchQuery, {
        getTitle: (item) => item.title,
        getYear: (item) => item.year,
        getLibraryTitle: (item) => item.library_title,
        getID: (item) => item.tmdb_id || item.rating_key,
      });

      // Store the filtered items in local storage
      setFilteredAndSortedMediaItems(filteredItems);
    };
    filterAndSortItems();
  }, [
    librarySections,
    filteredLibraries,
    setFilteredAndSortedMediaItems,
    searchQuery,
    filterInDB,
    filterIgnored,
    sortOption,
    sortOrder,
  ]);

  if (error) {
    return <ErrorMessage error={error} />;
  }

  const hasUpdatedAt = paginatedItems.some(
    (item) => item.updated_at !== undefined && item.updated_at !== null && item.updated_at > 0
  );
  const hasEpisodeAddedAt = paginatedItems.some(
    (item) =>
      item.latest_episode_added_at !== undefined &&
      item.latest_episode_added_at !== null &&
      item.latest_episode_added_at > 0
  );

  return (
    <div className="flex items-center justify-center">
      {!fullyLoaded && librarySections.length > 0 ? (
        <div className="min-h-screen pb-4 px-4 sm:px-10 w-full">
          {/* Progress bars */}
          <div className="flex flex-col items-center w-full px-4">
            {[...librarySections]
              .sort((a, b) => {
                const progressA = sectionProgress[a.id];
                const percentA =
                  progressA && progressA.total > 0 ? Math.min((progressA.loaded / progressA.total) * 100, 100) : 0;
                const progressB = sectionProgress[b.id];
                const percentB =
                  progressB && progressB.total > 0 ? Math.min((progressB.loaded / progressB.total) * 100, 100) : 0;
                return percentB - percentA; // Sort descending
              })
              .map((section) => {
                const progressInfo = sectionProgress[section.id];
                const percentage =
                  progressInfo && progressInfo.total > 0
                    ? Math.min((progressInfo.loaded / progressInfo.total) * 100, 100)
                    : 0;

                return (
                  <div key={section.id} className="mb-6 w-full max-w-xl flex flex-col items-center px-2">
                    <Label className="text-lg font-semibold text-center mb-2">Loading {section.title}</Label>
                    <Progress
                      value={percentage}
                      className={cn(
                        "w-full max-w-lg h-2 rounded-md overflow-hidden",
                        percentage < 100 && "animate-pulse",
                        percentage >= 0 && percentage < 20 && "[&>div]:bg-yellow-100",
                        percentage >= 20 && percentage < 40 && "[&>div]:bg-yellow-300",
                        percentage >= 40 && percentage < 60 && "[&>div]:bg-green-200",
                        percentage >= 60 && percentage < 80 && "[&>div]:bg-green-300",
                        percentage >= 80 && percentage < 100 && "[&>div]:bg-green-400",
                        percentage === 100 && "[&>div]:bg-green-500"
                      )}
                    />

                    {percentage < 100 && <Loader className="animate-spin mt-2" />}
                    <span className="mt-2 text-base text-muted-foreground font-medium">
                      {Math.round(percentage)}%
                      {typeof progressInfo?.total === "number" && progressInfo.total > 0
                        ? ` - ${progressInfo.loaded} / ${progressInfo.total} items`
                        : ""}
                    </span>
                  </div>
                );
              })}
          </div>
          <HomeMediaItemCardSkeletonGrid />
        </div>
      ) : (
        <div className="min-h-screen pb-4 px-4 sm:px-10 w-full">
          {/* Filter & Sort Controls */}
          <div className="w-full flex items-center justify-center mb-4 mt-4">
            <FilterHome
              librarySections={librarySections}
              filteredLibraries={filteredLibraries}
              setFilteredLibraries={setFilteredLibraries}
              filterInDB={filterInDB}
              setFilterInDB={setFilterInDB}
              filterIgnored={filterIgnored}
              setFilterIgnored={setFilterIgnored}
              hasUpdatedAt={hasUpdatedAt}
              hasEpisodeAddedAt={hasEpisodeAddedAt && enableSortByNewEpisode}
              sortOption={sortOption}
              setSortOption={setSortOption}
              sortOrder={sortOrder}
              setSortOrder={setSortOrder}
              setCurrentPage={setCurrentPage}
              itemsPerPage={itemsPerPage}
              setItemsPerPage={setItemsPerPage}
            />
          </div>

          {/* Grid of Cards */}
          <ResponsiveGrid size="regular">
            {paginatedItems.length === 0 && fullyLoaded && (searchQuery || filteredLibraries.length > 0) ? (
              <div className="col-span-full text-center text-red-500">
                <ErrorMessage
                  error={ReturnErrorMessage<string>(
                    `No items found${searchQuery ? ` matching "${searchQuery}"` : ""} in ${
                      filteredLibraries.length > 0 ? filteredLibraries.join(", ") : "any library"
                    }${
                      filterInDB === "notInDB"
                        ? " that are not in the database."
                        : filterInDB === "inDB"
                          ? " that are already in the database."
                          : ""
                    }`
                  )}
                />
              </div>
            ) : (
              paginatedItems.map((item) => <HomeMediaItemCard key={item.rating_key} item={item} />)
            )}
          </ResponsiveGrid>

          {/* Pagination */}
          <CustomPagination
            currentPage={currentPage}
            totalPages={totalPages}
            setCurrentPage={setCurrentPage}
            scrollToTop={true}
            filterItemsLength={filteredAndSortedMediaItems.length}
            itemsPerPage={itemsPerPage}
          />
          {/* Refresh Button */}
          <RefreshButton onClick={() => getMediaItems(false)} />
        </div>
      )}
    </div>
  );
}
