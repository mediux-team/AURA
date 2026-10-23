package database

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"database/sql"
	"errors"
)

const LATEST_DB_VERSION = 14

var Client DB

type SQliteDB struct {
	conn *sql.DB
}

type DB interface {
	// Open Database Connection
	GetDBConnection(ctx context.Context) (conn *sql.DB, newDB bool, Err logging.LogErrorInfo)

	// Close Database Connection
	Close(ctx context.Context) (Err logging.LogErrorInfo)

	// Check the database can be read
	Check(ctx context.Context) error

	// Back up the database to the backups folder, keeping the newest backups
	CreateBackup(ctx context.Context, keep int) (backup DatabaseBackup, Err logging.LogErrorInfo)

	// Initialize the database connection
	Init(ctx context.Context) (newDB bool, Err logging.LogErrorInfo)

	// Create Main Tables
	CreateTables(ctx context.Context) (Err logging.LogErrorInfo)

	// Create VERSION table
	CreateVersionTable(ctx context.Context) (Err logging.LogErrorInfo)

	// Get all stored config values
	GetAllConfigValues(ctx context.Context) (values map[string]string, Err logging.LogErrorInfo)

	// Save config values. Only changed values are written. If replace is true, other stored values are removed.
	SaveConfigValues(ctx context.Context, values map[string]string, replace bool) (Err logging.LogErrorInfo)

	// Get the current database version (from VERSION table)
	GetCurrentVersion(ctx context.Context) (version int, Err logging.LogErrorInfo)

	// Update the VERSION table to newVersion
	UpdateVersionTable(ctx context.Context, newVersion int) (Err logging.LogErrorInfo)

	// Perform database vacuuming to optimize the database
	Vacuum(ctx context.Context) (Err logging.LogErrorInfo)

	// Create Auth table
	CreateAuthTable(ctx context.Context) (Err logging.LogErrorInfo)

	// Get Auth Token Secret
	GetAuthTokenSecret(ctx context.Context) (secret string, Err logging.LogErrorInfo)

	// Backup Database
	Backup(ctx context.Context, currentVersion, newVersion int) (Err logging.LogErrorInfo)

	// Upsert Converted Saved Item
	UpsertSavedItem(ctx context.Context, newItem models.DBSavedItem) (Err logging.LogErrorInfo)

	// Check Media Item Exists
	CheckIfMediaItemExists(ctx context.Context, TMDB_ID, libraryTitle, edition string) (ignored bool, ignoredMode string, sets []models.DBSavedSet, logErr logging.LogErrorInfo)

	// Get All Media Items
	GetAllMediaItems(ctx context.Context) (items []models.MediaItem, logErr logging.LogErrorInfo)

	// Get All Media Items with info on whether they have saved sets or are ignored
	GetAllMediaItemsWithFlags(ctx context.Context) (items []MediaItemWithFlags, logErr logging.LogErrorInfo)

	// Update Media Item
	UpdateMediaItem(ctx context.Context, updatedItem models.MediaItem) (Err logging.LogErrorInfo)

	// Delete Media Item and Ignored Item entries for a given TMDB ID and Library Title
	DeleteMediaItemAndIgnoredStatus(ctx context.Context, TMDB_ID, libraryTitle, edition string) (Err logging.LogErrorInfo)

	// Get All Saved Sets
	GetAllSavedSets(ctx context.Context, dbFilter models.DBFilter) (out PagedSavedItems, logErr logging.LogErrorInfo)

	// Get All Unique Users from Saved Sets
	GetAllUniqueUsers(ctx context.Context) (users []string, logErr logging.LogErrorInfo)

	// Get Count of Saved Sets (Unique TMDB ID + Library Title combinations)
	//GetCountSavedSets(ctx context.Context) (count int, logErr logging.LogErrorInfo)

	// Delete Poster Set (and associated images) by ID
	DeletePosterSetForMediaItem(ctx context.Context, tmdbID, libraryTitle, edition, setID string) (Err logging.LogErrorInfo)

	// Delete All Poster Sets for Media Item
	DeleteAllPosterSetsForMediaItem(ctx context.Context, tmdbID, libraryTitle, edition string) (Err logging.LogErrorInfo)

	// Ignore Media Item
	IgnoreMediaItem(ctx context.Context, tmdbID, libraryTitle, edition, mode, currentSets string) (Err logging.LogErrorInfo)

	// Stop Ignoring Media Item
	StopIgnoringMediaItem(ctx context.Context, TMDB_ID, libraryTitle, edition string) (Err logging.LogErrorInfo)

	// Get Temp Ignored Items
	GetTempIgnoredItems(ctx context.Context) (items []models.MediaItem, Err logging.LogErrorInfo)

	// Update Media Item on_server flag
	UpdateMediaItemOnServer(ctx context.Context, tmdbID string, libraryTitle string, edition string, onServer bool) (logErr logging.LogErrorInfo)

	// Get the saved sets and ignored state of every item in a library
	GetLibraryDBStates(ctx context.Context, libraryTitle string) (map[string]MediaItemDBState, logging.LogErrorInfo)

	// Save a page of the main media server's items
	SaveLibraryItems(ctx context.Context, items []models.MediaItem, seenAt int64) logging.LogErrorInfo

	// Get the creators of the saved sets and how many of their sets are saved
	GetSavedSetCreators(ctx context.Context) ([]CreatorSetCount, logging.LogErrorInfo)

	// Get the main media server's items that are on the server
	GetLibraryItemsOnServer(ctx context.Context) ([]models.MediaItem, logging.LogErrorInfo)

	// Remove the library items that are no longer on the main media server
	FinishLibraryLoad(ctx context.Context, usedLibraries []string, loadedSince map[string]int64) logging.LogErrorInfo
	SaveCollections(ctx context.Context, collections []models.CollectionItem) logging.LogErrorInfo
	GetCollections(ctx context.Context) ([]models.CollectionItem, logging.LogErrorInfo)

	// Move saved Media Items to the items on a new main media server, all or none of them
	MoveMediaItems(ctx context.Context, moves []MediaItemMove) (Err logging.LogErrorInfo)

	// Put jobs another worker was processing back in the queue
	ResetInterruptedDownloadQueueJobs(ctx context.Context, workerID string) (count int64, Err logging.LogErrorInfo)
	// Count the download queue jobs that are waiting or running
	CountActiveDownloadQueueJobs(ctx context.Context) (count int, Err logging.LogErrorInfo)

	// Reconcile a Media Item whose Edition changed
	ReconcileMediaItemEdition(ctx context.Context, tmdbID, libraryTitle, oldEdition string, updatedItem models.MediaItem) (Err logging.LogErrorInfo)

	// Enqueue a new Download Queue Job
	EnqueueDownloadQueueJob(ctx context.Context, item models.DBSavedItem) (jobID int64, Err logging.LogErrorInfo)

	// Atomically claim the next pending Download Queue Job, if any
	ClaimNextDownloadQueueJob(ctx context.Context, workerID string) (job DownloadQueueJob, found bool, Err logging.LogErrorInfo)

	// Mark a Download Queue Job as finished with its result
	FinishDownloadQueueJob(ctx context.Context, jobID int64, status string, message string, errs, warnings []string) (Err logging.LogErrorInfo)

	// Get All Download Queue Jobs
	GetAllDownloadQueueJobs(ctx context.Context) (jobs []DownloadQueueJob, Err logging.LogErrorInfo)

	// Delete a single Download Queue Job by ID
	DeleteDownloadQueueJob(ctx context.Context, jobID int64) (Err logging.LogErrorInfo)

	// Delete all Download Queue Jobs (except those currently processing)
	DeleteAllDownloadQueueJobs(ctx context.Context) (Err logging.LogErrorInfo)

	// Insert a Download History entry, or update the one for the same item and set
	InsertDownloadHistoryEntry(ctx context.Context, entry DownloadHistoryEntry) (Err logging.LogErrorInfo)

	// Get Download History entries
	GetDownloadHistory(ctx context.Context, limit, offset int) (entries []DownloadHistoryEntry, total int, Err logging.LogErrorInfo)

	// Get a single Download History entry by ID
	GetDownloadHistoryEntry(ctx context.Context, id int64) (entry DownloadHistoryEntry, found bool, Err logging.LogErrorInfo)

	// Get the failed downloads due to be retried automatically
	GetDueDownloadRetries(ctx context.Context) (entries []DownloadHistoryEntry, Err logging.LogErrorInfo)

	// Count a retry of a failed download
	MarkDownloadHistoryRetried(ctx context.Context, id int64) (Err logging.LogErrorInfo)

	// Stop the automatic retries of a failed download
	StopDownloadRetries(ctx context.Context, id int64) (Err logging.LogErrorInfo)

	// Delete a single Download History entry by ID
	DeleteDownloadHistoryEntry(ctx context.Context, id int64) (Err logging.LogErrorInfo)

	// Delete an item's failures for a set that downloaded without any
	DeleteDownloadHistoryForSet(ctx context.Context, tmdbID, libraryTitle, edition, setID string) (Err logging.LogErrorInfo)

	// Delete all Download History entries
	DeleteAllDownloadHistory(ctx context.Context) (Err logging.LogErrorInfo)

	// Get all saved Silo Item IDs
	GetAllSiloItemIDs(ctx context.Context) (items []SiloItemIDs, Err logging.LogErrorInfo)

	// Insert or update Silo Item IDs
	UpsertSiloItemIDs(ctx context.Context, items []SiloItemIDs) (Err logging.LogErrorInfo)
}

