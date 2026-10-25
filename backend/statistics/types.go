package statistics

import "time"

// Statistics is everything the Statistics page shows
type Statistics struct {
	GeneratedAt  time.Time          `json:"generated_at"`
	Days         int                `json:"days"`   // How many days back the graphs go
	Period       string             `json:"period"` // What each point of the graphs is: day, week or month
	Library      LibraryStats       `json:"library"`
	MediaServers []MediaServerStats `json:"media_servers"`
	SetsApplied  SetsAppliedStats   `json:"sets_applied"`
	Downloads    DownloadStats      `json:"downloads"`
	Creators     CreatorStats       `json:"creators"`
	Sources      []GroupCount       `json:"sources"`
	Failures     FailureStats       `json:"failures"`
	Data         DataStats          `json:"data"`
	Collections  CollectionStats    `json:"collections"`
	History      []Snapshot         `json:"history"` // The daily numbers in the range
}

// ItemCounts is how many items of each kind a library or media server has
type ItemCounts struct {
	Items    int `json:"items"`
	Movies   int `json:"movies"`
	Shows    int `json:"shows"`
	Seasons  int `json:"seasons"`
	Episodes int `json:"episodes"`
}

// LibraryStats is the main media server's libraries
type LibraryStats struct {
	Totals    ItemCounts      `json:"totals"`
	Libraries []LibraryCounts `json:"libraries"`
	Growth    []GrowthPoint   `json:"growth"` // Items in the libraries at the end of each point, by when they were added
}

// LibraryCounts is one library's items
type LibraryCounts struct {
	Title  string     `json:"title"`
	Type   string     `json:"type"`
	Counts ItemCounts `json:"counts"`
}

// GrowthPoint is how many movies and shows there were at the end of a day, week or month
type GrowthPoint struct {
	Date   string `json:"date"`
	Movies int    `json:"movies"`
	Shows  int    `json:"shows"`
}

// MediaServerStats is one media server's items, and for another media server, how it compares to the main one
type MediaServerStats struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Type              string     `json:"type"`
	Main              bool       `json:"main"`
	Counts            ItemCounts `json:"counts"`
	Matched           int        `json:"matched"`              // Items in both media servers
	OnlyInMainCount   int        `json:"only_in_main_count"`   // Items in the main media server, not in this one
	OnlyInServerCount int        `json:"only_in_server_count"` // Items in this media server, not in the main one
	OnlyInMain        []ItemRef  `json:"only_in_main"`         // The first of them, by title
	OnlyInServer      []ItemRef  `json:"only_in_server"`
	UnmappedLibraries []string   `json:"unmapped_libraries"` // Libraries that don't match a main media server library, so aren't compared
}

// ItemRef is an item in a list
type ItemRef struct {
	Title   string `json:"title"`
	Year    int    `json:"year"`
	Type    string `json:"type"`
	Library string `json:"library"`
	TMDB_ID string `json:"tmdb_id"`
}

// SetsAppliedStats is how many of the main media server's items have saved sets
type SetsAppliedStats struct {
	Items         int               `json:"items"`           // Movies and shows
	ItemsWithSets int               `json:"items_with_sets"` // Movies and shows with at least one saved set
	SavedSets     int               `json:"saved_sets"`
	Libraries     []LibraryCoverage `json:"libraries"`
	ImageTypes    []TypeCoverage    `json:"image_types"`
}

// LibraryCoverage is how many of a library's items have saved sets
type LibraryCoverage struct {
	Title         string `json:"title"`
	Items         int    `json:"items"`
	ItemsWithSets int    `json:"items_with_sets"`
}

// TypeCoverage is how many items have an image type selected in a saved set, out of the items that can have it
type TypeCoverage struct {
	Type  string `json:"type"`
	Items int    `json:"items"`
	Total int    `json:"total"`
}

// DownloadStats is the sets and images downloaded
type DownloadStats struct {
	AllTime    DownloadTotals  `json:"all_time"`
	InRange    DownloadTotals  `json:"in_range"`
	Timeline   []DownloadPoint `json:"timeline"`
	ImageTypes []GroupCount    `json:"image_types"` // Images downloaded of each type, in the range
}

// DownloadTotals is the downloads in a time
type DownloadTotals struct {
	Sets             int   `json:"sets"`              // Sets downloaded
	Images           int   `json:"images"`            // Images downloaded, with updates and collection images
	Redownloads      int   `json:"redownloads"`       // Sets downloaded again for the same item
	Updates          int   `json:"updates"`           // Sets Auto Download updated
	UpdatedImages    int   `json:"updated_images"`    // Images Auto Download downloaded
	Reapplies        int   `json:"reapplies"`         // Saved images applied again
	SavedOnly        int   `json:"saved_only"`        // Sets saved without downloading
	CollectionImages int   `json:"collection_images"` // Collections' own images
	ImagesFailed     int   `json:"images_failed"`
	Bytes            int64 `json:"bytes"`
	EstimatedSets    int   `json:"estimated_sets"` // Sets counted from the saved sets, before downloads were recorded
}

// DownloadPoint is the downloads of a day, week or month
type DownloadPoint struct {
	Date         string `json:"date"`
	Sets         int    `json:"sets"`
	Images       int    `json:"images"`
	Updates      int    `json:"updates"`
	Reapplies    int    `json:"reapplies"`
	ImagesFailed int    `json:"images_failed"`
	Bytes        int64  `json:"bytes"`
}

// GroupCount is the sets and images of a creator, source or image type
type GroupCount struct {
	Name     string `json:"name"`
	Sets     int    `json:"sets"`
	Images   int    `json:"images"`
	Favorite bool   `json:"favorite,omitempty"` // A favorite creator
}

// CreatorStats is the sets downloaded by creator, in the range
type CreatorStats struct {
	Top           []GroupCount `json:"top"`
	FavoriteSets  int          `json:"favorite_sets"` // Sets from favorite creators
	OtherSets     int          `json:"other_sets"`
	FavoriteCount int          `json:"favorite_count"` // How many favorite creators there are
}

// FailureStats is the images that couldn't be downloaded
type FailureStats struct {
	ImagesFailed int         `json:"images_failed"` // In the range
	FailureRate  float64     `json:"failure_rate"`  // Images failed out of the images tried, in the range
	OpenFailures int         `json:"open_failures"` // Download History entries
	TopReasons   []NameCount `json:"top_reasons"`
}

// NameCount is a name with how many times it was counted
type NameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// DataStats is the size of the images
type DataStats struct {
	BytesInRange    int64 `json:"bytes_in_range"`
	BytesAllTime    int64 `json:"bytes_all_time"`
	ImageCacheBytes int64 `json:"image_cache_bytes"` // MediUX images cached on disk
	ImageCacheFiles int   `json:"image_cache_files"`
}

// CollectionStats is the main media server's collections
type CollectionStats struct {
	Collections        int `json:"collections"`
	WithImages         int `json:"with_images"`           // Collections aura applied their own images to
	CollectionImages   int `json:"collection_images"`     // Their images applied, all time
	SavedCollectionSet int `json:"saved_collection_sets"` // Collection sets saved for movies
}

// Snapshot is the numbers of a day
type Snapshot struct {
	Date          string     `json:"date"`
	Counts        ItemCounts `json:"counts"`
	ItemsWithSets int        `json:"items_with_sets"`
	SavedSets     int        `json:"saved_sets"`
}
