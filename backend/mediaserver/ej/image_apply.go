package ej

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
)

func applyImageToMediaItem(ctx context.Context, msConfig config.Config_MediaServer, item *models.MediaItem, imageFile models.ImageFile, imageData []byte, imageType string) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Applying %s Image for %s",
		msConfig.Type, utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Determine the Item Rating Key from Emby/Jellyfin
	itemRatingKey := getItemRatingKeyFromImageFile(*item, imageFile)
	if itemRatingKey == "" {
		logAction.SetError("Failed to determine Rating Key for Media Item", "Ensure the Media Item and Image File data are correct", nil)
		return *logAction.Error
	}

	// Handling for Backdrops are different than Primary Images
	if imageFile.Type != "backdrop" {
		// Apply the Image to the Media Item
		Err = uploadImage(ctx, msConfig, item, itemRatingKey, imageFile, imageData, imageType)
		if Err.Message != "" {
			return Err
		}
	} else {
		// For Backdrops, we need to set the index of the new upload to 0
		// To do this, we get a list of current images
		// Then we upload the new image
		// Then we get a list of current images again and find the new image's ID
		// Then we set the index of that image to 0

		// Get current images
		currentImages, Err := getCurrentImages(ctx, msConfig, item, "Current")
		if Err.Message != "" {
			return Err
		}

		// Upload the new image
		Err = uploadImage(ctx, msConfig, item, itemRatingKey, imageFile, imageData, imageType)
		if Err.Message != "" {
			return Err
		}

		if len(currentImages) != 0 {
			// Get images after upload
			updatedImages, Err := getCurrentImages(ctx, msConfig, item, "Updated")
			if Err.Message != "" {
				return Err
			}

			// Find the new image's ID
			newImage := findNewImage(ctx, currentImages, updatedImages)
			if newImage.ImageTag == "" && newImage.ImagePath == "" {
				logAction.SetError("Failed to find newly uploaded Backdrop image", "Ensure the image upload was successful", map[string]any{
					"item":           *item,
					"image_file":     imageFile,
					"current_images": currentImages,
					"updated_images": updatedImages,
				})
				return *logAction.Error
			}

			// Now we change the image index to 0, if it's not already 0
			if newImage.ImageIndex != 0 {
				err := updateImageIndex(ctx, msConfig, item, newImage)
				if err.Message != "" {
					return err
				}
			}
		}
	}

	return Err
}
