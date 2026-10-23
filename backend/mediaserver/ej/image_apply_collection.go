package ej

import (
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func (ej *EJ) ApplyCollectionImage(ctx context.Context, collectionItem *models.CollectionItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Applying %s Image to %s",
		ej.Config.Type, cases.Title(language.English).String(imageFile.Type), utils.CollectionItemInfo(*collectionItem),
	), logging.LevelDebug)
	defer logAction.Complete()

	// Get the MediUX Image Data
	formatDate := imageFile.Modified.Format("20060102150405")
	imageData, imageType, Err := mediux.GetImage(ctx, imageFile.ID, formatDate, mediux.ImageQualityOriginal)
	if Err.Message != "" {
		return Err
	}

	if imageFile.Type != "collection_backdrop" {
		// Apply the Image to the Collection
		Err = uploadCollectionImage(ctx, ej.Config, collectionItem, imageFile, imageData, imageType)
		if Err.Message != "" {
			return Err
		}
	} else {
		// For Collection Backdrops, we need to set the index of the new upload to 0
		// To do this, we get a list of current images
		// Then we upload the new image
		// Then we get a list of current images again and find the new image's ID
		// Then we set the index of that image to 0

		// Get current images
		currentImages, Err := getCurrentCollectionImages(ctx, ej.Config, collectionItem, "Current")
		if Err.Message != "" {
			return Err
		}

		// Upload the new image
		Err = uploadCollectionImage(ctx, ej.Config, collectionItem, imageFile, imageData, imageType)
		if Err.Message != "" {
			return Err
		}

		if len(currentImages) != 0 {
			// Get images after upload
			updatedImages, Err := getCurrentCollectionImages(ctx, ej.Config, collectionItem, "Updated")
			if Err.Message != "" {
				return Err
			}

			// Find the new image's ID
			newImage := findNewImage(ctx, currentImages, updatedImages)
			if newImage.ImageTag == "" && newImage.ImagePath == "" {
				logAction.SetError("Failed to find newly uploaded Backdrop image", "Ensure the image upload was successful", map[string]any{
					"item":           *collectionItem,
					"image_file":     imageFile,
					"current_images": currentImages,
					"updated_images": updatedImages,
				})
				return *logAction.Error
			}

			// Now we change the image index to 0, if it's not already 0
			if newImage.ImageIndex != 0 {
				err := updateCollectionImageIndex(ctx, ej.Config, collectionItem, newImage)
				if err.Message != "" {
					return err
				}
			}
		}
	}

	return Err

}
