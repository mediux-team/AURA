import { create } from "zustand";
import { persist } from "zustand/middleware";

import { PageStore } from "@/lib/stores/stores";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { PaginationStore, SortStore } from "@/types/store-interfaces";
import type {
  TYPE_FILTER_IGNORED_OPTIONS,
  TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS,
  TYPE_SORT_ORDER_OPTIONS,
} from "@/types/ui-options";
import type { TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS } from "@/types/ui-options";

interface Home_PageStore extends SortStore<string, TYPE_SORT_ORDER_OPTIONS>, PaginationStore<number, number> {
  // Filters
  filteredLibraries: string[];
  setFilteredLibraries: (libraries: string[]) => void;
  filterInDB: TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS;
  setFilterInDB: (filter: TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS) => void;
  filterIgnored: TYPE_FILTER_IGNORED_OPTIONS;
  setFilterIgnored: (ignored: TYPE_FILTER_IGNORED_OPTIONS) => void;
  hasSetsAvailableFilter: TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS;
  setHasSetsAvailableFilter: (filter: TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS) => void;

  // Adjacent Items
  previousMediaItem: MediaItem | null;
  setPreviousMediaItem: (mediaItem: MediaItem | null) => void;
  nextMediaItem: MediaItem | null;
  setNextMediaItem: (mediaItem: MediaItem | null) => void;

  // Hydrate and Clear
  hasHydrated: boolean;
  hydrate: () => void;
  clear: () => void;
  resetToFactoryDefaults: () => void;
  resetToSavedDefaults: () => void;
}

export const useHomePageStore = create<Home_PageStore>()(
  persist(
    (set) => ({
      sortOption: "dateAdded",
      setSortOption: (option) => set({ sortOption: option }),

      sortOrder: "desc",
      setSortOrder: (order) => set({ sortOrder: order }),

      currentPage: 1,
      setCurrentPage: (page) => set({ currentPage: page }),

      itemsPerPage: 20,
      setItemsPerPage: (itemsPerPage) => set({ itemsPerPage }),

      filteredLibraries: [],
      setFilteredLibraries: (libraries) => set({ filteredLibraries: libraries }),

      filterInDB: "",
      setFilterInDB: (filter) => set({ filterInDB: filter }),

      filterIgnored: "",
      setFilterIgnored: (ignored) => set({ filterIgnored: ignored }),

      hasSetsAvailableFilter: "",
      setHasSetsAvailableFilter: (filter) => set({ hasSetsAvailableFilter: filter }),

      previousMediaItem: null,
      setPreviousMediaItem: (mediaItem) => set({ previousMediaItem: mediaItem }),

      nextMediaItem: null,
      setNextMediaItem: (mediaItem) => set({ nextMediaItem: mediaItem }),

      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),

      resetToFactoryDefaults: () =>
        set({
          sortOption: "dateAdded",
          sortOrder: "desc",
          currentPage: 1,
          itemsPerPage: 20,
          filteredLibraries: [],
          filterInDB: "",
          filterIgnored: "",
          hasSetsAvailableFilter: "",
        }),

      resetToSavedDefaults: () => {
        const prefs = useUserPreferencesStore.getState();
        set({
          sortOption: prefs.defaultSort || "dateAdded",
          sortOrder: (prefs.defaultSortOrder as TYPE_SORT_ORDER_OPTIONS) || "desc",
          currentPage: 1,
          itemsPerPage: prefs.defaultItemsPerPage || 20,
          filteredLibraries: prefs.defaultLibraries || [],
          filterInDB: (prefs.defaultFilterInDB as TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS) || "",
          filterIgnored: (prefs.defaultFilterIgnored as TYPE_FILTER_IGNORED_OPTIONS) || "",
          hasSetsAvailableFilter:
            (prefs.defaultHasSetsAvailableFilter as TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS) || "",
        });
      },

      clear: () =>
        set({
          sortOption: "dateAdded",
          sortOrder: "desc",
          currentPage: 1,
          itemsPerPage: 20,
          filteredLibraries: [],
          filterInDB: "",
          filterIgnored: "",
          hasSetsAvailableFilter: "",
          previousMediaItem: null,
          nextMediaItem: null,
          hasHydrated: false,
        }),
    }),
    {
      name: "Home",
      storage: PageStore,
      partialize: (state) => ({
        sortOption: state.sortOption,
        sortOrder: state.sortOrder,
        currentPage: state.currentPage,
        itemsPerPage: state.itemsPerPage,
        filteredLibraries: state.filteredLibraries,
        filterInDB: state.filterInDB,
        filterIgnored: state.filterIgnored,
        hasSetsAvailableFilter: state.hasSetsAvailableFilter,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);
