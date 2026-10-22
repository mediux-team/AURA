import { UpdateAppConfigSection } from "@/services/config/update-section";
import { create } from "zustand";
import { persist } from "zustand/middleware";

import { log } from "@/lib/logger";
import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";
import { GlobalStore } from "@/lib/stores/stores";

import type { AppConfigUserPreferences, PageView } from "@/types/config/config";
import { DOWNLOAD_IMAGE_TYPE_OPTIONS, type TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS } from "@/types/ui-options";

// User Preferences are saved in the aura config, so they're the same on every device.
// They are also kept locally, so pages don't show the defaults while loading.

interface UserPreferences {
  downloadDefaults: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[];
  showOnlyDownloadDefaults: boolean;
  showDateModified: boolean;
  defaultAddToQueue: boolean;
  defaultAutoAddNewCollectionItems: boolean;
  defaultAddToDBOnly: boolean;
  autoDownloadNewItems: boolean;
  autoUpgradeSets: boolean;
  favoriteCreators: string[]; // MediUX usernames ranked best first
  pageViews: Record<string, PageView>; // Each page's sort and filter options, by page
}

// The pages whose sort and filter options are saved, the same as PageViewPages in aura
export type PageViewPage = "home" | "saved_sets" | "collections" | "logs" | "media_item" | "download_queue";

export type UserPreferencesSaveStatus = "idle" | "saving" | "saved" | "error";

interface UserPreferencesStore extends UserPreferences {
  // Whether the last change was saved to the server, shown in Settings
  saveStatus: UserPreferencesSaveStatus;
  saveError?: string;
  // Save the current preferences again after a failed save
  retrySave: () => void;
  setDownloadDefaults: (downloadDefaults: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[]) => void;
  setShowOnlyDownloadDefaults: (showOnlyDownloadDefaults: boolean) => void;
  setShowDateModified: (showDateModified: boolean) => void;
  setDefaultAddToQueue: (defaultAddToQueue: boolean) => void;
  setDefaultAutoAddNewCollectionItems: (defaultAutoAddNewCollectionItems: boolean) => void;
  setDefaultAddToDBOnly: (defaultAddToDBOnly: boolean) => void;
  setAutoDownloadNewItems: (autoDownloadNewItems: boolean) => void;
  setAutoUpgradeSets: (autoUpgradeSets: boolean) => void;
  setFavoriteCreators: (favoriteCreators: string[]) => void;
  setPageView: (page: PageViewPage, view: PageView) => void;
  // Apply the preferences from the aura config, without saving them again
  applyServerPreferences: (preferences: UserPreferences) => void;
  hasHydrated: boolean;
  hydrate: () => void;
  clear: () => void;
}

const defaultPreferences = (): UserPreferences => ({
  downloadDefaults: DOWNLOAD_IMAGE_TYPE_OPTIONS.map((option) => option.value),
  showOnlyDownloadDefaults: false,
  showDateModified: false,
  defaultAddToQueue: false,
defaultAutoAddNewCollectionItems: false,
  defaultAddToDBOnly: false,
  autoDownloadNewItems: false,
  autoUpgradeSets: false,
  favoriteCreators: [],
  pageViews: {},
});

const toServerPreferences = (preferences: UserPreferences): AppConfigUserPreferences => ({
  download_defaults: preferences.downloadDefaults,
  show_only_download_defaults: preferences.showOnlyDownloadDefaults,
  show_date_modified: preferences.showDateModified,
  default_add_to_queue: preferences.defaultAddToQueue,
default_auto_add_new_collection_items: preferences.defaultAutoAddNewCollectionItems,
  default_add_to_db_only: preferences.defaultAddToDBOnly,
  auto_download_new_items: preferences.autoDownloadNewItems,
  auto_upgrade_sets: preferences.autoUpgradeSets,
  favorite_creators: preferences.favoriteCreators,
  page_views: preferences.pageViews,
});

const fromServerPreferences = (preferences: AppConfigUserPreferences): UserPreferences => ({
  downloadDefaults: preferences.download_defaults ?? [],
  showOnlyDownloadDefaults: preferences.show_only_download_defaults ?? false,
  showDateModified: preferences.show_date_modified ?? false,
  defaultAddToQueue: preferences.default_add_to_queue ?? false,
defaultAutoAddNewCollectionItems: preferences.default_auto_add_new_collection_items ?? false,
  defaultAddToDBOnly: preferences.default_add_to_db_only ?? false,
  autoDownloadNewItems: preferences.auto_download_new_items ?? false,
  autoUpgradeSets: preferences.auto_upgrade_sets ?? false,
  favoriteCreators: preferences.favorite_creators ?? [],
  pageViews: preferences.page_views ?? {},
});

const preferencesEqual = (a: UserPreferences, b: UserPreferences) =>
  JSON.stringify(toServerPreferences(a)) === JSON.stringify(toServerPreferences(b));

// Save shortly after the last change, so quick changes are saved together
const SAVE_DELAY_MS = 400;
const SAVED_MESSAGE_MS = 2500;
let saveTimer: ReturnType<typeof setTimeout> | null = null;
let savedMessageTimer: ReturnType<typeof setTimeout> | null = null;
let savesInProgress = 0;

