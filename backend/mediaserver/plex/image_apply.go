package plex

import (
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func applyImageToMediaItemViaMediuxURL(ctx context.Context, msConfig config.Config_MediaServer, item *models.MediaItem, itemRatingKey string, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	fileDownloadName := utils.GetFileDownloadName(item.Title, imageFile)
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Applying '%s' Image via MediUX URL", fileDownloadName), logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Construct the MediUX Image URL
	imageURL, Err := mediux.ConstructImageUrl(ctx, imageFile.ID, imageFile.Modified.String(), mediux.ImageQualityOriginal)
	if Err.Message != "" {
		return Err
	}

	// Refresh the Plex Item
	//RefreshItemMetadata(ctx, msConfig, item, itemRatingKey, false)

	// Set the Poster using the MediUX URL
	Err = applyImageToMediaItem(ctx, msConfig, item, itemRatingKey, imageURL, imageFile.Type)
	if Err.Message != "" {
		return Err
	}

	return Err
}

func applyImageToMediaItem(ctx context.Context, msConfig config.Config_MediaServer, item *models.MediaItem, itemRatingKey, imageKey, imageType string) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Applying '%s' Image to %s",
		cases.Title(language.English).String(imageType), utils.MediaItemInfo(*item),
	), logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// PUT Method is used when saving images locally
	// PUT Method requires the posterType to be singular (poster or art)
	//
	// POST Method is used when not using a local image
	// POST Method requires the posterType to be plural (posters or arts)

	var requestMethod string
	if strings.HasPrefix(imageKey, "metadata://") {
		// Local asset, use PUT method
		requestMethod = "PUT"
		if imageType == "backdrop" {
			imageType = "art"
		} else {
			imageType = "poster"
		}
	} else if strings.HasPrefix(imageKey, "http://") || strings.HasPrefix(imageKey, "https://") {
		// Remote asset, use POST method
		requestMethod = "POST"
		if imageType == "backdrop" {
			imageType = "arts"
		} else {
			imageType = "posters"
		}
	} else {
		logAction.SetError("Invalid image key format", "Image key must start with 'metadata://' or 'http(s)://'", map[string]any{"imageKey": imageKey})
		return *logAction.Error
	}

	// Construct the URL for the Plex API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "metadata", itemRatingKey, imageType)
	query := u.Query()
	query.Set("url", imageKey)
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, msConfig, URL, requestMethod, nil)
	if Err.Message != "" {
		return Err
	}
	defer resp.Body.Close()

	if http.MethodPut == requestMethod {
		if !strings.HasPrefix(string(respBody), "/library/metadata/") {
			logAction.SetError("Plex Server did not return a valid response for PUT image request", "Ensure the image key is correct and accessible", nil)
			return *logAction.Error
		}
	}

	return logging.LogErrorInfo{}
}
