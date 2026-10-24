"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetMovieCollections } from "@/services/mediaserver/get-movie-collections";

import { useCallback, useEffect, useRef, useState } from "react";

import { CustomPagination } from "@/components/shared/custom-pagination";
import { ErrorMessage } from "@/components/shared/error-message";
import { FilterCollections } from "@/components/shared/filter-collections";
import Loader from "@/components/shared/loader";
import HomeMediaItemCard from "@/components/shared/media-item-card";
import { RefreshButton } from "@/components/shared/refresh-button";
import { RefreshProgress } from "@/components/shared/refresh-progress";
import { ResponsiveGrid } from "@/components/shared/responsive-grid";

import { log } from "@/lib/logger";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useCollectionsPageStore } from "@/lib/stores/page-store-collections";

import { searchItems } from "@/hooks/search-query";
import { useRefreshing } from "@/hooks/use-refreshing";

import type { APIResponse } from "@/types/api/api-response";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";

export interface CollectionItem {
  rating_key: string;
  index: string;
  title: string;
  summary?: string;
  child_count: number;
  media_items: MediaItem[];
  library_title?: string;
}

export default function CollectionsPage() {
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

  const [collectionItems, setCollectionItems] = useState<CollectionItem[]>([]);

  // State to track the CollectionsPageStore values
  const {
    filteredLibraries,
    setFilteredLibraries,
    currentPage,
    setCurrentPage,
    itemsPerPage,
    setItemsPerPage,
    sortOption,
    setSortOption,
    sortOrder,
    setSortOrder,
    filteredAndSortedCollectionItems,
    setFilteredAndSortedCollectionItems,
  } = useCollectionsPageStore();

  // -------------------------------
  // Derived values
  // -------------------------------
  const paginatedItems = filteredAndSortedCollectionItems.slice(
    (currentPage - 1) * itemsPerPage,
    currentPage * itemsPerPage
  );
  const totalPages = Math.ceil(filteredAndSortedCollectionItems.length / itemsPerPage);

  // Set sortOption to "title" if its not title, numberOfItems
  useEffect(() => {
    if (sortOption !== "title" && sortOption !== "numberOfItems") {
      setSortOption("title");
      setSortOrder("desc");
    }
  }, [sortOption, setSortOption, setSortOrder]);

  // Get the collections, keeping the old ones showing during a refresh
  const fetchCollections = useCallback(async (keepShowing: boolean) => {
    if (isMounted.current && !keepShowing) return;
    setError(null);
    if (!keepShowing) {
      setFullyLoaded(false);
      setCollectionItems([]);
    }
    try {
      const response = await GetMovieCollections();
      if (response.status === "error") {
        setError(response);
        setFullyLoaded(true);
        return;
      }

      const fetchedCollectionItems = response.data?.collections || [];
      if (fetchedCollectionItems.length === 0) {
        setError(ReturnErrorMessage("No Collection Items found in Media Server"));
        return;
      }

      log("INFO", "Collections Page", "Fetched Collection Items:", `Fetched ${fetchedCollectionItems.length} items`, {
        fetchedCollectionItems,
      });
      setCollectionItems(fetchedCollectionItems);
      setFullyLoaded(true);
    } catch (error) {
      setError(ReturnErrorMessage<unknown>(error));
    } finally {
      isMounted.current = false;
    }
  }, []);

  useEffect(() => {
    fetchCollections(false);
    isMounted.current = true;
  }, [fetchCollections]);

  const [refreshing, handleRefresh] = useRefreshing(() => fetchCollections(true));

  useEffect(() => {
    if (searchQuery !== prevSearchQuery.current) {
      setCurrentPage(1);
      prevSearchQuery.current = searchQuery;
    }
  }, [searchQuery, setCurrentPage]);

  // Filter Items
  useEffect(() => {
    const filterAndSortItems = async () => {
      let items = [...collectionItems];

      // Filter out items with no ChildCount
      items = items.filter((item) => item.child_count > 0);

      // Sort items by Title
      if (sortOption === "title") {
        if (sortOrder === "asc") {
          items.sort((a, b) => a.title.localeCompare(b.title));
        } else if (sortOrder === "desc") {
          items.sort((a, b) => b.title.localeCompare(a.title));
        }
      } else if (sortOption === "numberOfItems") {
        if (sortOrder === "asc") {
          items.sort((a, b) => a.child_count - b.child_count);
        } else if (sortOrder === "desc") {
          items.sort((a, b) => b.child_count - a.child_count);
        }
      }

      // Filter by Libraries
      if (filteredLibraries.length > 0) {
        items = items.filter((item) => item.library_title && filteredLibraries.includes(item.library_title));
      }

      // Filter out items by search
      const filteredItems = searchItems(items, searchQuery, {
        getTitle: (item) => item.title,
        getLibraryTitle: (item) => item.library_title,
        getID: (item) => item.rating_key,
      });

      // Store the filtered and sorted items in local storage
      setFilteredAndSortedCollectionItems(filteredItems);
    };

    filterAndSortItems();
  }, [collectionItems, filteredLibraries, sortOption, sortOrder, setFilteredAndSortedCollectionItems, searchQuery]);

  if (error) {
    return <ErrorMessage error={error} />;
  }

  return (
    <div className="flex items-center justify-center">
      {fullyLoaded ? (
        <div className="min-h-screen pb-4 px-4 sm:px-10 w-full">
          {refreshing && <RefreshProgress title="Loading the collections from the media server" className="mt-4" />}

          {/* Filter & Sort Controls */}
          <div className="w-full flex items-center justify-center mb-4 mt-4">
            <FilterCollections
              librarySections={[...new Set(collectionItems.map((item) => item.library_title || "Unknown"))]}
              filteredLibraries={filteredLibraries}
              setFilteredLibraries={setFilteredLibraries}
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
                    `No collection items found${searchQuery ? ` matching "${searchQuery}"` : ""} in ${
                      filteredLibraries.length > 0 ? filteredLibraries.join(", ") : "any library"
                    }`
                  )}
                />
              </div>
            ) : (
              paginatedItems.map((item, index) => (
                <HomeMediaItemCard key={`${item.library_title}-${item.rating_key}`} item={item} priority={index < 6} />
              ))
            )}
          </ResponsiveGrid>

          {/* Pagination */}
          <CustomPagination
            currentPage={currentPage}
            totalPages={totalPages}
            setCurrentPage={setCurrentPage}
            scrollToTop={true}
            filterItemsLength={filteredAndSortedCollectionItems.length}
            itemsPerPage={itemsPerPage}
          />

          {/* Refresh Button */}
          <RefreshButton onClick={() => void handleRefresh()} refreshing={refreshing} />
        </div>
      ) : (
        <Loader className="mt-20" message="Loading Collection Items" />
      )}
    </div>
  );
}
