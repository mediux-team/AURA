package models

import "time"

type BaseMediuxItemInfo struct {
	TMDB_ID           string    `json:"tmdb_id"`            // TMDB ID of the item
	Type              string    `json:"type"`               // Item type (movie, show)
	DateUpdated       time.Time `json:"date_updated"`       // Last update for the Movie/Show info
	Status            string    `json:"status"`             // Status of the item (returning, ended, etc.)
	Title             string    `json:"title"`              // Title of the item
	Tagline           string    `json:"tagline"`            // Tagline of the item
	ReleaseDate       string    `json:"release_date"`       // Release date of the item
	TvdbID            string    `json:"tvdb_id"`            // TVDB ID (for shows)
	ImdbID            string    `json:"imdb_id"`            // IMDB ID of the item
	TraktID           string    `json:"trakt_id"`           // Trakt ID of the item
	Slug              string    `json:"slug"`               // Slug of the item
	TMDB_PosterPath   string    `json:"tmdb_poster_path"`   // TMDB Poster path
	TMDB_BackdropPath string    `json:"tmdb_backdrop_path"` // TMDB Backdrop path
}

type BaseSetInfo struct {
	ID               string    `json:"id"`                // Set ID
	Title            string    `json:"title"`             // Set Title
	Type             string    `json:"type"`              // Set Type (movie, show, collection, boxset)
	UserCreated      string    `json:"user_created"`      // User who created the set
	DateCreated      time.Time `json:"date_created"`      // Creation date
	DateUpdated      time.Time `json:"date_updated"`      // Last updated date
	Popularity       int       `json:"popularity"`        // Popularity score of the set
	PopularityGlobal int       `json:"popularity_global"` // Global popularity score of the set
}

type ImageFile struct {
	ID            string    `json:"id"`                       // Unique asset ID for the poster file
	Type          string    `json:"type"`                     // (poster, backdrop, seasonPoster, specialSeasonPoster, titlecard)
	Modified      time.Time `json:"modified"`                 // Last modified date
	FileSize      int64     `json:"file_size"`                // Size of the file in bytes
	Src           string    `json:"src"`                      // Source URL (ID+Date - calculated by MediUX API)
	Blurhash      string    `json:"blurhash"`                 // Blurhash string for preview
	Language      string    `json:"language"`                 // Language Name
	ItemTMDB_ID   string    `json:"item_tmdb_id"`             // TMDB ID of the parent item
	Title         string    `json:"title,omitempty"`          // Present for Titlecards
	SeasonNumber  *int      `json:"season_number,omitempty"`  // Present for Season Posters and Titlecards
	EpisodeNumber *int      `json:"episode_number,omitempty"` // Present for Titlecards

}

type SetRef struct {
	PosterSet
	ItemIDs    []string        `json:"item_ids"`             // IDs of items in the set
	Collection *CollectionItem `json:"collection,omitempty"` // For a collection set on a movie's page, the media server collection its collection images apply to
}

type BoxsetRef struct {
	BaseSetInfo
	SetIDs map[string][]string `json:"set_ids"` // IDs of sets (show, movie, collection) in this boxset
}

type IncludedItem struct {
	MediuxInfo BaseMediuxItemInfo `json:"mediux_info"` // MediUX item info
	MediaItem  MediaItem          `json:"media_item"`  // Full media item info from Aura
}

type PosterSetsResponse struct {
	Sets          []SetRef                `json:"sets"`           // List of sets
	IncludedItems map[string]IncludedItem `json:"included_items"` // Map of included items by TMDB ID
}

type CreatorSetsResponse struct {
	ShowSets       []SetRef                `json:"show_sets"`       // List of show sets
	MovieSets      []SetRef                `json:"movie_sets"`      // List of movie sets
	CollectionSets []SetRef                `json:"collection_sets"` // List of collection sets
	Boxsets        []BoxsetRef             `json:"boxsets"`         // List of boxsets
	IncludedItems  map[string]IncludedItem `json:"included_items"`  // Map of included items by TMDB ID
}
