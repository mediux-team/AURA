package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
	"slices"
	"strings"
)

// SwitchMainItem is what happens to a saved item when the main media server changes
type SwitchMainItem struct {
	Title           string `json:"title"`
	Year            int    `json:"year"`
	TMDB_ID         string `json:"tmdb_id"`
	Type            string `json:"type"`
	LibraryTitle    string `json:"library_title"`
	Edition         string `json:"edition"`
	NewLibraryTitle string `json:"new_library_title"` // Empty if no library on the new main media server matches
	NewEdition      string `json:"new_edition"`
	Found           bool   `json:"found"`  // Whether the item is on the new main media server
	Merged          bool   `json:"merged"` // Whether another saved item becomes the same item, e.g. two editions

	newItem models.MediaItem
}

// SwitchMainPlan is what happens when the main media server changes
type SwitchMainPlan struct {
	MediaServerName string            `json:"media_server_name"`
	LibraryChanges  map[string]string `json:"library_changes"` // Library on the new main media server for each library on the current one
	Items           []SwitchMainItem  `json:"items"`
	Moved           int               `json:"moved"`
	NotFound        int               `json:"not_found"`
	Merged          int               `json:"merged"`
	Blocked         string            `json:"blocked,omitempty"` // Why the switch can't happen right now

	server          config.Config_MediaServer
	serverName      string // The name the media server reports
	mainMediaServer string
}

// PlanMainMediaServerSwitch works out what happens to the saved items if another media server becomes the main one.
// Nothing is changed. The media server's libraries are loaded again first, so the plan is up to date.
func PlanMainMediaServerSwitch(ctx context.Context, serverID string) (plan SwitchMainPlan, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Planning Main Media Server Switch", logging.LevelInfo)
	defer logAction.Complete()

	current := config.Current.Clone()
	libraryChanges, err := config.SwitchMainMediaServer(&current, serverID)
	if err != nil {
		logAction.SetError("The main media server can't be changed to this media server", err.Error(), nil)
		return plan, *logAction.Error
	}
	plan.mainMediaServer = config.Current.MainMediaServer
	plan.LibraryChanges = libraryChanges
	for _, server := range config.Current.MediaServers {
		if server.ID == serverID {
			plan.server = server
		}
	}
	plan.MediaServerName = plan.server.Name

	// Queued downloads use the current main media server's items
	activeJobs, Err := database.CountActiveDownloadQueueJobs(ctx)
	if Err.Message != "" {
		return plan, Err
	}
	if activeJobs > 0 {
		plan.Blocked = fmt.Sprintf("%d downloads are in the queue. Wait for them to finish, then switch.", activeJobs)
	}
	if name, running := MediaServerSyncRunning(); running {
		plan.Blocked = fmt.Sprintf("Saved sets are being applied to %s. Wait for it to finish or stop it, then switch.", name)
	}

	// Load the media server's libraries again
	connected, serverName := connectMediaServer(ctx, &plan.server)
	if !connected {
		logAction.SetError(fmt.Sprintf("Could not connect to %s", plan.server.Name), "Check that the media server is running, then try again", nil)
		return plan, *logAction.Error
	}
	plan.serverName = serverName
	store := cache.ServerLibraryStore(plan.server.ID)
	if !loadLibrariesIntoCache(ctx, plan.server, store, false) {
		logAction.SetError(fmt.Sprintf("Could not load the libraries of %s", plan.server.Name), "Check that the media server is running, then try again", nil)
		return plan, *logAction.Error
	}

	savedItems, Err := database.GetAllMediaItems(ctx)
	if Err.Message != "" {
		return plan, Err
	}
	newKeys := map[string]int{}
	for _, saved := range savedItems {
		item := SwitchMainItem{
			Title: saved.Title, Year: saved.Year, TMDB_ID: saved.TMDB_ID, Type: saved.Type,
			LibraryTitle: saved.LibraryTitle, Edition: saved.Edition,
			NewLibraryTitle: libraryChanges[saved.LibraryTitle], NewEdition: saved.Edition,
		}
		newItem, library, found := findInLibraryStore(store, mappedLibraries(plan.server, saved.LibraryTitle), saved)
		if found {
			item.Found, item.newItem, item.NewLibraryTitle, item.NewEdition = true, newItem, library, newItem.Edition
		}
		if item.NewLibraryTitle != "" {
			newKeys[item.TMDB_ID+"|"+item.NewLibraryTitle+"|"+item.NewEdition]++
		}
		plan.Items = append(plan.Items, item)
	}

	for i := range plan.Items {
		item := &plan.Items[i]
		item.Merged = item.NewLibraryTitle != "" && newKeys[item.TMDB_ID+"|"+item.NewLibraryTitle+"|"+item.NewEdition] > 1
		switch {
		case !item.Found:
			plan.NotFound++
		case item.Merged:
			plan.Merged++
		default:
			plan.Moved++
		}
	}
	slices.SortFunc(plan.Items, func(a, b SwitchMainItem) int {
		if a.Found != b.Found {
			if a.Found {
				return 1
			}
			return -1
		}
		return strings.Compare(a.Title, b.Title)
	})
	return plan, logging.LogErrorInfo{}
}

