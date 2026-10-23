package ej

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/url"
	"path"
)

func (e *EJ) GetLibrarySections(ctx context.Context, msConfig config.Config_MediaServer) (sections []models.LibrarySection, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Fetching Library Sections from %s Media Server", msConfig.Type), logging.LevelDebug)
	defer logAction.Complete()

	sections = []models.LibrarySection{}
	Err = logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return sections, *logAction.Error
	}
	u.Path = path.Join(u.Path, "Users", msConfig.UserID, "Items")
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, msConfig, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return sections, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var ejResp EmbyJellyLibrarySectionsResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &ejResp, fmt.Sprintf("%s Library Sections Response", msConfig.Type))
	if Err.Message != "" {
		return sections, Err
	}

	logAction.AppendResult("total_sections", len(ejResp.Items))
	for _, item := range ejResp.Items {
		if item.CollectionType == "" {
			item.CollectionType = "mixed"
		}
		if item.CollectionType != "movies" && item.CollectionType != "tvshows" && item.CollectionType != "mixed" {
			logAction.AppendWarning(fmt.Sprintf("skipped '%s'", item.Name), map[string]string{"collection_type": item.CollectionType})
			continue
		}
		section := models.LibrarySection{}
		section.ID = item.ID
		section.Title = item.Name
		section.Type = map[string]string{
			"movies":  "movie",
			"tvshows": "show",
			"mixed":   "mixed",
		}[item.CollectionType]
		sections = append(sections, section)
	}

	logAction.AppendResult("found", len(sections))
	return sections, Err
}
