package mediaserver

import (
	"aura/config"
	"aura/models"
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

func setupSyncTest(t *testing.T, savedItems func(ctx context.Context) ([]models.DBSavedItem, error)) *[]string {
	t.Helper()
	calls, _ := setupOtherMediaServersTest(t)
	previousLoad, previousSaved := syncLoadMediaServer, syncGetSavedItems
	t.Cleanup(func() {
		syncLoadMediaServer, syncGetSavedItems = previousLoad, previousSaved
		syncMu.Lock()
		syncState, syncCancel = nil, nil
		syncMu.Unlock()
	})
	// The libraries are already in the test caches
	syncLoadMediaServer = func(ctx context.Context, server *config.Config_MediaServer) error { return nil }
	syncGetSavedItems = savedItems
	return calls
}

func waitForSync(t *testing.T) MediaServerSync {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if state, ok := GetMediaServerSync(); ok && !state.Running {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the sync didn't finish")
	return MediaServerSync{}
}

func savedMovie(tmdbID, library string, selected models.SelectedTypes) models.DBSavedItem {
	return models.DBSavedItem{
		MediaItem: models.MediaItem{TMDB_ID: tmdbID, Type: "movie", LibraryTitle: library, Title: "Movie " + tmdbID},
		PosterSets: []models.DBPosterSetDetail{{
			PosterSet:     models.PosterSet{Images: []models.ImageFile{{ID: "p" + tmdbID, Type: "poster"}, {ID: "b" + tmdbID, Type: "backdrop"}}},
			SelectedTypes: selected,
		}},
	}
}

func TestSyncMediaServerAppliesTheSavedSets(t *testing.T) {
	calls := setupSyncTest(t, func(ctx context.Context) ([]models.DBSavedItem, error) {
		return []models.DBSavedItem{
			savedMovie("1", "Movies", models.SelectedTypes{Poster: true}),
			savedMovie("2", "4K Movies", models.SelectedTypes{Poster: true, Backdrop: true}),
			savedMovie("9", "Movies", models.SelectedTypes{Poster: true}), // Not on Silo
			savedMovie("3", "Kids", models.SelectedTypes{Poster: true}),   // Silo's Kids library doesn't match any main library
		}, nil
	})

	if _, err := StartMediaServerSync("silo"); err != nil {
		t.Fatalf("StartMediaServerSync: %v", err)
	}
	state := waitForSync(t)

	if state.Total != 4 || state.Done != 4 || state.Applied != 3 || state.NotFound != 2 || state.Failed != 0 || state.Error != "" {
		t.Errorf("got %+v, want 4 checked, 3 images applied and 2 not found", state)
	}
	// Only the selected images, and only on Silo
	want := []string{"silo: image movie-tmdb-1", "silo: image movie-tmdb-2", "silo: image movie-tmdb-2"}
	if !slices.Equal(*calls, want) {
		t.Errorf("calls = %v, want %v", *calls, want)
	}
}

func TestSyncMediaServerOnlyOtherEnabledServers(t *testing.T) {
	setupSyncTest(t, func(ctx context.Context) ([]models.DBSavedItem, error) { return nil, nil })

	for _, id := range []string{"main", "paused", "missing"} {
		if _, err := StartMediaServerSync(id); err == nil {
			t.Errorf("StartMediaServerSync(%q) should fail", id)
		}
	}
}

func TestSyncMediaServerStopsAndRunsOneAtATime(t *testing.T) {
	release := make(chan struct{})
	setupSyncTest(t, func(ctx context.Context) ([]models.DBSavedItem, error) {
		<-release
		return []models.DBSavedItem{savedMovie("1", "Movies", models.SelectedTypes{Poster: true})}, nil
	})

	if _, err := StartMediaServerSync("silo"); err != nil {
		t.Fatalf("StartMediaServerSync: %v", err)
	}
	if _, err := StartMediaServerSync("plex2"); err == nil {
		t.Error("a second sync should not start while one is running")
	}
	if name, running := MediaServerSyncRunning(); !running || name != "Silo" {
		t.Errorf("MediaServerSyncRunning = %q, %v, want Silo, true", name, running)
	}

	if !StopMediaServerSync() {
		t.Fatal("StopMediaServerSync should stop the running sync")
	}
	close(release)
	state := waitForSync(t)
	if !state.Stopped || state.Applied != 0 || state.Error != "" {
		t.Errorf("got %+v, want stopped before anything was applied", state)
	}
}

func TestSyncMediaServerPanicFailsTheSync(t *testing.T) {
	setupSyncTest(t, func(ctx context.Context) ([]models.DBSavedItem, error) {
		var item *models.DBSavedItem
		return []models.DBSavedItem{*item}, nil
	})

	if _, err := StartMediaServerSync("silo"); err != nil {
		t.Fatalf("StartMediaServerSync: %v", err)
	}
	// The sync fails instead of stopping aura
	if state := waitForSync(t); !strings.HasPrefix(state.Error, "unexpected error:") {
		t.Errorf("Error = %q, want the panic", state.Error)
	}
}
