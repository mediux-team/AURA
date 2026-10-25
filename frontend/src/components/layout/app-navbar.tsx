"use client";

import { Logout } from "@/services/auth/logout";
import { subscribeToDownloadQueue } from "@/services/downloads/queue-stream";
import {
  ArrowLeftCircle,
  ArrowRightCircle,
  Bookmark as BookmarkIcon,
  ChartColumn,
  Clock,
  FileCog as FileCogIcon,
  LayoutGrid,
  ListOrdered,
  LogOutIcon,
  Logs,
  MenuIcon,
  Sparkles,
} from "lucide-react";

import { useEffect, useRef, useState } from "react";

import Image from "next/image";
import { usePathname, useRouter } from "next/navigation";

import { DynamicSearch } from "@/components/layout/app-search-bar";
import { ViewDensitySlider } from "@/components/shared/view-density-context";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

import { cn } from "@/lib/cn";
import { DeepResetAllStores } from "@/lib/stores/clear-all-stores";
import { useCollectionStore } from "@/lib/stores/global-store-collection-store";
import { useFailedDownloadsStore } from "@/lib/stores/global-store-failed-downloads";
import { useMediaStore } from "@/lib/stores/global-store-media-store";
import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useCollectionsPageStore } from "@/lib/stores/page-store-collections";
import { useHomePageStore } from "@/lib/stores/page-store-home";

import { useAppVersion } from "@/hooks/app-version";

type AppNavbarProps = {
  version?: string;
};

