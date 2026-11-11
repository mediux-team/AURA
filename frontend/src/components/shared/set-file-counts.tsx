import { makePlural } from "@/helper/make_plural";
import { AlertTriangle } from "lucide-react";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Lead } from "@/components/ui/typography";

import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { IncludedItem, SetRef } from "@/types/media-and-posters/sets";

interface SetFileCountsProps {
  mediaItem: MediaItem;
  set: SetRef;
  includedItems?: { [tmdb_id: string]: IncludedItem };
}

type EpisodeInfo = {
  episode_number: number;
  inMediaItem: boolean;
  setHasTitlecard: boolean;
  inSet: boolean;
  title?: string;
};

type SeasonInfo = {
  season_number: number;
  inMediaItem: boolean;
  setHasSeasonPoster: boolean;
  inSet: boolean;
  episodes: EpisodeInfo[];
};

type SeasonsAndEpisodesMap = Map<number, SeasonInfo>;

export function SetFileCounts({ mediaItem, set, includedItems }: SetFileCountsProps) {
  if (!set || !set.images || set.images.length === 0) return null;

  // aura only adds the library's item to an included item when it is in the library
  const inLibrary = (tmdbID: string) => !!includedItems?.[tmdbID]?.media_item?.rating_key;

  // --- MOVIE LOGIC ---
  const getMovieFileCounts = () => {
    let secondary = "";

    const posterCount = set.images?.filter((img) => img.type === "poster").length || 0;
    const backdropCount = set.images?.filter((img) => img.type === "backdrop").length || 0;

    const parts: string[] = [];
    if (posterCount > 0) parts.push(`${posterCount} ${makePlural(posterCount, "Poster")}`);
    if (backdropCount > 0) parts.push(`${backdropCount} ${makePlural(backdropCount, "Backdrop")}`);
    const primary = parts.join(" • ");

    // The collection's own images, when the set has them
    const collection = [
      set.images?.some((img) => img.type === "collection_poster") ? "Collection Poster" : "",
      set.images?.some((img) => img.type === "collection_backdrop") ? "Collection Backdrop" : "",
    ]
      .filter(Boolean)
      .join(" • ");

    const itemIDs = Array.from(
      new Set(
        (set.images ?? [])
          .map((img) => img.item_tmdb_id)
          .filter((id): id is string => typeof id === "string" && id.trim() !== "")
      )
    );
    let missingMovies: { id: string; title: string }[] = [];
    if (itemIDs.length) {
      missingMovies = itemIDs
        .filter((item_id) => !inLibrary(item_id))
        .map((item_id) => ({ id: item_id, title: includedItems?.[item_id]?.mediux_info?.title || item_id }));

      const totalTmdbIDs = itemIDs.length;
      const missingTmdbIDs = missingMovies.length;

      if (missingTmdbIDs === 0) {
        secondary = "";
      } else if (missingTmdbIDs === totalTmdbIDs) {
        secondary = `No items in Library`;
      } else {
        secondary = `${missingTmdbIDs} of ${totalTmdbIDs} ${makePlural(totalTmdbIDs, "movie")} missing from library`;
      }
    }

    return {
      collection,
      primary,
      secondary,
      breakdown: {
        missingFromLibrary: missingMovies,
      },
    };
  };

  // --- SHOW LOGIC ---
  const getShowFileCounts = () => {
    if (!mediaItem.series)
      return {
        line1: "",
        line2: "",
        hasMissing: false,
        missingFromSet: [],
        missingFromServer: [],
      };

    const posterCount = set.images?.filter((img) => img.type === "poster").length || 0;
    const backdropCount = set.images?.filter((img) => img.type === "backdrop").length || 0;
    const seasonPosterCount =
      set.images?.filter((img) => img.type === "season_poster" && img.season_number !== 0).length || 0;
    const specialSeasonPosterCount =
      set.images?.filter((img) => img.type === "season_poster" && img.season_number === 0).length || 0;
    const titleCardCount = set.images?.filter((img) => img.type === "titlecard").length || 0;

    const line1Parts: string[] = [];
    if (posterCount > 0) line1Parts.push(`${posterCount} ${makePlural(posterCount, "Poster")}`);
    if (backdropCount > 0) line1Parts.push(`${backdropCount} ${makePlural(backdropCount, "Backdrop")}`);
    const line2Parts: string[] = [];
    if (seasonPosterCount > 0)
      line2Parts.push(`${seasonPosterCount} ${makePlural(seasonPosterCount, "Season Poster")}`);
    if (specialSeasonPosterCount > 0)
      line2Parts.push(`${specialSeasonPosterCount} ${makePlural(specialSeasonPosterCount, "Special Season Poster")}`);
    if (titleCardCount > 0) line2Parts.push(`${titleCardCount} ${makePlural(titleCardCount, "Titlecard")}`);

    const missingFromSet: string[] = [];
    const missingFromServer: string[] = [];

    // --- Build SeasonsAndEpisodesMap ---
    const mediaSeasons = mediaItem.series?.seasons ?? [];
    const setSeasonPosters = set.images.filter(
      (img) => img.type === "season_poster" && typeof img.season_number === "number"
    );
    const setTitlecards = set.images.filter(
      (img) =>
        img.type === "titlecard" && typeof img.season_number === "number" && typeof img.episode_number === "number"
    );

    const allSeasonNumbers = new Set<number>([
      ...mediaSeasons.map((s) => s.season_number).filter((n): n is number => typeof n === "number"),
      ...setSeasonPosters.map((img) => img.season_number as number),
      ...setTitlecards.map((img) => img.season_number as number),
    ]);

    const SeasonsAndEpisodesMap: SeasonsAndEpisodesMap = new Map();

    allSeasonNumbers.forEach((season_number) => {
      const mediaSeason = mediaSeasons.find((s) => s.season_number === season_number);
      const setHasSeasonPoster = setSeasonPosters.some((img) => img.season_number === season_number);
      const setHasTitlecards = setTitlecards.some((img) => img.season_number === season_number);

      const mediaEpisodes = mediaSeason?.episodes ?? [];
      const setEpisodes = setTitlecards.filter((img) => img.season_number === season_number);

      const allEpisodeNumbers = new Set<number>([
        ...mediaEpisodes.map((e) => e.episode_number).filter((n): n is number => typeof n === "number"),
        ...setEpisodes.map((img) => img.episode_number as number),
      ]);

      const episodes: EpisodeInfo[] = [];
      allEpisodeNumbers.forEach((episode_number) => {
        const mediaEp = mediaEpisodes.find((e) => e.episode_number === episode_number);
        const setEp = setEpisodes.find((img) => img.episode_number === episode_number);

        episodes.push({
          episode_number,
          inMediaItem: !!mediaEp,
          setHasTitlecard: !!setEp,
          inSet: !!setEp,
          title: mediaEp?.title || setEp?.title || "",
        });
      });

      SeasonsAndEpisodesMap.set(season_number, {
        season_number,
        inMediaItem: !!mediaSeason,
        setHasSeasonPoster,
        inSet: setHasSeasonPoster || setHasTitlecards,
        episodes,
      });
    });

    // --- Analyze for missing items ---

    // 1. Missing from Set
    // Only check for missing if the set contains season posters or titlecards
    const setHasAnySeasonPoster = set.images.some((img) => img.type === "season_poster");
    const setHasAnyTitlecard = set.images.some((img) => img.type === "titlecard");

    // For each season in the media item, check if it's missing from the set (season poster)
    if (setHasAnySeasonPoster) {
      mediaSeasons.forEach((season) => {
        const hasPoster = setSeasonPosters.some((img) => img.season_number === season.season_number);
        if (!hasPoster) {
          if (season.season_number === 0) {
            missingFromSet.push(`Special Season Poster`);
          } else {
            missingFromSet.push(`Season ${String(season.season_number).padStart(2, "0")} Poster`);
          }
        }
      });
    }

    // For each episode in the media item, check if it's missing from the set (titlecard)
    if (setHasAnyTitlecard) {
      mediaSeasons.forEach((season) => {
        const totalEpisodes = (season.episodes ?? []).length;
        let missingCount = 0;
        const samples: string[] = [];

        (season.episodes ?? []).forEach((ep) => {
          const hasTitlecard = setTitlecards.some(
            (img) => img.season_number === season.season_number && img.episode_number === ep.episode_number
          );
          if (!hasTitlecard) {
            missingCount++;
            if (season.season_number === 0) {
              samples.push(`Special Season Episode ${ep.episode_number}${ep.title ? ` - ${ep.title}` : ""}`);
            } else {
              samples.push(
                `S${String(season.season_number).padStart(2, "0")}E${String(ep.episode_number).padStart(2, "0")} Titlecard${ep.title ? ` - ${ep.title}` : ""}`
              );
            }
          }
        });

        if (missingCount === totalEpisodes && totalEpisodes > 0) {
          if (season.season_number === 0) {
            missingFromSet.push(`Special Season Titlecards (${totalEpisodes} ${makePlural(totalEpisodes, "episode")})`);
          } else {
            missingFromSet.push(
              `Season ${String(season.season_number).padStart(2, "0")} Titlecards (${totalEpisodes} ${makePlural(totalEpisodes, "episode")})`
            );
          }
          for (let i = 0; i < samples.length - 1; i++) {
            missingFromSet.push("");
          }
        } else {
          missingFromSet.push(...samples);
        }
      });
    }

    // Track summarized seasons
    const summarizedSeasons = new Set<number>();

    // For each titlecard in the set, check if the media item has this episode
    const missingEpisodesBySeason: { [season: number]: { total: number; missing: number; samples: string[] } } = {};

    setTitlecards.forEach((img) => {
      const season = mediaSeasons.find((s) => s.season_number === img.season_number);
      const hasEpisode = !!season && (season.episodes ?? []).some((ep) => ep.episode_number === img.episode_number);

      if (!hasEpisode) {
        if (!missingEpisodesBySeason[img.season_number!]) {
          const total = setTitlecards.filter((tc) => tc.season_number === img.season_number).length;
          missingEpisodesBySeason[img.season_number!] = { total, missing: 0, samples: [] };
        }
        missingEpisodesBySeason[img.season_number!].missing++;
        missingEpisodesBySeason[img.season_number!].samples.push(
          `S${String(img.season_number).padStart(2, "0")}E${String(img.episode_number).padStart(2, "0")}${img.title ? ` - ${img.title}` : ""}`
        );
      }
    });

    // If all episodes in a season are missing, just display the season summary
    Object.entries(missingEpisodesBySeason).forEach(([seasonNum, info]) => {
      if (info.missing === info.total) {
        summarizedSeasons.add(Number(seasonNum));
        if (seasonNum === "0") {
          missingFromServer.push(`Special Season (${info.total} ${makePlural(info.total, "Episode")})`);
        } else {
          missingFromServer.push(
            `Season ${String(seasonNum).padStart(2, "0")} (${info.total} ${makePlural(info.total, "Episode")})`
          );
        }
        // Remove individual episode samples for this season
        for (let i = 0; i < info.samples.length - 1; i++) {
          missingFromServer.push("");
        }
      } else {
        missingFromServer.push(...info.samples);
      }
    });

    // For each season poster in the set, check if the media item has this season
    setSeasonPosters.forEach((img) => {
      // Only push if not already summarized above
      if (
        !mediaSeasons.some((season) => season.season_number === img.season_number) &&
        !summarizedSeasons.has(img.season_number!)
      ) {
        missingFromServer.push(
          img.season_number === 0 ? `Special Season` : `Season ${String(img.season_number).padStart(2, "0")}`
        );
      }
    });

    missingFromSet.sort();
    missingFromServer.sort((a, b) => {
      // Special Season first
      if (a.startsWith("Special Season") && !b.startsWith("Special Season")) return -1;
      if (!a.startsWith("Special Season") && b.startsWith("Special Season")) return 1;

      // Then Season XX
      const seasonRegex = /^Season \d{2}/;
      const aIsSeason = seasonRegex.test(a);
      const bIsSeason = seasonRegex.test(b);

      if (aIsSeason && !bIsSeason) return -1;
      if (!aIsSeason && bIsSeason) return 1;

      // Otherwise, sort alphabetically
      return a.localeCompare(b, undefined, { numeric: true });
    });

    return {
      line1: line1Parts.join(" • "),
      line2: line2Parts.join(" • "),
      hasMissing: missingFromSet.length > 0 || missingFromServer.length > 0,
      missingFromSet,
      missingFromServer,
    };
  };

  const movieResult = mediaItem.type === "movie" ? getMovieFileCounts() : null;
  const showResult = mediaItem.type === "show" ? getShowFileCounts() : null;

  return (
    <div>
      {mediaItem.type === "movie" && movieResult && (
        <>
          {movieResult.collection && (
            <Lead className="text-sm text-muted-foreground ml-1">{movieResult.collection}</Lead>
          )}
          {movieResult.primary && <Lead className="text-sm text-muted-foreground ml-1">{movieResult.primary}</Lead>}
          {movieResult.secondary && (
            <div className="flex items-center mb-2">
              <Lead className="text-sm ml-1 text-yellow-500">{movieResult.secondary}</Lead>
              <Popover>
                <PopoverTrigger asChild>
                  <button className="ml-1 text-muted-foreground hover:text-primary">
                    <AlertTriangle className="text-yellow-500" size={16} />
                  </button>
                </PopoverTrigger>
                <PopoverContent className="p-4 max-w-xs">
                  <div>
                    {"missingFromLibrary" in movieResult.breakdown &&
                    movieResult.breakdown.missingFromLibrary.length > 0 ? (
                      <div>
                        <div className="font-semibold">Missing from Library</div>
                        <ul className="text-sm list-disc list-inside">
                          {movieResult.breakdown.missingFromLibrary.map((movie, idx) => (
                            <li key={idx}>{movie.title}</li>
                          ))}
                        </ul>
                      </div>
                    ) : (
                      <div className="text-sm text-muted-foreground">No missing items</div>
                    )}
                  </div>
                </PopoverContent>
              </Popover>
            </div>
          )}
        </>
      )}
      {mediaItem.type === "show" && showResult && (
        <>
          {showResult.line1 && <Lead className="text-sm text-muted-foreground ml-1">{showResult.line1}</Lead>}
          {showResult.line2 && <Lead className="text-sm text-muted-foreground ml-1">{showResult.line2}</Lead>}
          {showResult.hasMissing && (
            <div className="flex items-center mb-2">
              {showResult.missingFromSet.length > 0 && showResult.missingFromServer.length > 0 ? (
                <Lead className="text-sm ml-1 text-yellow-500">
                  {showResult.missingFromSet.length} {makePlural(showResult.missingFromSet.length, "item")} missing from
                  set • {showResult.missingFromServer.length} {makePlural(showResult.missingFromServer.length, "item")}{" "}
                  missing from server
                </Lead>
              ) : showResult.missingFromSet.length > 0 ? (
                <Lead className="text-sm ml-1 text-yellow-500">
                  {showResult.missingFromSet.length} {makePlural(showResult.missingFromSet.length, "item")} missing from
                  set
                </Lead>
              ) : (
                <Lead className="text-sm ml-1 text-yellow-500">
                  {showResult.missingFromServer.length} {makePlural(showResult.missingFromServer.length, "item")}{" "}
                  missing from server
                </Lead>
              )}

              <Popover>
                <PopoverTrigger asChild>
                  <button className="ml-1 text-muted-foreground hover:text-primary">
                    <AlertTriangle className="text-yellow-500" size={16} />
                  </button>
                </PopoverTrigger>
                <PopoverContent className="p-4 max-w-xs">
                  <div>
                    {showResult.missingFromSet.length > 0 && (
                      <>
                        <div className="font-semibold">Missing from Set</div>
                        <ul className="text-sm list-disc list-inside">
                          {showResult.missingFromSet.map((item, idx) => item !== "" && <li key={idx}>{item}</li>)}
                        </ul>
                      </>
                    )}
                    {showResult.missingFromServer.length > 0 && (
                      <>
                        <div className="font-semibold mt-2">Missing from Server</div>
                        <ul className="text-sm list-disc list-inside">
                          {showResult.missingFromServer.map((item, idx) => item !== "" && <li key={idx}>{item}</li>)}
                        </ul>
                      </>
                    )}
                    {showResult.missingFromSet.length === 0 && showResult.missingFromServer.length === 0 && (
                      <div className="text-sm text-muted-foreground">No missing items</div>
                    )}
                  </div>
                </PopoverContent>
              </Popover>
            </div>
          )}
        </>
      )}
    </div>
  );
}
