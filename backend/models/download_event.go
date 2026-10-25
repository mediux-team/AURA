package models

import "time"

// What a download event did
const (
	DownloadKindDownload = "download" // A set was downloaded
	DownloadKindUpdate   = "update"   // Auto Download downloaded the new or changed images of a saved set
	DownloadKindReapply  = "reapply"  // Saved images were applied again, after a Plex refresh or a Sonarr upgrade
	DownloadKindSaved    = "saved"    // A set was saved without downloading, with Add to Database Only
)

// Where a download event came from, along with the Download Queue sources
const (
	SourceDownloadWindow = "download_window" // Downloaded from the download window
	SourceDownloadQueue  = "download_queue"  // Added to the Download Queue from the download window
	SourceAutoDownload   = "auto_download"   // The Auto Download job
	SourceSonarrWebhook  = "sonarr_webhook"  // Title cards applied again after Sonarr upgraded or renamed an episode
	SourcePlexRefresh    = "plex_refresh"    // Saved images applied again after Plex refreshed an item
	SourceBackfill       = "backfill"        // Counted from the saved sets when the statistics were added
)

// DownloadEvent is one set downloaded, updated or applied again for an item, for the statistics
type DownloadEvent struct {
	TMDB_ID         string
	LibraryTitle    string
	Edition         string
	MediaItemTitle  string
	MediaItemType   string
	SetID           string
	SetTitle        string
	SetType         string
	SetCreator      string
	Source          string
	Kind            string
	ImagesSucceeded int
	ImagesFailed    int
	ImageTypes      map[string]int // Images downloaded, by type
	Bytes           int64          // Size of the images downloaded
	Estimated       bool           // Counted from a saved set, not recorded when it was downloaded
	CreatedAt       time.Time
}

// NewDownloadEvent starts an event for a set of an item
func NewDownloadEvent(item MediaItem, set PosterSet, source, kind string) DownloadEvent {
	if source == "" {
		source = SourceDownloadQueue
	}
	return DownloadEvent{
		TMDB_ID:        item.TMDB_ID,
		LibraryTitle:   item.LibraryTitle,
		Edition:        item.Edition,
		MediaItemTitle: item.Title,
		MediaItemType:  item.Type,
		SetID:          set.ID,
		SetTitle:       set.Title,
		SetType:        set.Type,
		SetCreator:     set.UserCreated,
		Source:         source,
		Kind:           kind,
		ImageTypes:     map[string]int{},
	}
}

// NewCollectionDownloadEvent starts an event for a collection's own images
func NewCollectionDownloadEvent(collection CollectionItem, set *BaseSetInfo, source string) DownloadEvent {
	event := NewDownloadEvent(MediaItem{TMDB_ID: collection.TMDB_ID, LibraryTitle: collection.LibraryTitle, Title: collection.Title, Type: "collection"},
		PosterSet{}, source, DownloadKindDownload)
	if set != nil {
		event.SetID, event.SetTitle, event.SetType, event.SetCreator = set.ID, set.Title, set.Type, set.UserCreated
	}
	return event
}

// AddImage counts an image that was downloaded or failed
func (event *DownloadEvent) AddImage(image ImageFile, succeeded bool) {
	if !succeeded {
		event.ImagesFailed++
		return
	}
	if event.ImageTypes == nil {
		event.ImageTypes = map[string]int{}
	}
	event.ImagesSucceeded++
	event.ImageTypes[ImageTypeName(image)]++
	event.Bytes += image.FileSize
}

// ImageTypeName is the image type, with a special season poster on its own
func ImageTypeName(image ImageFile) string {
	if image.Type == "season_poster" && image.SeasonNumber != nil && *image.SeasonNumber == 0 {
		return "special_season_poster"
	}
	return image.Type
}

// Attempted is whether any image was downloaded or failed
func (event DownloadEvent) Attempted() bool {
	return event.ImagesSucceeded > 0 || event.ImagesFailed > 0
}
