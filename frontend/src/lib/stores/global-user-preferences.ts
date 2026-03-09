import { create } from "zustand";
import { persist } from "zustand/middleware";

import { GlobalStore } from "@/lib/stores/stores";

import { DOWNLOAD_IMAGE_TYPE_OPTIONS, type TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS } from "@/types/ui-options";

interface UserPreferencesStore {
  downloadDefaults: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[];
  setDownloadDefaults: (downloadDefaults: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[]) => void;

  showOnlyDownloadDefaults: boolean;
  setShowOnlyDownloadDefaults: (showOnlyDownloadDefaults: boolean) => void;

  showDateModified: boolean;
  setShowDateModified: (showDateModified: boolean) => void;

  enableSortByNewEpisode: boolean;
  setEnableSortByNewEpisode: (enableSortByNewEpisode: boolean) => void;

  hasHydrated: boolean;
  hydrate: () => void;
  clear: () => void;
}

export const useUserPreferencesStore = create<UserPreferencesStore>()(
  persist(
    (set) => ({
      downloadDefaults: DOWNLOAD_IMAGE_TYPE_OPTIONS.map((option) => option.value),
      setDownloadDefaults: (downloadDefaults: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[]) => set({ downloadDefaults }),

      showOnlyDownloadDefaults: false,
      setShowOnlyDownloadDefaults: (showOnlyDownloadDefaults: boolean) => set({ showOnlyDownloadDefaults }),

      showDateModified: false,
      setShowDateModified: (showDateModified: boolean) => set({ showDateModified }),

      enableSortByNewEpisode: true,
      setEnableSortByNewEpisode: (enableSortByNewEpisode: boolean) => set({ enableSortByNewEpisode }),

      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),

      clear: () =>
        set({
          downloadDefaults: DOWNLOAD_IMAGE_TYPE_OPTIONS.map((option) => option.value),
          showOnlyDownloadDefaults: false,
          showDateModified: false,
          enableSortByNewEpisode: true,
        }),
    }),
    {
      name: "UserPreferences",
      storage: GlobalStore,
      partialize: (state) => ({
        downloadDefaults: state.downloadDefaults,
        showOnlyDownloadDefaults: state.showOnlyDownloadDefaults,
        showDateModified: state.showDateModified,
        enableSortByNewEpisode: state.enableSortByNewEpisode,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);
