"use client";

import type { CollectionItem } from "@/app/collections/page";
import { Database, EyeClosedIcon, EyeOffIcon } from "lucide-react";

import React from "react";

import { useRouter } from "next/navigation";

import { AssetImage } from "@/components/shared/asset-image";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";

import { cn } from "@/lib/cn";
import { useCollectionStore } from "@/lib/stores/global-store-collection-store";
import { useMediaStore } from "@/lib/stores/global-store-media-store";

import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";

interface HomeMediaItemCardProps {
  item: MediaItem | CollectionItem;
  priority?: boolean; // Load the poster right away, for the cards at the top of the page
}

const HomeMediaItemCard: React.FC<HomeMediaItemCardProps> = ({ item, priority = false }) => {
  const router = useRouter();

  const { setMediaItem } = useMediaStore();
  const { setCollectionItem } = useCollectionStore();

  // Helper type guards
  function isMediaItem(item: MediaItem | CollectionItem): item is MediaItem {
    return "db_saved_sets" in item;
  }

  function isCollectionItem(item: MediaItem | CollectionItem): item is CollectionItem {
    return "child_count" in item && "media_items" in item;
  }

  const handleMediaItemCardClick = (mediaItem: MediaItem) => {
    setMediaItem(mediaItem);
    //router.push(formatMediaItemUrl(mediaItem));
    router.push("/media-item/");
  };

  const handleCollectionItemCardClick = (collectionItem: CollectionItem) => {
    setCollectionItem(collectionItem);
    router.push("/collection-item/");
  };

  return (
    <Card
      key={`${item.library_title}-${item.rating_key}`}
      className="relative items-center cursor-pointer border border-1 hover:shadow-xl transition-shadow p-0 rounded-xl"
      onClick={() => {
        if (isMediaItem(item)) {
          handleMediaItemCardClick(item);
        } else if (isCollectionItem(item)) {
          handleCollectionItemCardClick(item);
        }
      }}
    >
      {/* Database Existence Indicator */}
      {isMediaItem(item) && item.db_saved_sets && item.db_saved_sets.length > 0 && (
        <div className="absolute top-1 right-1 z-10 rounded-full p-1 border border-green-800">
          <Database className="text-green-500" size={20} />
        </div>
      )}
      {isMediaItem(item) && item.ignored_in_db && item.ignored_mode && (
        <div
          className={cn(
            "absolute top-1 right-1 z-10 rounded-full p-1 border",
            item.ignored_mode === "always" && "border-red-800",
            item.ignored_mode === "until-set-available" && "border-orange-800",
            item.ignored_mode === "until-new-set-available" && "border-yellow-800"
          )}
        >
          {item.ignored_mode === "always" ? (
            <EyeOffIcon className="text-red-500" size={20} />
          ) : item.ignored_mode === "until-set-available" ? (
            <EyeClosedIcon className="text-orange-500" size={20} />
          ) : (
            <EyeClosedIcon className="text-yellow-500" size={20} />
          )}
        </div>
      )}

      {/* Poster Image */}
      <AssetImage
        image={item}
        imageType={isMediaItem(item) ? "item" : "collection"}
        className="w-[100%] h-auto transition-transform hover:scale-102 rounded-xl mb-0"
        priority={priority}
      />

      {/* Badges */}
      <CardContent className="flex flex-col justify-center items-center mt-0">
        <div className="flex flex-row gap-2">
          <Badge variant="default" className="text-xs">
            {isMediaItem(item) ? item.year : `${item.child_count} items`}
          </Badge>
          {item.library_title && (
            <Badge variant="default" className="text-xs">
              {item.library_title}
            </Badge>
          )}
        </div>
        <div className="flex flex-row gap-2 mt-1">
          {isMediaItem(item) && item.edition && <Badge className="flex items-center text-xs">{item.edition}</Badge>}
        </div>
        {/* Title */}
        {item.title && (
          <span className="text-center text-md text-foreground font-semibold mt-2 mb-2">
            {item.title.length > 55 ? `${item.title.slice(0, 55)}...` : item.title}
          </span>
        )}
      </CardContent>
    </Card>
  );
};

export default HomeMediaItemCard;
