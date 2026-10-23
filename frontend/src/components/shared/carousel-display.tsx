"use client";

import React from "react";

import { AssetImage } from "@/components/shared/asset-image";
import { CarouselItem } from "@/components/ui/carousel";

import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { IncludedItem, SetRef } from "@/types/media-and-posters/sets";
import type { TYPE_DOWNLOAD_DEFAULT_TYPE_OPTIONS } from "@/types/ui-options";

export function CarouselDisplay({
  sets,
  includedItems = {},
  dimNotFound = false,
  currentTmdbId,
}: {
  sets: SetRef[];
  includedItems?: { [tmdb_id: string]: IncludedItem };
  dimNotFound?: boolean;
  currentTmdbId?: string; // The item the page is for, shown first after the collection's own images
}) {
  const downloadDefaultTypes = useUserPreferencesStore((state) => state.downloadDefaults);
  const showOnlyDownloadDefaults = useUserPreferencesStore((state) => state.showOnlyDownloadDefaults);

  function shouldShow(type: TYPE_DOWNLOAD_DEFAULT_TYPE_OPTIONS) {
    return !showOnlyDownloadDefaults || downloadDefaultTypes.includes(type);
  }

  return (
    <>
      {sets.map((set) => {
        if (!set.images || set.images.length === 0) return null;

        return (
          <React.Fragment key={set.id}>
            {/* The collection's own poster and backdrop, first */}
            {(() => {
              const collectionPosters = shouldShow("collection_poster")
                ? set.images.filter((img) => img.type === "collection_poster")
                : [];
              const collectionBackdrops = shouldShow("collection_backdrop")
                ? set.images.filter((img) => img.type === "collection_backdrop")
                : [];
              if (collectionPosters.length === 0 && collectionBackdrops.length === 0) return null;
              // set.collection is only set when the collection is on the media server
              const collectionClassName = `w-full ${!set.collection && dimNotFound ? "opacity-35" : ""}`;
              return (
                <CarouselItem key={`${set.id}-collection`}>
                  <div className="space-y-2">
                    {collectionPosters.map((img) => (
                      <AssetImage
                        key={img.id}
                        image={img}
                        imageType="mediux"
                        aspect="poster"
                        className={collectionClassName}
                      />
                    ))}
                    {collectionBackdrops.map((img) => (
                      <AssetImage
                        key={img.id}
                        image={img}
                        imageType="mediux"
                        aspect="backdrop"
                        className={collectionClassName}
                      />
                    ))}
                  </div>
                </CarouselItem>
              );
            })()}

            {/* Posters and Backdrops grouped by item_tmdb_id */}
            {(() => {
              // Unique tmdbIds for posters/backdrops
              const uniqueTmdbIds = [
                ...new Set(
                  set.images
                    .filter(
                      (img) =>
                        (img.type === "poster" || img.type === "backdrop") &&
                        (shouldShow("poster") || shouldShow("backdrop")) &&
                        typeof img.item_tmdb_id === "string" &&
                        img.item_tmdb_id.trim() !== ""
                    )
                    .map((img) => img.item_tmdb_id as string)
                ),
              ];

              // Sort: the item the page is for, then available-first (only when dimNotFound=true),
              // then by release date (newest first)
              const sortedTmdbIds = uniqueTmdbIds.sort((a, b) => {
                if (currentTmdbId && (a === currentTmdbId) !== (b === currentTmdbId)) {
                  return a === currentTmdbId ? -1 : 1;
                }
                if (dimNotFound) {
                  const aAvail =
                    isInServer(includedItems, a) && typeof isInServer(includedItems, a) !== "boolean" ? 1 : 0;
                  const bAvail =
                    isInServer(includedItems, b) && typeof isInServer(includedItems, b) !== "boolean" ? 1 : 0;
                  if (aAvail !== bAvail) return bAvail - aAvail;
                }

                return getReleaseEpoch(includedItems, b) - getReleaseEpoch(includedItems, a);
              });

              return sortedTmdbIds.map((tmdbId) => {
                const posters = set.images.filter((img) => img.type === "poster" && img.item_tmdb_id === tmdbId);
                const backdrops = set.images.filter((img) => img.type === "backdrop" && img.item_tmdb_id === tmdbId);

                const available = isInServer(includedItems, tmdbId);
                const isAvailable = available && typeof available !== "boolean";

                if (posters.length === 0 && backdrops.length === 0) return null;

                return (
                  <CarouselItem key={`${set.id}-media-${tmdbId}`}>
                    <div className="space-y-2">
                      {shouldShow("poster") &&
                        posters.map((img) => (
                          <AssetImage
                            key={img.id}
                            image={img}
                            imageType="mediux"
                            aspect="poster"
                            className={`w-full ${!isAvailable && dimNotFound ? "opacity-35" : ""}`}
                            includedItems={includedItems}
                            matchedToItem={isAvailable}
                          />
                        ))}
                      {shouldShow("backdrop") &&
                        backdrops.map((img) => (
                          <AssetImage
                            key={img.id}
                            image={img}
                            imageType="mediux"
                            aspect="backdrop"
                            className={`w-full ${!isAvailable && dimNotFound ? "opacity-35" : ""}`}
                            includedItems={includedItems}
                            matchedToItem={isAvailable}
                          />
                        ))}
                    </div>
                  </CarouselItem>
                );
              });
            })()}

            {/* Season Posters with Latest Titlecards */}
            {(() => {
              const seasonPosterCandidates = (set.images ?? []).filter((img) => {
                const isSeasonPoster = img.type === "season_poster" || img.type === "special_season_poster";
                if (!isSeasonPoster) return false;

                // respect toggles
                if (img.type === "season_poster" && !shouldShow("season_poster")) return false;
                if (img.type === "special_season_poster" && !shouldShow("special_season_poster")) return false;

                return typeof img.item_tmdb_id === "string" && img.item_tmdb_id.trim() !== "";
              });

              if (seasonPosterCandidates.length === 0) return null;

              // Deduplicate: keep newest poster per (tmdbId, season_number)
              const bestByKey = new Map<string, (typeof set.images)[number]>();
              for (const img of seasonPosterCandidates) {
                const tmdbId = String(img.item_tmdb_id);
                const seasonNum = img.season_number ?? 0;
                const key = `${tmdbId}:${seasonNum}`;

                const prev = bestByKey.get(key);
                if (!prev) {
                  bestByKey.set(key, img);
                  continue;
                }

                const prevT = new Date(prev.modified ?? 0).getTime();
                const nextT = new Date(img.modified ?? 0).getTime();
                if (nextT > prevT) bestByKey.set(key, img);
              }

              const seasonPosters = Array.from(bestByKey.values()).sort((a, b) => {
                const aTmdb = typeof a.item_tmdb_id === "string" ? a.item_tmdb_id : "";
                const bTmdb = typeof b.item_tmdb_id === "string" ? b.item_tmdb_id : "";

                const aSeason = a.season_number ?? 0;
                const bSeason = b.season_number ?? 0;

                if (dimNotFound) {
                  const aAvail = aTmdb ? (hasSeason(includedItems, aTmdb, aSeason) ? 1 : 0) : 0;
                  const bAvail = bTmdb ? (hasSeason(includedItems, bTmdb, bSeason) ? 1 : 0) : 0;
                  if (aAvail !== bAvail) return bAvail - aAvail;
                }

                // keep shows with newer release dates first (tie-breaker across multiple tmdbIds)
                const rel = getReleaseEpoch(includedItems, bTmdb) - getReleaseEpoch(includedItems, aTmdb);
                if (rel !== 0) return rel;

                // season high -> low (Specials=0 naturally fall to the end)
                if (aSeason !== bSeason) return bSeason - aSeason;

                // newest modified first
                return new Date(b.modified ?? 0).getTime() - new Date(a.modified ?? 0).getTime();
              });

              return seasonPosters.map((seasonPoster) => {
                const tmdbId = seasonPoster.item_tmdb_id as string;
                const seasonNum = seasonPoster.season_number ?? 0;

                // Find the latest titlecard for THIS tmdbId + season
                const matchingTitlecards = (set.images ?? []).filter(
                  (img) => img.type === "titlecard" && img.item_tmdb_id === tmdbId && img.season_number === seasonNum
                );

                const latestTitlecard =
                  matchingTitlecards.length > 0
                    ? matchingTitlecards.slice().sort((a, b) => (b.episode_number ?? 0) - (a.episode_number ?? 0))[0]
                    : null;

                return (
                  <CarouselItem key={`${set.id}-season-${seasonPoster.id}`}>
                    <div className="space-y-2">
                      {shouldShow("season_poster") && seasonPoster.type === "season_poster" && (
                        <AssetImage
                          image={seasonPoster}
                          imageType="mediux"
                          aspect="poster"
                          className={`w-full ${
                            !hasSeason(includedItems, tmdbId, seasonNum) && dimNotFound ? "opacity-35" : ""
                          }`}
                          includedItems={includedItems}
                          matchedToItem={hasSeason(includedItems, tmdbId, seasonNum)}
                        />
                      )}

                      {shouldShow("special_season_poster") && seasonPoster.type === "special_season_poster" && (
                        <AssetImage
                          image={seasonPoster}
                          imageType="mediux"
                          aspect="poster"
                          className={`w-full ${
                            !hasSeason(includedItems, tmdbId, seasonNum) && dimNotFound ? "opacity-35" : ""
                          }`}
                          includedItems={includedItems}
                          matchedToItem={hasSeason(includedItems, tmdbId, seasonNum)}
                        />
                      )}

                      {shouldShow("titlecard") && latestTitlecard && (
                        <AssetImage
                          image={latestTitlecard}
                          imageType="mediux"
                          aspect="titlecard"
                          className={`w-full ${
                            !hasSeason(includedItems, tmdbId, seasonNum) && dimNotFound ? "opacity-35" : ""
                          }`}
                          includedItems={includedItems}
                          matchedToItem={hasSeason(includedItems, tmdbId, seasonNum)}
                        />
                      )}
                    </div>
                  </CarouselItem>
                );
              });
            })()}

            {/* Standalone Titlecards (only if no season posters exist) */}
            {!set.images.some((img) => img.type === "season_poster" || img.type === "special_season_poster") &&
              shouldShow("titlecard") && (
                <>
                  {Object.entries(
                    set.images
                      .filter((img) => img.type === "titlecard" && img.season_number != null)
                      .reduce(
                        (acc, img) => {
                          const season = img.season_number!;
                          if (!acc[season]) acc[season] = [];
                          acc[season].push(img);
                          return acc;
                        },
                        {} as Record<number, typeof set.images>
                      )
                  )
                    .sort(([aSeason, aCards], [bSeason, bCards]) => {
                      const aSeasonNum = Number(aSeason);
                      const bSeasonNum = Number(bSeason);

                      if (dimNotFound) {
                        const aFirst = aCards[0];
                        const bFirst = bCards[0];

                        const aTmdbId = typeof aFirst?.item_tmdb_id === "string" ? aFirst.item_tmdb_id : "";
                        const bTmdbId = typeof bFirst?.item_tmdb_id === "string" ? bFirst.item_tmdb_id : "";

                        const aAvail = aTmdbId ? (hasSeason(includedItems, aTmdbId, aSeasonNum) ? 1 : 0) : 0;
                        const bAvail = bTmdbId ? (hasSeason(includedItems, bTmdbId, bSeasonNum) ? 1 : 0) : 0;

                        if (aAvail !== bAvail) return bAvail - aAvail; // available first
                      }

                      return bSeasonNum - aSeasonNum; // high -> low
                    })
                    .map(([season, cards]) =>
                      (cards as typeof set.images)
                        .sort((a, b) => {
                          // Prioritize available episodes if dimNotFound is enabled
                          if (dimNotFound) {
                            const aAvail = hasEpisode(
                              includedItems,
                              a.item_tmdb_id as string,
                              a.season_number || 0,
                              a.episode_number
                            )
                              ? 1
                              : 0;
                            const bAvail = hasEpisode(
                              includedItems,
                              b.item_tmdb_id as string,
                              b.season_number || 0,
                              b.episode_number
                            )
                              ? 1
                              : 0;
                            if (aAvail !== bAvail) return bAvail - aAvail; // available first
                          }
                          // Newest modified first
                          return new Date(b.modified).getTime() - new Date(a.modified).getTime();
                        })
                        .slice(0, 3)
                        .map((card) => (
                          <CarouselItem key={`${set.id}-titlecard-${season}-${card.id}`}>
                            <div className="space-y-2">
                              <AssetImage
                                image={card}
                                imageType="mediux"
                                aspect="titlecard"
                                className={`w-full ${!hasEpisode(includedItems, card.item_tmdb_id as string, card.season_number || 0, card.episode_number) && dimNotFound ? "opacity-35" : ""}`}
                                includedItems={includedItems}
                                matchedToItem={hasEpisode(
                                  includedItems,
                                  card.item_tmdb_id as string,
                                  card.season_number || 0,
                                  card.episode_number
                                )}
                              />
                            </div>
                          </CarouselItem>
                        ))
                    )}
                </>
              )}
          </React.Fragment>
        );
      })}
    </>
  );
}

