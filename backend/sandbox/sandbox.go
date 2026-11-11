// Package sandbox is a test sandbox for dev builds. It changes an item's saved data as if something changed
// since the last download, makes image downloads fail, and slows them down.
package sandbox

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"slices"
	"sync"
	"time"
)

// Enabled reports whether the sandbox is on, which is only in dev builds
func Enabled() bool {
	return config.AppBuildNumber == "dev"
}

// FailureRule makes an item's image downloads fail
type FailureRule struct {
	TMDB_ID      string   `json:"tmdb_id"`
	LibraryTitle string   `json:"library_title"`
	Edition      string   `json:"edition"`
	Title        string   `json:"title"`
	Types        []string `json:"types"`     // The image types that fail, every type when empty
	Remaining    int      `json:"remaining"` // How many more downloads fail, until it's cleared when 0
	Reason       string   `json:"reason"`
}

// State is how the sandbox is set up
type State struct {
	Enabled      bool          `json:"enabled"`
	DelaySeconds float64       `json:"delay_seconds"` // The wait before each image downloads
	Failures     []FailureRule `json:"failures"`
}

var (
	mu    sync.Mutex
	state = State{Failures: []FailureRule{}}
)

// GetState is how the sandbox is set up now
func GetState() State {
	mu.Lock()
	defer mu.Unlock()
	current := state
	current.Enabled = Enabled()
	current.Failures = slices.Clone(state.Failures)
	return current
}

// SetDelay sets the wait before each image downloads, 0 for none
func SetDelay(seconds float64) {
	mu.Lock()
	defer mu.Unlock()
	state.DelaySeconds = max(0, seconds)
}

// AddFailure makes an item's image downloads fail
func AddFailure(rule FailureRule) {
	mu.Lock()
	defer mu.Unlock()
	if rule.Reason == "" {
		rule.Reason = "Sandbox: this download was made to fail"
	}
	state.Failures = append(state.Failures, rule)
}

// ClearFailures stops making downloads fail
func ClearFailures() {
	mu.Lock()
	defer mu.Unlock()
	state.Failures = []FailureRule{}
}

// BeforeApplyImage waits and fails an image download as the sandbox is set up. It's set as mediaserver.BeforeApplyImage in dev builds.
func BeforeApplyImage(ctx context.Context, item models.MediaItem, image models.ImageFile) logging.LogErrorInfo {
	mu.Lock()
	delay := time.Duration(state.DelaySeconds * float64(time.Second))
	mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for i, rule := range state.Failures {
		if rule.TMDB_ID != item.TMDB_ID || rule.LibraryTitle != item.LibraryTitle || rule.Edition != item.Edition {
			continue
		}
		if len(rule.Types) > 0 && !slices.Contains(rule.Types, imageTypeName(image)) {
			continue
		}
		if rule.Remaining > 0 {
			state.Failures[i].Remaining--
			if state.Failures[i].Remaining == 0 {
				state.Failures = slices.Delete(state.Failures, i, i+1)
			}
		}
		return logging.LogErrorInfo{Message: rule.Reason, Help: "This failure is from the test sandbox", Detail: map[string]any{"image_id": image.ID, "image_type": image.Type}}
	}
	return logging.LogErrorInfo{}
}

// imageTypeName is the image's type, with the special season poster on its own like the selected types
func imageTypeName(image models.ImageFile) string {
	if image.Type == "season_poster" && image.SeasonNumber != nil && *image.SeasonNumber == 0 {
		return "special_season_poster"
	}
	return image.Type
}
