export interface ImageTypeLabelInput {
  image_type: string;
  season_number?: number | null;
  episode_number?: number | null;
}

// Formats an image's type/season/episode into a short human-readable label,
// e.g. "Poster", "Backdrop", "Season 01 Poster", "S01E03 Titlecard".
export const formatImageTypeLabel = (image: ImageTypeLabelInput): string => {
  const season =
    image.season_number !== undefined && image.season_number !== null
      ? String(image.season_number).padStart(2, "0")
      : undefined;
  const episode =
    image.episode_number !== undefined && image.episode_number !== null
      ? String(image.episode_number).padStart(2, "0")
      : undefined;

  switch (image.image_type) {
    case "poster":
      return "Poster";
    case "backdrop":
      return "Backdrop";
    case "season_poster":
      return `Season ${season ?? "??"} Poster`;
    case "titlecard":
      return `S${season ?? "??"}E${episode ?? "??"} Titlecard`;
    case "collection_poster":
      return "Collection Poster";
    case "collection_backdrop":
      return "Collection Backdrop";
    default:
      return "";
  }
};
