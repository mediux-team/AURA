import { setRefsToFormItems, sortFormItems } from "@/helper/download-modal/set-to-form-item";
import { formatLastUpdatedDate } from "@/helper/format-date-last-updates";
import { CircleAlert, Database, User, ZoomInIcon } from "lucide-react";

import Link from "next/link";
import { useRouter } from "next/navigation";

import { CarouselDisplay } from "@/components/shared/carousel-display";
import DownloadModal, { type FormItemDisplay } from "@/components/shared/download-modal";
import { SetFileCounts } from "@/components/shared/set-file-counts";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Carousel, CarouselContent } from "@/components/ui/carousel";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Lead } from "@/components/ui/typography";

import { cn } from "@/lib/cn";
import { useMediaStore } from "@/lib/stores/global-store-media-store";
import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";
import { usePosterSetsStore } from "@/lib/stores/global-store-poster-sets";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";

import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { CollectionItemImageFile, IncludedItem, SetRef } from "@/types/media-and-posters/sets";

type MediaCarouselProps = {
  set: SetRef;
  includedItems?: { [tmdb_id: string]: IncludedItem };
  mediaItem: MediaItem;
  onMediaItemChange?: (item: MediaItem) => void;
  dimNotFound?: boolean;
  otherLibraryMediaItems?: MediaItem[];
};