func NewDatabaseClient() (DB, logging.LogErrorInfo) {
	return &SQliteDB{}, logging.LogErrorInfo{}
}

func GetDBConnection(ctx context.Context) (conn *sql.DB, newDB bool, Err logging.LogErrorInfo) {
	if Client == nil {
		return nil, false, logging.Error_DBClientNotInitialized()
	}
	return Client.GetDBConnection(ctx)
}

func Init(ctx context.Context) (newDB bool, Err logging.LogErrorInfo) {
	dbClient, err := NewDatabaseClient()
	if err.Message != "" {
		return false, err
	}
	Client = dbClient
	return dbClient.Init(ctx)
}

func Close(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.LogErrorInfo{}
	}
	return Client.Close(ctx)
}

// IsOpen reports whether the database has been opened yet
func IsOpen() bool {
	return Client != nil
}

func Check(ctx context.Context) error {
	if Client == nil {
		return errors.New("the database is not open")
	}
	return Client.Check(ctx)
}

func CreateBackup(ctx context.Context, keep int) (backup DatabaseBackup, Err logging.LogErrorInfo) {
	if Client == nil {
		return backup, logging.Error_DBClientNotInitialized()
	}
	return Client.CreateBackup(ctx, keep)
}

func GetAllConfigValues(ctx context.Context) (values map[string]string, Err logging.LogErrorInfo) {
	if Client == nil {
		return map[string]string{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllConfigValues(ctx)
}

func SaveConfigValues(ctx context.Context, values map[string]string, replace bool) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.SaveConfigValues(ctx, values, replace)
}

// configStore stores the config in the database
type configStore struct{}

func (configStore) GetAllConfigValues(ctx context.Context) (map[string]string, logging.LogErrorInfo) {
	return GetAllConfigValues(ctx)
}

func (configStore) SaveConfigValues(ctx context.Context, values map[string]string, replace bool) logging.LogErrorInfo {
	return SaveConfigValues(ctx, values, replace)
}

// ConfigStore returns the store used to load and save the config
func ConfigStore() config.Store {
	return configStore{}
}

func CreateVersionTable(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.CreateVersionTable(ctx)
}

func GetCurrentVersion(ctx context.Context) (version int, Err logging.LogErrorInfo) {
	if Client == nil {
		return 0, logging.Error_DBClientNotInitialized()
	}
	return Client.GetCurrentVersion(ctx)
}

func UpdateVersionTable(ctx context.Context, newVersion int) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.UpdateVersionTable(ctx, newVersion)
}

