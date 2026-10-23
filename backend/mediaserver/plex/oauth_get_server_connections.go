package plex

import (
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func OAuth_GetServerConnections(ctx context.Context, authToken string) (connections []PlexServersResponse, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Plex: Getting OAuth Server Connections", logging.LevelDebug)
	defer logAction.Complete()

	connections = []PlexServersResponse{}
	Err = logging.LogErrorInfo{}

	// Construct the URL for the Plex OAuth Server Connections request
	u, err := url.Parse("https://plex.tv/api/v2/resources")
	if err != nil {
		logAction.SetError("Failed to parse Plex OAuth Server Connections URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return connections, *logAction.Error
	}
	query := u.Query()
	query.Set("includeHttps", "1")
	query.Set("X-Plex-Token", authToken)
	query.Set("X-Plex-Client-Identifier", "aura")
	u.RawQuery = query.Encode()
	URL := u.String()

	headers := map[string]string{
		"Accept": "application/json",
	}

	// Make the HTTP Request to Plex OAuth to get server connections
	resp, respBody, Err := httpx.MakeHTTPRequest(ctx, URL, "GET", headers, 60, nil, "Plex OAuth")
	if Err.Message != "" {
		return connections, Err
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		logAction.SetError("Plex OAuth Server Connections returned a non-success status code", fmt.Sprintf("Status Code: %d", resp.StatusCode), nil)
		return connections, *logAction.Error
	}

	// Decode the Response
	var plexResp []PlexServersResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex OAuth Server Connections Response")
	if Err.Message != "" {
		return connections, Err
	}

	// Remove any servers that are not owned by the user
	for _, server := range plexResp {
		if server.Owned {
			connections = append(connections, server)
		}
	}

	return connections, Err
}
