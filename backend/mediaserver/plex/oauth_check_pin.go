package plex

import (
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func OAuth_CheckIDForAuth(ctx context.Context, plexID string) (isValid bool, authToken string, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Plex: Checking OAuth Pin", logging.LevelDebug)
	defer logAction.Complete()

	isValid = false
	authToken = ""
	Err = logging.LogErrorInfo{}

	// Construct the URL for the Plex OAuth Pin check request
	u, err := url.Parse(fmt.Sprintf("https://plex.tv/api/v2/pins/%s", plexID))
	if err != nil {
		logAction.SetError("Failed to parse Plex OAuth Pin URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return isValid, authToken, *logAction.Error
	}
	query := u.Query()
	query.Set("X-Plex-Product", "AURA")
	query.Set("X-Plex-Client-Identifier", "aura")
	u.RawQuery = query.Encode()
	URL := u.String()

	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}

	// Make the HTTP Request to Plex OAuth to check the pin
	resp, respBody, Err := httpx.MakeHTTPRequest(ctx, URL, "GET", headers, 60, nil, "Plex OAuth Pin Check")
	if Err.Message != "" {
		return isValid, authToken, Err
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		logAction.SetError("Plex OAuth Pin check returned a non-success status code", fmt.Sprintf("Status Code: %d", resp.StatusCode), nil)
		return isValid, authToken, *logAction.Error
	}

	// Decode the Response
	var plexResp PlexGetPinResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex OAuth Pin Check Response")
	if Err.Message != "" {
		return isValid, authToken, Err
	}

	// Determine if the pin is valid based on the presence of an Auth token
	if plexResp.Auth != "" {
		isValid = true
		authToken = plexResp.Auth
	}

	return isValid, authToken, Err
}