func CreateAuthTable(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.CreateAuthTable(ctx)
}

func GetAuthTokenSecret(ctx context.Context) (secret string, Err logging.LogErrorInfo) {
	if Client == nil {
		return "", logging.Error_DBClientNotInitialized()
	}
	return Client.GetAuthTokenSecret(ctx)
}

func Vacuum(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.Vacuum(ctx)
}

func CreateTables(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.CreateTables(ctx)
}

func Backup(ctx context.Context, currentVersion, newVersion int) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.Backup(ctx, currentVersion, newVersion)
}

func UpsertSavedItem(ctx context.Context, newItem models.DBSavedItem) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	Err = Client.UpsertSavedItem(ctx, newItem)
	if Err.Message != "" {
		return Err
	}

	// Silo uses one item for every library it is in, so images applied in one library are shown in all of them
	// Save the same sets for the item in the other libraries as well
	if config.Current.MediaServer.Type == "Silo" && newItem.MediaItem.RatingKey != "" {
		for _, otherItem := range cache.LibraryStore.GetMediaItemsByRatingKey(newItem.MediaItem.RatingKey) {
			if otherItem.LibraryTitle == newItem.MediaItem.LibraryTitle {
				continue
			}
			sharedItem := newItem
			sharedItem.MediaItem.LibraryTitle = otherItem.LibraryTitle
			// Use the file info for the other library, if it is known
			if otherItem.Movie != nil {
				sharedItem.MediaItem.Movie = otherItem.Movie
			}
			if otherItem.Series != nil {
				sharedItem.MediaItem.Series = otherItem.Series
			}
			Err = Client.UpsertSavedItem(ctx, sharedItem)
			if Err.Message != "" {
				return Err
			}
			syncLibraryCache(ctx, sharedItem.MediaItem.TMDB_ID, sharedItem.MediaItem.Edition, sharedItem.MediaItem.LibraryTitle)
		}
	}
	syncLibraryCache(ctx, newItem.MediaItem.TMDB_ID, newItem.MediaItem.Edition, newItem.MediaItem.LibraryTitle)
	return logging.LogErrorInfo{}
}

