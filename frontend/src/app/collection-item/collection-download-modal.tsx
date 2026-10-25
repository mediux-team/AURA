"use client";

import type { CollectionItem } from "@/app/collections/page";
import { formatDownloadSize } from "@/helper/format-download-size";
import { DownloadImageFileForCollectionItem } from "@/services/downloads/download-collection-image";
import { zodResolver } from "@hookform/resolvers/zod";
import { Check, Download, LoaderIcon, User, X } from "lucide-react";
import { z } from "zod";

import { useEffect, useMemo, useRef, useState } from "react";
import React from "react";
import { useForm, useWatch } from "react-hook-form";

import Link from "next/link";

import { AssetImage } from "@/components/shared/asset-image";
import { DownloadProgressBar } from "@/components/shared/download-progress-bar";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Form, FormControl, FormField, FormItem, FormLabel } from "@/components/ui/form";

import { cn } from "@/lib/cn";
import { log } from "@/lib/logger";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import type { CollectionItemImageFile, CollectionItemSetRef } from "@/types/media-and-posters/sets";
import {
  DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS,
  TYPE_DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS,
} from "@/types/ui-options";

export interface CollectionsDownloadModalProps {
  item: CollectionItem;
  set: CollectionItemSetRef;
}

const downloadSchema = z.object({
  poster: z.boolean(),
  backdrop: z.boolean(),
});

