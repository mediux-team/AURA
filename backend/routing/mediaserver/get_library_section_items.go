package routes_ms

import (
	"aura/cache"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
	"time"
)

type GetLibrarySectionItems_Response struct {
	LibrarySection models.LibrarySection `json:"library_section"`
}

// GetLibrarySectionItems godoc
// @Summary      Get Library Section Items
// @Description  Retrieve items from a specific library section in the media server. This endpoint accepts query parameters to identify the library section and pagination options, and returns the items contained within that library section, allowing clients to display the media items available in the selected library section.
// @Tags         MediaServer
// @Accept       json
// @Produce      json
// @Param        section_id query string true "Library Section ID"
// @Param        section_title query string true "Library Section Title"
// @Param        section_type query string true "Library Section Type"
// @Param        section_start_index query string true "Start Index for Pagination"
// @Security 	 BearerAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetLibrarySectionItems_Response}
// @Failure	  500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediaserver/library/items [get]
func GetLibrarySectionItems(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Library Section Items", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetLibrarySectionItems_Response

	actionGetQueryParams := logAction.AddSubAction("Get all query params", logging.LevelTrace)
	// Get the following information from the URL
	// Section ID
	// Section Title
	// Section Type
	// Item Start Index
	sectionID := r.URL.Query().Get("section_id")
	sectionTitle := r.URL.Query().Get("section_title")
	sectionType := r.URL.Query().Get("section_type")
	sectionStartIndex := r.URL.Query().Get("section_start_index")
	enableSortByNewEpisodeParam := r.URL.Query().Get("enable_sort_by_new_episode")
	enableSortByNewEpisode := enableSortByNewEpisodeParam != "false"

	// Validate the section ID, title, type, and start index
	if sectionID == "" || sectionTitle == "" || sectionType == "" || sectionStartIndex == "" {
		actionGetQueryParams.SetError("Missing Query Parameters", "One or more required query parameters are missing",
			map[string]any{
				"section_id":          sectionID,
				"section_title":       sectionTitle,
				"section_type":        sectionType,
				"section_start_index": sectionStartIndex,
			})
		httpx.SendResponse(w, ld, response)
		return
	}
	actionGetQueryParams.Complete()

	// Fetch the section items from the media server
	response.LibrarySection = models.LibrarySection{
		LibrarySectionBase: models.LibrarySectionBase{
			ID:    sectionID,
			Title: sectionTitle,
			Type:  sectionType,
		},
	}
	mediaItems, totalSize, Err := mediaserver.GetLibrarySectionItems(ctx, response.LibrarySection, sectionStartIndex, "", enableSortByNewEpisode)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.LibrarySection.MediaItems = mediaItems
	response.LibrarySection.TotalSize = totalSize
	cache.LibraryStore.LastFullUpdate = time.Now().Unix()
	httpx.SendResponse(w, ld, response)
}
