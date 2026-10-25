package routes_download

import (
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

type DownloadCollectionImage_Request struct {
	ImageFile      models.ImageFile      `json:"image_file"`
	CollectionItem models.CollectionItem `json:"collection_item"`
	Set            *models.BaseSetInfo   `json:"set,omitempty"` // The set the image is from, for the statistics
}

type DownloadCollectionImage_Response struct {
	Result string `json:"result"`
}

// DownloadImageFileForCollectionItem godoc
// @Summary      Download Collection Image
// @Description  Download and apply an image for a Collection Item in the media server.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      DownloadCollectionImage_Request  true  "Download Collection Image Request"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=DownloadCollectionImage_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/image/collection [post]
func DownloadImageFileForCollectionItem(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Collection Image", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req DownloadCollectionImage_Request
	var response DownloadCollectionImage_Response

	// Parse the request body
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Download Collection Image - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	actionValidate := logAction.AddSubAction("Validate Request Body Fields", logging.LevelDebug)
	// Make sure that the collectionItem has the following fields set
	// 1. CollectionItem.RatingKey
	// 2. CollectionItem.LibraryTitle
	if req.CollectionItem.RatingKey == "" || req.CollectionItem.LibraryTitle == "" {
		actionValidate.SetError("Invalid Collection Item Data", "RatingKey and LibraryTitle are required in Collection Item", map[string]any{
			"rating_key":    req.CollectionItem.RatingKey,
			"library_title": req.CollectionItem.LibraryTitle,
		})
		actionValidate.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}

	// Make sure that the ImageFile has the following fields set
	// 1. ImageFile.ID
	// 2. ImageFile.Type
	if req.ImageFile.ID == "" || req.ImageFile.Type == "" {
		actionValidate.SetError("Invalid Image File Data", "ID and Type are required in Image File", map[string]any{
			"id":   req.ImageFile.ID,
			"type": req.ImageFile.Type,
		})
		actionValidate.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}
	actionValidate.Complete()

	// Make the download and apply the image
	Err = mediaserver.ApplyCollectionImage(ctx, &req.CollectionItem, req.ImageFile)
	event := models.NewCollectionDownloadEvent(req.CollectionItem, req.Set, models.SourceDownloadWindow)
	event.AddImage(req.ImageFile, Err.Message == "")
	database.RecordDownloadEvent(ctx, event)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Result = "ok"
	httpx.SendResponse(w, ld, response)
}
