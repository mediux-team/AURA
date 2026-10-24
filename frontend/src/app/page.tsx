"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import {
  GetLibraryItems,
  type LibraryItemsQuery,
  type LibraryLoad,
  type LibraryProgress,
  RefreshLibrary,
  homeLibraryQuery,
  subscribeToLibraryProgress,
} from "@/services/mediaserver/library-items";

import { useEffect, useMemo, useRef, useState } from "react";

import { CustomPagination } from "@/components/shared/custom-pagination";
import { ErrorMessage } from "@/components/shared/error-message";
import { FilterHome } from "@/components/shared/filter-home";
import { LoadTimingBadge } from "@/components/shared/load-timing-badge";
import HomeMediaItemCard from "@/components/shared/media-item-card";
import { HomeMediaItemCardSkeletonGrid } from "@/components/shared/media-item-card-skeleton";
import { RefreshButton } from "@/components/shared/refresh-button";
import { RefreshProgress } from "@/components/shared/refresh-progress";
import { ResponsiveGrid } from "@/components/shared/responsive-grid";

import { cn } from "@/lib/cn";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useHomePageStore } from "@/lib/stores/page-store-home";

import { useFirstPageOnSearch, usePagedLoad } from "@/hooks/use-paged-load";

import type { LibrarySection, MediaItem } from "@/types/media-and-posters/media-item-and-library";

