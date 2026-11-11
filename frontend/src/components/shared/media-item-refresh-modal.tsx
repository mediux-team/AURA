"use client";

import { makePlural } from "@/helper/make_plural";
import { RefreshMediaItemMetadata } from "@/services/mediaserver/media-item-refresh";

import { useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Progress } from "@/components/ui/progress";

import { cn } from "@/lib/cn";

import type { MediaItem, MediaItemEpisode, MediaItemSeason } from "@/types/media-and-posters/media-item-and-library";

export interface MediaItemDetailsRefreshMetadataProps {
  mediaItem: MediaItem;
  isOpen: boolean;
  onClose: () => void;
}

type RefreshTaskStatus = "pending" | "in-progress" | "completed" | "failed" | "skipped";
type RefreshTaskKind = "refresh-item" | "refresh-season" | "refresh-episode";

type RefreshTask = {
  id: string;
  kind: RefreshTaskKind;
  status: RefreshTaskStatus;
  label: string;
  refresh_key?: string;
  error?: string;
};

type RefreshProgress = {
  currentText: string;
  totalPlanned: number;
  tasks: RefreshTask[];
};

const newId = () => globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`;

const getRefreshCounts = (state: RefreshProgress) => {
  const relevant = state.tasks;
  const done = relevant.filter(
    (t) => t.status === "completed" || t.status === "failed" || t.status === "skipped"
  ).length;

  const total = Math.max(1, state.totalPlanned || relevant.length || 1);
  return { done, total };
};

const getRefreshPercent = (state: RefreshProgress) => {
  const { done, total } = getRefreshCounts(state);
  return Math.min(100, Math.round((done / total) * 100));
};

export const RefreshMetadataModal = ({ mediaItem, isOpen, onClose }: MediaItemDetailsRefreshMetadataProps) => {
  const isShow = mediaItem.type === "show";

  const [refreshProgress, setRefreshProgress] = useState<RefreshProgress>({
    currentText: "",
    totalPlanned: 0,
    tasks: [],
  });

  const [selectedSeasons, setSelectedSeasons] = useState<Record<number, boolean>>({});

  const seasons = useMemo(() => {
    if (!isShow) return [];
    if (!mediaItem.series) return [];
    if (!Array.isArray(mediaItem.series.seasons)) return [];
    if (mediaItem.series.seasons.length === 0) return [];

    return mediaItem.series.seasons
      .filter((s) => s?.season_number && s.season_number > 0)
      .map((s) => s!.season_number!);
  }, [isShow, mediaItem.series]);

  // Map: seasonNumber -> episodeCount (from mediaItem.Series.Seasons)
  const seasonEpisodeCounts = useMemo(() => {
    const map = new Map<number, number>();
    if (!isShow) return map;

    const seasonObjs = mediaItem.series?.seasons ?? [];
    for (const s of seasonObjs) {
      if (!s?.season_number || s.season_number === 0) continue;
      const count = Array.isArray(s.episodes) ? s.episodes.length : 0;
      map.set(s.season_number, count);
    }

    return map;
  }, [isShow, mediaItem.series?.seasons]);

  useEffect(() => {
    if (!isOpen) return;

    const initial: Record<number, boolean> = {};
    for (const s of seasons) initial[s] = true;
    setSelectedSeasons(initial);
  }, [isOpen, seasons]);

  const selectedSeasonNumbers = useMemo(() => {
    if (!isShow) return [];
    return seasons.filter((s) => selectedSeasons[s]);
  }, [isShow, seasons, selectedSeasons]);

  const selectedEpisodeCount = useMemo(() => {
    if (!isShow) return 0;
    return selectedSeasonNumbers.reduce((acc, s) => acc + (seasonEpisodeCounts.get(s) ?? 0), 0);
  }, [isShow, selectedSeasonNumbers, seasonEpisodeCounts]);

  const resetRefreshProgress = () => {
    setRefreshProgress({ currentText: "", totalPlanned: 0, tasks: [] });
  };

  useEffect(() => {
    // When closing, clear progress so the next open starts clean
    if (!isOpen) resetRefreshProgress();
  }, [isOpen]);

  const updateRefreshTask = (taskId: string, updater: (t: RefreshTask) => RefreshTask) => {
    setRefreshProgress((prev) => ({
      ...prev,
      tasks: prev.tasks.map((t) => (t.id === taskId ? updater(t) : t)),
    }));
  };

  const setRefreshText = (text: string) => {
    setRefreshProgress((prev) => ({ ...prev, currentText: text }));
  };

  const buildRefreshTasks = (): RefreshTask[] => {
    // Movie: single task
    if (!isShow) {
      return [
        {
          id: newId(),
          kind: "refresh-item",
          status: "pending",
          label: `Refresh "${mediaItem.title}"`,
          refresh_key: mediaItem.rating_key,
        },
      ];
    }

    // Show: refresh item + each season + each episode in selected seasons
    const tasks: RefreshTask[] = [];

    // refresh show itself
    tasks.push({
      id: newId(),
      kind: "refresh-item",
      status: "pending",
      label: `${mediaItem.title}`,
      refresh_key: mediaItem.rating_key,
    });

    const seasonObjs = mediaItem.series?.seasons ?? [];
    for (const seasonNum of selectedSeasonNumbers) {
      const season = seasonObjs.find((s) => s?.season_number === seasonNum);

      // Season refresh
      tasks.push({
        id: newId(),
        kind: "refresh-season",
        status: "pending",
        label: `Season ${seasonNum}`,
        refresh_key: (season as MediaItemSeason)?.rating_key,
      });

      // Episode refreshes
      const episodes = (season as MediaItemSeason)?.episodes ?? [];
      for (const ep of episodes) {
        const epNum = (ep as MediaItemEpisode)?.episode_number;
        const epTitle = `"${ep.title}" (S${seasonNum.toString().padStart(2, "0")}E${epNum?.toString().padStart(2, "0")})`;
        tasks.push({
          id: newId(),
          kind: "refresh-episode",
          status: "pending",
          label: `${epTitle}`,
          refresh_key: (ep as MediaItemEpisode)?.rating_key,
        });
      }
    }

    return tasks;
  };

  const refreshByRatingKey = async (refreshKey: string, refreshTitle: string) => {
    try {
      const response = await RefreshMediaItemMetadata(mediaItem, refreshTitle, refreshKey);
      if (response.status === "error") {
        throw new Error(response.error?.message || "Unknown error refreshing media server item");
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unknown error";
      throw new Error(message, { cause: error });
    }
  };

  const runRefreshTask = async (t: RefreshTask) => {
    // Missing ratingKey => skip (counts as done so progress completes)
    if (!t.refresh_key) {
      updateRefreshTask(t.id, (x) => ({
        ...x,
        status: "skipped",
        error: "Missing ratingKey for refresh",
      }));
      return;
    }

    updateRefreshTask(t.id, (x) => ({ ...x, status: "in-progress", error: undefined }));
    setRefreshText(`Refreshing ${t.label}`);

    try {
      await refreshByRatingKey(t.refresh_key, t.label);
      updateRefreshTask(t.id, (x) => ({ ...x, status: "completed" }));
    } catch (e) {
      updateRefreshTask(t.id, (x) => ({
        ...x,
        status: "failed",
        error: e instanceof Error ? e.message : "Unknown error",
      }));
    }
  };

  const handleRefresh = async () => {
    const tasks = buildRefreshTasks();
    setRefreshProgress({
      currentText: "Starting...",
      totalPlanned: tasks.length,
      tasks,
    });

    for (const t of tasks) {
      await runRefreshTask(t);
    }

    setRefreshText("Completed!");
  };

  const percent = getRefreshPercent(refreshProgress);
  const { done, total } = getRefreshCounts(refreshProgress);
  const hasProgress = refreshProgress.totalPlanned > 0;
  const hasErrors = refreshProgress.tasks.some((t) => t.status === "failed");
  const isAllSeasonsSelected = isShow && seasons.length > 0 && selectedSeasonNumbers.length === seasons.length;

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className={cn("max-h-[80vh] overflow-y-auto sm:max-w-[700px]", "border border-primary")}>
        <DialogHeader>
          <DialogTitle className="text-lg font-bold">Refresh Metadata</DialogTitle>

          {!isShow ? (
            <DialogDescription className="text-sm text-muted-foreground">
              Refresh metadata for <span className="font-semibold">'{mediaItem.title}'</span>
            </DialogDescription>
          ) : (
            <DialogDescription className="text-sm text-muted-foreground">
              Select which seasons to refresh for <span className="font-semibold">'{mediaItem.title}'</span>
            </DialogDescription>
          )}
        </DialogHeader>

        {isShow && (
          <div className="mt-2 space-y-3">
            <div className="flex items-center justify-between gap-2">
              <div className="text-sm font-medium">Seasons</div>

              <div className="flex items-center gap-2">
                <Badge
                  className={cn(
                    "cursor-pointer transition duration-200 hover:brightness-110 active:scale-95",
                    isAllSeasonsSelected
                      ? "bg-primary text-primary-foreground"
                      : "bg-secondary text-secondary-foreground"
                  )}
                  onClick={() => {
                    const allSelected: Record<number, boolean> = {};
                    for (const s of seasons) allSelected[s] = true;
                    setSelectedSeasons(allSelected);
                  }}
                >
                  Select All
                </Badge>

                <Badge
                  className="cursor-pointer bg-secondary text-secondary-foreground hover:brightness-110 transition duration-200 active:scale-95"
                  onClick={() => {
                    setSelectedSeasons({});
                  }}
                >
                  Deselect All
                </Badge>
              </div>
            </div>

            <div className="flex flex-wrap gap-2 mt-2">
              {seasons.map((s) => {
                const epCount = seasonEpisodeCounts.get(s) ?? 0;
                const isSelected = !!selectedSeasons[s];

                return (
                  <Badge
                    key={s}
                    className={`flex items-center transition duration-200 hover:brightness-120 active:scale-95 ${
                      isSelected ? "cursor-pointer bg-primary" : "cursor-pointer bg-secondary"
                    }`}
                    onClick={() => {
                      setSelectedSeasons((prev) => ({ ...prev, [s]: !isSelected }));
                    }}
                  >
                    <span className="min-w-0">
                      <span className={cn(isSelected ? "" : "text-muted-foreground")}>
                        {s === 0 ? "Special Season" : `Season ${s}`}
                      </span>
                      <span className={cn("ml-2 text-xs tabular-nums", isSelected ? "" : "text-muted-foreground")}>
                        ({epCount} {makePlural(epCount, "ep")})
                      </span>
                    </span>
                  </Badge>
                );
              })}
            </div>
          </div>
        )}

        {isShow && (
          <div className="mt-1 text-xs text-muted-foreground">
            Selected: {selectedSeasonNumbers.length}/{seasons.length} {makePlural(seasons.length, "season")}
            {" • "}
            {selectedEpisodeCount} {makePlural(selectedEpisodeCount, "episode")}
          </div>
        )}

        {/* Progress Bar */}
        {hasProgress ? (
          <div className="mt-3 min-h-[24px]">
            <div className="flex items-center justify-between gap-2">
              <div className="relative w-full min-w-0">
                <Progress
                  value={percent}
                  className={cn(
                    "w-full rounded-md overflow-hidden",
                    percent < 100 ? "h-5" : "h-3",
                    percent === 100 && !hasErrors && "[&>div]:bg-green-500",
                    percent === 100 && hasErrors && "[&>div]:bg-destructive",
                    percent < 100 && "[&>div]:bg-primary"
                  )}
                />
                {percent < 100 && (
                  <span
                    className={cn(
                      "absolute inset-0 flex items-center justify-center",
                      "text-xs text-white pointer-events-none",
                      "px-2 min-w-0"
                    )}
                    title={refreshProgress.currentText}
                  >
                    <span className="w-full min-w-0 truncate text-center">{refreshProgress.currentText}</span>
                  </span>
                )}
              </div>

              <span className="text-sm text-muted-foreground min-w-[56px] text-right tabular-nums">{percent}%</span>
            </div>

            {done !== total && (
              <div className="mt-1 text-xs text-muted-foreground tabular-nums text-right">
                {done}/{total}
              </div>
            )}
          </div>
        ) : null}

        <div className="mt-4 flex justify-end space-x-2">
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>

          <Button onClick={handleRefresh}>
            {isShow && selectedSeasonNumbers.length === 0 && "Refresh Show"}
            {!isShow && "Refresh"}
            {isShow &&
              selectedSeasonNumbers.length > 0 &&
              `Refresh Show & ${selectedSeasonNumbers.length} ${makePlural(selectedSeasonNumbers, "season")}`}
          </Button>
        </div>

        {refreshProgress.tasks.some((t) => t.status === "failed") && (
          <div className="mt-4 space-y-2">
            <div className="text-sm font-medium text-destructive">Some items failed to refresh:</div>
            <div className="max-h-40 overflow-y-auto rounded-md border border-destructive">
              {refreshProgress.tasks
                .filter((t) => t.status === "failed")
                .map((t) => (
                  <div key={t.id} className="flex flex-col p-2 bg-destructive/10">
                    <div className="text-sm font-medium">{t.label}</div>
                    <div className="text-xs text-destructive">{t.error}</div>
                  </div>
                ))}
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
};
