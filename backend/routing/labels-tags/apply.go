package routes_labels_tags

import (
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	sonarr_radarr "aura/sonarr-radarr"
	"aura/utils/httpx"
	"net/http"
)

type applyLabelsTagsRequest struct {
	MediaItem     models.MediaItem     `json:"media_item"`
	SelectedTypes models.SelectedTypes `json:"selected_types"`
}

type applyLabelsTagsResponse struct {
	Message string `json:"message"`
}

// ApplyLabelsTags godoc
// @Summary      Apply Labels/Tags
// @Description  Apply labels and tags to a Media Item in the media server and Sonarr/Radarr.
// @Tags         Labels/Tags
// @Accept       json
// @Produce      json
// @Param        req  body      applyLabelsTagsRequest  true  "Apply Labels/Tags Request"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=applyLabelsTagsResponse}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/labels-tags [post]
func ApplyLabelsAndTagsToItem(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Apply Labels/Tags", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req applyLabelsTagsRequest
	var response applyLabelsTagsResponse

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Apply Labels/Tags - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// Make sure that Media Item has all of the required fields
	if req.MediaItem.Title == "" || req.MediaItem.LibraryTitle == "" || req.MediaItem.TMDB_ID == "" || req.MediaItem.RatingKey == "" {
		logAction.SetError("Invalid Media Item structure",
			"Ensure that the request body contains a valid Media Item with all required fields",
			map[string]any{
				"media_item": req.MediaItem,
			},
		)
		httpx.SendResponse(w, ld, response)
		return
	}

	mediaserver.AddLabelToMediaItem(ctx, req.MediaItem, req.SelectedTypes)
	Err = sonarr_radarr.HandleTags(ctx, req.MediaItem, req.SelectedTypes)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Message = "Labels/Tags applied successfully"
	httpx.SendResponse(w, ld, response)
}
