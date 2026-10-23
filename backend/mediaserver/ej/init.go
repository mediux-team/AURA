package ej

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/url"
	"path"
)

type EJ struct {
	Config config.Config_MediaServer
}

func (e *EJ) GetAdminUser(ctx context.Context, msConfig config.Config_MediaServer) (userID string, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Getting Admin User for %s Media Server", msConfig.Type), logging.LevelDebug)
	defer logAction.Complete()

	userID = ""
	Err = logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return userID, *logAction.Error
	}
	u.Path = path.Join(u.Path, "Users")
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, msConfig, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return userID, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var ejResp EmbyJellyUserIDResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &ejResp, fmt.Sprintf("%s User ID Response", msConfig.Type))
	if Err.Message != "" {
		return userID, Err
	}

	// Find the first Admin user ID
	for _, user := range ejResp {
		if user.Policy.IsAdministrator {
			userID = user.ID
			logAction.AppendResult("admin_user_id", user.ID)
			return userID, Err
		}
	}

	logAction.SetError("No administrator user found on Jellyfin server", "Ensure that there is at least one admin user on the server", nil)
	return userID, *logAction.Error
}
