package sonarr_radarr

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/url"
	"path"
)

func (s *SonarrApp) GetItemInfoFromTMDBID(ctx context.Context, app config.Config_SonarrRadarrApp, tmdbID int) (resp any, Err logging.LogErrorInfo) {
	return srGetItemInfoFromTMDBID(ctx, app, tmdbID)
}

func (r *RadarrApp) GetItemInfoFromTMDBID(ctx context.Context, app config.Config_SonarrRadarrApp, tmdbID int) (resp any, Err logging.LogErrorInfo) {
	return srGetItemInfoFromTMDBID(ctx, app, tmdbID)
}

func GetItemInfoFromTMDBID(ctx context.Context, app config.Config_SonarrRadarrApp, tmdbID int) (resp any, Err logging.LogErrorInfo) {
	interfaceSR, Err := NewSonarrRadarrInterface(ctx, app)
	if Err.Message != "" {
		return nil, Err
	}
	return interfaceSR.GetItemInfoFromTMDBID(ctx, app, tmdbID)
}

func srGetItemInfoFromTMDBID(ctx context.Context, app config.Config_SonarrRadarrApp, tmdbID int) (resp any, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Fetching full item info from TMDB ID %d from %s (%s)", tmdbID, app.Type, app.Library), logging.LevelInfo)
	defer logAction.Complete()

	actionGetBaseInfo := logAction.AddSubAction("Getting base item info", logging.LevelDebug)
	actionGetURL := actionGetBaseInfo.AddSubAction("Constructing API URL", logging.LevelDebug)
	urlEndpoint := ""
	switch app.Type {
	case "Sonarr":
		urlEndpoint = "series"
	case "Radarr":
		urlEndpoint = "movie"
	default:
		actionGetURL.SetError("Unsupported application type", "Ensure the application type is either 'Sonarr' or 'Radarr'", map[string]any{"appType": app.Type})
		return nil, *actionGetURL.Error
	}

	// Construct the URL
	u, err := url.Parse(app.URL)
	if err != nil {
		actionGetURL.SetError(fmt.Sprintf("Invalid %s URL", app.Type),
			"Make sure that the URL is properly formatted",
			map[string]any{
				"url":   app.URL,
				"error": err.Error(),
			})
		return nil, *actionGetURL.Error
	}
	u.Path = path.Join(u.Path, "api", "v3", urlEndpoint, "lookup")
	query := u.Query()
	query.Set("term", fmt.Sprintf("tmdb:%d", tmdbID))
	u.RawQuery = query.Encode()
	URL := u.String()
	actionGetURL.Complete()

	// Make the request to Sonarr/Radarr
	_, respBody, Err := makeRequest(ctx, app, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return nil, *logAction.Error
	}

	// Decode the response body
	var resultsResp []struct {
		ID      int `json:"id"`
		TMDB_ID int `json:"tmdbId"`
	}
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &resultsResp, fmt.Sprintf("Decoding %s Item Info Response", app.Type))
	if Err.Message != "" {
		actionGetBaseInfo.SetError(fmt.Sprintf("Failed to Decode %s Item Info Response", app.Type),
			"Ensure that the Sonarr/Radarr server is running a compatible version",
			map[string]any{
				"response": string(respBody),
			})
		return nil, Err
	}
	if len(resultsResp) == 0 {
		actionGetBaseInfo.SetError("No item found with the provided TMDB ID",
			"Ensure that the TMDB ID is correct and that the item exists in Sonarr/Radarr",
			map[string]any{
				"tmdb_id": tmdbID,
			})
		return nil, *actionGetBaseInfo.Error
	}

	result := resultsResp[0]

	// Make sure that the TMDB ID matches
	if result.TMDB_ID != tmdbID {
		actionGetBaseInfo.SetError("TMDB ID mismatch in response",
			"Ensure that the TMDB ID is correct and that the item exists in Sonarr/Radarr",
			map[string]any{
				"requested_tmdb_id": tmdbID,
				"response_tmdb_id":  result.TMDB_ID,
			})
		return nil, *actionGetBaseInfo.Error
	}

	// If the ID is not found, then the item is not in Sonarr/Radarr
	if result.ID == 0 {
		actionGetBaseInfo.SetError("Item not found in Sonarr/Radarr",
			"Ensure that the item exists in Sonarr/Radarr",
			map[string]any{
				"tmdb_id": tmdbID,
			})
		return nil, *actionGetBaseInfo.Error
	}
	actionGetBaseInfo.Complete()

	// Now that we have the base info, fetch the full item info
	actionFetchFullInfo := logAction.AddSubAction("Fetching full item info", logging.LevelDebug)

	u, err = url.Parse(app.URL)
	if err != nil {
		actionFetchFullInfo.SetError(fmt.Sprintf("Invalid %s URL", app.Type),
			"Make sure that the URL is properly formatted",
			map[string]any{
				"url":   app.URL,
				"error": err.Error(),
			})
		return nil, *actionFetchFullInfo.Error
	}
	u.Path = path.Join(u.Path, "api", "v3", urlEndpoint, fmt.Sprintf("%d", result.ID))
	URL = u.String()

	// Make the request to Sonarr/Radarr
	_, respBody, Err = makeRequest(ctx, app, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return nil, *logAction.Error
	}

	// Parse the full item response body
	item, err := parseSRItem(respBody)
	if err != nil {
		actionFetchFullInfo.SetError(fmt.Sprintf("Failed to decode %s item info", app.Type), "Ensure the response format is correct", map[string]any{"error": err.Error()})
		return nil, *actionFetchFullInfo.Error
	}
	actionFetchFullInfo.Complete()
	return item, logging.LogErrorInfo{}
}