export function Navbar({ version = "dev" }: AppNavbarProps) {
  // Router
  const router = useRouter();

  // Pathname
  const pathName = usePathname();
  // Page Logic
  const isHomePage = pathName === "/";
  const isMediaPage = pathName.startsWith("/media-item") || pathName.startsWith("/media-item/");
  const isOnboardingPage = pathName === "/onboarding" || pathName === "/onboarding/";
  const isLoginPage = pathName === "/login" || pathName === "/login/";
  const isLogsPage = pathName === "/logs" || pathName === "/logs/";
  const isChangeLogPage = pathName === "/change-log" || pathName === "/change-log/";
  const isCollectionItemPage = pathName.startsWith("/collection-item") || pathName.startsWith("/collection-item/");
  const isAppLoadingPage = pathName === "/app-loading" || pathName === "/app-loading/";

  // Auth State
  const [isAuthed, setIsAuthed] = useState(false);

  // Logo state
  const [logoSrc, setLogoSrc] = useState("/aura_word_logo.svg");

  // Search States
  const { setSearchQuery } = useSearchQueryStore(); // Global store for search query

  // Home Page Store
  const { setCurrentPage, setFilteredLibraries, setFilterInDB, setFilterIgnored, setHasSetsAvailableFilter } =
    useHomePageStore();
  const nextMediaItem = useHomePageStore((state) => state.nextMediaItem);
  const previousMediaItem = useHomePageStore((state) => state.previousMediaItem);

  // Collection Item Page Store
  const nextCollectionItem = useCollectionsPageStore((state) => state.nextCollectionItem);
  const previousCollectionItem = useCollectionsPageStore((state) => state.previousCollectionItem);

  // Onboarding Store
  const { fetchStatus } = useOnboardingStore();
  const status = useOnboardingStore((state) => state.status);
  const hasHydrated = useOnboardingStore((state) => state.hasHydrated);

  // Prevent repeated redirects while loading
  const hasRedirectedToLoadingRef = useRef(false);

  // Check if the screen is mobile
  const [isWideScreen, setIsWideScreen] = useState(false);

  // App Version Hook
  const { latestVersion, isNewerVersion } = useAppVersion(version);

  // Some media servers do not support collections

  // Fetch onboarding/status on mount, and again whenever the route changes
  useEffect(() => {
    void fetchStatus();
  }, [fetchStatus, pathName]);

  // App Not Fully Loaded Redirect Logic
  useEffect(() => {
    if (!hasHydrated || !status) return;

    if (status.app_fully_loaded === false && status.needs_setup === false) {
      if (!isAppLoadingPage) {
        hasRedirectedToLoadingRef.current = true;
        router.replace("/app-loading");
      }
      return;
    }

    // Reset redirect lock once app is ready
    hasRedirectedToLoadingRef.current = false;
  }, [hasHydrated, status, isAppLoadingPage, router]);

  // Onboarding Redirect Logic
  useEffect(() => {
    if (!hasHydrated || !status) return;

    // Skip onboarding redirects while app is still loading
    if (status.app_fully_loaded === false && status.needs_setup === false) {
      if (!isAppLoadingPage && !hasRedirectedToLoadingRef.current) {
        router.replace("/app-loading");
      }
      return;
    }

    // If needs setup and not on onboarding page, redirect to onboarding
    if (status.needs_setup) {
      if (!isOnboardingPage && !isLogsPage && !isChangeLogPage) {
        router.replace("/onboarding");
      }
    } else {
      // If does not need setup and on onboarding page, redirect to home
      if (isOnboardingPage) {
        router.replace("/");
      }
    }
  }, [status, pathName, router, hasHydrated, isOnboardingPage, isLogsPage, isChangeLogPage, isAppLoadingPage]);

  // Change isWideScreen on window resize
  useEffect(() => {
    const handleResize = () => {
      setIsWideScreen(window.innerWidth >= 950);
    };
    handleResize();
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  useEffect(() => {
    // Update the Logo based on the screen size
    setLogoSrc(isWideScreen ? "/aura_word_logo.svg" : "/aura_logo.svg");
  }, [isWideScreen]);

  useEffect(() => {
    document.title = status?.media_server_name ? `aura | ${status.media_server_name}` : "aura";
  }, [status?.media_server_name]);

  // On mount, check auth status
  useEffect(() => {
    if (status?.current_setup?.auth?.enabled === false) {
      setIsAuthed(true);
      return;
    }
    setIsAuthed(status !== null);
  }, [pathName, status]);

  // When clicking on the logo, navigate to home
  // If already on homepage, reset home page states
  const handleHomeClick = () => {
    if (!isAuthed) {
      router.push("/login");
      return;
    }
    if (isHomePage) {
      setSearchQuery("");
      setCurrentPage(1);
      setFilteredLibraries([]);
      setFilterInDB("");
      setFilterIgnored("");
      setHasSetsAvailableFilter("");
    }
    router.push("/");
  };

  // Hide the menu when it would be empty
  // While the backend is starting, hide the menu and search
  const appLoading = !!status && !status.needs_setup && !status.app_fully_loaded;
  const appReady = !!status && !status.needs_setup && status.app_fully_loaded;
  const hasNewVersion = !appLoading && isNewerVersion(latestVersion ?? "", version);
  const showLogout = !appLoading && isAuthed && !!status?.current_setup.auth.enabled && !status?.auth_disabled_by_env;
  const menuHasItems = appReady || hasNewVersion || showLogout;

  // Failed downloads, kept current as Download Queue jobs finish
  const failedCount = useFailedDownloadsStore((state) => state.failedCount);
  const refreshFailedCount = useFailedDownloadsStore((state) => state.refreshFailedCount);
  useEffect(() => {
    if (!appReady || !isAuthed) return;
    void refreshFailedCount();
    return subscribeToDownloadQueue({ onJobFinished: () => void refreshFailedCount() });
  }, [appReady, isAuthed, refreshFailedCount]);

  // Handle Logout
  const handleLogout = async () => {
    await Logout(); // clears the HttpOnly session cookie server-side
    await DeepResetAllStores();
    window.location.href = "/login";
  };

  return (
    <nav
      suppressHydrationWarning
      className="sticky top-0 z-50 flex items-center px-6 py-4 justify-between shadow-md bg-background dark:bg-background-dark border-b border-border dark:border-border-dark"
    >
      {/* Left: Logo */}
      <div className="relative flex-shrink-0">
        <div
          onClick={handleHomeClick}
          className="relative cursor-pointer active:scale-95 transition-transform select-none"
          style={{
            width: logoSrc === "/aura_logo.svg" ? "50px" : "150px",
            height: logoSrc === "/aura_logo.svg" ? "30px" : "35px",
          }}
        >
          <Image src={logoSrc} alt="Logo" fill className="object-contain filter" loading="eager" />
        </div>
      </div>

      {/* Center: Search, only once set up and loaded */}
      <div className="relative flex-1 flex justify-center mx-3">
        {appReady && !isLoginPage && !isOnboardingPage && (
          <div className="relative w-full max-w-2xl">
            <DynamicSearch placeholder="Search" />
          </div>
        )}
      </div>

      {/* Right: Arrows and/or Settings */}
      <div className="flex items-center gap-2 flex-shrink-0">
        {isMediaPage && (
          <>
            <ArrowLeftCircle
              className={`h-8 w-8 hover:scale-105 active:scale-95 transition-colors cursor-pointer ${!previousMediaItem ? "opacity-30 pointer-events-none" : "text-primary hover:text-primary/80"}`}
              onClick={() => {
                if (previousMediaItem) useMediaStore.setState({ mediaItem: previousMediaItem });
              }}
            />
            <ArrowRightCircle
              className={`h-8 w-8 hover:scale-105 active:scale-95 transition-colors cursor-pointer ${!nextMediaItem ? "opacity-30 pointer-events-none" : "text-primary hover:text-primary/80"}`}
              onClick={() => {
                if (nextMediaItem) useMediaStore.setState({ mediaItem: nextMediaItem });
              }}
            />
          </>
        )}
        {isCollectionItemPage && (
          <>
            <ArrowLeftCircle
              className={`h-8 w-8 hover:scale-105 active:scale-95 transition-colors cursor-pointer ${!previousCollectionItem ? "opacity-30 pointer-events-none" : "text-primary hover:text-primary/80"}`}
              onClick={() => {
                if (previousCollectionItem) useCollectionStore.setState({ collectionItem: previousCollectionItem });
              }}
            />
            <ArrowRightCircle
              className={`h-8 w-8 hover:scale-105 active:scale-95 transition-colors cursor-pointer ${!nextCollectionItem ? "opacity-30 pointer-events-none" : "text-primary hover:text-primary/80"}`}
              onClick={() => {
                if (nextCollectionItem) useCollectionStore.setState({ collectionItem: nextCollectionItem });
              }}
            />
          </>
        )}
        {isAuthed && !isLoginPage && menuHasItems && (
          <DropdownMenu>
            <DropdownMenuTrigger
              asChild
              className="cursor-pointer hover:brightness-120 active:scale-95 transition text-muted-foreground"
            >
              <MenuIcon
                className={cn(
                  "w-8 h-8 ml-2",
                  hasNewVersion && "text-yellow-500 animate-pulse",
                  appReady && failedCount > 0 && "text-red-500"
                )}
              />
            </DropdownMenuTrigger>
            <DropdownMenuContent className="w-56 md:w-64" side="bottom" align="end">
              {appReady && (
                <>
                  <DropdownMenuItem
                    className="cursor-pointer flex items-center active:scale-95 hover:brightness-120"
                    onClick={() => router.push("/saved-sets")}
                  >
                    <BookmarkIcon className="w-6 h-6 mr-2" />
                    Saved Sets
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    className="cursor-pointer flex items-center active:scale-95 hover:brightness-120"
                    onClick={() => router.push("/collections")}
                  >
                    <LayoutGrid className="w-6 h-6 mr-2" />
                    Collections
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    className="cursor-pointer flex items-center active:scale-95 hover:brightness-120"
                    onClick={() => router.push("/download-queue")}
                  >
                    <ListOrdered className="w-6 h-6 mr-2" />
                    <div className="flex flex-col">
                      <span>Download Queue</span>
                      {failedCount > 0 && <span className="text-xs text-red-500">{failedCount} Failed</span>}
                    </div>
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    className="cursor-pointer flex items-center active:scale-95 hover:brightness-120"
                    onClick={() => router.push("/statistics")}
                  >
                    <ChartColumn className="w-6 h-6 mr-2" />
                    Statistics
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="cursor-pointer flex items-center active:scale-95 hover:brightness-120"
                    onClick={() => router.push("/settings")}
                  >
                    <FileCogIcon className="w-6 h-6 mr-2" />
                    Settings
                  </DropdownMenuItem>
                  {isWideScreen && (
                    <DropdownMenuItem className="cursor-pointer flex items-center active:scale-95 hover:brightness-120">
                      <ViewDensitySlider />
                    </DropdownMenuItem>
                  )}
                </>
              )}
              {appReady && (
                <DropdownMenuItem
                  className="cursor-pointer flex items-center active:scale-95 hover:brightness-120"
                  onClick={() => router.push("/logs")}
                >
                  <Logs className="w-6 h-6 mr-2" />
                  Logs
                </DropdownMenuItem>
              )}
              {appReady && (
                <DropdownMenuItem
                  className="cursor-pointer flex items-center active:scale-95 hover:brightness-120"
                  onClick={() => router.push("/jobs")}
                >
                  <Clock className="w-6 h-6 mr-2" />
                  Jobs
                </DropdownMenuItem>
              )}
              {hasNewVersion && (
                <DropdownMenuItem
                  className="cursor-pointer flex items-center active:scale-95 hover:brightness-120 text-yellow-500 animate-pulse"
                  onClick={() =>
                    router.push(
                      `/change-log?currentVersion=${encodeURIComponent(version)}&updates=true&latestVersion=${encodeURIComponent(latestVersion ?? "")}`
                    )
                  }
                >
                  <Sparkles className="w-6 h-6 mr-2 text-yellow-500" />
                  New Version Available ({latestVersion})
                </DropdownMenuItem>
              )}
              {showLogout && (
                <>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="cursor-pointer flex items-center active:scale-95 hover:brightness-120 text-red-600 focus:text-red-700"
                    onClick={handleLogout}
                  >
                    <LogOutIcon className="w-6 h-6 mr-2" />
                    Logout
                  </DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
    </nav>
  );
}
