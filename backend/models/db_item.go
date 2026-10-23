package models

import "time"

// What is used to save a record into the database
// This contains the MediaItem details, as well as an array of PosterSets that are associated with it
type DBSavedItem struct {
	MediaItem  MediaItem           `json:"media_item"`
	PosterSets []DBPosterSetDetail `json:"poster_sets,omitempty"`
	Source     string              `json:"source,omitempty"` // What added it to the Download Queue, empty when it was the user
	// Saved sets of the item this replaces, they're removed once the new sets are saved
	ReplaceSetIDs []string `json:"replace_set_ids,omitempty"`
	// A collection's own poster and backdrop, applied to the media server collection after the sets. They aren't saved.
	CollectionImages *QueuedCollectionImages `json:"collection_images,omitempty"`
}

// QueuedCollectionImages are a collection's own images, and the media server collection they apply to
type QueuedCollectionImages struct {
	Collection CollectionItem `json:"collection"`
	Images     []ImageFile    `json:"images"`
}

// HasCollectionImages reports whether the item has collection images to apply
func (item DBSavedItem) HasCollectionImages() bool {
	return item.CollectionImages != nil && len(item.CollectionImages.Images) > 0
}

// IsCollectionOnly reports whether the item only has collection images to apply, and no sets
func (item DBSavedItem) IsCollectionOnly() bool {
	return len(item.PosterSets) == 0 && item.HasCollectionImages()
}

// QueueEntry returns the TMDB ID, library, edition and title the Download Queue lists the item by.
// An item with only collection images is listed by its collection.
func (item DBSavedItem) QueueEntry() (tmdbID, libraryTitle, edition, title string) {
	if item.IsCollectionOnly() {
		collection := item.CollectionImages.Collection
		return collection.TMDB_ID, collection.LibraryTitle, "", collection.Title
	}
	return item.MediaItem.TMDB_ID, item.MediaItem.LibraryTitle, item.MediaItem.Edition, item.MediaItem.Title
}

// WithSetImageTypes returns the item with each set's selected types limited to the image types the set can have
func (item DBSavedItem) WithSetImageTypes() DBSavedItem {
	sets := make([]DBPosterSetDetail, len(item.PosterSets))
	for i, set := range item.PosterSets {
		set.SelectedTypes = set.SelectedTypes.ForSetType(set.Type)
		sets[i] = set
	}
	item.PosterSets = sets
	return item
}

// The sources of Download Queue items that the user didn't add
const (
	SourceFavoriteCreatorsNewItem  = "favorite_creators_new_item"  // A new item, from a favorite creator
	SourceFavoriteCreatorsAllItems = "favorite_creators_all_items" // Run for All Items, from a favorite creator
	SourceSiloPlugin               = "silo_plugin"                 // Silo used an image from the aura plugin
	SourceAutoUpgrade              = "auto_upgrade"                // A higher ranked favorite creator's set replaces a saved set
)

// PosterSetDetail groups poster set details per media item.
type DBPosterSetDetail struct {
	PosterSet
	LastDownloaded            time.Time     `json:"last_downloaded"`
	SelectedTypes             SelectedTypes `json:"selected_types"`
	AutoDownload              bool          `json:"auto_download"`
	AutoAddNewCollectionItems bool          `json:"auto_add_new_collection_items"`
	NoAutoUpgrade             bool          `json:"no_auto_upgrade"` // Keep this set, Automatically Upgrade Sets doesn't replace it
	ToDelete                  bool          `json:"to_delete"`       // Flag to indicate if the poster set should be deleted (Not used in DB)
}

type PosterSet struct {
	BaseSetInfo
	Images []ImageFile `json:"images"`
}

type DBFilter struct {
	ItemTMDB_ID       string   `json:"item_tmdb_id"`
	ItemLibraryTitle  string   `json:"item_library_title"`
	ItemYear          int      `json:"item_year"`
	ItemTitle         string   `json:"item_title"`
	SetID             string   `json:"set_id"`
	LibraryTitles     []string `json:"library_titles"`
	ImageTypes        []string `json:"image_types"`
	Autodownload      string   `json:"autodownload"`
	MultiSetOnly      bool     `json:"multiset_only"`
	Usernames         []string `json:"usernames"`
	MediaItemOnServer string   `json:"media_item_on_server"`
	ItemsPerPage      int      `json:"items_per_page"`
	PageNumber        int      `json:"page_number"`
	SortOption        string   `json:"sort_option"`
	SortOrder         string   `json:"sort_order"`
}
