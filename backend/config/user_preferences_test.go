package config

import (
	"aura/logging"
	"context"
	"slices"
	"testing"
)

func TestValidateUserPreferencesFavoriteCreators(t *testing.T) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))

	prefs := Config_UserPreferences{FavoriteCreators: []string{" willtong93 ", "", "benno", "WillTong93", "Pejamas"}}
	if !ValidateUserPreferences(ctx, &prefs) {
		t.Fatal("the preferences should be valid")
	}
	// Blank and repeated names go, each keeps its best rank
	if want := []string{"willtong93", "benno", "Pejamas"}; !slices.Equal(prefs.FavoriteCreators, want) {
		t.Errorf("favorite creators = %q, want %q", prefs.FavoriteCreators, want)
	}
}

func TestValidateUserPreferencesHomeDefaults(t *testing.T) {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "test")
	ctx = logging.WithCurrentAction(ctx, ld.AddAction("test", logging.LevelInfo))

	prefs := Config_UserPreferences{
		DefaultSort:                   "",
		DefaultSortOrder:              "invalid",
		DefaultItemsPerPage:           -5,
		DefaultFilterInDB:             "bad_filter",
		DefaultFilterIgnored:          "bad_ignored",
		DefaultHasSetsAvailableFilter: "bad_sets",
	}

	if !ValidateUserPreferences(ctx, &prefs) {
		t.Fatal("the preferences should be valid")
	}

	if prefs.DefaultSort != "dateAdded" {
		t.Errorf("DefaultSort = %q, want dateAdded", prefs.DefaultSort)
	}
	if prefs.DefaultSortOrder != "desc" {
		t.Errorf("DefaultSortOrder = %q, want desc", prefs.DefaultSortOrder)
	}
	if prefs.DefaultItemsPerPage != 20 {
		t.Errorf("DefaultItemsPerPage = %d, want 20", prefs.DefaultItemsPerPage)
	}
	if prefs.DefaultFilterInDB != "" {
		t.Errorf("DefaultFilterInDB = %q, want empty", prefs.DefaultFilterInDB)
	}
	if prefs.DefaultFilterIgnored != "" {
		t.Errorf("DefaultFilterIgnored = %q, want empty", prefs.DefaultFilterIgnored)
	}
	if prefs.DefaultHasSetsAvailableFilter != "" {
		t.Errorf("DefaultHasSetsAvailableFilter = %q, want empty", prefs.DefaultHasSetsAvailableFilter)
	}
}
