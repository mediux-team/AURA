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

func GetMovieCollectionSection(ctx context.Context, msConfig config.Config_MediaServer) (section models.LibrarySection, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("%s: Get Movie Collection Section", msConfig.Type), logging.LevelInfo)

	section = models.LibrarySection{}
	Err = logging.LogErrorInfo{}

	// Construct the URL for the EJ server API request
	u, err := url.Parse(msConfig.URL)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return section, *logAction.Error
	}
	u.Path = path.Join(u.Path, "Users", msConfig.UserID, "Items")
	URL := u.String()

	// Make the HTTP Request to EJ
	resp, respBody, Err := makeRequest(ctx, msConfig, URL, "GET", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return section, *logAction.Error
	}
	defer resp.Body.Close()

	// Decode the Response
	var ejResp EmbyJellyLibrarySectionsResponse
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &ejResp, fmt.Sprintf("%s Library Sections Response", msConfig.Type))
	if Err.Message != "" {
		return section, Err
	}

	for _, item := range ejResp.Items {
		if item.CollectionType != "boxsets" {
			continue
		}
		section = models.LibrarySection{
			LibrarySectionBase: models.LibrarySectionBase{
				ID:    item.ID,
				Title: item.Name,
				Type:  item.CollectionType,
			},
		}
		break
	}

	return section, Err
}
