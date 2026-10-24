"use client";

import type { CollectionItem } from "@/app/collections/page";
import { formatExactDateTime } from "@/helper/format-date-last-updates";
import { formatDownloadSize } from "@/helper/format-download-size";
import { downloadImageFileForMediaItem } from "@/services/downloads/download-image";
import { decode, isBlurhashValid } from "blurhash";
import { Download } from "lucide-react";
import { toast } from "sonner";

import { useMemo, useState } from "react";

import Image from "next/image";

import { Skeleton } from "@/components/ui/skeleton";

import { cn } from "@/lib/cn";
import { type AspectRatio, getAspectRatioClass, getImageSizes } from "@/lib/image-sizes";
import { log } from "@/lib/logger";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { CollectionItemImageFile, ImageFile, IncludedItem } from "@/types/media-and-posters/sets";

interface AssetImageProps {
  image: ImageFile | MediaItem | CollectionItem | string | CollectionItemImageFile;
  imageType: "item" | "collection" | "mediux" | "url";
  aspect?: AspectRatio;
  className?: string;
  imageClassName?: string;
  priority?: boolean;
  matchedToItem?: boolean;
  includedItems?: { [key: string]: IncludedItem };
}

/**
 * Decodes a blurhash string to a data URL using canvas
 * @param blurhash The blurhash string (3x3 components, ~15-20 chars)
 * @returns Data URL string ready for blurDataURL prop, or undefined on error
 */
function decodeBlurhashToDataURL(blurhash: string): string | undefined {
  try {
    // Debug: Warn if blurhash is longer than expected for 3x3 components (~15-20 chars)
    if (blurhash.length > 30) {
      log(
        "WARN",
        "AssetImage",
        "decodeBlurhashToDataURL",
        `Large blurhash detected (${blurhash.length} chars). Expected 3x3 components (~15-20 chars).`
      );
    }

    // Some blurhash strings from MediUX are malformed, so skip the blur placeholder for those
    const validation = isBlurhashValid(blurhash);
    if (!validation.result) {
      log("INFO", "AssetImage", "decodeBlurhashToDataURL", `Skipping invalid blurhash: ${validation.errorReason}`);
      return undefined;
    }

    // Decode blurhash to pixel data (2x2 for absolute minimal size)
    // For blur placeholders, very small dimensions are sufficient
    const width = 2;
    const height = 2;
    const pixels = decode(blurhash, width, height);

    // Create canvas and draw pixels
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext("2d");
    if (!ctx) {
      throw new Error("Failed to get canvas context");
    }

    // Create ImageData from pixels and draw to canvas
    const imageData = ctx.createImageData(width, height);
    imageData.data.set(pixels);
    ctx.putImageData(imageData, 0, 0);

    // PNG is typically smaller than JPEG for very small images
    return canvas.toDataURL("image/png");
  } catch (error) {
    log("ERROR", "AssetImage", "decodeBlurhashToDataURL", "Failed to decode blurhash", error);
    return undefined;
  }
}

