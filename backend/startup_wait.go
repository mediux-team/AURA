package main

import (
	"aura/config"
	"aura/logging"
	"net/http"
	"time"
)

// How long startup keeps trying the media server and MediUX, so a media server that starts with aura has time to start
var startupWaitLimit = 5 * time.Minute

// Sleeps between attempts, replaced in tests
var startupSleep = time.Sleep

// waitUntilReady runs check until it works, it's rejected, or startupWaitLimit has been spent waiting.
// A server that refuses the request (400, 401, 403 or 404) is rejected. Anything else, like no connection
// or a 503 from a media server that's still starting, is tried again with a longer wait each time.
func waitUntilReady(name string, waitingStep string, check func() logging.LogErrorInfo) logging.LogErrorInfo {
	wait := 5 * time.Second
	var waited time.Duration
	for attempt := 1; ; attempt++ {
		Err := check()
		if Err.Message == "" || isRejected(Err) || waited+wait > startupWaitLimit {
			return Err
		}
		logging.LOGGER.Warn().Timestamp().Int("attempt", attempt).Str("error", Err.Message).
			Msgf("%s isn't ready yet, trying again in %s", name, wait)
		config.SetAppLoadingStep(waitingStep)
		startupSleep(wait)
		waited += wait
		wait = min(wait*2, 30*time.Second)
	}
}

// isRejected reports whether the server was reached and refused the request, so trying again won't help
func isRejected(Err logging.LogErrorInfo) bool {
	statusCode, _ := Err.Detail["status_code"].(int)
	switch statusCode {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return true
	}
	return false
}
