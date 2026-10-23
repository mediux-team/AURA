package plex

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"net/url"
	"path"
	"time"
)

func (p *Plex) RefreshMediaItemMetadata(ctx context.Context, item *models.MediaItem, refreshKey string, updateImage bool) (Err logging.LogErrorInfo) {
	return RefreshItemMetadata(ctx, p.Config, item, refreshKey, updateImage)
}

func RefreshItemMetadata(ctx context.Context, msConfig config.Config_MediaServer, item *models.MediaItem, refreshKey string, updateImage bool) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Refreshing Metadata for %s (Refresh Key: %s)",
		utils.MediaItemInfo(*item),
		refreshKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	// Construct the URL for the Plex API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "metadata", refreshKey, "refresh")
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, _, Err := makeRequest(ctx, msConfig, URL, "PUT", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	defer resp.Body.Close()

	if !updateImage {
		time.Sleep(200 * time.Millisecond) // Give Plex a moment to process the refresh before any further actions
		return logging.LogErrorInfo{}
	}

	// When we refresh metadata, we can also update the selected image to be local (if present)
	// We run this in a separate goroutine to avoid blocking
	currentImages, Err := getAllImages(ctx, msConfig, item, refreshKey, "All", "poster")
	if Err.Message != "" {
		return Err
	}

	if len(currentImages) > 0 {
		hasSelectedImage := false
		for _, img := range currentImages {
			if img.Selected {
				hasSelectedImage = true
				break
			}
		}
		if !hasSelectedImage {
			hasLocal := false
			// If there is no selected image, select a local one if it exists, else select the first one
			for _, img := range currentImages {
				if img.Provider == "local" {
					Err = applyImageToMediaItem(ctx, msConfig, item, refreshKey, img.RatingKey, "poster")
					if Err.Message != "" {
						return Err
					}
					hasLocal = true
					break
				}
			}
			if !hasLocal {
				Err = applyImageToMediaItem(ctx, msConfig, item, refreshKey, currentImages[0].RatingKey, "poster")
				if Err.Message != "" {
					return Err
				}
			}
		}
	}

	time.Sleep(500 * time.Millisecond) // Give Plex a moment to process the refresh before any further actions
	return logging.LogErrorInfo{}
}
