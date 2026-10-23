// Sort Order
export type TYPE_SORT_ORDER_OPTIONS = "asc" | "desc";
// Items Per Page
export const ITEMS_PER_PAGE_OPTIONS: number[] = [10, 20, 30, 50, 100];

// Saved Sets View Types
export type TYPE_SAVED_SET_VIEW_TYPE_OPTIONS = "card" | "table";
// Set Types
export type TYPE_SET_TYPE_OPTIONS = "show" | "movie" | "collection" | "boxset";
// Database Set Types (for filtering, since "boxset" is not a real type in the database)
export type TYPE_DB_SET_TYPE_OPTIONS = "show" | "movie" | "collection";
// Saved Set Auto-Download Filter Options
export type TYPE_FILTER_AUTO_DOWNLOAD_OPTIONS = "" | "on" | "off";
export const FILTER_AUTO_DOWNLOAD_OPTIONS: {
  value: TYPE_FILTER_AUTO_DOWNLOAD_OPTIONS;
  label: string;
}[] = [
  { value: "on", label: "Auto-Download On" },
  { value: "off", label: "Auto-Download Off" },
];

export type TYPE_FILTER_MEDIA_ITEM_ON_SERVER_OPTIONS = "" | "true" | "false";
export const FILTER_MEDIA_ITEM_ON_SERVER_OPTIONS: {
  value: TYPE_FILTER_MEDIA_ITEM_ON_SERVER_OPTIONS;
  label: string;
}[] = [
  { value: "true", label: "On Server" },
  { value: "false", label: "Not on Server" },
];

// In Database Filter Options (Home Page)
export type TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS = "" | "notInDB" | "inDB";
export const HOME_PAGE_FILTER_IN_DB_OPTIONS: {
  value: TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS;
  label: string;
}[] = [
  { value: "notInDB", label: "Not in Database" },
  { value: "inDB", label: "In Database" },
];

// Has Sets Available Filter Options (Home Page)
export type TYPE_HOME_PAGE_FILTER_HAS_SETS_AVAILABLE_OPTIONS = "" | "hasSetsAvailable" | "noSetsAvailable";
// In Database Filter Options (User Page) - includes "otherSetInDB" to filter items that are in the database but only in other sets, not the current set
export type TYPE_USER_PAGE_FILTER_IN_DB_OPTIONS = TYPE_HOME_PAGE_FILTER_IN_DB_OPTIONS | "otherSetInDB";
export const USER_PAGE_FILTER_IN_DB_OPTIONS: {
  value: TYPE_USER_PAGE_FILTER_IN_DB_OPTIONS;
  label: string;
}[] = [...HOME_PAGE_FILTER_IN_DB_OPTIONS, { value: "otherSetInDB", label: "In Database (Other Sets)" }];

// Ignored Filter Options
export type TYPE_FILTER_IGNORED_OPTIONS =
  "" | "ignored" | "always" | "until-set-available" | "until-new-set-available" | "not_ignored";
export const FILTER_IGNORED_OPTIONS: {
  value: TYPE_FILTER_IGNORED_OPTIONS;
  label: string;
}[] = [
  { value: "ignored", label: "Ignored" },
  { value: "always", label: "Always Ignored" },
  { value: "until-set-available", label: "Until Set Available" },
  { value: "until-new-set-available", label: "Until New Set Available" },
  { value: "not_ignored", label: "Not Ignored" },
];

// Download Default Type Options
export type TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS =
  "poster" | "backdrop" | "season_poster" | "special_season_poster" | "titlecard";
export const DOWNLOAD_IMAGE_TYPE_OPTIONS: {
  value: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS;
  label: string;
}[] = [
  { value: "poster", label: "Poster" },
  { value: "backdrop", label: "Backdrop" },
  { value: "season_poster", label: "Season Poster" },
  { value: "special_season_poster", label: "Special Season Poster" },
  { value: "titlecard", label: "Titlecard" },
];

export type TYPE_DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS = "collection_poster" | "collection_backdrop";
export const DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS: {
  value: TYPE_DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS;
  label: string;
}[] = [
  { value: "collection_poster", label: "Collection Poster" },
  { value: "collection_backdrop", label: "Collection Backdrop" },
];

// Download Defaults can include the collection images, offered for collection sets on a movie's page
export type TYPE_DOWNLOAD_DEFAULT_TYPE_OPTIONS =
  TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS | TYPE_DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS;
export const DOWNLOAD_DEFAULT_TYPE_OPTIONS: {
  value: TYPE_DOWNLOAD_DEFAULT_TYPE_OPTIONS;
  label: string;
}[] = [...DOWNLOAD_IMAGE_TYPE_OPTIONS, ...DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS];

export type TYPE_MEDIA_ITEM_TYPE_OPTIONS = "movie" | "show";
