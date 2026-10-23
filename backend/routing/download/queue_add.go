package routes_download

import (
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

type AddItemToDownloadQueue_Request struct {
	Item models.DBSavedItem `json:"item"`
}

type AddItemToDownloadQueue_Response struct {
	Result string `json:"result"`
}

// AddItemToDownloadQueue godoc
// @Summary      Download Queue - Add Item
// @Description  Add a Media Item and its associated Poster Sets to the download queue. The item will be processed by the download worker and removed from the queue once completed.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      AddItemToDownloadQueue_Request  true  "Queue Add Item Request"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=AddItemToDownloadQueue_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue/item [post]
func AddItemToDownloadQueue(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Add Item", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req AddItemToDownloadQueue_Request
	var response AddItemToDownloadQueue_Response

	// Parse and validate request body
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Queue Add Item - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// Validate the JSON structure
	validateAction := logAction.AddSubAction("Validate Save Item", logging.LevelDebug)

	// Only collection images, applied to their media server collection
	if req.Item.IsCollectionOnly() {
		collection := req.Item.CollectionImages.Collection
		if collection.RatingKey == "" || collection.LibraryTitle == "" {
			validateAction.SetError("Invalid collection",
				"Ensure that the collection has a rating key and library",
				map[string]any{"collection": collection})
			validateAction.Complete()
			httpx.SendResponse(w, ld, response)
			return
		}
	}

	// Items with sets need their media item
	if !req.Item.IsCollectionOnly() && (req.Item.MediaItem.Title == "" || req.Item.MediaItem.LibraryTitle == "" || req.Item.MediaItem.TMDB_ID == "" || req.Item.MediaItem.RatingKey == "") {
		validateAction.SetError("Invalid Save Item structure",
			"Ensure that the request body contains a valid Save Item with all required fields",
			map[string]any{
				"item": req.Item,
			})
		validateAction.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}

	// Make sure there is at least one Poster Set, or collection images to apply
	if len(req.Item.PosterSets) == 0 && !req.Item.HasCollectionImages() {
		validateAction.SetError("Invalid Save Item structure - No Poster Sets",
			"Ensure that the Media Item contains at least one Poster Set",
			map[string]any{
				"item": req.Item,
			})
		validateAction.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}

	// Make sure that each Poster Set has a Set ID
	for _, posterSet := range req.Item.PosterSets {
		if posterSet.ID == "" {
			validateAction.SetError("Invalid Save Item structure - Poster Set missing Set ID",
				"Ensure that each Poster Set in the Media Item contains a valid Set ID",
				map[string]any{
					"set": posterSet,
				})
			validateAction.Complete()
			httpx.SendResponse(w, ld, response)
			return
		}
	}
	validateAction.Complete()

	// Add the item to the download queue
	addErr := downloadqueue.AddToQueue(ctx, req.Item)
	if addErr.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Result = "Item added to download queue"
	httpx.SendResponse(w, ld, response)
}