// Look at the included items to see if the Media Item is filled out for that tmdb id
export function isInServer(includedItems: { [tmdb_id: string]: IncludedItem }, tmdbId: string): MediaItem | boolean {
  const includedItem = includedItems ? includedItems[tmdbId] : undefined;
  if (!includedItem) return false;
  if (!includedItem.media_item) return false;
  if (
    includedItem.media_item &&
    includedItem.media_item.library_title !== "" &&
    includedItem.media_item.rating_key !== ""
  )
    return includedItem.media_item;
  return false;
}

export function hasSeason(
  includedItems: { [tmdb_id: string]: IncludedItem },
  tmdbId: string,
  season_number: number
): boolean {
  const inServer = isInServer(includedItems, tmdbId);
  if (!inServer || typeof inServer === "boolean") return false;
  const mediaItem = inServer as MediaItem;
  if (!mediaItem.series) return false;
  const season = mediaItem.series.seasons.find((s) => s.season_number === season_number);
  return season !== undefined;
}

export function hasEpisode(
  includedItems: { [tmdb_id: string]: IncludedItem },
  tmdbId: string,
  season_number: number,
  episode_number?: number
): boolean {
  const inServer = isInServer(includedItems, tmdbId);
  if (!inServer || typeof inServer === "boolean") {
    return false;
  }
  const mediaItem = inServer as MediaItem;
  if (!mediaItem.series) {
    return false;
  }
  const season = mediaItem.series.seasons.find((s) => s.season_number === season_number);
  if (!season) {
    return false;
  }
  if (episode_number == null) {
    return false;
  }
  const episode = season.episodes.find((e) => e.episode_number === episode_number);
  if (!episode) {
    return false;
  }
  return true;
}

type MediuxInfoDateFields = {
  release_date?: string | null;
  first_air_date?: string | null;
  air_date?: string | null;
  date?: string | null;
};

export function getReleaseEpoch(includedItems: { [tmdb_id: string]: IncludedItem }, tmdbId: string): number {
  const ii = includedItems?.[tmdbId] as (IncludedItem & { mediux_info?: MediuxInfoDateFields }) | undefined;
  const info = ii?.mediux_info;

  const raw = info?.release_date ?? info?.first_air_date ?? info?.air_date ?? info?.date ?? null;

  if (typeof raw !== "string" || raw.trim() === "") return 0;

  const t = Date.parse(raw);
  return Number.isFinite(t) ? t : 0;
}