// SwitchMainMediaServer makes another media server the main one.
// The saved items move to its items, the library maps change to its libraries, and the caches are loaded again.
func SwitchMainMediaServer(ctx context.Context, serverID string) (plan SwitchMainPlan, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Switching Main Media Server", logging.LevelInfo)
	defer logAction.Complete()

	plan, Err = PlanMainMediaServerSwitch(ctx, serverID)
	if Err.Message != "" {
		return plan, Err
	}
	if plan.Blocked != "" {
		logAction.SetError("The main media server can't be changed right now", plan.Blocked, nil)
		return plan, *logAction.Error
	}

	// Get the full details of each item before changing anything, so the switch doesn't stop half way
	msClient, Err := NewMediaServerClient(&plan.server)
	if Err.Message != "" {
		return plan, Err
	}
	moves := make([]database.MediaItemMove, 0, len(plan.Items))
	for _, item := range plan.Items {
		move := database.MediaItemMove{
			TMDB_ID: item.TMDB_ID, OldLibrary: item.LibraryTitle, OldEdition: item.Edition,
			NewItem: models.MediaItem{LibraryTitle: item.NewLibraryTitle}, Found: item.Found,
		}
		if item.Found {
			newItem := item.newItem
			found, Err := msClient.GetMediaItemDetails(ctx, &newItem)
			if Err.Message != "" || !found {
				logAction.SetError(fmt.Sprintf("Could not get '%s' from %s", item.Title, plan.server.Name), "Check that the media server is running, then try again",
					map[string]any{"error": Err.Message})
				return plan, *logAction.Error
			}
			newItem.LibraryTitle, newItem.Edition = item.NewLibraryTitle, item.NewEdition
			move.NewItem = newItem
		}
		moves = append(moves, move)
	}

	config.UpdateLock.Lock()
	// The config may have changed while the details were loaded
	if config.Current.MainMediaServer != plan.mainMediaServer {
		config.UpdateLock.Unlock()
		logAction.SetError("The main media server changed while switching", "Try again", nil)
		return plan, *logAction.Error
	}
	newConfig := config.Current.Clone()
	if _, err := config.SwitchMainMediaServer(&newConfig, serverID); err != nil {
		config.UpdateLock.Unlock()
		logAction.SetError("The main media server can't be changed to this media server", err.Error(), nil)
		return plan, *logAction.Error
	}
	newConfig.MediaServer.UserID = plan.server.UserID

	if Err := database.MoveMediaItems(ctx, moves); Err.Message != "" {
		config.UpdateLock.Unlock()
		return plan, Err
	}
	if Err := newConfig.Save(ctx); Err.Message != "" {
		config.UpdateLock.Unlock()
		logAction.SetError("The saved items moved, but the config could not be saved", Err.Message, Err.Detail)
		return plan, *logAction.Error
	}
	config.Current = newConfig
	config.MediaServerName = plan.serverName
	config.MediaServerValid = true
	config.UpdateLock.Unlock()

	logging.LOGGER.Info().Timestamp().Str("media_server", plan.server.Name).
		Int("moved", plan.Moved).Int("merged", plan.Merged).Int("not_found", plan.NotFound).
		Msg("Main media server changed")

	// Load the new main media server into the main cache, and the others again
	cache.LibraryStore.ClearAllSections()
	cache.CollectionsStore.ClearAllCollections()
	if Err := database.SaveCollections(ctx, nil); Err.Message != "" {
		logging.LOGGER.Warn().Timestamp().Str("error", Err.Message).Msg("Failed to remove the old main media server's collections")
	}
	GetAllLibrarySectionsAndItems(ctx, true)
	return plan, logging.LogErrorInfo{}
}
