import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

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

// Clean up legacy IndexedDB entry if previously persisted there
if (typeof window !== "undefined") {
  PageStore.removeItem("Home").catch(() => {});
}

interface Home_PageStore extends SortStore<string, TYPE_SORT_ORDER_OPTIONS>, PaginationStore<number, number> {
  // Session tracking
  sessionModified: boolean;

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
      sessionModified: false,

      sortOption: "dateAdded",
      setSortOption: (option) => set({ sortOption: option, sessionModified: true }),

      sortOrder: "desc",
      setSortOrder: (order) => set({ sortOrder: order, sessionModified: true }),

      currentPage: 1,
      setCurrentPage: (page) => set({ currentPage: page }),

      itemsPerPage: 20,
      setItemsPerPage: (itemsPerPage) => set({ itemsPerPage, sessionModified: true }),

      filteredLibraries: [],
      setFilteredLibraries: (libraries) => set({ filteredLibraries: libraries, sessionModified: true }),

      filterInDB: "",
      setFilterInDB: (filter) => set({ filterInDB: filter, sessionModified: true }),

      filterIgnored: "",
      setFilterIgnored: (ignored) => set({ filterIgnored: ignored, sessionModified: true }),

      hasSetsAvailableFilter: "",
      setHasSetsAvailableFilter: (filter) => set({ hasSetsAvailableFilter: filter, sessionModified: true }),

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
          sessionModified: false,
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
          sessionModified: false,
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
          sessionModified: false,
        }),
    }),
    {
      name: "Home",
      storage: createJSONStorage(() => sessionStorage),
      partialize: (state) => ({
        sessionModified: state.sessionModified,
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
        if (state && !state.sessionModified) {
          state.resetToSavedDefaults();
        }
      },
    }
  )
);

// Sync saved server preferences when user preferences load or change, if this session hasn't been modified
useUserPreferencesStore.subscribe((state, prev) => {
  if (typeof window === "undefined") return;
  const homeStore = useHomePageStore.getState();
  if (!homeStore.sessionModified) {
    if (
      state.defaultSort !== prev.defaultSort ||
      state.defaultSortOrder !== prev.defaultSortOrder ||
      state.defaultItemsPerPage !== prev.defaultItemsPerPage ||
      state.defaultLibraries !== prev.defaultLibraries ||
      state.defaultFilterInDB !== prev.defaultFilterInDB ||
      state.defaultFilterIgnored !== prev.defaultFilterIgnored ||
      state.defaultHasSetsAvailableFilter !== prev.defaultHasSetsAvailableFilter ||
      (state.hasHydrated && !prev.hasHydrated)
    ) {
      homeStore.resetToSavedDefaults();
    }
  }
});
