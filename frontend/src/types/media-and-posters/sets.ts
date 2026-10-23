import type { CollectionItem } from "@/app/collections/page";

import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type {
  TYPE_DB_SET_TYPE_OPTIONS,
  TYPE_DOWNLOAD_DEFAULT_TYPE_OPTIONS,
  TYPE_SET_TYPE_OPTIONS,
} from "@/types/ui-options";

export interface BaseMediuxItemInfo {
  tmdb_id: string;
  type: string;
  date_updated: string;
  status: string;
  title: string;
  tagline: string;
  release_date: string;
  tvdb_id: string;
  imdb_id: string;
  trakt_id: string;
  slug: string;
  tmdb_poster_path: string;
  tmdb_backdrop_path: string;
}

export interface BaseSetInfo {
  id: string;
  title: string;
  type: TYPE_SET_TYPE_OPTIONS;
  user_created: string;
  date_created: string;
  date_updated: string;
  popularity: number;
  popularity_global: number;
}

export interface ImageFile {
  id: string;
  type: TYPE_DOWNLOAD_DEFAULT_TYPE_OPTIONS;
  modified: string;
  file_size: number;
  src: string;
  blurhash: string;
  language: string;
  item_tmdb_id: string;
  title?: string;
  season_number?: number;
  episode_number?: number;
}

export interface SetRef extends Omit<BaseSetInfo, "type"> {
  type: TYPE_DB_SET_TYPE_OPTIONS;
  images: ImageFile[];
  item_ids: string[];
  collection?: CollectionItem; // For a collection set on a movie's page, the media server collection its collection images apply to
}

export interface CollectionItemSetRef extends Omit<BaseSetInfo, "images"> {
  images: CollectionItemImageFile[];
}

export interface CollectionItemImageFile extends Omit<ImageFile, "type"> {
  type: "collection_poster" | "collection_backdrop";
}

export interface BoxsetRef extends Omit<BaseSetInfo, "type"> {
  type: "boxset";
  set_ids: { [key: string]: string[] }; // IDs of sets (show, movie, collection) in this boxset
}

export interface IncludedItem {
  mediux_info: BaseMediuxItemInfo;
  media_item: MediaItem;
}

export interface PosterSetsResponse {
  sets: SetRef[];
  included_items: { [tmdb_id: string]: IncludedItem };
}
