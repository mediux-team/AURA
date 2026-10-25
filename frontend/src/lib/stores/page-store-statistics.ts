import { create } from "zustand";
import { persist } from "zustand/middleware";

import { syncPageView } from "@/lib/stores/page-view-sync";
import { PageStore } from "@/lib/stores/stores";

// Days the graphs go back when none are typed
export const STATISTICS_DEFAULT_DAYS = 30;

interface Statistics_PageStore {
  // How many days back the graphs go
  days: number;
  setDays: (days: number) => void;

  // Show the seasons and episodes, not only movies and shows
  showSeasonsEpisodes: boolean;
  setShowSeasonsEpisodes: (show: boolean) => void;

  // Hydration and Clear
  hasHydrated: boolean;
  hydrate: () => void;
  clear: () => void;
}

export const useStatisticsPageStore = create<Statistics_PageStore>()(
  persist(
    (set) => ({
      days: STATISTICS_DEFAULT_DAYS,
      setDays: (days) => set({ days }),

      showSeasonsEpisodes: false,
      setShowSeasonsEpisodes: (showSeasonsEpisodes) => set({ showSeasonsEpisodes }),

      // Hydration and Clear
      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),

      clear: () => set({ days: STATISTICS_DEFAULT_DAYS, showSeasonsEpisodes: false, hasHydrated: false }),
    }),
    {
      name: "Statistics",
      storage: PageStore,
      partialize: (state) => ({ days: state.days, showSeasonsEpisodes: state.showSeasonsEpisodes }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);

// The options are the same on every device
syncPageView("statistics", useStatisticsPageStore, ["days", "showSeasonsEpisodes"]);
