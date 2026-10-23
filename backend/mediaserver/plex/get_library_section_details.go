package plex

import (
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/url"
	"path"
)

func (p *Plex) GetLibrarySectionDetails(ctx context.Context, library *models.LibrarySection) (found bool, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Fetching Details for Library Section: %s", library.Title,
	), logging.LevelDebug)
	defer logAction.Complete()

	found = false

	// Construct the URL for the Plex library sections API request
	u, err := url.Parse(p.Config.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return found, *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "sections", "all")
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, respBody, Err := makeRequest(ctx, p.Config, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return found, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var plexResp PlexLibrarySectionsWrapper
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &plexResp, "Plex Library Sections Response")
	if Err.Message != "" {
		return found, Err
	}

	// Find the specific library section and update its details
	for _, plexSection := range plexResp.MediaContainer.Directory {
		if plexSection.Type != "movie" && plexSection.Type != "show" {
			continue
		}
		if plexSection.Title == library.Title {
			library.Type = plexSection.Type
			library.ID = plexSection.Key
			for _, location := range plexSection.Location {
				library.Paths = append(library.Paths, location.Path)
			}
			found = true
			break
		}
	}
	if !found {
		logAction.SetError("Library section not found", fmt.Sprintf("Ensure the library section '%s' exists on the Plex server.", library.Title), map[string]any{
			"URL": URL,
		})
		return false, *logAction.Error
	}

	return found, logging.LogErrorInfo{}
}
