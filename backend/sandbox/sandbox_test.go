package sandbox

import (
	"aura/models"
	"context"
	"testing"
)

func TestBeforeApplyImage(t *testing.T) {
	t.Cleanup(func() { ClearFailures(); SetDelay(0) })
	season := func(n int) *int { return &n }
	item := models.MediaItem{TMDB_ID: "1", LibraryTitle: "Series", Title: "Show"}
	poster := models.ImageFile{ID: "p", Type: "poster"}
	titlecard := models.ImageFile{ID: "t", Type: "titlecard", SeasonNumber: season(1), EpisodeNumber: season(1)}
	special := models.ImageFile{ID: "s", Type: "season_poster", SeasonNumber: season(0)}

	if Err := BeforeApplyImage(context.Background(), item, poster); Err.Message != "" {
		t.Fatalf("without a rule the image downloads, got %q", Err.Message)
	}

	// The next title card fails once, other types download
	AddFailure(FailureRule{TMDB_ID: "1", LibraryTitle: "Series", Types: []string{"titlecard"}, Remaining: 1})
	if Err := BeforeApplyImage(context.Background(), item, poster); Err.Message != "" {
		t.Error("a poster shouldn't fail for a title card rule")
	}
	if Err := BeforeApplyImage(context.Background(), item, titlecard); Err.Message == "" {
		t.Error("the title card should fail")
	}
	if Err := BeforeApplyImage(context.Background(), item, titlecard); Err.Message != "" {
		t.Error("the rule should be used up after one failure")
	}
	if len(GetState().Failures) != 0 {
		t.Errorf("failures = %+v, want the used up rule removed", GetState().Failures)
	}

	// The special season poster is its own type, and a rule without a count keeps failing
	AddFailure(FailureRule{TMDB_ID: "1", LibraryTitle: "Series", Types: []string{"special_season_poster"}, Reason: "made up"})
	for range 2 {
		if Err := BeforeApplyImage(context.Background(), item, special); Err.Message != "made up" {
			t.Errorf("the special season poster should fail with the reason, got %q", Err.Message)
		}
	}

	// Another item, or another library, isn't affected
	if Err := BeforeApplyImage(context.Background(), models.MediaItem{TMDB_ID: "1", LibraryTitle: "4K Series"}, special); Err.Message != "" {
		t.Error("another library shouldn't fail")
	}
}
