package ej

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"aura/utils/httpx"
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
)

type EmbyJellyItemImagesResponse struct {
	ImageType  string `json:"ImageType"`
	ImageIndex int    `json:"ImageIndex,omitempty"`
	ImageTag   string `json:"ImageTag"`       // Used in Jellyfin responses
	ImagePath  string `json:"Path,omitempty"` // Used in Emby responses
}

func getCurrentImages(ctx context.Context, msConfig config.Config_MediaServer, item *models.MediaItem, itr string) ([]EmbyJellyItemImagesResponse, logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Getting %s Images for %s",
		msConfig.Type, itr, utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	var images []EmbyJellyItemImagesResponse
	Err := logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return images, *logAction.Error
	}
	u.Path = path.Join(u.Path, "Items", item.RatingKey, "Images")
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, msConfig, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return images, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &images, fmt.Sprintf("%s Media Item Images Response", msConfig.Type))
	if Err.Message != "" {
		return images, Err
	}

	logAction.AppendResult("image_count", len(images))
	logAction.AppendResult("images", images)
	return images, Err
}

func getCurrentCollectionImages(ctx context.Context, msConfig config.Config_MediaServer, collectionItem *models.CollectionItem, itr string) ([]EmbyJellyItemImagesResponse, logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Getting %s Images for Collection %s",
		msConfig.Type, itr, utils.CollectionItemInfo(*collectionItem),
	), logging.LevelDebug)
	defer logAction.Complete()

	var images []EmbyJellyItemImagesResponse
	Err := logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return images, *logAction.Error
	}
	u.Path = path.Join(u.Path, "Items", collectionItem.RatingKey, "Images")
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, msConfig, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return images, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &images, fmt.Sprintf("%s Collection Images Response", msConfig.Type))
	if Err.Message != "" {
		return images, Err
	}

	logAction.AppendResult("image_count", len(images))
	logAction.AppendResult("images", images)
	return images, Err
}

func findNewImage(ctx context.Context, currentImages, newImages []EmbyJellyItemImagesResponse) (newImage EmbyJellyItemImagesResponse) {
	for _, updatedImage := range newImages {
		if updatedImage.ImageType == "" || updatedImage.ImageType != "Backdrop" {
			continue
		}
		found := false
		for _, currentImage := range currentImages {
			if updatedImage.ImageType == currentImage.ImageType &&
				updatedImage.ImageIndex == currentImage.ImageIndex &&
				((updatedImage.ImageTag != "" && updatedImage.ImageTag == currentImage.ImageTag) ||
					(updatedImage.ImagePath != "" && updatedImage.ImagePath == currentImage.ImagePath)) {
				found = true
				break
			}
		}
		if !found && updatedImage.ImageType == "Backdrop" {
			return updatedImage
		}
	}
	return EmbyJellyItemImagesResponse{}
}

func updateImageIndex(ctx context.Context, msConfig config.Config_MediaServer, item *models.MediaItem, image EmbyJellyItemImagesResponse) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Updating Image Index for Backdrop Image on %s",
		msConfig.Type, utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "Items", item.RatingKey, "Images", "Backdrop", fmt.Sprintf("%d", image.ImageIndex), "Index")
	query := u.Query()
	query.Set("newIndex", "0")
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, _, Err := makeRequest(ctx, msConfig, URL, "POST", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	defer resp.Body.Close()

	logAction.AppendResult("message", fmt.Sprintf("Image index changed to 0 for item '%s'", item.Title))
	return Err
}

func updateCollectionImageIndex(ctx context.Context, msConfig config.Config_MediaServer, collectionItem *models.CollectionItem, image EmbyJellyItemImagesResponse) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Updating Image Index for Collection Backdrop Image on Collection %s",
		msConfig.Type, utils.CollectionItemInfo(*collectionItem),
	), logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "Items", collectionItem.RatingKey, "Images", "Backdrop", fmt.Sprintf("%d", image.ImageIndex), "Index")
	query := u.Query()
	query.Set("newIndex", "0")
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, _, Err := makeRequest(ctx, msConfig, URL, "POST", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	defer resp.Body.Close()

	logAction.AppendResult("message", fmt.Sprintf("Image index changed to 0 for collection '%s'", collectionItem.Title))
	return Err
}

func uploadImage(ctx context.Context, msConfig config.Config_MediaServer, item *models.MediaItem, itemRatingKey string, imageFile models.ImageFile, imageData []byte, imageType string) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Uploading %s Image for %s",
		msConfig.Type, utils.GetFileDownloadName(item.Title, imageFile), utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	posterType := "Primary"
	if imageFile.Type == "backdrop" {
		posterType = "Backdrop"
	}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "Items", itemRatingKey, "Images", posterType)
	URL := u.String()

	// Encode the image data as Base64
	base64ImageData := base64.StdEncoding.EncodeToString(imageData)

	// Make the HTTP Request to EJ
	resp, _, Err := makeRequest(ctx, msConfig, URL, "POST", []byte(base64ImageData), imageType)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return Err
	}
	defer resp.Body.Close()

	return Err
}

func uploadCollectionImage(ctx context.Context, msConfig config.Config_MediaServer, collectionItem *models.CollectionItem, imageFile models.ImageFile, imageData []byte, imageType string) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"%s: Uploading %s Image for Collection %s",
		msConfig.Type, utils.GetFileDownloadName(collectionItem.Title, imageFile), utils.CollectionItemInfo(*collectionItem),
	), logging.LevelDebug)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	var posterType string
	if imageFile.Type == "collection_backdrop" {
		posterType = "Backdrop"
	} else {
		posterType = "Primary"
	}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "Items", collectionItem.RatingKey, "Images", posterType)
	URL := u.String()

	// Encode the image data as Base64
	base64ImageData := base64.StdEncoding.EncodeToString(imageData)

	// Make the HTTP Request to EJ
	resp, _, Err := makeRequest(ctx, msConfig, URL, "POST", []byte(base64ImageData), imageType)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	defer resp.Body.Close()

	return Err
}
