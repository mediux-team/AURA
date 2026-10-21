package ej

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/url"
	"path"
	"time"
)

func (ej *EJ) TestConnection(ctx context.Context, msConfig config.Config_MediaServer) (connectionOK bool, serverName string, serverVersion string, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("%s: Checking Media Server Connection", msConfig.Type), logging.LevelDebug)
	defer logAction.Complete()

	connectionOK = false
	serverName = ""
	serverVersion = ""
	Err = logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return connectionOK, serverName, serverVersion, *logAction.Error
	}
	u.Path = path.Join(u.Path, "System", "Info")
	URL := u.String()

	var lastErrorMsg string
	var lastErrorDetail map[string]any

	// Try up to 3 times with a 5 second delay between attempts
	for attempt := 1; attempt <= 3; attempt++ {
		attemptAction := logAction.AddSubAction(fmt.Sprintf("Attempt %d to connect to %s", attempt, msConfig.Type), logging.LevelTrace)
		resp, respBody, reqErr := makeRequest(ctx, msConfig, URL, "GET", nil)
		if reqErr.Message != "" {
			attemptAction.AppendWarning(fmt.Sprintf("attempt_%d", attempt), map[string]any{"error": reqErr.Message})
			lastErrorMsg = reqErr.Message
			lastErrorDetail = reqErr.Detail
			if attempt < 3 {
				time.Sleep(5 * time.Second)
			}
			continue
		}
		defer resp.Body.Close()

		var statusResp struct {
			Version    string `json:"Version"`
			ServerName string `json:"ServerName"`
		}

		// Decode the Response
		reqErr = httpx.DecodeResponseToJSON(ctx, respBody, &statusResp, "Jellyfin Connection Info")
		if reqErr.Message != "" {
			attemptAction.AppendWarning(fmt.Sprintf("attempt_%d", attempt), map[string]any{"decode_error": reqErr.Message})
			lastErrorMsg = reqErr.Message
			lastErrorDetail = reqErr.Detail
			if attempt < 3 {
				time.Sleep(5 * time.Second)
			}
			continue
		}

		// Get the server version
		serverVersion = statusResp.Version
		serverName = statusResp.ServerName

		// If the server version is empty, consider the connection failed
		if serverVersion == "" {
			attemptAction.AppendWarning(fmt.Sprintf("attempt_%d", attempt), map[string]any{"error": "empty server version"})
			lastErrorMsg = "Failed to retrieve Jellyfin server version"
			lastErrorDetail = map[string]any{"URL": URL}
			if attempt < 3 {
				time.Sleep(5 * time.Second)
			}
			continue
		}

		connectionOK = true
		return connectionOK, serverName, serverVersion, logging.LogErrorInfo{}
	}

	// All attempts failed
	finalError := logging.LogErrorInfo{Message: lastErrorMsg, Detail: lastErrorDetail}
	return connectionOK, serverName, serverVersion, finalError
}
