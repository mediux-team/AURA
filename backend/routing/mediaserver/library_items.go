package routes_ms

import (
	"aura/cache"
	"aura/library"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"net/http"
	"strconv"
	"time"
)

// libraryQuery reads the Home page's filters, search, sort and page from the URL
func libraryQuery(r *http.Request) library.Query {
	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	perPage, _ := strconv.Atoi(query.Get("per_page"))
	return library.Query{
		Libraries:     query["library"],
		Search:        query.Get("search"),
		InDB:          query.Get("in_db"),
		Ignored:       query.Get("ignored"),
		SetsAvailable: query.Get("sets_available"),
		Sort:          query.Get("sort"),
		Order:         query.Get("order"),
		Page:          page,
		PerPage:       perPage,
	}
}

// GetLibraryItems godoc
// @Summary      Get Library Items
// @Description  One page of the main media server's library, filtered, searched and sorted like the Home page. Read from aura's cache, not the media server.
// @Tags         MediaServer
// @Produce      json
// @Param        library         query  []string  false  "Only these libraries" collectionFormat(multi)
// @Param        search          query  string    false  "Search, like the search bar, e.g. matrix Y:1999:"
// @Param        in_db           query  string    false  "inDB or notInDB"
// @Param        ignored         query  string    false  "ignored, not_ignored, always, until-set-available or until-new-set-available"
// @Param        sets_available  query  string    false  "hasSetsAvailable or noSetsAvailable"
// @Param        sort            query  string    false  "title, dateUpdated, dateAdded, dateReleased or newEpisodeAdded"
// @Param        order           query  string    false  "asc or desc"
// @Param        page            query  int       false  "Page, from 1"
// @Param        per_page        query  int       false  "Items per page"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=library.Result}
// @Router       /api/mediaserver/library/items [get]
func GetLibraryItems(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	httpx.SendResponse(w, ld, library.Find(cache.LibraryStore, libraryQuery(r)))
}

type GetRelatedLibraryItems_Response struct {
	OtherLibraries []models.MediaItem `json:"other_libraries"` // The same item in other libraries
	Previous       *library.Item      `json:"previous"`        // The item before it on the Home page
	Next           *library.Item      `json:"next"`            // The item after it on the Home page
}

// GetRelatedLibraryItems godoc
// @Summary      Get Related Library Items
// @Description  The same item in the other libraries, and the items before and after it on the Home page with the same filters, search and sort. Read from aura's cache, not the media server.
// @Tags         MediaServer
// @Produce      json
// @Param        item_library    query  string    true   "Library of the item"
// @Param        rating_key      query  string    true   "Rating key of the item"
// @Param        tmdb_id         query  string    false  "TMDB ID of the item"
// @Param        library         query  []string  false  "Home page: only these libraries" collectionFormat(multi)
// @Param        search          query  string    false  "Home page: search"
// @Param        in_db           query  string    false  "Home page: inDB or notInDB"
// @Param        ignored         query  string    false  "Home page: ignored filter"
// @Param        sets_available  query  string    false  "Home page: hasSetsAvailable or noSetsAvailable"
// @Param        sort            query  string    false  "Home page: sort"
// @Param        order           query  string    false  "Home page: asc or desc"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetRelatedLibraryItems_Response}
// @Router       /api/mediaserver/library/item/related [get]
func GetRelatedLibraryItems(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	query := r.URL.Query()
	libraryTitle := query.Get("item_library")

	var response GetRelatedLibraryItems_Response
	response.OtherLibraries = library.InOtherLibraries(cache.LibraryStore, libraryTitle, query.Get("tmdb_id"))
	response.Previous, response.Next = library.Neighbours(cache.LibraryStore, libraryQuery(r), libraryTitle, query.Get("rating_key"))
	httpx.SendResponse(w, ld, response)
}

// RefreshLibrary godoc
// @Summary      Refresh Library
// @Description  Start loading the main media server's libraries into aura's cache again. It runs in the background, use GET /api/mediaserver/library/refresh/stream for its progress. A refresh that is already running is not started again.
// @Tags         MediaServer
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=mediaserver.LibraryProgress}
// @Router       /api/mediaserver/library/refresh [post]
func RefreshLibrary(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)

	if !mediaserver.CurrentLibraryProgress().Running {
		go func() {
			defer logging.RecoverPanic("Refresh Library")
			ctx, ld := logging.CreateLoggingContext(context.Background(), "Refresh Library")
			logAction := ld.AddAction("Refresh Library", logging.LevelInfo)
			ctx = logging.WithCurrentAction(ctx, logAction)
			defer ld.Log()

			if !mediaserver.GetAllLibrarySectionsAndItems(ctx, true) {
				logAction.SetError("Could not load the libraries from the media server", "Check that the media server is running, then try again", nil)
			}
		}()
		// Wait for the refresh to start, so the progress returned shows it running
		for range 20 {
			if mediaserver.CurrentLibraryProgress().Running {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	httpx.SendResponse(w, ld, mediaserver.CurrentLibraryProgress())
}

// StreamLibraryProgress godoc
// @Summary      Refresh Library - Stream Progress
// @Description  Server-Sent Events stream of how far loading the main media server's libraries has got. Sends the progress now on connect, then every time it changes.
// @Tags         MediaServer
// @Produce      text/event-stream
// @Security     SessionCookie
// @Router       /api/mediaserver/library/refresh/stream [get]
func StreamLibraryProgress(w http.ResponseWriter, r *http.Request) {
	stream, ok := httpx.StartSSE(w)
	if !ok {
		return
	}

	ch, current := mediaserver.SubscribeLibraryProgress()
	defer mediaserver.UnsubscribeLibraryProgress(ch)
	stream.Send(current)
	httpx.StreamEvents(r.Context(), stream, ch, func(progress mediaserver.LibraryProgress) { stream.Send(progress) })
}
