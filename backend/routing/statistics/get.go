package routes_statistics

import (
	"aura/logging"
	"aura/statistics"
	"aura/utils/httpx"
	"net/http"
	"strconv"
)

// GetStatistics godoc
// @Summary      Get Statistics
// @Description  The media server libraries, how the media servers compare, the sets applied, and the downloads over time, for the Statistics page.
// @Tags         Statistics
// @Produce      json
// @Param        days  query     int  false  "How many days back the graphs go, 1 or more (default 30)"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=statistics.Statistics}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/statistics [get]
func GetStatistics(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Statistics", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	// Days that aren't a number use the default
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	stats, Err := statistics.Get(ctx, days)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, nil)
		return
	}
	httpx.SendResponse(w, ld, stats)
}
