import type { Statistics } from "@/types/statistics";

export type StatisticsPeriod = Statistics["period"];

export const formatNumber = (value: number) => value.toLocaleString();

export const formatPercent = (part: number, total: number) =>
  total > 0 ? `${Math.round((part / total) * 100)}%` : "0%";

export const formatBytes = (bytes: number) => {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
};

// formatPeriodDate is a day, week or month for an axis, or longer for a tooltip
export const formatPeriodDate = (date: string, period: StatisticsPeriod, long = false) => {
  const day = new Date(`${date}T00:00:00`);
  if (Number.isNaN(day.getTime())) return date;
  if (period === "month") {
    return day.toLocaleDateString(undefined, { month: "short", year: long ? "numeric" : "2-digit" });
  }
  if (period === "week" && long) {
    return `Week of ${day.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })}`;
  }
  return day.toLocaleDateString(undefined, { month: "short", day: "numeric", year: long ? "numeric" : undefined });
};

export { sourceLabel } from "@/helper/format-download-source";

const IMAGE_TYPE_LABELS: Record<string, string> = {
  poster: "Posters",
  backdrop: "Backdrops",
  season_poster: "Season posters",
  special_season_poster: "Special season posters",
  titlecard: "Title cards",
  collection_poster: "Collection posters",
  collection_backdrop: "Collection backdrops",
};

export const imageTypeLabel = (imageType: string) => IMAGE_TYPE_LABELS[imageType] ?? imageType;
