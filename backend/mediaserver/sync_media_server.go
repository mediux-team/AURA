package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// MediaServerSync is the progress of applying the saved sets to a media server
type MediaServerSync struct {
	ServerID   string     `json:"server_id"`
	ServerName string     `json:"server_name"`
	Running    bool       `json:"running"`
	Total      int        `json:"total"`             // Saved items
	Done       int        `json:"done"`              // Saved items checked
	Applied    int        `json:"applied"`           // Images applied
	NotFound   int        `json:"not_found"`         // Saved items that aren't on the media server
	Failed     int        `json:"failed"`            // Images or items that couldn't be applied
	Failures   []string   `json:"failures"`          // The first few things that couldn't be applied
	Current    string     `json:"current,omitempty"` // The item being applied
	Error      string     `json:"error,omitempty"`   // Why the sync stopped before the end
	Stopped    bool       `json:"stopped"`           // Whether it was stopped from Settings
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// Only this many failures are kept, the rest are in the log
const maxSyncFailures = 20

var (
	syncMu     sync.Mutex
	syncState  *MediaServerSync // The latest sync, nil if there hasn't been one
	syncCancel context.CancelFunc
)

// Replaced in tests
var (
	syncLoadMediaServer = loadMediaServerForSync
	syncGetSavedItems   = getSavedItemsForSync
)

// StartMediaServerSync applies every saved set to a media server that isn't the main one.
// It runs in the background, GetMediaServerSync returns its progress.
func StartMediaServerSync(serverID string) (MediaServerSync, error) {
	syncMu.Lock()
	defer syncMu.Unlock()

	if syncState != nil && syncState.Running {
		return *syncState, fmt.Errorf("saved sets are already being applied to %s", syncState.ServerName)
	}

	server, err := syncableMediaServer(serverID)
	if err != nil {
		return MediaServerSync{}, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	syncCancel = cancel
	syncState = &MediaServerSync{ServerID: server.ID, ServerName: server.Name, Running: true, Failures: []string{}, StartedAt: time.Now()}
	go runMediaServerSync(ctx, server)
	return syncState.copy(), nil
}

// GetMediaServerSync returns the progress of the latest sync
func GetMediaServerSync() (MediaServerSync, bool) {
	syncMu.Lock()
	defer syncMu.Unlock()
	if syncState == nil {
		return MediaServerSync{}, false
	}
	return syncState.copy(), true
}

// StopMediaServerSync stops the running sync. The image being applied is finished first.
func StopMediaServerSync() bool {
	syncMu.Lock()
	defer syncMu.Unlock()
	if syncState == nil || !syncState.Running {
		return false
	}
	syncState.Stopped = true
	syncCancel()
	return true
}

// MediaServerSyncRunning returns the name of the media server saved sets are being applied to, if a sync is running
func MediaServerSyncRunning() (serverName string, running bool) {
	syncMu.Lock()
	defer syncMu.Unlock()
	if syncState == nil || !syncState.Running {
		return "", false
	}
	return syncState.ServerName, true
}

func (s *MediaServerSync) copy() MediaServerSync {
	c := *s
	c.Failures = append([]string{}, s.Failures...)
	return c
}

// updateSync changes the progress of the running sync
func updateSync(update func(state *MediaServerSync)) {
	syncMu.Lock()
	defer syncMu.Unlock()
	update(syncState)
}

// syncableMediaServer returns the media server, if saved sets can be applied to it
func syncableMediaServer(serverID string) (config.Config_MediaServer, error) {
	current := config.Current.Clone()
	for _, server := range current.MediaServers {
		if server.ID != serverID {
			continue
		}
		switch {
		case server.ID == current.MainMediaServer:
			return server, errors.New("saved sets already belong to the main media server")
		case !server.Enabled:
			return server, fmt.Errorf("%s is turned off", server.Name)
		}
		return server, nil
	}
	return config.Config_MediaServer{}, errors.New("the media server was not found")
}

func runMediaServerSync(ctx context.Context, server config.Config_MediaServer) {
	logCtx, ld := logging.CreateLoggingContext(context.Background(), "Apply Saved Sets to Media Server")
	logAction := ld.AddAction(fmt.Sprintf("Applying Saved Sets to %s", server.Name), logging.LevelInfo)
	logCtx = logging.WithCurrentAction(logCtx, logAction)

	err := syncMediaServerRecovered(ctx, logCtx, server)

	syncMu.Lock()
	now := time.Now()
	syncState.Running, syncState.Current, syncState.FinishedAt = false, "", &now
	if err != nil && !syncState.Stopped {
		syncState.Error = err.Error()
	}
	result := syncState.copy()
	syncMu.Unlock()

	logAction.AppendResult("saved_items", result.Total)
	logAction.AppendResult("checked", result.Done)
	logAction.AppendResult("images_applied", result.Applied)
	logAction.AppendResult("not_found", result.NotFound)
	logAction.AppendResult("failed", result.Failed)
	if result.Stopped {
		logAction.AppendResult("stopped", true)
	}
	if result.Error != "" {
		logAction.SetError(fmt.Sprintf("Could not finish applying the saved sets to %s", server.Name), result.Error, nil)
	}
	logAction.Complete()
	ld.Log()
}

// syncMediaServerRecovered runs the sync, and returns a panic as an error so aura keeps running
func syncMediaServerRecovered(ctx, logCtx context.Context, server config.Config_MediaServer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			logging.LOGGER.Error().Timestamp().Interface("recover", r).Str("stack", string(debug.Stack())).Msg("PANIC: while applying saved sets to a media server")
			err = fmt.Errorf("unexpected error: %v", r)
		}
	}()
	return syncMediaServer(ctx, logCtx, server)
}

