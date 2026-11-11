package routes_sandbox

import (
	autodownload "aura/download/auto"
	"aura/logging"
	"aura/models"
	"aura/sandbox"
	"aura/utils/httpx"
	"net/http"
)

// GetState godoc
// @Summary      Sandbox - Get State
// @Description  How the test sandbox is set up: the wait before each image downloads, and the downloads made to fail. Only in dev builds.
// @Tags         Sandbox
// @Produce      json
// @Security     SessionCookie
// @Success      200  {object}  httpx.JSONResponse{data=sandbox.State}
// @Router       /api/dev/sandbox [get]
func GetState(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	ld.AddAction("Sandbox - Get State", logging.LevelDebug).Complete()
	httpx.SendResponse(w, ld, sandbox.GetState())
}

type SetDelay_Request struct {
	Seconds float64 `json:"seconds"`
}

// SetDelay godoc
// @Summary      Sandbox - Set Download Wait
// @Description  Wait before each image downloads, 0 for no wait. Only in dev builds.
// @Tags         Sandbox
// @Accept       json
// @Produce      json
// @Param        req  body      SetDelay_Request  true  "The wait in seconds"
// @Security     SessionCookie
// @Success      200  {object}  httpx.JSONResponse{data=sandbox.State}
// @Router       /api/dev/sandbox/delay [put]
func SetDelay(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Sandbox - Set Download Wait", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req SetDelay_Request
	if Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Sandbox Delay"); Err.Message != "" {
		httpx.SendResponse(w, ld, nil)
		return
	}
	sandbox.SetDelay(req.Seconds)
	logAction.Complete()
	httpx.SendResponse(w, ld, sandbox.GetState())
}

// AddFailure godoc
// @Summary      Sandbox - Make Downloads Fail
// @Description  Make an item's image downloads fail. Only in dev builds.
// @Tags         Sandbox
// @Accept       json
// @Produce      json
// @Param        req  body      sandbox.FailureRule  true  "The item, image types and how many downloads fail"
// @Security     SessionCookie
// @Success      200  {object}  httpx.JSONResponse{data=sandbox.State}
// @Router       /api/dev/sandbox/failures [post]
func AddFailure(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Sandbox - Make Downloads Fail", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var rule sandbox.FailureRule
	if Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &rule, "Sandbox Failure"); Err.Message != "" {
		httpx.SendResponse(w, ld, nil)
		return
	}
	if rule.TMDB_ID == "" || rule.LibraryTitle == "" {
		logAction.SetError("The item is missing", "Pick an item first", nil)
		httpx.SendResponse(w, ld, nil)
		return
	}
	sandbox.AddFailure(rule)
	logAction.Complete()
	httpx.SendResponse(w, ld, sandbox.GetState())
}

// ClearFailures godoc
// @Summary      Sandbox - Stop Making Downloads Fail
// @Description  Clear every download failure. Only in dev builds.
// @Tags         Sandbox
// @Produce      json
// @Security     SessionCookie
// @Success      200  {object}  httpx.JSONResponse{data=sandbox.State}
// @Router       /api/dev/sandbox/failures [delete]
func ClearFailures(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	ld.AddAction("Sandbox - Stop Making Downloads Fail", logging.LevelInfo).Complete()
	sandbox.ClearFailures()
	httpx.SendResponse(w, ld, sandbox.GetState())
}

// Simulate godoc
// @Summary      Sandbox - Simulate a Change
// @Description  Change an item's saved data as if something changed since the last download. Only in dev builds.
// @Tags         Sandbox
// @Accept       json
// @Produce      json
// @Param        req  body      sandbox.Change  true  "The item and the change"
// @Security     SessionCookie
// @Success      200  {object}  httpx.JSONResponse
// @Router       /api/dev/sandbox/simulate [post]
func Simulate(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Sandbox - Simulate a Change", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var change sandbox.Change
	if Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &change, "Sandbox Change"); Err.Message != "" {
		httpx.SendResponse(w, ld, nil)
		return
	}
	sandbox.Simulate(ctx, change)
	logAction.Complete()
	httpx.SendResponse(w, ld, map[string]string{"result": "Changed the saved data"})
}

type Recheck_Request struct {
	TMDB_ID      string `json:"tmdb_id"`
	LibraryTitle string `json:"library_title"`
	Edition      string `json:"edition"`
}

// Recheck godoc
// @Summary      Sandbox - Run the Auto Download Check
// @Description  Run the Auto Download check for an item now, with its saved sets as they are in the database. Only in dev builds.
// @Tags         Sandbox
// @Accept       json
// @Produce      json
// @Param        req  body      Recheck_Request  true  "The item"
// @Security     SessionCookie
// @Success      200  {object}  httpx.JSONResponse{data=autodownload.AutoDownloadResult}
// @Router       /api/dev/sandbox/recheck [post]
func Recheck(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Sandbox - Run the Auto Download Check", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req Recheck_Request
	if Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Sandbox Recheck"); Err.Message != "" {
		httpx.SendResponse(w, ld, nil)
		return
	}
	// The check gets the item's saved sets from the database
	result := autodownload.CheckItem(ctx, models.DBSavedItem{MediaItem: models.MediaItem{TMDB_ID: req.TMDB_ID, LibraryTitle: req.LibraryTitle, Edition: req.Edition}})
	logAction.Complete()
	httpx.SendResponse(w, ld, result)
}