export function AssetImage({
  image,
  imageType,
  aspect = "poster",
  className,
  imageClassName,
  priority = false,
  matchedToItem = false,
  includedItems,
}: AssetImageProps) {
  const [imageLoaded, setImageLoaded] = useState(false);
  const [imageError, setImageError] = useState(false);

  const showDateModified = useUserPreferencesStore((state) => state.showDateModified);

  const [showInfo, setShowInfo] = useState(false);

  // Decode blurhash string to data URL client-side
  const blurDataURL = useMemo(() => {
    const blurhash = typeof image === "object" && "blurhash" in image ? image.blurhash : undefined;
    if (!blurhash) return undefined;
    return decodeBlurhashToDataURL(blurhash);
  }, [image]);

  let imageSrc = "";
  if (imageType === "url") {
    imageSrc = image as string;
  } else if (imageType === "mediux") {
    imageSrc = `/api/images/mediux/item?asset_id=${(image as ImageFile).id}&modified_date=${(image as ImageFile).modified}`;
  } else if (imageType === "item") {
    imageSrc = `/api/images/media/item?rating_key=${(image as MediaItem).rating_key}&image_type=${aspect}`;
  } else if (imageType === "collection") {
    imageSrc = `/api/images/media/collection?rating_key=${(image as CollectionItem).rating_key}&image_type=${aspect}&index=${(image as CollectionItem).index}`;
  } else {
    imageSrc = "";
  }

  const handleDoubleClick = () => {
    if (imageType === "mediux") {
      setShowInfo((prev) => !prev);
    }
  };

  const handleDownloadClick = async () => {
    if (imageType !== "mediux") return;
    if (!isMediuxImage(image)) return;
    if (!includedItems || includedItems[image.item_tmdb_id] === undefined) return;

    const downloadName =
      image.type === "poster"
        ? includedItems[image.item_tmdb_id].media_item.title + " Poster"
        : image.type === "backdrop"
          ? includedItems[image.item_tmdb_id].media_item.title + " Backdrop"
          : image.type === "season_poster" && image.season_number !== undefined
            ? includedItems[image.item_tmdb_id].media_item.title + ` Season ${image.season_number} Poster`
            : image.type === "special_season_poster"
              ? includedItems[image.item_tmdb_id].media_item.title + " Special Season Poster"
              : image.type === "titlecard" && image.title
                ? includedItems[image.item_tmdb_id].media_item.title +
                  ` S${image.season_number}E${image.episode_number} Titlecard`
                : `Image_${image.id}`;

    toast.loading(`Downloading ${downloadName}...`, { id: "download", duration: 4000 });
    try {
      const resp = await downloadImageFileForMediaItem(
        image,
        includedItems[image.item_tmdb_id].media_item,
        downloadName
      );
      if (resp.status === "error") {
        throw new Error(resp.error?.message || "Unknown error");
      }
      toast.success(`Downloaded ${downloadName} successfully!`, { id: "download", duration: 4000 });
    } catch (error) {
      toast.error(`Failed to download ${downloadName}: ${error instanceof Error ? error.message : "Unknown error"}`, {
        id: "download",
        duration: 4000,
      });
    }
  };

  const isMediuxImage = (img: AssetImageProps["image"]): img is ImageFile =>
    typeof img === "object" && img !== null && "type" in img;

  const imageContent = (
    <>
      {!imageError ? (
        <Image
          src={imageSrc}
          alt={typeof image === "string" ? `${image} ${aspect}` : "id" in image ? image.id : ""}
          fill
          sizes={getImageSizes(aspect)}
          className={cn(
            "object-cover",
            "border border-transparent hover:border-primary-dynamic/30",
            "rounded-sm",
            "transition-all duration-300",
            imageClassName
          )}
          unoptimized
          loading={priority ? "eager" : "lazy"}
          fetchPriority={priority ? "high" : undefined}
          draggable={false}
          style={{ userSelect: "none" }}
          placeholder={blurDataURL ? "blur" : undefined}
          blurDataURL={blurDataURL}
          onLoad={() => setImageLoaded(true)}
          onError={() => setImageError(true)}
        />
      ) : (
        <div
          className={cn(
            "flex items-center justify-center w-full h-full bg-muted text-muted-foreground",
            getAspectRatioClass(aspect)
          )}
        >
          <div className="flex flex-col items-center">
            <span className="text-xs">No Image Available</span>
            <Image
              src="/aura_logo.svg"
              alt="Aura Logo"
              width={40}
              height={40}
              className="mt-1 opacity-70"
              draggable={false}
            />
          </div>

          <Skeleton className={cn("absolute inset-0 rounded-md animate-pulse", getAspectRatioClass(aspect))} />
        </div>
      )}
      {/* Overlay that fades out when image loads, revealing the sharp image underneath */}
      {blurDataURL && !imageLoaded && !imageError && (
        <Skeleton className={cn("absolute inset-0 rounded-md animate-pulse", getAspectRatioClass(aspect))} />
      )}
    </>
  );

  return (
    <div className={className}>
      <div
        className={cn(
          "relative overflow-hidden rounded-md border border-primary-dynamic/40 hover:border-primary-dynamic transition-all duration-300 group",
          getAspectRatioClass(aspect)
        )}
        onDoubleClick={
          matchedToItem &&
          imageType === "mediux" &&
          includedItems &&
          isMediuxImage(image) &&
          includedItems[image.item_tmdb_id] &&
          !image.type.startsWith("collection")
            ? handleDoubleClick
            : undefined
        }
      >
        {imageContent}

        {imageType === "mediux" &&
          showInfo &&
          isMediuxImage(image) &&
          includedItems &&
          includedItems[image.item_tmdb_id] && (
            <div className="absolute inset-0 flex items-center justify-center bg-black/65 text-[10px] leading-snug text-white">
              <div className="w-full max-w-[90%] max-h-[85%] overflow-y-auto scrollbar-hide rounded-md border border-white/15 bg-black/60 p-2 backdrop-blur-sm">
                <div className="mb-1 flex items-center justify-between">
                  <span className="text-[10px] uppercase tracking-widest text-white/70">Image Info</span>
                  <Download
                    className="h-3 w-3 md:h-4 md:w-4 lg:h-5 lg:w-5"
                    onClick={() => {
                      handleDownloadClick();
                    }}
                  />
                </div>

                <div className="space-y-0">
                  <div className="text-white">
                    {image.type === "poster"
                      ? includedItems[image.item_tmdb_id].media_item.title + " Poster"
                      : image.type === "backdrop"
                        ? includedItems[image.item_tmdb_id].media_item.title + " Backdrop"
                        : ""}
                  </div>

                  {image.type === "season_poster" && image.season_number !== undefined && (
                    <div className="text-white">Season {image.season_number} Poster</div>
                  )}
                  {image.type === "special_season_poster" && <div className="text-white">Special Season Poster</div>}
                  {image.type === "titlecard" && image.title ? (
                    <div className="text-white">
                      Season {image.season_number} Episode {image.episode_number}: {image.title} Titlecard
                    </div>
                  ) : null}

                  <div className="mt-2 text-white/70">Last modified</div>
                  <div className="text-white">{image.modified ? formatExactDateTime(image.modified) : "Unknown"}</div>

                  <div className="mt-2 text-white/70">File size</div>
                  <div className="text-white">{image.file_size ? formatDownloadSize(image.file_size) : "Unknown"}</div>
                </div>
              </div>
            </div>
          )}
      </div>
      {imageType === "mediux" && showDateModified && (
        <div className="mt-1 text-xs text-white/80 text-center w-full">
          {typeof image === "object" && "modified" in image && image.modified
            ? formatExactDateTime(image.modified)
            : ""}
        </div>
      )}
    </div>
  );
}