export function MediaCarousel({
  set,
  includedItems,
  mediaItem,
  onMediaItemChange,
  dimNotFound,
  otherLibraryMediaItems,
}: MediaCarouselProps) {
  const router = useRouter();

  const { setPosterSets, setIncludedItems, setSetBaseInfo } = usePosterSetsStore();
  const { setMediaItem } = useMediaStore();
  const goToSetPage = () => {
    setPosterSets([set]);
    setIncludedItems(includedItems || {});
    setSetBaseInfo({
      id: set.id,
      title: set.title,
      type: set.type,
      user_created: set.user_created,
      date_created: set.date_created,
      date_updated: set.date_updated,
      popularity: set.popularity,
      popularity_global: set.popularity_global,
    });
    setMediaItem(mediaItem);
    router.push(`/sets/${set.id}`);
  };

  const { setSearchQuery } = useSearchQueryStore();

  const baseFormItems = setRefsToFormItems([set], includedItems || {});

  // Silo uses one item for every library it is in, so images apply to all libraries at once.
  // Only show one entry, and list the libraries it will apply to instead.
  const mediaServerType = useOnboardingStore((state) => state.status?.current_setup?.media_server?.type);
  const sharesItemsAcrossLibraries = mediaServerType === "Silo";
  const sharedLibraryTitles = sharesItemsAcrossLibraries
    ? [
        mediaItem.library_title,
        ...(otherLibraryMediaItems ?? [])
          .filter((otherItem) => otherItem.rating_key === mediaItem.rating_key)
          .map((otherItem) => otherItem.library_title),
      ]
    : [];

  const crossLibraryFormItems: FormItemDisplay[] = (
    sharesItemsAcrossLibraries ? [] : (otherLibraryMediaItems ?? [])
  ).flatMap((otherItem) =>
    baseFormItems
      .filter((item) => item.MediaItem.tmdb_id === otherItem.tmdb_id)
      .map((item) => ({ MediaItem: otherItem, Set: { ...item.Set } }))
  );
  // The item the page is for first, then the same item in other libraries, then the rest newest first
  const downloadModalFormItems = sortFormItems([...baseFormItems, ...crossLibraryFormItems], mediaItem);

  // A collection set's own poster and backdrop, for the media server collection they apply to
  const collectionImages = set.images.filter(
    (img): img is CollectionItemImageFile => img.type === "collection_poster" || img.type === "collection_backdrop"
  );

  return (
    <Carousel
      opts={{
        align: "start",
        dragFree: true,
        slidesToScroll: "auto",
      }}
      className="w-full"
    >
      <div className="flex flex-col">
        <div className="flex flex-row items-center justify-between mb-1">
          <Link
            href={`/sets/${set.id}`}
            className="text-primary-dynamic hover:text-primary cursor-pointer text-md font-semibold ml-1 w-3/4"
            onClick={(e) => {
              e.stopPropagation();
              goToSetPage();
            }}
          >
            {set.title}
          </Link>
          <div className={cn("ml-auto flex space-x-2", set.title && set.title.length > 29 && "mb-5 xs:mb-0")}>
            {mediaItem && mediaItem.db_saved_sets && mediaItem.db_saved_sets.length > 0 && (
              <Popover modal={true}>
                <PopoverTrigger>
                  <Database
                    className={cn(
                      "h-5 w-5 sm:h-7 sm:w-7  ml-2 cursor-pointer",
                      mediaItem.db_saved_sets.some((dbSet) => dbSet.id === set.id)
                        ? "text-green-500"
                        : "text-yellow-500"
                    )}
                  />
                </PopoverTrigger>
                <PopoverContent
                  className={cn(
                    "max-w-[400px] rounded-lg shadow-lg border-2 p-2 flex flex-col items-center justify-center",
                    mediaItem.db_saved_sets.some((dbSet) => dbSet.id === set.id)
                      ? "border-green-800"
                      : "border-yellow-800"
                  )}
                >
                  <div className="flex items-center mb-2">
                    <CircleAlert className="h-5 w-5 text-yellow-500 mr-2" />
                    <span className="text-sm text-muted-foreground">
                      This media item already exists in your database
                    </span>
                  </div>
                  <div className="text-xs text-muted-foreground mb-2">
                    You have previously saved it in the following sets
                  </div>
                  <ul className="space-y-2">
                    {mediaItem.db_saved_sets.map((dbSet) => (
                      <li key={dbSet.id} className="flex items-center rounded-md px-2 py-1 shadow-sm">
                        <Button
                          variant="outline"
                          className={cn(
                            "flex items-center transition-colors rounded-md px-2 py-1 cursor-pointer text-sm",
                            dbSet.id.toString() === set.id.toString()
                              ? "text-green-600  hover:bg-green-100  hover:text-green-600"
                              : "text-yellow-600 hover:bg-yellow-100 hover:text-yellow-700"
                          )}
                          aria-label={`View saved set ${dbSet.id} ${dbSet.user_created ? `by ${dbSet.user_created}` : ""}`}
                          onClick={(e) => {
                            e.stopPropagation();
                            setSearchQuery(
                              `${mediaItem.title} Y:${mediaItem.year}: ID:${mediaItem.tmdb_id}: L:${mediaItem.library_title}:`
                            );
                            router.push("/saved-sets");
                          }}
                        >
                          Set ID: {dbSet.id}
                          {dbSet.user_created ? ` by ${dbSet.user_created}` : ""}
                        </Button>
                      </li>
                    ))}
                  </ul>
                </PopoverContent>
              </Popover>
            )}
            <Link
              href={`/sets/${set.id}`}
              className="btn"
              onClick={(e) => {
                e.stopPropagation();
                goToSetPage();
              }}
            >
              <ZoomInIcon className="h-5 w-5 sm:h-7 sm:w-7 cursor-pointer active:scale-95 hover:text-primary" />
            </Link>
            <DownloadModal
              baseSetInfo={set}
              formItems={downloadModalFormItems}
              sharedLibraryTitles={sharedLibraryTitles}
              onDownloadComplete={onMediaItemChange}
              collectionImages={
                set.collection && collectionImages.length > 0
                  ? { collection: set.collection, images: collectionImages }
                  : undefined
              }
            />
          </div>
        </div>
        <div className="text-md text-muted-foreground mb-1 flex items-center">
          <div className="ml-1 flex items-center gap-1">
            <Avatar className="rounded-lg mr-1 w-4 h-4">
              <AvatarImage src={`/api/images/mediux/avatar?username=${set.user_created}`} className="w-4 h-4" />
              <AvatarFallback className="">
                <User className="w-4 h-4" />
              </AvatarFallback>
            </Avatar>
            <Link
              href={`/user/${set.user_created}`}
              className="text-sm hover:text-primary cursor-pointer underline truncate"
              style={{ wordBreak: "break-word" }}
            >
              {set.user_created}
            </Link>
          </div>
        </div>
        <Lead className="text-sm text-muted-foreground flex items-center mb-1 ml-1">
          Last Update: {formatLastUpdatedDate(set.date_updated, set.date_created)}
        </Lead>

        {mediaItem && mediaItem.rating_key && mediaItem.title && (
          <SetFileCounts mediaItem={mediaItem} set={set} includedItems={includedItems || {}} />
        )}
      </div>

      <CarouselContent>
        <CarouselDisplay
          sets={[set] as SetRef[]}
          includedItems={includedItems || {}}
          dimNotFound={dimNotFound}
          currentTmdbId={mediaItem.tmdb_id}
        />
      </CarouselContent>
    </Carousel>
  );
}
