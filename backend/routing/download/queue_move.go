package routes_download

import (
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type MoveDownloadQueueItemToTop_Request struct {
	JobID int64 `json:"job_id"`
}

type MoveDownloadQueueItemToTop_Response struct {
	Result string `json:"result"`
}

// MoveDownloadQueueItemToTop godoc
// @Summary      Download Queue - Move Item to the Top
// @Description  Make a waiting Job the next one to run.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      MoveDownloadQueueItemToTop_Request  true  "Queue Move Item to the Top Request"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=MoveDownloadQueueItemToTop_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue/item/top [post]
func MoveDownloadQueueItemToTop(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Move Item to the Top", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req MoveDownloadQueueItemToTop_Request
	var response MoveDownloadQueueItemToTop_Response

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Queue Move Item to the Top - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	validateAction := logAction.AddSubAction("Validate Move Item", logging.LevelDebug)
	if req.JobID == 0 {
		validateAction.SetError("Invalid Move Item request",
			"Ensure that the request body contains a valid job_id",
			map[string]any{
				"job_id": req.JobID,
			})
		validateAction.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}
	validateAction.Complete()

	Err = downloadqueue.MoveToTopOfQueue(ctx, req.JobID)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Result = "Moved to the top of the download queue"
	httpx.SendResponse(w, ld, response)
}
