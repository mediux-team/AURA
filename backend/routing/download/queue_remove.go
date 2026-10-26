package routes_download

import (
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type RemoveItemFromDownloadQueue_Request struct {
	JobID int64 `json:"job_id"`
}

type RemoveItemFromDownloadQueue_Response struct {
	Result string `json:"result"`
}

// RemoveItemFromQueue godoc
// @Summary      Download Queue - Remove Item
// @Description  Remove a specific Job from the download queue by ID. A job that is processing is stopped: the images already downloading finish, and the rest aren't downloaded.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      RemoveItemFromDownloadQueue_Request  true  "Queue Remove Item Request"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=RemoveItemFromDownloadQueue_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue/item [delete]
func RemoveItemFromDownloadQueue(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Remove Item", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req RemoveItemFromDownloadQueue_Request
	var response RemoveItemFromDownloadQueue_Response

	// Parse and validate request body
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Queue Remove Item - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	validateAction := logAction.AddSubAction("Validate Delete Item", logging.LevelDebug)
	if req.JobID == 0 {
		validateAction.SetError("Invalid Delete Item request",
			"Ensure that the request body contains a valid job_id",
			map[string]any{
				"job_id": req.JobID,
			})
		validateAction.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}
	validateAction.Complete()

	Err = downloadqueue.RemoveFromQueue(ctx, req.JobID)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Result = "Removed item from the download queue"
	httpx.SendResponse(w, ld, response)
}

type RemoveAllFromDownloadQueue_Response struct {
	Result string `json:"result"`
}

// RemoveAllFromDownloadQueue godoc
// @Summary      Download Queue - Clear All
// @Description  Remove all items from the download queue. Jobs currently processing are stopped: the images already downloading finish, and the rest aren't downloaded.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=RemoveAllFromDownloadQueue_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/queue [delete]
func RemoveAllFromDownloadQueue(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Clear All", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response RemoveAllFromDownloadQueue_Response

	Err := downloadqueue.RemoveAllFromQueue(ctx)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Result = "Cleared the download queue"
	httpx.SendResponse(w, ld, response)
}