const CollectionsDownloadModal: React.FC<CollectionsDownloadModalProps> = ({ item, set }) => {
  const [isMounted, setIsMounted] = useState(false);

  // Cancel Button Text
  const [cancelButtonText, setCancelButtonText] = useState<string>("Cancel");

  // Download Button Text
  const [downloadButtonText, setDownloadButtonText] = useState<string>("Download");

  const [progress, setProgress] = useState<{
    poster_progress?: string;
    backdrop_progress?: string;
  }>({});

  const [errors, setErrors] = useState<{
    poster_error?: string;
    backdrop_error?: string;
  }>({});

  // User Preferences
  const { downloadDefaults } = useUserPreferencesStore();

  const posterImage = set.images.find((img) => img.type === "collection_poster") || null;
  const backdropImage = set.images.find((img) => img.type === "collection_backdrop") || null;

  const form = useForm({
    resolver: zodResolver(downloadSchema),
    defaultValues: {
      poster: downloadDefaults.includes("poster") && !!posterImage,
      backdrop: downloadDefaults.includes("backdrop") && !!backdropImage,
    },
  });

  // Reset form on mount
  useEffect(() => {
    form.reset({
      poster: downloadDefaults.includes("poster") && !!posterImage,
      backdrop: downloadDefaults.includes("backdrop") && !!backdropImage,
    });
  }, [form, downloadDefaults, posterImage, backdropImage]);

  const selectedValues = useWatch({ control: form.control });

  const numberOfImagesSelected = useMemo(() => {
    let count = 0;
    if (selectedValues.poster) count += 1;
    if (selectedValues.backdrop) count += 1;
    return count;
  }, [selectedValues.poster, selectedValues.backdrop]);

  const sizeOfImagesSelected = useMemo(() => {
    let size = 0;
    if (selectedValues.poster && posterImage) size += posterImage.file_size || 0;
    if (selectedValues.backdrop && backdropImage) size += backdropImage.file_size || 0;
    return formatDownloadSize(size);
  }, [selectedValues.poster, selectedValues.backdrop, posterImage, backdropImage]);

  // Function - Close Modal
  const handleClose = () => {
    form.reset();
    // Reset progress and errors
    setProgress({});
    setErrors({});
    resetProgressValues();
    progressDownloadRef.current = 0;
  };

  type ProgressValues = {
    value: number;
  };

  // Download Progress
  const [progressValues, setProgressValues] = useState<ProgressValues>({
    value: 0,
  });

  const progressRef = useRef(0);
  const progressIncrementRef = useRef(0);
  const progressDownloadRef = useRef(0);

  const updateProgressValue = (value: number) => {
    // Update the ref immediately
    progressRef.current = Math.min(value, 100);

    // Update the state
    setProgressValues((prev) => ({
      ...prev,
      value: progressRef.current,
    }));
  };

  const resetProgressValues = () => {
    progressRef.current = 0;
    progressIncrementRef.current = 0;
    progressDownloadRef.current = 0;
    setProgressValues({
      value: 0,
    });
  };

  const downloadImageFileAndApply = async (
    imageType: TYPE_DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS,
    collectionItem: CollectionItem,
    imageFile: CollectionItemImageFile
  ) => {
    let isDone = false;
    let interval: NodeJS.Timeout | undefined;
    const imageLabel =
      DOWNLOAD_COLLECTION_IMAGE_TYPE_OPTIONS.find((opt) => opt.value === imageType)?.label || imageType;

    try {
      setProgress((prev) => ({
        ...prev,
        [`${imageType}_progress`]: `Downloading ${imageLabel}...`,
      }));

      const targetProgress = progressRef.current + progressIncrementRef.current;
      let currentProgress = progressRef.current;

      // Start progress animation
      interval = setInterval(() => {
        if (currentProgress < targetProgress && !isDone) {
          currentProgress += 1;
          updateProgressValue(currentProgress);
        }
      }, 50);

      const response = await DownloadImageFileForCollectionItem(imageType, collectionItem, imageFile, {
        id: set.id,
        title: set.title,
        type: set.type,
        user_created: set.user_created,
      });
      if (response.status === "error") {
        throw new Error(response.error?.message || `Unknown error downloading ${imageLabel}`);
      }

      // Mark as done and set progress to target
      isDone = true;
      clearInterval(interval);
      updateProgressValue(targetProgress);

      setProgress((prev) => ({
        ...prev,
        [`${imageType}_progress`]: `Downloaded ${imageLabel}`,
      }));
    } catch {
      isDone = true;
      if (interval) clearInterval(interval);
      setProgress((prev) => ({
        ...prev,
        [`${imageType}_progress`]: `Failed to download ${imageLabel}`,
      }));
      setErrors((prev) => ({
        ...prev,
        [`${imageType}_error`]: `Failed to download ${imageLabel}.`,
      }));
    } finally {
      updateProgressValue(progressRef.current + progressIncrementRef.current);
    }
  };

  const onSubmit = async (data: z.infer<typeof downloadSchema>) => {
    if (isMounted) return;

    if (!data.poster && !data.backdrop) {
      return;
    }

    setIsMounted(true);

    try {
      // Reset errors and progress
      setErrors({});
      setProgress({});
      resetProgressValues();
      progressDownloadRef.current = 0;

      log("INFO", "Collections Download Modal", "Debug Info", "Form submitted with data:", {
        data,
        item,
        set,
      });
      log("INFO", "Download Modal", "Debug Info", "Logging progress values:", progressValues);
      updateProgressValue(1);

      // Progress Increment Calculation
      const totalItemsToDownload = (data.poster && posterImage ? 1 : 0) + (data.backdrop && backdropImage ? 1 : 0);
      progressIncrementRef.current = 95 / (totalItemsToDownload * 2); // Multiply by 2 for start and end progress

      if (data.poster && posterImage) {
        await downloadImageFileAndApply(posterImage.type, item, posterImage);
      }

      if (data.backdrop && backdropImage) {
        await downloadImageFileAndApply(backdropImage.type, item, backdropImage);
      }

      updateProgressValue(100);
    } catch (error) {
      log("ERROR", "Collections Download Modal", "Debug Info", "Download Error:", error);
    } finally {
      setIsMounted(false);
      setDownloadButtonText("Download");
      setCancelButtonText("Close");
    }
  };

  const logInfoForPage = () => {
    log("INFO", "Collections Download Modal", "Debug Info", "Rendering with props:", {
      item,
      set,
      posterImage,
      backdropImage,
      downloadDefaults,
    });
  };

  return (
    <Dialog
      onOpenChange={(open) => {
        if (!open) {
          handleClose();
        }
      }}
    >
      <DialogTrigger asChild>
        <Download className="h-5 w-5 sm:h-7 sm:w-7 cursor-pointer active:scale-95 hover:text-primary" />
      </DialogTrigger>
      <DialogPortal>
        <DialogOverlay />
        <DialogContent
          className={cn("z-50", "max-h-[80vh] overflow-y-auto", "sm:max-w-[700px]", "border border-primary")}
        >
          <DialogHeader>
            <DialogTitle onClick={logInfoForPage}>{set.title}</DialogTitle>
            <DialogDescription>
              <div className="flex items-center justify-center sm:justify-start">
                <Avatar className="rounded-lg mr-1 w-4 h-4">
                  <AvatarImage src={`/api/images/mediux/avatar?username=${set.user_created}`} className="w-4 h-4" />
                  <AvatarFallback className="">
                    <User className="w-4 h-4" />
                  </AvatarFallback>
                </Avatar>
                <Link href={`/user/${set.user_created}`} className="hover:underline">
                  {set.user_created}
                </Link>
              </div>
            </DialogDescription>
            <DialogDescription>Library: {item.library_title}</DialogDescription>
            <DialogDescription>Collection ID: {set.id}</DialogDescription>
          </DialogHeader>

          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-2">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {posterImage && (
                  <FormField
                    control={form.control}
                    name="poster"
                    render={({ field }) => (
                      <FormItem className="flex flex-col items-center border rounded-md p-4">
                        <FormLabel className="font-medium mb-2">Poster</FormLabel>
                        <FormControl>
                          <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                        </FormControl>
                        {posterImage && (
                          <AssetImage
                            image={posterImage}
                            imageType="mediux"
                            aspect="poster"
                            className="w-32 h-auto mt-2 rounded shadow"
                          />
                        )}
                      </FormItem>
                    )}
                  />
                )}
                {backdropImage && (
                  <FormField
                    control={form.control}
                    name="backdrop"
                    render={({ field }) => (
                      <FormItem className="flex flex-col items-center border rounded-md p-4">
                        <FormLabel className="font-medium mb-2">Backdrop</FormLabel>
                        <FormControl>
                          <Checkbox checked={field.value} onCheckedChange={field.onChange} />
                        </FormControl>
                        {backdropImage && (
                          <AssetImage image={backdropImage} imageType="mediux" aspect="backdrop" className="w-full" />
                        )}
                      </FormItem>
                    )}
                  />
                )}
              </div>

              {numberOfImagesSelected > 0 ? (
                <>
                  <div className="text-sm text-muted-foreground">Number of Images: {numberOfImagesSelected}</div>
                  <div className="text-sm text-muted-foreground">Total Download Size: ~{sizeOfImagesSelected}</div>
                </>
              ) : (
                <div className="text-sm text-destructive">
                  No image types selected for download. Please select at least one image type to download.
                </div>
              )}

              {progressValues.value > 0 && (
                <div className="w-full">
                  <DownloadProgressBar
                    value={progressValues.value}
                    label={[progress.poster_progress, progress.backdrop_progress].find((status) =>
                      status?.toLowerCase().startsWith("downloading")
                    )}
                    hasErrors={!!(errors.poster_error || errors.backdrop_error)}
                  />

                  {(progress.poster_progress || progress.backdrop_progress) && (
                    <div className="space-y-1 mt-2">
                      <div className="text-xs font-semibold mb-1">Progress</div>
                      {progress.poster_progress && <div>{getProgressStatus(progress.poster_progress)}</div>}
                      {progress.backdrop_progress && <div>{getProgressStatus(progress.backdrop_progress)}</div>}
                    </div>
                  )}

                  {(errors.poster_error || errors.backdrop_error) && (
                    <div className="space-y-1 mt-2">
                      <div className="text-xs font-semibold mb-1 text-red-600">Errors</div>
                      {errors.poster_error && (
                        <div className="text-red-600 flex items-center gap-1">
                          <X className="mr-1 h-4 w-4" />
                          Poster: {errors.poster_error}
                        </div>
                      )}
                      {errors.backdrop_error && (
                        <div className="text-red-600 flex items-center gap-1">
                          <X className="mr-1 h-4 w-4" />
                          Backdrop: {errors.backdrop_error}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )}

              <DialogFooter>
                {/* Cancel button to close the modal */}
                <DialogClose asChild>
                  <Button
                    className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
                    variant="ghost"
                    onClick={() => {
                      handleClose();
                    }}
                  >
                    {cancelButtonText}
                  </Button>
                </DialogClose>
                <Button type="submit" disabled={!form.watch("poster") && !form.watch("backdrop")}>
                  {downloadButtonText}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        </DialogContent>
      </DialogPortal>
    </Dialog>
  );
};

// Helper for status styling and icon
const getProgressStatus = (status?: string) => {
  if (!status) return null;
  const lower = status.toLowerCase();
  if (lower.startsWith("downloading")) {
    return (
      <span className="flex items-center gap-1 text-muted-foreground">
        <LoaderIcon className="mr-1 animate-spin h-4 w-4" />
        {status}
      </span>
    );
  }
  if (lower.startsWith("downloaded")) {
    return (
      <span className="flex items-center gap-1 text-green-600">
        <Check className="mr-1 h-4 w-4" />
        {status}
      </span>
    );
  }
  if (lower.startsWith("failed")) {
    return (
      <span className="flex items-center gap-1 text-red-600">
        <X className="mr-1 h-4 w-4" />
        {status}
      </span>
    );
  }
  return <span>{status}</span>;
};

export default CollectionsDownloadModal;
