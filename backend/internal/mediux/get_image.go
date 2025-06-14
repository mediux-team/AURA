package mediux

import (
	"aura/internal/config"
	"aura/internal/logging"
	"aura/internal/utils"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/go-chi/chi/v5"
)

var MediuxThumbsTempImageFolder string
var MediuxFullTempImageFolder string

func init() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/config"
	}
	MediuxThumbsTempImageFolder = path.Join(configPath, "temp-images", "mediux", "thumbs")
	MediuxFullTempImageFolder = path.Join(configPath, "temp-images", "mediux", "full")
}

func GetMediuxImage(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	logging.LOG.Trace(r.URL.Path)

	// Get the asset ID from the URL
	assetID := chi.URLParam(r, "assetID")
	if assetID == "" {
		utils.SendErrorJSONResponse(w, http.StatusInternalServerError, logging.ErrorLog{Err: errors.New("missing asset ID"),
			Log: logging.Log{
				Message: "Missing asset ID in URL",
				Elapsed: utils.ElapsedTime(startTime),
			},
		})
		return
	}

	// Get the modified date from the URL query parameters
	modifiedDate := r.URL.Query().Get("modifiedDate")
	var modifiedDateTime time.Time
	var err error
	if modifiedDate == "" || modifiedDate == "0" || modifiedDate == "undefined" {
		// Use today's date if the modified date is not provided
		modifiedDateTime = time.Now()
	} else {
		// Try to parse the modified date as an ISO 8601 timestamp
		modifiedDateTime, err = time.Parse(time.RFC3339, modifiedDate)
		if err != nil {
			utils.SendErrorJSONResponse(w, http.StatusInternalServerError, logging.ErrorLog{Err: err,
				Log: logging.Log{
					Message: "Invalid modified date format. Expected ISO 8601.",
					Elapsed: utils.ElapsedTime(startTime),
				},
			})
			return
		}
	}
	formatDate := modifiedDateTime.Format("20060102")

	// Get Quality from the URL query parameters
	qualityParam := r.URL.Query().Get("quality")
	quality := false
	if qualityParam == "" {
		// Default to "thumb" if quality is not provided
		qualityParam = "thumb"
	}
	// Check if the quality is valid
	if qualityParam != "thumb" && qualityParam != "full" {
		utils.SendErrorJSONResponse(w, http.StatusBadRequest, logging.ErrorLog{
			Err: errors.New("invalid quality parameter"),
			Log: logging.Log{
				Message: fmt.Sprintf("Invalid quality parameter: %s. Expected 'thumb' or 'full'.", qualityParam),
				Elapsed: utils.ElapsedTime(startTime),
			},
		})
		return
	}
	if qualityParam == "full" {
		quality = true
	}

	// Check if the temporary folder has the image
	fileName := fmt.Sprintf("%s_%s.jpg", assetID, formatDate)
	filePath := path.Join(MediuxThumbsTempImageFolder, fileName)
	exists := utils.CheckIfImageExists(filePath)
	if exists {
		logging.LOG.Trace(fmt.Sprintf("Image %s already exists in temporary folder", fileName))
		// Serve the image from the temporary folder
		imagePath := path.Join(MediuxThumbsTempImageFolder, fileName)
		http.ServeFile(w, r, imagePath)
		return
	}

	// If the image does not exist, then get it from Mediux
	imageData, imageType, logErr := FetchImage(assetID, formatDate, quality)
	if logErr.Err != nil {
		utils.SendErrorJSONResponse(w, http.StatusInternalServerError, logErr)
		return
	}

	if config.Global.CacheImages {
		// Add the image to the temporary folder
		imagePath := path.Join(MediuxThumbsTempImageFolder, fileName)
		logErr = utils.CheckFolderExists(MediuxThumbsTempImageFolder)
		if logErr.Err != nil {
			utils.SendErrorJSONResponse(w, http.StatusInternalServerError, logErr)
			return
		}
		err := os.WriteFile(imagePath, imageData, 0644)
		if err != nil {
			utils.SendErrorJSONResponse(w, http.StatusInternalServerError, logging.ErrorLog{Err: err,
				Log: logging.Log{
					Message: "Failed to write image to temporary folder",
					Elapsed: utils.ElapsedTime(startTime),
				},
			})
			return
		}
	}

	w.Header().Set("Content-Type", imageType)
	w.WriteHeader(http.StatusOK)
	w.Write(imageData)
}

func FetchImage(assetID string, formatDate string, full bool) ([]byte, string, logging.ErrorLog) {
	logging.LOG.Trace(fmt.Sprintf("Getting image for asset ID: %s", assetID))

	// Construct the URL for the Mediux API request
	getThumb := ""
	//Add this in the future if needed to reduce load times
	if !full {
		getThumb = "&key=thumb"
	}
	url := fmt.Sprintf("%s/%s?%s%s", "https://staged.mediux.io/assets", assetID, formatDate, getThumb)

	response, body, logErr := utils.MakeHTTPRequest(url, "GET", nil, 60, nil, "Mediux")
	if logErr.Err != nil {
		return nil, "", logErr
	}
	defer response.Body.Close()

	// Check if the response body is empty
	if len(body) == 0 {
		return nil, "", logging.ErrorLog{
			Err: errors.New("empty response body"),
			Log: logging.Log{Message: "Mediux returned an empty response body"},
		}
	}

	// Get the image type from the response headers
	imageType := response.Header.Get("Content-Type")
	if imageType == "" {
		return nil, "", logging.ErrorLog{
			Err: errors.New("missing content type in response headers"),
			Log: logging.Log{Message: "Mediux did not return a content type in the response headers"},
		}
	}

	if response.StatusCode != http.StatusOK {
		return nil, "", logging.ErrorLog{
			Err: errors.New("failed to fetch image from Mediux"),
			Log: logging.Log{Message: fmt.Sprintf("Mediux returned status code %d", response.StatusCode)},
		}
	}

	// Return the image data
	return body, imageType, logging.ErrorLog{}
}
