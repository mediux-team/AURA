package plex

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"net/url"
	"path"
)

func (p *Plex) GetLibrarySections(ctx context.Context, msConfig config.Config_MediaServer) (sections []models.LibrarySection, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Plex: Fetching Library Sections", logging.LevelInfo)
	defer logAction.Complete()

	sections = []models.LibrarySection{}
	Err = logging.LogErrorInfo{}

	// Construct the URL for the Plex library sections API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return sections, *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "sections", "all")
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, msConfig, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return sections, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var plexResp PlexLibrarySectionsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex Library Sections Response")
	if Err.Message != "" {
		return sections, Err
	}

	// Map the Plex library sections to the LibrarySection model
	for _, plexSection := range plexResp.MediaContainer.Directory {
		if plexSection.Type != "movie" && plexSection.Type != "show" {
			continue
		}
		library := models.LibrarySection{}
		library.Title = plexSection.Title
		library.Type = plexSection.Type
		library.ID = plexSection.Key
		for _, location := range plexSection.Location {
			library.Paths = append(library.Paths, location.Path)
		}
		sections = append(sections, library)
	}

	logAction.AppendResult("found", len(sections))
	return sections, Err
}
