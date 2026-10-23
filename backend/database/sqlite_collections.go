package database

import (
	"aura/logging"
	"aura/models"
	"context"
)

// SaveCollections replaces the main media server's collections and the movies in them
func (s *SQliteDB) SaveCollections(ctx context.Context, collections []models.CollectionItem) logging.LogErrorInfo {
	fail := func(message string, err error) logging.LogErrorInfo {
		return dbError(ctx, "Saving collections to the database", message, err)
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fail("Failed to start transaction", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM CollectionItems;`); err != nil {
		return fail("Failed to remove the collection movies", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Collections;`); err != nil {
		return fail("Failed to remove the collections", err)
	}

	collectionStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO Collections (rating_key, library_title, collection_index, tmdb_id, title, summary, child_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(rating_key) DO NOTHING;`)
	if err != nil {
		return fail("Failed to prepare the collection save", err)
	}
	defer collectionStmt.Close()
	itemStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO CollectionItems (collection_rating_key, rating_key, tmdb_id, library_title, title, year)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(collection_rating_key, rating_key) DO NOTHING;`)
	if err != nil {
		return fail("Failed to prepare the collection movie save", err)
	}
	defer itemStmt.Close()

	for _, collection := range collections {
		if collection.RatingKey == "" {
			continue
		}
		if _, err := collectionStmt.ExecContext(ctx, collection.RatingKey, collection.LibraryTitle, collection.Index,
			collection.TMDB_ID, collection.Title, collection.Summary, collection.ChildCount); err != nil {
			return fail("Failed to save a collection", err)
		}
		for _, item := range collection.MediaItems {
			if item.RatingKey == "" {
				continue
			}
			if _, err := itemStmt.ExecContext(ctx, collection.RatingKey, item.RatingKey, item.TMDB_ID,
				item.LibraryTitle, item.Title, item.Year); err != nil {
				return fail("Failed to save a collection movie", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fail("Failed to commit transaction", err)
	}
	return logging.LogErrorInfo{}
}

// GetCollections returns the main media server's collections and the movies in them.
// The movies only have their rating key, TMDB ID, library, title and year.
func (s *SQliteDB) GetCollections(ctx context.Context) ([]models.CollectionItem, logging.LogErrorInfo) {
	collections := []models.CollectionItem{}
	fail := func(message string, err error) ([]models.CollectionItem, logging.LogErrorInfo) {
		return []models.CollectionItem{}, dbError(ctx, "Getting the collections from the database", message, err)
	}

	rows, err := s.conn.QueryContext(ctx, `
		SELECT rating_key, library_title, collection_index, tmdb_id, title, summary, child_count
		FROM Collections
		ORDER BY library_title, title;`)
	if err != nil {
		return fail("Failed to get the collections", err)
	}
	byRatingKey := map[string]int{}
	for rows.Next() {
		var collection models.CollectionItem
		if err := rows.Scan(&collection.RatingKey, &collection.LibraryTitle, &collection.Index, &collection.TMDB_ID,
			&collection.Title, &collection.Summary, &collection.ChildCount); err != nil {
			rows.Close()
			return fail("Failed to read a collection", err)
		}
		byRatingKey[collection.RatingKey] = len(collections)
		collections = append(collections, collection)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fail("Failed to read the collections", err)
	}

	itemRows, err := s.conn.QueryContext(ctx, `
		SELECT collection_rating_key, rating_key, tmdb_id, library_title, title, year
		FROM CollectionItems
		ORDER BY collection_rating_key, year;`)
	if err != nil {
		return fail("Failed to get the collection movies", err)
	}
	defer itemRows.Close()
	for itemRows.Next() {
		var collectionRatingKey string
		item := models.MediaItem{Type: "movie"}
		if err := itemRows.Scan(&collectionRatingKey, &item.RatingKey, &item.TMDB_ID, &item.LibraryTitle, &item.Title, &item.Year); err != nil {
			return fail("Failed to read a collection movie", err)
		}
		if i, ok := byRatingKey[collectionRatingKey]; ok {
			collections[i].MediaItems = append(collections[i].MediaItems, item)
		}
	}
	if err := itemRows.Err(); err != nil {
		return fail("Failed to read the collection movies", err)
	}
	return collections, logging.LogErrorInfo{}
}
