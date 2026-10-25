package silo

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
)

// LibraryEpisodeCount is how many episodes a show library has, as the item details don't have each show's episodes
func LibraryEpisodeCount(ctx context.Context, msConfig config.Config_MediaServer, libraryID string) (episodes int, Err logging.LogErrorInfo) {
	u, err := apiURL(msConfig.URL, "catalog")
	if err != nil {
		message, help, detail := logging.Error_BaseUrlParsing(err)
		return 0, logging.LogErrorInfo{Message: message, Help: help, Detail: detail}
	}
	query := u.Query()
	query.Set("library_id", libraryID)
	query.Set("type", "episode")
	query.Set("limit", "1")
	u.RawQuery = query.Encode()

	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		return 0, Err
	}
	var siloResp SiloCatalogResponse
	if Err := httpx.DecodeResponseToJSON(ctx, respBody, &siloResp, "Silo Library Episodes Response"); Err.Message != "" {
		return 0, Err
	}
	return siloResp.Total, logging.LogErrorInfo{}
}