const queueServerSave = () => {
  if (saveTimer) clearTimeout(saveTimer);
  if (savedMessageTimer) clearTimeout(savedMessageTimer);
  useUserPreferencesStore.setState({ saveStatus: "saving", saveError: undefined });
  saveTimer = setTimeout(async () => {
    saveTimer = null;
    savesInProgress++;
    const response = await UpdateAppConfigSection(
      "user_preferences",
      toServerPreferences(useUserPreferencesStore.getState()) as unknown as Record<string, unknown>
    );
    savesInProgress--;
    // A newer change is waiting to be saved
    if (saveTimer) return;
    if (response.status === "error") {
      log("ERROR", "User Preferences", "Save", `Failed to save user preferences: ${response.error?.message}`);
      useUserPreferencesStore.setState({
        saveStatus: "error",
        saveError: response.error?.message || "Failed to save",
      });
      return;
    }
    useUserPreferencesStore.setState({ saveStatus: "saved" });
    savedMessageTimer = setTimeout(() => {
      if (useUserPreferencesStore.getState().saveStatus === "saved") {
        useUserPreferencesStore.setState({ saveStatus: "idle" });
      }
    }, SAVED_MESSAGE_MS);
    // Keep the config status in sync
    if (response.data?.status) {
      useOnboardingStore.getState().setStatus(response.data.status);
    }
  }, SAVE_DELAY_MS);
};

export const useUserPreferencesStore = create<UserPreferencesStore>()(
  persist(
    (set) => ({
      ...defaultPreferences(),
      setDownloadDefaults: (downloadDefaults: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS[]) => {
        set({ downloadDefaults });
        queueServerSave();
      },
      setShowOnlyDownloadDefaults: (showOnlyDownloadDefaults: boolean) => {
        set({ showOnlyDownloadDefaults });
        queueServerSave();
      },
      setShowDateModified: (showDateModified: boolean) => {
        set({ showDateModified });
        queueServerSave();
      },
      setDefaultAddToQueue: (defaultAddToQueue: boolean) => {
        set({ defaultAddToQueue });
        queueServerSave();
      },
setDefaultAutoAddNewCollectionItems: (defaultAutoAddNewCollectionItems: boolean) => {
        set({ defaultAutoAddNewCollectionItems });
        queueServerSave();
      },
      setDefaultAddToDBOnly: (defaultAddToDBOnly: boolean) => {
        set({ defaultAddToDBOnly });
        queueServerSave();
      },
      setAutoDownloadNewItems: (autoDownloadNewItems: boolean) => {
        set({ autoDownloadNewItems });
        queueServerSave();
      },
      setAutoUpgradeSets: (autoUpgradeSets: boolean) => {
        set({ autoUpgradeSets });
        queueServerSave();
      },
      setFavoriteCreators: (favoriteCreators: string[]) => {
        set({ favoriteCreators });
        queueServerSave();
      },
      setPageView: (page, view) => {
        set((state) => ({ pageViews: { ...state.pageViews, [page]: view } }));
        queueServerSave();
      },
      applyServerPreferences: (preferences: UserPreferences) => set(preferences),
      saveStatus: "idle",
      saveError: undefined,
      retrySave: () => queueServerSave(),
      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),
      // Clears the local copy only. The saved preferences are applied on the next load
      clear: () => set(defaultPreferences()),
    }),
    {
      name: "UserPreferences",
      storage: GlobalStore,
      partialize: (state) => ({
        downloadDefaults: state.downloadDefaults,
        showOnlyDownloadDefaults: state.showOnlyDownloadDefaults,
        showDateModified: state.showDateModified,
        defaultAddToQueue: state.defaultAddToQueue,
defaultAutoAddNewCollectionItems: state.defaultAutoAddNewCollectionItems,
        defaultAddToDBOnly: state.defaultAddToDBOnly,
        autoDownloadNewItems: state.autoDownloadNewItems,
        autoUpgradeSets: state.autoUpgradeSets,
        favoriteCreators: state.favoriteCreators,
        pageViews: state.pageViews,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);

// Preferences used to be saved on each device only.
// The first time a device syncs, upload its preferences if the config still has the defaults.
const SYNCED_KEY = "UserPreferencesSynced";

const syncFromServer = () => {
  if (typeof window === "undefined") return;
  const store = useUserPreferencesStore.getState();
  const serverPreferences = useOnboardingStore.getState().status?.current_setup?.user_preferences;
  if (!store.hasHydrated || !serverPreferences) return;

  // Don't replace a change that is still being saved
  if (saveTimer || savesInProgress > 0) return;

  const server = fromServerPreferences(serverPreferences);
  const local: UserPreferences = {
    downloadDefaults: store.downloadDefaults,
    showOnlyDownloadDefaults: store.showOnlyDownloadDefaults,
    showDateModified: store.showDateModified,
    defaultAddToQueue: store.defaultAddToQueue,
defaultAutoAddNewCollectionItems: store.defaultAutoAddNewCollectionItems,
    defaultAddToDBOnly: store.defaultAddToDBOnly,
    autoDownloadNewItems: store.autoDownloadNewItems ?? false,
    autoUpgradeSets: store.autoUpgradeSets ?? false,
    favoriteCreators: store.favoriteCreators ?? [],
    pageViews: store.pageViews ?? {},
  };

  let synced = false;
  try {
    synced = localStorage.getItem(SYNCED_KEY) === "true";
    localStorage.setItem(SYNCED_KEY, "true");
  } catch {
    // Without localStorage, use the config's preferences
    synced = true;
  }

  if (!synced && preferencesEqual(server, defaultPreferences()) && !preferencesEqual(local, defaultPreferences())) {
    log("INFO", "User Preferences", "Sync", "Uploading this device's preferences to the aura config");
    queueServerSave();
    return;
  }

  if (!preferencesEqual(server, local)) {
    store.applyServerPreferences(server);
  }
};

// Sync when the config status changes, and once the local preferences have loaded
useOnboardingStore.subscribe((state, previous) => {
  if (state.status?.current_setup?.user_preferences !== previous.status?.current_setup?.user_preferences) {
    syncFromServer();
  }
});
useUserPreferencesStore.subscribe((state, previous) => {
  if (state.hasHydrated && !previous.hasHydrated) {
    syncFromServer();
  }
});
syncFromServer();
