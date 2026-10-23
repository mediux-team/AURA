package routes_db

import (
	"aura/cache"
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/models"
	sonarr_radarr "aura/sonarr-radarr"
	"aura/utils/httpx"
	"context"
	"net/http"
)

type addItemRequest struct {
	Complete                  bool                     `json:"complete"`
	MediaItem                 models.MediaItem         `json:"media_item"`
	PosterSet                 models.DBPosterSetDetail `json:"poster_set"`
	AddToDBOnly               bool                     `json:"add_to_db_only"` // If true, the item will be added to the database but not have any labels or tags applied. This is for users who want to manage labels and tags manually.
	AutoAddNewCollectionItems bool                     `json:"auto_add_new_collection_items"`
}

type addItemResponse struct {
	SavedItem models.DBSavedItem `json:"saved_item"`
}

// AddItem godoc
// @Summary      Add Item To Database
// @Description  Add a Media Item and Poster Set to the database. If the item already exists, it will be updated with the new Media Item and Poster Set information.
// @Tags         Database
// @Accept       json
// @Produce      json
// @Param        req  body      addItemRequest  true  "Add Item Request"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=addItemResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/db [post]
func AddNewItemToDB(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Add Item To Database", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req addItemRequest
	var response addItemResponse

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Add Item To Database - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	logAction.AppendResult("complete", req.Complete)

	// Validate the Save Item Media Item
	if req.MediaItem.TMDB_ID == "" || req.MediaItem.LibraryTitle == "" {
		logAction.SetError("Invalid Media Item Data", "TMDB ID and Library Title are required", map[string]any{
			"tmdb_id":       req.MediaItem.TMDB_ID,
			"library_title": req.MediaItem.LibraryTitle,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	// Make sure each Poster Set has an ID and Type
	if req.PosterSet.ID == "" || req.PosterSet.Type == "" {
		logAction.SetError("Invalid Poster Set Data", "Each Poster Set must have an ID and Type", map[string]any{
			"tmdb_id":       req.MediaItem.TMDB_ID,
			"library_title": req.MediaItem.LibraryTitle,
			"poster_set":    req.PosterSet,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	// If the request body is not complete, we need to get the information again
	// This is to prevent incomplete data from being added to the database
	// In the frontend, we omit MediaItem details (besides important fields)
	// and ImageFiles in PosterSets to reduce payload size
	fullSet := req.PosterSet
	fullSet.AutoAddNewCollectionItems = req.AutoAddNewCollectionItems
	if !req.Complete {
		// Get the MediaItem details from the Media Server
		found, Err := mediaserver.GetMediaItemDetails(ctx, &req.MediaItem)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
		if !found {
			logAction.SetError("Media Item not found on Media Server", "Make sure the media item exists on the connected media server", map[string]any{
				"tmdb_id":       req.MediaItem.TMDB_ID,
				"library_title": req.MediaItem.LibraryTitle,
			})
			httpx.SendResponse(w, ld, response)
			return
		}

		// We also need a full PosterSet with ImageFiles
		switch req.PosterSet.Type {
		case "show":
			showSet, _, Err := mediux.GetShowSetByID(ctx, req.PosterSet.ID, req.MediaItem.LibraryTitle, req.MediaItem.Edition)
			if Err.Message != "" {
				httpx.SendResponse(w, ld, response)
				return
			}
			fullSet.PosterSet = showSet.PosterSet
		case "movie":
			movieSet, _, Err := mediux.GetMovieSetByID(ctx, req.PosterSet.ID, req.MediaItem.LibraryTitle, req.MediaItem.Edition)
			if Err.Message != "" {
				httpx.SendResponse(w, ld, response)
				return
			}
			fullSet.PosterSet = movieSet.PosterSet
		case "collection":
			collectionSet, _, Err := mediux.GetMovieCollectionSetByID(ctx, req.PosterSet.ID, req.MediaItem.TMDB_ID, req.MediaItem.LibraryTitle, req.MediaItem.Edition, true)
			if Err.Message != "" {
				httpx.SendResponse(w, ld, response)
				return
			}
			fullSet.PosterSet = collectionSet.PosterSet
		default:
			logAction.SetError("Invalid Poster Set Type", "Poster Set type must be either 'show' or 'movie'", map[string]any{
				"set_type": req.PosterSet.Type,
			})
			httpx.SendResponse(w, ld, response)
			return
		}
	}

	saveItem := models.DBSavedItem{
		MediaItem:  req.MediaItem,
		PosterSets: []models.DBPosterSetDetail{fullSet},
	}

	// Save the Item to the Database
	Err = database.UpsertSavedItem(ctx, saveItem)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// If this is the first time adding the item, we need to update the cache
	// Run this asynchronously
	go func() {
		defer logging.RecoverPanic("Update Saved Sets in the Cache")
		_, _, dbSets, _ := database.CheckIfMediaItemExists(ctx, saveItem.MediaItem.TMDB_ID, saveItem.MediaItem.LibraryTitle, saveItem.MediaItem.Edition)
		saveItem.MediaItem.DBSavedSets = dbSets
		cache.LibraryStore.UpdateMediaItem(saveItem.MediaItem.LibraryTitle, &saveItem.MediaItem)
	}()
	response.SavedItem = saveItem

	if req.AddToDBOnly {
		httpx.SendResponse(w, ld, response)
		return
	}

	// Handle any labels and tags asynchronously
	go func() {
		defer logging.RecoverPanic("Labels and Tags Handling")
		ctx, ld := logging.CreateLoggingContext(context.Background(), "Labels and Tags Handling")
		logAction := ld.AddAction("Handle Labels and Tags for Added Item", logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, logAction)
		defer ld.Log()

		mediaserver.AddLabelToMediaItem(ctx, saveItem.MediaItem, fullSet.SelectedTypes)
		sonarr_radarr.HandleTags(ctx, saveItem.MediaItem, fullSet.SelectedTypes)
	}()

	httpx.SendResponse(w, ld, response)
}
