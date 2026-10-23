package database

import (
	"aura/logging"
	"context"
	"database/sql"
)

func (s *SQliteDB) CreateTables(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating Database Tables", logging.LevelInfo)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	steps := []func(context.Context, *sql.DB) logging.LogErrorInfo{
		v2_CreateMediaItemsTable,
		v2_CreateMoviesTable,
		v2_CreateSeriesTable,
		v2_CreateSeasonsTable,
		v2_CreateEpisodesTable,
		v2_CreatePosterSetsTables,
		v2_CreateImageFilesTable,
		v2_CreateSavedItemsTable,
		v2_CreateIgnoredItemsTable,
		v2_AddIndexesToNewTables,
		v7_CreateDownloadQueueJobsTable,
		v7_CreateDownloadHistoryTable,
		v8_CreateSiloItemIDsTable,
		v14_CreateCollectionsTables,
	}

	for _, step := range steps {
		errInfo := step(ctx, s.conn)
		if errInfo.Message != "" {
			return *logAction.Error
		}
	}

	return Err
}

func v2_CreateMediaItemsTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating MediaItems Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE MediaItems (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	tmdb_id TEXT NOT NULL,
	library_title TEXT NOT NULL,
	edition TEXT NOT NULL DEFAULT '',
	rating_key TEXT NOT NULL,
	type TEXT NOT NULL CHECK (type IN ('movie','show')),
	title TEXT NOT NULL,
	year INTEGER NOT NULL,
	on_server INTEGER NOT NULL DEFAULT 0 CHECK (on_server IN (0,1)),
	added_at INTEGER NOT NULL DEFAULT 0,
	updated_at INTEGER NOT NULL DEFAULT 0,
	released_at INTEGER NOT NULL DEFAULT 0,
	latest_episode_added_at INTEGER NOT NULL DEFAULT 0,
	content_rating TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	guids TEXT NOT NULL DEFAULT '[]',
	first_seen_at INTEGER NOT NULL DEFAULT 0,
	last_seen_at INTEGER NOT NULL DEFAULT 0,
	UNIQUE (tmdb_id, library_title, edition)
);
	`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create MediaItems table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v2_CreateMoviesTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating Movies Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE Movies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL UNIQUE,
    path TEXT NOT NULL,
    size INTEGER NOT NULL,
    duration INTEGER NOT NULL,
    FOREIGN KEY (media_item_id) REFERENCES MediaItems(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE
);
	`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create Movies table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v2_CreateSeriesTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating Series Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE Series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_item_id INTEGER NOT NULL UNIQUE,
    season_count INTEGER,
    episode_count INTEGER,
    location TEXT,
    FOREIGN KEY (media_item_id) REFERENCES MediaItems(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE
);
	`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create Series table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v2_CreateSeasonsTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating Seasons Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE Seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL,
	rating_key TEXT NOT NULL,
    season_number INTEGER NOT NULL,
    episode_count INTEGER,
    FOREIGN KEY (series_id) REFERENCES Series(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    UNIQUE (series_id, season_number)
);
	`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create Seasons table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v2_CreateEpisodesTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating Episodes Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE Episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    season_id INTEGER NOT NULL,
	rating_key TEXT NOT NULL,
    episode_number INTEGER NOT NULL,
    title TEXT,
    path TEXT NOT NULL,
    size INTEGER NOT NULL,
    duration INTEGER NOT NULL,
    FOREIGN KEY (season_id) REFERENCES Seasons(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE,
    UNIQUE (season_id, episode_number)
);
	`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create Episodes table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v2_CreatePosterSetsTables(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating PosterSets Tables", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE PosterSets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id TEXT NOT NULL UNIQUE,
	type TEXT NOT NULL CHECK (type IN ('show','movie','collection')),
    title TEXT NOT NULL,
    user TEXT NOT NULL,
    date_created DATETIME,
    date_updated DATETIME
);
`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create PosterSets table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v2_CreateImageFilesTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating ImageFiles Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE ImageFiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    poster_set_id INTEGER NOT NULL,

    -- Which item inside the set this image belongs to (required for collection sets)
    item_tmdb_id TEXT NOT NULL,

    image_id TEXT NOT NULL,
    image_type TEXT NOT NULL CHECK (image_type IN ('poster','backdrop','season_poster','titlecard')),
    image_last_updated DATETIME NOT NULL,
    image_season_number INTEGER,
    image_episode_number INTEGER,

    FOREIGN KEY (poster_set_id) REFERENCES PosterSets(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE,

    -- Same image_id can exist multiple times in a set if it belongs to different items (e.g., collections)
    UNIQUE (poster_set_id, image_id, item_tmdb_id)
);
`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create ImageFiles table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v2_CreateSavedItemsTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating SavedItems Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE SavedItems (
    tmdb_id TEXT NOT NULL,
    library_title TEXT NOT NULL,
    edition TEXT NOT NULL DEFAULT '',
    poster_set_id INTEGER NOT NULL,

    -- Per MediaItem/Set toggles
    poster_selected INTEGER NOT NULL DEFAULT 0 CHECK (poster_selected IN (0,1)),
    backdrop_selected INTEGER NOT NULL DEFAULT 0 CHECK (backdrop_selected IN (0,1)),
    season_poster_selected INTEGER NOT NULL DEFAULT 0 CHECK (season_poster_selected IN (0,1)),
    special_season_poster_selected INTEGER NOT NULL DEFAULT 0 CHECK (special_season_poster_selected IN (0,1)),
    titlecard_selected INTEGER NOT NULL DEFAULT 0 CHECK (titlecard_selected IN (0,1)),

    autodownload INTEGER NOT NULL DEFAULT 0 CHECK (autodownload IN (0,1)),
	auto_add_new_collection_items INTEGER NOT NULL DEFAULT 0 CHECK (auto_add_new_collection_items IN (0,1)),
    no_auto_upgrade INTEGER NOT NULL DEFAULT 0 CHECK (no_auto_upgrade IN (0,1)),
    last_downloaded DATETIME NOT NULL,

    PRIMARY KEY (tmdb_id, library_title, edition, poster_set_id),

    FOREIGN KEY (poster_set_id) REFERENCES PosterSets(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE,

    FOREIGN KEY (tmdb_id, library_title, edition) REFERENCES MediaItems(tmdb_id, library_title, edition)
        ON DELETE CASCADE
        ON UPDATE CASCADE
) WITHOUT ROWID;
`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create SavedItems table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v2_CreateIgnoredItemsTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating IgnoredItems Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE IgnoredItems (
    tmdb_id TEXT NOT NULL,
    library_title TEXT NOT NULL,
    edition TEXT NOT NULL DEFAULT '',

    -- 'always' = persist until user un-ignores
    -- 'until-set-available'   = cleared by cron job when a set becomes available
	-- 'until-new-set-available' = never cleared by cron job, but user notified when a new set becomes available and given the option to clear the ignore manually
    mode TEXT NOT NULL CHECK (mode IN ('always','until-set-available','until-new-set-available')),

	-- Sets that currently available for this item (array stored as JSON string)
	current_sets TEXT NOT NULL DEFAULT '[]',

    PRIMARY KEY (tmdb_id, library_title, edition)
) WITHOUT ROWID;
`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create IgnoredItems table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v7_CreateDownloadQueueJobsTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating DownloadQueueJobs Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE DownloadQueueJobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tmdb_id TEXT NOT NULL,
    library_title TEXT NOT NULL,
    edition TEXT NOT NULL DEFAULT '',
    media_item_title TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','processing','success','warning','error')),
    result_message TEXT NOT NULL DEFAULT '',
    result_errors TEXT NOT NULL DEFAULT '[]',
    result_warnings TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    finished_at DATETIME,
    worker_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_downloadqueuejobs_status ON DownloadQueueJobs(status, created_at);
CREATE INDEX idx_downloadqueuejobs_item ON DownloadQueueJobs(tmdb_id, library_title, edition);
	`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create DownloadQueueJobs table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v7_CreateDownloadHistoryTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating DownloadHistory Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE DownloadHistory (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tmdb_id TEXT NOT NULL,
    library_title TEXT NOT NULL,
    edition TEXT NOT NULL DEFAULT '',
    media_item_title TEXT NOT NULL,
    media_item_year INTEGER NOT NULL,
    set_id TEXT NOT NULL,
    set_title TEXT NOT NULL,
    set_creator TEXT NOT NULL DEFAULT '',
    selected_types TEXT NOT NULL DEFAULT '{}',
    warning_count INTEGER NOT NULL DEFAULT 0,
    images_succeeded INTEGER NOT NULL DEFAULT 0,
    images_failed INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK (status IN ('success','warning','error')),
    failed_images TEXT NOT NULL DEFAULT '[]',
    retry_count INTEGER NOT NULL DEFAULT 0,
    next_retry_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_downloadhistory_status_created ON DownloadHistory(status, created_at);
	`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create DownloadHistory table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v8_CreateSiloItemIDsTable(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating SiloItemIDs Table", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE SiloItemIDs (
    content_id TEXT PRIMARY KEY,
    tmdb_id TEXT NOT NULL DEFAULT '',
    imdb_id TEXT NOT NULL DEFAULT '',
    tvdb_id TEXT NOT NULL DEFAULT '',
    first_air_date TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
	`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create SiloItemIDs table", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v14_CreateCollectionsTables(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating Collections Tables", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE TABLE Collections (
    rating_key TEXT PRIMARY KEY,
    library_title TEXT NOT NULL,
    collection_index TEXT NOT NULL DEFAULT '',
    tmdb_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    child_count INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE CollectionItems (
    collection_rating_key TEXT NOT NULL,
    rating_key TEXT NOT NULL,
    tmdb_id TEXT NOT NULL DEFAULT '',
    library_title TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    year INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (collection_rating_key, rating_key)
);
CREATE INDEX idx_collectionitems_tmdb_id ON CollectionItems(tmdb_id);
	`
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create Collections tables", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}

func v2_AddIndexesToNewTables(ctx context.Context, conn *sql.DB) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Adding Indexes to New Tables", logging.LevelTrace)
	defer logAction.Complete()
	Err = logging.LogErrorInfo{}

	query := `
CREATE INDEX idx_seasons_series_id ON Seasons(series_id);
CREATE INDEX idx_episodes_season_id ON Episodes(season_id);

CREATE INDEX idx_imagefiles_poster_set_id ON ImageFiles(poster_set_id);
CREATE INDEX idx_imagefiles_set_type ON ImageFiles(poster_set_id, image_type);
CREATE INDEX idx_imagefiles_item_tmdb_id ON ImageFiles(item_tmdb_id);
CREATE INDEX idx_imagefiles_item_tmdb_type ON ImageFiles(item_tmdb_id, image_type);

CREATE INDEX idx_saveditems_poster_set_id ON SavedItems(poster_set_id);
CREATE INDEX idx_saveditems_item ON SavedItems(tmdb_id, library_title);

CREATE INDEX idx_ignoreditems_mode ON IgnoredItems(mode);
    `
	_, err := conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to add indexes to new tables", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	return Err
}
