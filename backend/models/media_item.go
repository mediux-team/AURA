package models

type MediaItem struct {
	TMDB_ID      string           `json:"tmdb_id"`          // TMDB ID of the media item
	LibraryTitle string           `json:"library_title"`    // Title of the library/section the item belongs to
	RatingKey    string           `json:"rating_key"`       // RatingKey is the internal ID from the media server
	Type         string           `json:"type"`             // "movie" or "show"
	Title        string           `json:"title"`            // Title of the media item
	Year         int              `json:"year"`             // Release year of the media item
	Movie        *MediaItemMovie  `json:"movie,omitempty"`  // Present if Type is "movie"; Contains file info
	Series       *MediaItemSeries `json:"series,omitempty"` // Present if Type is "show"; Contains seasons and episodes info

	// Used in MediaItem Details Page - For poster sets
	DBSavedSets []DBSavedSet `json:"db_saved_sets"` // Poster sets saved in the database for this item
	IgnoredInDB bool         `json:"ignored_in_db"` // Whether the item is marked as ignored
	IgnoredMode string       `json:"ignored_mode"`  // Mode of ignoring (e.g., "always", "until-set-available", "until-new-set-available")
	IgnoredSets []string     `json:"ignored_sets"`  // List of set IDs that were present when the item was ignored (used for "until-new-set-available" mode)

	// Used in Home Page - sorting and filtering
	HasMediuxSets        bool  `json:"has_mediux_sets"` // Whether the item has MediUX sets
	UpdatedAt            int64 `json:"updated_at"`      // Last updated timestamp
	AddedAt              int64 `json:"added_at"`        // Added timestamp
	ReleasedAt           int64 `json:"released_at"`     // Release date timestamp
	LatestEpisodeAddedAt int64 `json:"latest_episode_added_at"`

	// Used in MediaItem Details Page - For ratings
	Guids []MediaItemGuid `json:"guids"` // List of GUIDs from different providers

	// Used in MediaItem Details Page - For more information about the item
	ContentRating string `json:"content_rating"` // Content rating (e.g., "PG-13")
	Summary       string `json:"summary"`        // Summary or description of the media item
	Edition       string `json:"edition"`        // Edition of the media item (e.g., "Director's Cut")
}

type MediaItemGuid struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Rating   string `json:"rating"`
}

type MediaItemMovie struct {
	File MediaItemFile `json:"file"`
}

type MediaItemSeries struct {
	Seasons      []MediaItemSeason `json:"seasons"`
	SeasonCount  int               `json:"season_count"`
	EpisodeCount int               `json:"episode_count"`
	Location     string            `json:"location"`
}

type MediaItemSeason struct {
	RatingKey    string             `json:"rating_key"`
	SeasonNumber int                `json:"season_number"`
	Title        string             `json:"title"`
	Episodes     []MediaItemEpisode `json:"episodes"`
}

type MediaItemEpisode struct {
	RatingKey     string        `json:"rating_key"`
	Title         string        `json:"title"`
	SeasonNumber  int           `json:"season_number"`
	EpisodeNumber int           `json:"episode_number"`
	// MediuxEpisodeNumber is the episode number to match against MediUX images.
	// It usually equals EpisodeNumber, but for Emby/Jellyfin it can diverge when
	// an earlier episode in the season is a merged multi-episode file: Jellyfin
	// numbers the merged file across multiple index slots (e.g. 1-2), while
	// MediUX (TMDB-based) counts it as a single slot, shifting every following
	// episode's MediUX number down by the difference.
	MediuxEpisodeNumber int           `json:"mediux_episode_number"`
	AddedAt              int64         `json:"added_at"`
	File                 MediaItemFile `json:"file"`
}

type MediaItemFile struct {
	Path     string `json:"path"`
	Size     int64  `json:"size,omitempty"`
	Duration int64  `json:"duration,omitempty"`
}

type DBSavedSet struct {
	ID            string        `json:"id"`
	UserCreated   string        `json:"user_created"`
	SelectedTypes SelectedTypes `json:"selected_types"`
}

type SelectedTypes struct {
	Poster              bool `json:"poster"`
	Backdrop            bool `json:"backdrop"`
	SeasonPoster        bool `json:"season_poster"`
	SpecialSeasonPoster bool `json:"special_season_poster"`
	Titlecard           bool `json:"titlecard"`
}

type CollectionItem struct {
	RatingKey    string      `json:"rating_key"`
	Index        string      `json:"index"` // Unique identifier for the collection in Plex
	TMDB_ID      string      `json:"tmdb_id,omitempty"`
	Title        string      `json:"title"`
	Summary      string      `json:"summary,omitempty"`
	ChildCount   int         `json:"child_count"`
	MediaItems   []MediaItem `json:"media_items"`
	LibraryTitle string      `json:"library_title,omitempty"`
}
