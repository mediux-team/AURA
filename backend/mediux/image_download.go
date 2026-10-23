package mediux

import (
	"aura/config"
	"aura/logging"
	"aura/utils"
	"context"
	"fmt"
	"net/url"
	"os"
	"path"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func GetImage(ctx context.Context, assetID string, formatDate string, imageQuality ImageQuality) (imageData []byte, imageType string, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("MediUX: Getting %s Image for Asset ID '%s'",
		cases.Title(language.English).String(string(imageQuality)), assetID), logging.LevelDebug)
	defer logAction.Complete()

	imageData = nil
	imageType = ""
	Err = logging.LogErrorInfo{}

	// Determine the folder path based on qualityParam
	var folderPath string
	switch imageQuality {
	case ImageQualityOriginal:
		folderPath = FullTempImageFolder
	case ImageQualityOptimized, ImageQualityThumb:
		folderPath = ThumbsTempImageFolder
	}

	// File Name Base (extension is determined by the actual image content type,
	// since MediUX images may be JPEG, PNG, WEBP, or GIF)
	fileNameBase := fmt.Sprintf("%s_%s", assetID, formatDate)

	// Serve the image from the cache folder when it is there
	if config.Current.Images.CacheImages.Enabled {
		if cached, cachedType, found := readCachedImage(folderPath, fileNameBase); found {
			logAction.AppendResult("size", len(cached))
			logAction.AppendResult("imageType", cachedType)
			logAction.AppendResult("source", "cache")
			return cached, cachedType, Err
		}
	}

	// If we reach here, we need to fetch the image from MediUX
	// It may be because caching is disabled, the file doesn't exist, or reading the cached file failed

	// Construct the MediUX URL
	mediuxURL, Err := ConstructImageUrl(ctx, assetID, formatDate, imageQuality)
	if Err.Message != "" {
		return imageData, imageType, Err
	}

	// Make the HTTP Request to MediUX
	resp, respBody, Err := makeRequest(ctx, mediuxURL, "GET", nil, "", false)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return imageData, imageType, *logAction.Error
	}
	defer resp.Body.Close()

	// Check if response has image content
	if len(respBody) == 0 {
		logAction.SetError("MediUX returned an empty image response",
			"Ensure the asset ID is correct and the image exists on the MediUX server.",
			map[string]any{
				"URL": mediuxURL,
			})
		return imageData, imageType, *logAction.Error
	}

	// Get the Content-Type from response headers
	respContentType := resp.Header.Get("Content-Type")
	if respContentType == "" {
		logAction.SetError("MediUX response missing Content-Type header",
			"Ensure MediUX is returning valid image data.",
			map[string]any{
				"URL": mediuxURL,
			})
		return imageData, imageType, *logAction.Error
	}
	imageType = respContentType
	imageData = respBody

	logAction.AppendResult("size", len(imageData))
	logAction.AppendResult("imageType", respContentType)
	logAction.AppendResult("source", "MediUX")

	// Save it to the cache folder in the background
	if config.Current.Images.CacheImages.Enabled {
		fileName := fileNameBase + utils.GetExtensionFromContentType(imageType)
		logging.Go("Cache Image", func() { cacheImage(folderPath, fileName, imageData) })
	}

	return imageData, imageType, Err
}

// cachedImageTypes are the extensions the cache folder can have, with their content types
var cachedImageTypes = []struct{ extension, contentType string }{
	{".jpg", "image/jpeg"},
	{".png", "image/png"},
	{".webp", "image/webp"},
	{".gif", "image/gif"},
}

// readCachedImage reads an image from the cache folder, whichever type it was saved as
func readCachedImage(folderPath, fileNameBase string) (imageData []byte, imageType string, found bool) {
	for _, t := range cachedImageTypes {
		imageData, err := os.ReadFile(path.Join(folderPath, fileNameBase+t.extension))
		if err == nil {
			return imageData, t.contentType, true
		}
	}
	return nil, "", false
}

// cacheImage saves an image to the cache folder
func cacheImage(folderPath, fileName string, imageData []byte) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Caching - MediUX Image")
	logAction := ld.AddAction("Caching MediUX Image", logging.LevelDebug)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer ld.Log()

	if Err := utils.CreateFolderIfNotExists(ctx, folderPath); Err.Message != "" {
		return
	}
	// Write to a temporary file first, so it's never read half written
	filePath := path.Join(folderPath, fileName)
	if err := writeFileAtomic(filePath, imageData); err != nil {
		logAction.SetError("Failed to write image to MediUX thumbs temp image folder",
			"Ensure the application has write permissions to the temp image folder.",
			map[string]any{
				"filePath": filePath,
				"error":    err.Error(),
			})
		return
	}
	logAction.AppendResult("filePath", filePath)
	logAction.Complete()
}

func GetAvatarImage(ctx context.Context, avatarID string) (imageData []byte, imageType string, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("MediUX: Getting Avatar Image for Avatar ID '%s'", avatarID), logging.LevelDebug)
	defer logAction.Complete()

	imageData = nil
	imageType = ""
	Err = logging.LogErrorInfo{}

	// Construct the URL for the MediUX API request
	u, err := url.Parse(MediuxApiURL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return imageData, imageType, *logAction.Error
	}
	u.Path = path.Join(u.Path, "assets", avatarID)
	URL := u.String()

	// Make the HTTP Request to MediUX
	resp, respBody, Err := makeRequest(ctx, URL, "GET", nil, "", false)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return imageData, imageType, *logAction.Error
	}
	defer resp.Body.Close()

	// Check if response has image content
	if len(respBody) == 0 {
		logAction.SetError("MediUX returned an empty avatar image response",
			"Ensure the avatar ID is correct and the image exists on the MediUX server.",
			map[string]any{
				"URL": URL,
			})
		return imageData, imageType, *logAction.Error
	}

	// Get the Content-Type from response headers
	respContentType := resp.Header.Get("Content-Type")
	if respContentType == "" {
		logAction.SetError("MediUX avatar response missing Content-Type header",
			"Ensure MediUX is returning valid image data.",
			map[string]any{
				"URL": URL,
			})
		return imageData, imageType, *logAction.Error
	}
	imageType = respContentType
	imageData = respBody

	logAction.AppendResult("size", len(imageData))
	logAction.AppendResult("imageType", respContentType)

	return imageData, imageType, Err
}

// writeFileAtomic writes a file in full, then moves it into place
func writeFileAtomic(filePath string, data []byte) error {
	tmp, err := os.CreateTemp(path.Dir(filePath), ".caching-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filePath)
}
