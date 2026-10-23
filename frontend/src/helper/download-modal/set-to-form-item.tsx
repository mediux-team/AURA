import type { FormItemDisplay } from "@/components/shared/download-modal";

import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { ImageFile, IncludedItem, SetRef } from "@/types/media-and-posters/sets";

const createBaseItem = (set: SetRef): FormItemDisplay => ({
  MediaItem: {} as MediaItem,
  Set: {
    id: set.id,
    title: set.title,
    type: set.type,
    user_created: set.user_created,
    date_created: set.date_created,
    date_updated: set.date_updated,
    popularity: set.popularity,
    popularity_global: set.popularity_global,
    images: [] as ImageFile[],
  },
});

// When an item was released, for sorting newest first
const releasedAt = (item: FormItemDisplay) => item.MediaItem.released_at || item.MediaItem.year || 0;

// sortFormItems puts the item the page is for first, then the same item in other libraries,
// then the rest newest first. Without a current item, every item is newest first.
export const sortFormItems = (items: FormItemDisplay[], current?: MediaItem): FormItemDisplay[] => {
  const rank = (item: FormItemDisplay) => {
    if (!current) return 2;
    if (item.MediaItem.rating_key === current.rating_key && item.MediaItem.library_title === current.library_title)
      return 0;
    return item.MediaItem.tmdb_id === current.tmdb_id ? 1 : 2;
  };
  return [...items].sort((a, b) => rank(a) - rank(b) || releasedAt(b) - releasedAt(a));
};

export const setRefsToFormItems = (
  sets: SetRef[],
  includedItems: { [tmdb_id: string]: IncludedItem }
): FormItemDisplay[] => {
  const items: FormItemDisplay[] = [];
  sets.forEach((set) => {
    if (!set.item_ids || set.item_ids.length === 0 || !includedItems || !set.images || set.images.length === 0) {
      return; // Skip sets with no item_ids or images
    }

    // Get the MediaItem info from includedItems
    // If not found, don't append this item to the list
    // We can use the first item_id to look up the MediaItem
    // Then we can find the match images that have the same item_tmdb_id

    for (const itemID of set.item_ids) {
      const includedItem = includedItems[itemID];
      if (
        includedItem &&
        includedItem.media_item &&
        includedItem.media_item.rating_key &&
        includedItem.media_item.library_title
      ) {
        const item = createBaseItem(set);
        item.MediaItem = includedItem.media_item;

        const matchedImages = set.images.filter((image) => image.item_tmdb_id === includedItem.mediux_info.tmdb_id);

        if (matchedImages.length === 0) {
          continue; // Skip this item if no images match
        }

        item.Set.images = matchedImages;

        items.push(item);
      }
    }
  });

  return sortFormItems(items);
};
