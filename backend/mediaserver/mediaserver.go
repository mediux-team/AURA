package mediaserver

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver/ej"
	"aura/mediaserver/plex"
	"aura/models"
	"context"
	"fmt"
)

type MediaServerInterface interface {
	// Test Connection (returns the Server Version if connection is successful)
	TestConnection(ctx context.Context, msConfig config.Config_MediaServer) (connectionOk bool, serverName string, serverVersion string, Err logging.LogErrorInfo)

	// Get Admin User (this is for Emby/Jellyfin)
	GetAdminUser(ctx context.Context, msConfig config.Config_MediaServer) (userID string, Err logging.LogErrorInfo)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	// Get a list of all library sections
	GetLibrarySections(ctx context.Context, msConfig config.Config_MediaServer) (sections []models.LibrarySection, Err logging.LogErrorInfo)

	// Get full details about a specific library section
	GetLibrarySectionDetails(ctx context.Context, library *models.LibrarySection) (found bool, Err logging.LogErrorInfo)

	// Get items in a specific library section
	GetLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string, enableSortByNewEpisode bool) ([]models.MediaItem, int, logging.LogErrorInfo)

	// Get Movie Collections for a specific library section
	GetMovieCollections(ctx context.Context, section models.LibrarySection) (collections []models.CollectionItem, Err logging.LogErrorInfo)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	// Get the image for a specific media item
	GetMediaItemImage(ctx context.Context, item *models.MediaItem, imageRatingKey string, imageType string) (imageData []byte, Err logging.LogErrorInfo)

	// Get the image for a specific media item
	GetCollectionItemImage(ctx context.Context, item *models.CollectionItem, imageType string) (imageData []byte, Err logging.LogErrorInfo)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	// Get a collection's children items
	GetMovieCollectionChildrenItems(ctx context.Context, collection *models.CollectionItem) (Err logging.LogErrorInfo)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	// Get full details about a specific media item
	GetMediaItemDetails(ctx context.Context, item *models.MediaItem) (found bool, Err logging.LogErrorInfo)

	// Refresh the metadata for a specific media item
	RefreshMediaItemMetadata(ctx context.Context, item *models.MediaItem, refreshRatingKey string, updateImage bool) (Err logging.LogErrorInfo)

	// Handle Labels (Plex Exclusive)
	AddLabelToMediaItem(ctx context.Context, item models.MediaItem, selectedTypes models.SelectedTypes) (Err logging.LogErrorInfo)

	// Rate a specific media item (Plex Exclusive)
	RateMediaItem(ctx context.Context, item *models.MediaItem, rating float64) (Err logging.LogErrorInfo)

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	// Download an image for a specific Media Item
	DownloadApplyImageToMediaItem(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) (Err logging.LogErrorInfo)

	// Apply a collection image to a specific Collection Item
	ApplyCollectionImage(ctx context.Context, collectionItem *models.CollectionItem, imageFile models.ImageFile) (Err logging.LogErrorInfo)
}

func resolveMediaServerConfig(ms *config.Config_MediaServer) *config.Config_MediaServer {
	if ms == nil {
		return &config.Current.MediaServer
	}
	return ms
}

func NewMediaServerClient(mediaServerConfig *config.Config_MediaServer) (MediaServerInterface, logging.LogErrorInfo) {
	cfg := resolveMediaServerConfig(mediaServerConfig)

	switch cfg.Type {
	case "Plex":
		return &plex.Plex{Config: *cfg}, logging.LogErrorInfo{}
	case "Jellyfin", "Emby":
		return &ej.EJ{Config: *cfg}, logging.LogErrorInfo{}
	default:
		return nil, logging.LogErrorInfo{
			Message: fmt.Sprintf("unsupported media server type: %s", cfg.Type),
		}
	}
}

func TestConnection(ctx context.Context, mediaServerConfig *config.Config_MediaServer) (connectionOk bool, serverName string, serverVersion string, Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(mediaServerConfig)
	if Err.Message != "" {
		return false, "", "", Err
	}
	return msClient.TestConnection(ctx, *mediaServerConfig)
}

func GetAdminUser(ctx context.Context, mediaServerConfig *config.Config_MediaServer) (userID string, Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(mediaServerConfig)
	if Err.Message != "" {
		return "", Err
	}
	return msClient.GetAdminUser(ctx, *mediaServerConfig)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func GetLibrarySections(ctx context.Context, mediaServerConfig *config.Config_MediaServer) (sections []models.LibrarySection, Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(mediaServerConfig)
	if Err.Message != "" {
		return nil, Err
	}
	return msClient.GetLibrarySections(ctx, *mediaServerConfig)
}

func GetLibrarySectionDetails(ctx context.Context, library *models.LibrarySection) (found bool, Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return false, Err
	}
	return msClient.GetLibrarySectionDetails(ctx, library)
}

func GetLibrarySectionItems(ctx context.Context, section models.LibrarySection, sectionStartIndex string, limit string, enableSortByNewEpisode bool) ([]models.MediaItem, int, logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return nil, 0, Err
	}
	return msClient.GetLibrarySectionItems(ctx, section, sectionStartIndex, limit, enableSortByNewEpisode)
}

func GetMovieCollections(ctx context.Context, section models.LibrarySection) (collections []models.CollectionItem, Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return nil, Err
	}
	return msClient.GetMovieCollections(ctx, section)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func GetMediaItemImage(ctx context.Context, item *models.MediaItem, imageRatingKey string, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return nil, Err
	}
	return msClient.GetMediaItemImage(ctx, item, imageRatingKey, imageType)
}

func GetCollectionItemImage(ctx context.Context, item *models.CollectionItem, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return nil, Err
	}
	return msClient.GetCollectionItemImage(ctx, item, imageType)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func GetCollectionChildrenItems(ctx context.Context, collection *models.CollectionItem) (Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return Err
	}
	return msClient.GetMovieCollectionChildrenItems(ctx, collection)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func GetMediaItemDetails(ctx context.Context, item *models.MediaItem) (found bool, Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return false, Err
	}
	return msClient.GetMediaItemDetails(ctx, item)
}

func RefreshMediaItemMetadata(ctx context.Context, item *models.MediaItem, refreshRatingKey string, updateImage bool) (Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return Err
	}
	return msClient.RefreshMediaItemMetadata(ctx, item, refreshRatingKey, updateImage)
}

func AddLabelToMediaItem(ctx context.Context, item models.MediaItem, selectedTypes models.SelectedTypes) (Err logging.LogErrorInfo) {
	if config.Current.MediaServer.Type != "Plex" {
		return logging.LogErrorInfo{}
	} else if len(config.Current.LabelsAndTags.Applications) == 0 {
		return logging.LogErrorInfo{}
	}
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return Err
	}
	return msClient.AddLabelToMediaItem(ctx, item, selectedTypes)
}

func RateMediaItem(ctx context.Context, item *models.MediaItem, rating float64) (Err logging.LogErrorInfo) {
	if config.Current.MediaServer.Type != "Plex" {
		return logging.LogErrorInfo{}
	}
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return Err
	}
	return msClient.RateMediaItem(ctx, item, rating)
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func DownloadApplyImageToMediaItem(ctx context.Context, item *models.MediaItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return Err
	}
	return msClient.DownloadApplyImageToMediaItem(ctx, item, imageFile)
}

func ApplyCollectionImage(ctx context.Context, collectionItem *models.CollectionItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
	if Err.Message != "" {
		return Err
	}
	return msClient.ApplyCollectionImage(ctx, collectionItem, imageFile)
}