// The Home page, filtered, searched, sorted and paged by aura
export default function Home() {
  const { searchQuery } = useSearchQueryStore();
  const {
    filteredLibraries,
    setFilteredLibraries,
    filterInDB,
    setFilterInDB,
    filterIgnored,
    setFilterIgnored,
    hasSetsAvailableFilter,
    setHasSetsAvailableFilter,
    currentPage,
    setCurrentPage,
    itemsPerPage,
    setItemsPerPage,
    sortOption,
    setSortOption,
    sortOrder,
    setSortOrder,
  } = useHomePageStore();
  const hasHydrated = useHomePageStore((state) => state.hasHydrated);

  const { shown, loading, error, setError, timing, load } = usePagedLoad(
    GetLibraryItems,
    "Could not load the library",
    setCurrentPage
  );
  const data = shown?.data ?? null;
  const [progress, setProgress] = useState<LibraryProgress | null>(null);
  const refreshing = !!progress?.running;

  const query = useMemo<LibraryItemsQuery>(
    () =>
      homeLibraryQuery(
        {
          filteredLibraries,
          filterInDB,
          filterIgnored,
          hasSetsAvailableFilter,
          sortOption,
          sortOrder,
          currentPage,
          itemsPerPage,
        },
        searchQuery
      ),
    [
      filteredLibraries,
      searchQuery,
      filterInDB,
      filterIgnored,
      hasSetsAvailableFilter,
      sortOption,
      sortOrder,
      currentPage,
      itemsPerPage,
    ]
  );

  useEffect(() => {
    if (!hasHydrated) return;
    void load(query);
  }, [hasHydrated, query, load]);

  useFirstPageOnSearch(searchQuery, setCurrentPage);

  // Show the new items when a refresh finishes, including one started somewhere else
  const queryRef = useRef(query);
  queryRef.current = query;
  useEffect(() => {
    let wasRunning = false;
    return subscribeToLibraryProgress((next) => {
      setProgress(next);
      if (wasRunning && !next.running) {
        if (next.failed) {
          setError(ReturnErrorMessage<unknown>("Could not load the libraries from the media server, check the logs"));
        }
        void load(queryRef.current);
      }
      wasRunning = next.running;
    });
  }, [load, setError]);

  // Reload the libraries from the media server, the progress comes from the stream
  const handleRefresh = async () => {
    setError(null);
    const response = await RefreshLibrary();
    if (response.status === "error") {
      setError(response);
      return;
    }
    if (response.data) setProgress(response.data);
  };

  // Refresh progress by items, each library counts the same until all sizes are known
  const refreshProgress = useMemo(() => {
    if (!progress?.running) return null;
    const libraries = progress.libraries;
    const loaded = (l: LibraryLoad) => (l.done ? l.total : Math.min(l.loaded, l.total));
    const totalItems = libraries.reduce((sum, l) => sum + l.total, 0);
    const sizesKnown = totalItems > 0 && libraries.every((l) => l.done || l.total > 0);
    const percent = sizesKnown
      ? (libraries.reduce((sum, l) => sum + loaded(l), 0) / totalItems) * 100
      : libraries.length > 0
        ? (libraries.reduce((sum, l) => sum + (l.done ? 1 : l.total > 0 ? loaded(l) / l.total : 0), 0) /
            libraries.length) *
          100
        : 0;
    const current = libraries.find((l) => !l.done);
    const done = libraries.filter((l) => l.done).length;
    return {
      percent,
      label: current
        ? current.total > 0
          ? `Loading ${current.title} · ${current.loaded.toLocaleString()} of ${current.total.toLocaleString()} items`
          : `Loading ${current.title}`
        : undefined,
      shortLabel: current
        ? current.total > 0
          ? `${current.title} · ${current.loaded.toLocaleString()}/${current.total.toLocaleString()}`
          : current.title
        : undefined,
      detail: `${done} of ${libraries.length} ${libraries.length === 1 ? "library" : "libraries"} loaded`,
    };
  }, [progress]);

  // The filter only needs each library's title and type
  const librarySections = useMemo(
    () =>
      (data?.libraries ?? []).map(
        (library) =>
          ({
            id: library.title,
            title: library.title,
            type: library.type,
            total_size: library.count,
            media_items: [],
          }) as LibrarySection
      ),
    [data?.libraries]
  );

  if (error && !data) {
    return <ErrorMessage error={error} />;
  }

  return (
    <div className="flex items-center justify-center">
      <div className="min-h-screen pb-4 px-4 sm:px-10 w-full">
        {/* Refresh Progress */}
        {refreshProgress && (
          <RefreshProgress
            title="Loading the libraries from the media server"
            percent={refreshProgress.percent}
            label={refreshProgress.label}
            shortLabel={refreshProgress.shortLabel}
            detail={refreshProgress.detail}
            className="mt-4"
          />
        )}
        {error && data && <ErrorMessage error={error} />}

        {/* Filter & Sort Controls */}
        <div className="w-full flex flex-col items-center justify-center gap-2 mb-4 mt-4">
          <FilterHome
            librarySections={librarySections}
            filteredLibraries={filteredLibraries}
            setFilteredLibraries={setFilteredLibraries}
            filterInDB={filterInDB}
            setFilterInDB={setFilterInDB}
            filterIgnored={filterIgnored}
            setFilterIgnored={setFilterIgnored}
            hasSetsAvailableFilter={hasSetsAvailableFilter}
            setHasSetsAvailableFilter={setHasSetsAvailableFilter}
            hasUpdatedAt={data?.has_updated_at ?? false}
            hasEpisodeAddedAt={data?.has_episode_added_at ?? false}
            sortOption={sortOption}
            setSortOption={setSortOption}
            sortOrder={sortOrder}
            setSortOrder={setSortOrder}
            setCurrentPage={setCurrentPage}
            itemsPerPage={itemsPerPage}
            setItemsPerPage={setItemsPerPage}
          />
          <LoadTimingBadge
            label="Home"
            startedAt={timing.startedAt}
            finishedAt={timing.finishedAt}
            urlMatch="/api/mediaserver/library/items"
          />
        </div>

        {!data ? (
          <HomeMediaItemCardSkeletonGrid />
        ) : (
          <>
            {/* Grid of Cards */}
            <ResponsiveGrid size="regular" className={cn(loading && "opacity-60 transition-opacity")}>
              {data.items.length === 0 ? (
                <div className="col-span-full text-center text-red-500">
                  <ErrorMessage
                    error={ReturnErrorMessage<string>(
                      `No items found${searchQuery ? ` matching "${searchQuery}"` : ""} in ${
                        filteredLibraries.length > 0 ? filteredLibraries.join(", ") : "any library"
                      }`
                    )}
                  />
                </div>
              ) : (
                data.items.map((item, index) => (
                  <HomeMediaItemCard
                    key={`${item.library_title}-${item.rating_key}`}
                    item={item as MediaItem}
                    priority={index < 6}
                  />
                ))
              )}
            </ResponsiveGrid>

            {/* Pagination */}
            <CustomPagination
              currentPage={data.page}
              totalPages={data.total_pages}
              setCurrentPage={setCurrentPage}
              scrollToTop={true}
              filterItemsLength={data.total}
              itemsPerPage={data.per_page}
            />
          </>
        )}

        {/* Refresh Button */}
        <RefreshButton onClick={() => void handleRefresh()} refreshing={refreshing} />
      </div>
    </div>
  );
}
