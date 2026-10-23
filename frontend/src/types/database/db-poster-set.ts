import type { CollectionItem } from "@/app/collections/page";

import type { MediaItem, SelectedTypes } from "@/types/media-and-posters/media-item-and-library";
import type { BaseSetInfo, CollectionItemImageFile, ImageFile } from "@/types/media-and-posters/sets";
import type { TYPE_DB_SET_TYPE_OPTIONS } from "@/types/ui-options";

// What is used to save a record into the database
// This contains the MediaItem details, as well as an array of PosterSets that are associated with it
export interface DBSavedItem {
  //tmdb_id: string;
  //library_title: string;
  media_item: MediaItem;
  poster_sets: DBPosterSetDetail[];
  // A collection's own poster and backdrop, applied to the media server collection by the Download Queue. They aren't saved.
  collection_images?: { collection: CollectionItem; images: CollectionItemImageFile[] };
}

export interface DBPosterSetDetail extends PosterSet {
  last_downloaded: string;
  selected_types: SelectedTypes;
  auto_download: boolean;
  auto_add_new_collection_items: boolean;
  no_auto_upgrade?: boolean; // Keep this set, Automatically Upgrade Sets doesn't replace it
  to_delete: boolean; // Flag to indicate if the poster set should be deleted (Not used in DB)
}

export interface PosterSet extends Omit<BaseSetInfo, "type"> {
  type: TYPE_DB_SET_TYPE_OPTIONS;
  images: ImageFile[];
}
