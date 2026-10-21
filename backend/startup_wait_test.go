package main

import (
	"aura/config"
	"aura/logging"
	"slices"
	"testing"
	"time"
)

// stubStartupSleep records the waits instead of sleeping
func stubStartupSleep(t *testing.T) *[]time.Duration {
	t.Helper()
	slept := []time.Duration{}
	previousSleep, previousLimit := startupSleep, startupWaitLimit
	t.Cleanup(func() { startupSleep, startupWaitLimit = previousSleep, previousLimit })
	startupSleep = func(d time.Duration) { slept = append(slept, d) }
	startupWaitLimit = 5 * time.Minute
	return &slept
}

func statusError(statusCode int) logging.LogErrorInfo {
	return logging.LogErrorInfo{Message: "Jellyfin Server returned a non-success status code", Detail: map[string]any{"status_code": statusCode}}
}

func TestWaitUntilReady(t *testing.T) {
	t.Run("a media server that is still starting is tried again", func(t *testing.T) {
		slept := stubStartupSleep(t)
		calls := 0
		Err := waitUntilReady("Jellyfin", "Waiting for Media Server to Start", func() logging.LogErrorInfo {
			calls++
			if calls < 3 {
				return statusError(503)
			}
			return logging.LogErrorInfo{}
		})
		if Err.Message != "" || calls != 3 || !slices.Equal(*slept, []time.Duration{5 * time.Second, 10 * time.Second}) {
			t.Errorf("error %q, %d calls, slept %v", Err.Message, calls, *slept)
		}
		if config.AppLoadingStep != "Waiting for Media Server to Start" {
			t.Errorf("loading step = %q", config.AppLoadingStep)
		}
	})

	t.Run("a rejected request isn't tried again", func(t *testing.T) {
		for _, statusCode := range []int{400, 401, 403, 404} {
			slept := stubStartupSleep(t)
			calls := 0
			Err := waitUntilReady("MediUX", "Waiting for MediUX", func() logging.LogErrorInfo {
				calls++
				return statusError(statusCode)
			})
			if Err.Message == "" || calls != 1 || len(*slept) != 0 {
				t.Errorf("status %d: error %q, %d calls, slept %v", statusCode, Err.Message, calls, *slept)
			}
		}
	})

	t.Run("a server that can't be reached is tried until the limit", func(t *testing.T) {
		slept := stubStartupSleep(t)
		calls := 0
		Err := waitUntilReady("Jellyfin", "Waiting for Media Server to Start", func() logging.LogErrorInfo {
			calls++
			return logging.LogErrorInfo{Message: "Failed to send GET request to Jellyfin"}
		})
		var total time.Duration
		for _, d := range *slept {
			total += d
			if d > 30*time.Second {
				t.Errorf("waited %s, want at most 30s between attempts", d)
			}
		}
		if Err.Message == "" || calls != len(*slept)+1 || total > startupWaitLimit || total < startupWaitLimit-30*time.Second {
			t.Errorf("error %q, %d calls, waited %s in total", Err.Message, calls, total)
		}
	})
}
