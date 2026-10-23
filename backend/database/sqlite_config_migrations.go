package database

import (
	"aura/config"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Config migrations update stored values when a config field is renamed, moved or changes type.
// Adding or removing a field doesn't need a migration.
// TestConfigSchemaSnapshot fails when a field changes without one.
//
// To add a migration, append it to configMigrations:
//
//	{
//		Description: "Rename MediaServer.Token to MediaServer.ApiToken",
//		Migrate:     renameConfigKey("MediaServer.Token", "MediaServer.ApiToken"),
//	},
type configMigration struct {
	Description string
	Migrate     func(ctx context.Context, tx *sql.Tx) error
}

// configMigrations[0] migrates from version 1 to version 2, and so on
var configMigrations = []configMigration{
	{
		Description: "Move MediaServer to MediaServers, as the main media server",
		Migrate:     moveMediaServerToMediaServers,
	},
	{
		Description: "Replace Images.SaveImagesLocally.EpisodeNamingConvention with the title card file name",
		Migrate:     moveEpisodeNamingConventionToTitlecardName,
	},
	{
		Description: "Add the collection poster and backdrop to UserPreferences.DownloadDefaults",
		Migrate:     addCollectionImagesToDownloadDefaults,
	},
	{
		Description: "Move Auth.OIDC.RedirectURL to Auth.OIDC.RedirectURLs",
		Migrate:     moveRedirectURLToRedirectURLs,
	},
}

// Version 1 is the first config schema, stored without any migrations
var LATEST_CONFIG_SCHEMA_VERSION = len(configMigrations) + 1

// renameConfigKey moves a stored value to a new key. The value is kept as is.
func renameConfigKey(oldKey, newKey string) func(ctx context.Context, tx *sql.Tx) error {
	return func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
UPDATE OR REPLACE Config SET key = ?, updated_at = CURRENT_TIMESTAMP WHERE key = ?;`, newKey, oldKey)
		if err != nil {
			return fmt.Errorf("failed to rename config key '%s' to '%s': %w", oldKey, newKey, err)
		}
		return nil
	}
}

// moveMediaServerToMediaServers moves the MediaServer values into the MediaServers list, as the main media server
func moveMediaServerToMediaServers(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT key, value FROM Config WHERE key LIKE 'MediaServer.%';`)
	if err != nil {
		return fmt.Errorf("failed to get MediaServer values: %w", err)
	}
	server := map[string]json.RawMessage{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			rows.Close()
			return fmt.Errorf("failed to read MediaServer value: %w", err)
		}
		server[strings.TrimPrefix(key, "MediaServer.")] = json.RawMessage(value)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("failed to read MediaServer values: %w", err)
	}
	rows.Close()

	// Only add the media server if one was set up
	mediaServers, mainMediaServer := "null", `""`
	var serverType string
	_ = json.Unmarshal(server["Type"], &serverType)
	if serverType != "" {
		id, _ := json.Marshal(config.NewMediaServerID())
		server["ID"] = id
		server["Name"] = server["Type"]
		server["Enabled"] = json.RawMessage("true")
		list, err := json.Marshal([]map[string]json.RawMessage{server})
		if err != nil {
			return fmt.Errorf("failed to build MediaServers: %w", err)
		}
		mediaServers, mainMediaServer = string(list), string(id)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT OR REPLACE INTO Config (key, value, updated_at) VALUES
    ('MediaServers', ?, CURRENT_TIMESTAMP),
    ('MainMediaServer', ?, CURRENT_TIMESTAMP);`, mediaServers, mainMediaServer); err != nil {
		return fmt.Errorf("failed to save MediaServers: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Config WHERE key LIKE 'MediaServer.%';`); err != nil {
		return fmt.Errorf("failed to remove MediaServer values: %w", err)
	}
	return nil
}

// moveEpisodeNamingConventionToTitlecardName keeps the static episode naming as the title card file name, match is the standard name
func moveEpisodeNamingConventionToTitlecardName(ctx context.Context, tx *sql.Tx) error {
	var value string
	err := tx.QueryRowContext(ctx, `SELECT value FROM Config WHERE key = 'Images.SaveImagesLocally.EpisodeNamingConvention';`).Scan(&value)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get EpisodeNamingConvention: %w", err)
	}
	var convention string
	_ = json.Unmarshal([]byte(value), &convention)
	if strings.EqualFold(convention, "static") {
		name, _ := json.Marshal("S{{SeasonNumber}}E{{EpisodeNumber}}")
		if _, err := tx.ExecContext(ctx, `
INSERT OR REPLACE INTO Config (key, value, updated_at) VALUES ('Images.SaveImagesLocally.NameTemplates.Titlecard', ?, CURRENT_TIMESTAMP);`, string(name)); err != nil {
			return fmt.Errorf("failed to set the title card file name: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Config WHERE key = 'Images.SaveImagesLocally.EpisodeNamingConvention';`); err != nil {
		return fmt.Errorf("failed to remove EpisodeNamingConvention: %w", err)
	}
	return nil
}

// addCollectionImagesToDownloadDefaults adds the new collection image types to the saved download defaults, like new installs have
func addCollectionImagesToDownloadDefaults(ctx context.Context, tx *sql.Tx) error {
	var value string
	err := tx.QueryRowContext(ctx, `SELECT value FROM Config WHERE key = 'UserPreferences.DownloadDefaults';`).Scan(&value)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get DownloadDefaults: %w", err)
	}
	var defaults []string
	if err := json.Unmarshal([]byte(value), &defaults); err != nil {
		return nil
	}
	for _, imageType := range []string{"collection_poster", "collection_backdrop"} {
		if !slices.Contains(defaults, imageType) {
			defaults = append(defaults, imageType)
		}
	}
	updated, err := json.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("failed to build DownloadDefaults: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE Config SET value = ?, updated_at = CURRENT_TIMESTAMP WHERE key = 'UserPreferences.DownloadDefaults';`, string(updated)); err != nil {
		return fmt.Errorf("failed to save DownloadDefaults: %w", err)
	}
	return nil
}

// moveRedirectURLToRedirectURLs keeps the OIDC Redirect URL as the first of the Redirect URLs
func moveRedirectURLToRedirectURLs(ctx context.Context, tx *sql.Tx) error {
	var value string
	err := tx.QueryRowContext(ctx, `SELECT value FROM Config WHERE key = 'Auth.OIDC.RedirectURL';`).Scan(&value)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get Auth.OIDC.RedirectURL: %w", err)
	}
	var redirectURL string
	_ = json.Unmarshal([]byte(value), &redirectURL)
	if redirectURL = strings.TrimSpace(redirectURL); redirectURL != "" {
		urls, _ := json.Marshal([]string{redirectURL})
		if _, err := tx.ExecContext(ctx, `
INSERT OR REPLACE INTO Config (key, value, updated_at) VALUES ('Auth.OIDC.RedirectURLs', ?, CURRENT_TIMESTAMP);`, string(urls)); err != nil {
			return fmt.Errorf("failed to save Auth.OIDC.RedirectURLs: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Config WHERE key = 'Auth.OIDC.RedirectURL';`); err != nil {
		return fmt.Errorf("failed to remove Auth.OIDC.RedirectURL: %w", err)
	}
	return nil
}