// syncMediaServer applies the saved sets. ctx is cancelled when the sync is stopped, logCtx is for logging.
func syncMediaServer(ctx, logCtx context.Context, server config.Config_MediaServer) error {
	// Load the media server's libraries again, so new items are found
	if err := syncLoadMediaServer(logCtx, &server); err != nil {
		return err
	}
	clearOtherMediaItems()

	savedItems, err := syncGetSavedItems(logCtx)
	if err != nil {
		return err
	}
	updateSync(func(state *MediaServerSync) { state.Total = len(savedItems) })

	msClient, Err := newOtherMediaServerClient(&server)
	if Err.Message != "" {
		return errors.New(Err.Message)
	}

	for _, saved := range savedItems {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// The media server could be removed or made the main one while this runs
		if _, err := syncableMediaServer(server.ID); err != nil {
			return fmt.Errorf("stopped as the media server changed: %w", err)
		}

		title := utils.MediaItemInfo(saved.MediaItem)
		updateSync(func(state *MediaServerSync) { state.Current = title })

		applied, failures, found := applySavedItem(ctx, logCtx, server, msClient, saved)
		updateSync(func(state *MediaServerSync) {
			state.Done++
			state.Applied += applied
			if !found {
				state.NotFound++
			}
			state.Failed += len(failures)
			for _, failure := range failures {
				if len(state.Failures) < maxSyncFailures {
					state.Failures = append(state.Failures, fmt.Sprintf("%s: %s", title, failure))
				}
			}
		})
	}
	return nil
}

// applySavedItem applies the selected images of every saved set of an item
func applySavedItem(ctx, logCtx context.Context, server config.Config_MediaServer, msClient MediaServerInterface, saved models.DBSavedItem) (applied int, failures []string, found bool) {
	item, found, Err := FindMediaItemOnMediaServer(logCtx, server, saved.MediaItem)
	if Err.Message != "" {
		return 0, []string{Err.Message}, true
	}
	if !found {
		return 0, nil, false
	}

	for _, posterSet := range saved.PosterSets {
		for _, image := range models.SortImagesForApply(posterSet.Images) {
			if !models.IsKnownImageType(image.Type) || !models.ImageIsSelected(item, posterSet.SelectedTypes, image) {
				continue
			}
			if ctx.Err() != nil {
				return applied, failures, true
			}
			imageCtx, logAction := logging.AddSubActionToContext(logCtx, fmt.Sprintf("Applying the %s image", image.Type), logging.LevelDebug)
			Err := msClient.DownloadApplyImageToMediaItem(imageCtx, &item, image)
			logAction.Complete()
			if Err.Message != "" {
				failures = append(failures, fmt.Sprintf("%s image: %s", image.Type, Err.Message))
				logging.LOGGER.Warn().Timestamp().Str("media_server", server.Name).Str("item", utils.MediaItemInfo(saved.MediaItem)).
					Str("image_type", image.Type).Str("error", Err.Message).Msg("Could not apply a saved image to the media server")
				continue
			}
			applied++
		}
	}
	return applied, failures, true
}

// loadMediaServerForSync connects to the media server and loads its libraries into its cache
func loadMediaServerForSync(ctx context.Context, server *config.Config_MediaServer) error {
	connected, _ := connectMediaServer(ctx, server)
	if !connected {
		return fmt.Errorf("could not connect to %s", server.Name)
	}
	if !loadLibrariesIntoCache(ctx, *server, cache.ServerLibraryStore(server.ID), false) {
		return fmt.Errorf("could not load the libraries of %s", server.Name)
	}
	return nil
}

func getSavedItemsForSync(ctx context.Context) ([]models.DBSavedItem, error) {
	out, Err := database.GetAllSavedSets(ctx, models.DBFilter{ItemsPerPage: -1})
	if Err.Message != "" {
		return nil, errors.New(Err.Message)
	}
	return out.Items, nil
}
