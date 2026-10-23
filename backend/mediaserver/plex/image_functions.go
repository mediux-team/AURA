package plex

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/url"
	"path"
)

func getAllImages(ctx context.Context, msConfig config.Config_MediaServer, item *models.MediaItem, itemRatingKey string, itr string, imageType string) (images []PlexGetAllImagesMetadata, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Getting %s Images for %s", itr, utils.MediaItemInfo(*item)),
		logging.LevelDebug)
	defer logAction.Complete()

	images = []PlexGetAllImagesMetadata{}
	Err = logging.LogErrorInfo{}

	if imageType == "backdrop" {
		imageType = "arts"
	} else {
		imageType = "posters"
	}

	// Construct the URL for the Plex API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return images, *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "metadata", itemRatingKey, imageType)
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, msConfig, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return images, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var respData PlexGetAllImagesWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &respData, fmt.Sprintf("%s Media Item Images Response", msConfig.Type))
	if Err.Message != "" {
		return images, Err
	}

	for _, img := range respData.MediaContainer.Metadata {
		if img.Provider != "local" {
			continue
		}
		images = append(images, img)
	}

	logAction.AppendResult("image_rating_keys", func() []string {
		keys := make([]string, len(images))
		for i, img := range images {
			keys[i] = img.RatingKey
		}
		return keys
	}())
	logAction.AppendResult("current_image_count", len(images))

	return images, Err
}
