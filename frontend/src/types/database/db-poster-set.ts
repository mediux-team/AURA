import type { CollectionItem } from "@/app/collections/page";

import type { MediaItem, SelectedTypes } from "@/types/media-and-posters/media-item-and-library";
import type { BaseSetInfo, CollectionItemImageFile, ImageFile, SetSummary } from "@/types/media-and-posters/sets";
import type { TYPE_DB_SET_TYPE_OPTIONS } from "@/types/ui-options";

// What is used to save a record into the database
// This contains the MediaItem details, as well as an array of PosterSets that are associated with it
export interface DBSavedItem {
  //tmdb_id: string;
  //library_title: string;
  media_item: MediaItem;
  poster_sets: DBPosterSetDetail[];
  source?: string; // What added it to the Download Queue, empty when it was the user
  // A collection's own poster and backdrop, applied to the media server collection by the Download Queue. They aren't saved.
  collection_images?: { collection: CollectionItem; images: CollectionItemImageFile[]; set?: SetSummary };
}

export interface DBPosterSetDetail extends PosterSet {
  last_downloaded: string;
  selected_types: SelectedTypes;
  auto_download: boolean;
  auto_add_new_collection_items: boolean;
  no_auto_upgrade?: boolean; // Keep this set, Automatically Upgrade Sets doesn't replace it
  to_delete: boolean; // Flag to indicate if the poster set should be deleted (Not used in DB)
  only_images?: QueuedImage[]; // The images a Download Queue job downloads, when it updates only some of the set
}

// One of a set's images for a Download Queue job to download, and why
export interface QueuedImage {
  id: string;
  type: string;
  reason_title?: string;
  reason?: string;
}

export interface PosterSet extends Omit<BaseSetInfo, "type"> {
  type: TYPE_DB_SET_TYPE_OPTIONS;
  images: ImageFile[];
}