func CheckIfMediaItemExists(ctx context.Context, TMDB_ID, libraryTitle, edition string) (ignored bool, ignoreMode string, sets []models.DBSavedSet, logErr logging.LogErrorInfo) {
	if Client == nil {
		return false, "", []models.DBSavedSet{}, logging.Error_DBClientNotInitialized()
	}
	return Client.CheckIfMediaItemExists(ctx, TMDB_ID, libraryTitle, edition)
}

func GetAllMediaItems(ctx context.Context) (items []models.MediaItem, logErr logging.LogErrorInfo) {
	if Client == nil {
		return []models.MediaItem{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllMediaItems(ctx)
}

func GetAllMediaItemsWithFlags(ctx context.Context) (items []MediaItemWithFlags, logErr logging.LogErrorInfo) {
	if Client == nil {
		return []MediaItemWithFlags{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllMediaItemsWithFlags(ctx)
}

func UpdateMediaItem(ctx context.Context, updatedItem models.MediaItem) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.UpdateMediaItem(ctx, updatedItem)
}

func DeleteMediaItemAndIgnoredStatus(ctx context.Context, TMDB_ID, libraryTitle, edition string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.DeleteMediaItemAndIgnoredStatus(ctx, TMDB_ID, libraryTitle, edition)
}

func GetAllSavedSets(ctx context.Context, dbFilter models.DBFilter) (out PagedSavedItems, logErr logging.LogErrorInfo) {
	if Client == nil {
		return PagedSavedItems{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllSavedSets(ctx, dbFilter)
}

func GetAllUniqueUsers(ctx context.Context) (users []string, logErr logging.LogErrorInfo) {
	if Client == nil {
		return []string{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllUniqueUsers(ctx)
}

// func GetCountSavedSets(ctx context.Context) (count int, logErr logging.LogErrorInfo) {
// 	if Client == nil {
// 		return 0, logging.Error_DBClientNotInitialized()
// 	}
// 	return Client.GetCountSavedSets(ctx)
// }

func DeletePosterSetForMediaItem(ctx context.Context, tmdbID, libraryTitle, edition, setID string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	Err = Client.DeletePosterSetForMediaItem(ctx, tmdbID, libraryTitle, edition, setID)
	if Err.Message != "" {
		return Err
	}

	// Silo uses one item for every library it is in, so remove the set from the other libraries as well
	applyToSharedLibraries(ctx, tmdbID, libraryTitle, edition, func(otherLibraryTitle string) logging.LogErrorInfo {
		return Client.DeletePosterSetForMediaItem(ctx, tmdbID, otherLibraryTitle, edition, setID)
	})
	return logging.LogErrorInfo{}
}

func DeleteAllPosterSetsForMediaItem(ctx context.Context, tmdbID, libraryTitle, edition string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	Err = Client.DeleteAllPosterSetsForMediaItem(ctx, tmdbID, libraryTitle, edition)
	if Err.Message != "" {
		return Err
	}

	// Silo uses one item for every library it is in, so remove the sets from the other libraries as well
	applyToSharedLibraries(ctx, tmdbID, libraryTitle, edition, func(otherLibraryTitle string) logging.LogErrorInfo {
		return Client.DeleteAllPosterSetsForMediaItem(ctx, tmdbID, otherLibraryTitle, edition)
	})
	return logging.LogErrorInfo{}
}

func IgnoreMediaItem(ctx context.Context, tmdbID, libraryTitle, edition, mode, currentSets string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	Err = Client.IgnoreMediaItem(ctx, tmdbID, libraryTitle, edition, mode, currentSets)
	if Err.Message != "" {
		return Err
	}

	// Silo uses one item for every library it is in, so ignore the item in the other libraries as well
	applyToSharedLibraries(ctx, tmdbID, libraryTitle, edition, func(otherLibraryTitle string) logging.LogErrorInfo {
		return Client.IgnoreMediaItem(ctx, tmdbID, otherLibraryTitle, edition, mode, currentSets)
	})
	return logging.LogErrorInfo{}
}

func StopIgnoringMediaItem(ctx context.Context, TMDB_ID, libraryTitle, edition string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	Err = Client.StopIgnoringMediaItem(ctx, TMDB_ID, libraryTitle, edition)
	if Err.Message != "" {
		return Err
	}

	// Silo uses one item for every library it is in, so stop ignoring the item in the other libraries as well
	applyToSharedLibraries(ctx, TMDB_ID, libraryTitle, edition, func(otherLibraryTitle string) logging.LogErrorInfo {
		return Client.StopIgnoringMediaItem(ctx, TMDB_ID, otherLibraryTitle, edition)
	})
	return logging.LogErrorInfo{}
}

func GetTempIgnoredItems(ctx context.Context) (items []models.MediaItem, Err logging.LogErrorInfo) {
	if Client == nil {
		return []models.MediaItem{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetTempIgnoredItems(ctx)
}

// GetLibraryDBStates returns the saved sets and ignored state of every item in a library that has either
func GetLibraryDBStates(ctx context.Context, libraryTitle string) (map[string]MediaItemDBState, logging.LogErrorInfo) {
	if Client == nil {
		return map[string]MediaItemDBState{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetLibraryDBStates(ctx, libraryTitle)
}

// SaveLibraryItems saves a page of the main media server's items
func SaveLibraryItems(ctx context.Context, items []models.MediaItem, seenAt int64) logging.LogErrorInfo {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.SaveLibraryItems(ctx, items, seenAt)
}

// GetSavedSetCreators returns the creators of the saved sets, the most saved sets first
func GetSavedSetCreators(ctx context.Context) ([]CreatorSetCount, logging.LogErrorInfo) {
	if Client == nil {
		return []CreatorSetCount{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetSavedSetCreators(ctx)
}

// GetLibraryItemsOnServer returns the main media server's items that are on the server
func GetLibraryItemsOnServer(ctx context.Context) ([]models.MediaItem, logging.LogErrorInfo) {
	if Client == nil {
		return []models.MediaItem{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetLibraryItemsOnServer(ctx)
}

// SaveCollections replaces the main media server's collections and the movies in them
func SaveCollections(ctx context.Context, collections []models.CollectionItem) logging.LogErrorInfo {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.SaveCollections(ctx, collections)
}

// GetCollections returns the main media server's collections and the movies in them
func GetCollections(ctx context.Context) ([]models.CollectionItem, logging.LogErrorInfo) {
	if Client == nil {
		return []models.CollectionItem{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetCollections(ctx)
}

// FinishLibraryLoad removes the library items that are no longer on the main media server
func FinishLibraryLoad(ctx context.Context, usedLibraries []string, loadedSince map[string]int64) logging.LogErrorInfo {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.FinishLibraryLoad(ctx, usedLibraries, loadedSince)
}

func UpdateMediaItemOnServer(ctx context.Context, tmdbID string, libraryTitle string, edition string, onServer bool) (logErr logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.UpdateMediaItemOnServer(ctx, tmdbID, libraryTitle, edition, onServer)
}

func ReconcileMediaItemEdition(ctx context.Context, tmdbID, libraryTitle, oldEdition string, updatedItem models.MediaItem) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.ReconcileMediaItemEdition(ctx, tmdbID, libraryTitle, oldEdition, updatedItem)
}

func EnqueueDownloadQueueJob(ctx context.Context, item models.DBSavedItem) (jobID int64, Err logging.LogErrorInfo) {
	if Client == nil {
		return 0, logging.Error_DBClientNotInitialized()
	}
	return Client.EnqueueDownloadQueueJob(ctx, item)
}

func ClaimNextDownloadQueueJob(ctx context.Context, workerID string) (job DownloadQueueJob, found bool, Err logging.LogErrorInfo) {
	if Client == nil {
		return DownloadQueueJob{}, false, logging.Error_DBClientNotInitialized()
	}
	return Client.ClaimNextDownloadQueueJob(ctx, workerID)
}

func FinishDownloadQueueJob(ctx context.Context, jobID int64, status string, message string, errs, warnings []string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.FinishDownloadQueueJob(ctx, jobID, status, message, errs, warnings)
}

func GetAllDownloadQueueJobs(ctx context.Context) (jobs []DownloadQueueJob, Err logging.LogErrorInfo) {
	if Client == nil {
		return []DownloadQueueJob{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllDownloadQueueJobs(ctx)
}

func DeleteDownloadQueueJob(ctx context.Context, jobID int64) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.DeleteDownloadQueueJob(ctx, jobID)
}

func DeleteAllDownloadQueueJobs(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.DeleteAllDownloadQueueJobs(ctx)
}

func InsertDownloadHistoryEntry(ctx context.Context, entry DownloadHistoryEntry) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.InsertDownloadHistoryEntry(ctx, entry)
}

func GetDownloadHistory(ctx context.Context, limit, offset int) (entries []DownloadHistoryEntry, total int, Err logging.LogErrorInfo) {
	if Client == nil {
		return []DownloadHistoryEntry{}, 0, logging.Error_DBClientNotInitialized()
	}
	return Client.GetDownloadHistory(ctx, limit, offset)
}

func GetDownloadHistoryEntry(ctx context.Context, id int64) (entry DownloadHistoryEntry, found bool, Err logging.LogErrorInfo) {
	if Client == nil {
		return DownloadHistoryEntry{}, false, logging.Error_DBClientNotInitialized()
	}
	return Client.GetDownloadHistoryEntry(ctx, id)
}

func GetDueDownloadRetries(ctx context.Context) (entries []DownloadHistoryEntry, Err logging.LogErrorInfo) {
	if Client == nil {
		return []DownloadHistoryEntry{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetDueDownloadRetries(ctx)
}

func MarkDownloadHistoryRetried(ctx context.Context, id int64) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.MarkDownloadHistoryRetried(ctx, id)
}

func StopDownloadRetries(ctx context.Context, id int64) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.StopDownloadRetries(ctx, id)
}

func DeleteDownloadHistoryEntry(ctx context.Context, id int64) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.DeleteDownloadHistoryEntry(ctx, id)
}

func DeleteDownloadHistoryForSet(ctx context.Context, tmdbID, libraryTitle, edition, setID string) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.DeleteDownloadHistoryForSet(ctx, tmdbID, libraryTitle, edition, setID)
}

func DeleteAllDownloadHistory(ctx context.Context) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.DeleteAllDownloadHistory(ctx)
}

func GetAllSiloItemIDs(ctx context.Context) (items []SiloItemIDs, Err logging.LogErrorInfo) {
	if Client == nil {
		return []SiloItemIDs{}, logging.Error_DBClientNotInitialized()
	}
	return Client.GetAllSiloItemIDs(ctx)
}

func UpsertSiloItemIDs(ctx context.Context, items []SiloItemIDs) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.UpsertSiloItemIDs(ctx, items)
}

// getSiloSharedLibraryTitles returns the other libraries that have the same item.
// Silo uses one item (rating key) for every library it is in, so changes in one library apply to all of them.
// Returns nothing for other media servers.
func getSiloSharedLibraryTitles(tmdbID, libraryTitle, edition string) []string {
	if config.Current.MediaServer.Type != "Silo" {
		return nil
	}
	item, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(libraryTitle, tmdbID, edition)
	if !found || item.RatingKey == "" {
		return nil
	}
	libraryTitles := []string{}
	for _, otherItem := range cache.LibraryStore.GetMediaItemsByRatingKey(item.RatingKey) {
		if otherItem.LibraryTitle != libraryTitle {
			libraryTitles = append(libraryTitles, otherItem.LibraryTitle)
		}
	}
	return libraryTitles
}

// applyToSharedLibraries makes a change to the same Silo item in the other libraries, then copies the item to aura's library cache
func applyToSharedLibraries(ctx context.Context, tmdbID, libraryTitle, edition string, change func(otherLibraryTitle string) logging.LogErrorInfo) {
	otherLibraryTitles := getSiloSharedLibraryTitles(tmdbID, libraryTitle, edition)
	for _, otherLibraryTitle := range otherLibraryTitles {
		logSharedLibraryError(change(otherLibraryTitle), tmdbID, otherLibraryTitle)
	}
	syncLibraryCache(ctx, tmdbID, edition, append(otherLibraryTitles, libraryTitle)...)
}

// syncLibraryCache copies an item's saved sets and ignored state from the database to aura's library cache
func syncLibraryCache(ctx context.Context, tmdbID, edition string, libraryTitles ...string) {
	for _, libraryTitle := range libraryTitles {
		ignored, ignoredMode, sets, Err := Client.CheckIfMediaItemExists(ctx, tmdbID, libraryTitle, edition)
		if Err.Message != "" {
			continue
		}
		if ignored {
			sets = []models.DBSavedSet{}
		} else {
			ignoredMode = ""
		}
		cache.LibraryStore.UpdateMediaItemDBState(libraryTitle, tmdbID, edition, sets, ignored, ignoredMode)
	}
}

// logSharedLibraryError logs a failed change in another library without failing the original change
func logSharedLibraryError(Err logging.LogErrorInfo, tmdbID, libraryTitle string) {
	if Err.Message == "" {
		return
	}
	logging.LOGGER.Warn().Timestamp().Str("tmdb_id", tmdbID).Str("library_title", libraryTitle).Str("error", Err.Message).
		Msg("Failed to apply the change to the same Silo item in another library")
}

func MoveMediaItems(ctx context.Context, moves []MediaItemMove) (Err logging.LogErrorInfo) {
	if Client == nil {
		return logging.Error_DBClientNotInitialized()
	}
	return Client.MoveMediaItems(ctx, moves)
}

func ResetInterruptedDownloadQueueJobs(ctx context.Context, workerID string) (count int64, Err logging.LogErrorInfo) {
	if Client == nil {
		return 0, logging.Error_DBClientNotInitialized()
	}
	return Client.ResetInterruptedDownloadQueueJobs(ctx, workerID)
}

func CountActiveDownloadQueueJobs(ctx context.Context) (count int, Err logging.LogErrorInfo) {
	if Client == nil {
		return 0, logging.Error_DBClientNotInitialized()
	}
	return Client.CountActiveDownloadQueueJobs(ctx)
}
