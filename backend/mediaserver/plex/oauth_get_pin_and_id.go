package plex

import (
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type PlexGetPinResponse struct {
	Pin  string `json:"code"`
	ID   int64  `json:"id"`
	Auth string `json:"authToken"`
}

func OAuth_GetPinAndID(ctx context.Context) (pin string, id int64, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Plex: Getting OAuth Pin and ID", logging.LevelDebug)
	defer logAction.Complete()

	pin = ""
	id = 0
	Err = logging.LogErrorInfo{}

	// Construct the URL for the Plex OAuth request
	u, err := url.Parse("https://plex.tv/api/v2/pins")
	if err != nil {
		logAction.SetError("Failed to parse Plex OAuth URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return pin, id, *logAction.Error
	}
	query := u.Query()
	query.Set("strong", "true")
	query.Set("X-Plex-Product", "AURA")
	query.Set("X-Plex-Client-Identifier", "aura")
	u.RawQuery = query.Encode()
	URL := u.String()

	headers := map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	}

	// Make the HTTP Request to Plex OAuth
	resp, respBody, Err := httpx.MakeHTTPRequest(ctx, URL, "POST", headers, 60, nil, "Plex OAuth")
	if Err.Message != "" {
		return pin, id, Err
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusCreated {
		logAction.SetError("Plex OAuth returned a non-success status code", fmt.Sprintf("Status Code: %d", resp.StatusCode), nil)
		return pin, id, *logAction.Error
	}

	// Decode the Response
	var plexResp PlexGetPinResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex OAuth Get Pin Response")
	if Err.Message != "" {
		return pin, id, Err
	}

	pin = plexResp.Pin
	id = plexResp.ID

	return pin, id, logging.LogErrorInfo{}
}
