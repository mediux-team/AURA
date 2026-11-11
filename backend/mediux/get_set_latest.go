package mediux

import (
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
)

// GetLatestSet gets a set's latest info and images from MediUX, for the item's library. A collection set only has the item's images.
func GetLatestSet(ctx context.Context, setID, setType string, item models.MediaItem) (set models.PosterSet, Err logging.LogErrorInfo) {
	var latest models.SetRef
	switch setType {
	case "show":
		latest, _, Err = GetShowSetByID(ctx, setID, item.LibraryTitle, item.Edition)
	case "movie":
		latest, _, Err = GetMovieSetByID(ctx, setID, item.LibraryTitle, item.Edition)
	case "collection":
		latest, _, Err = GetMovieCollectionSetByID(ctx, setID, item.TMDB_ID, item.LibraryTitle, item.Edition, true)
	default:
		return set, logging.LogErrorInfo{Message: fmt.Sprintf("Unknown set type '%s'", setType)}
	}
	return latest.PosterSet, Err
}
