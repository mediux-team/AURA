package ej

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/http"
	"strings"
)

func makeRequest(ctx context.Context, msConfig config.Config_MediaServer, url string, method string, body []byte) (resp *http.Response, respBody []byte, Err logging.LogErrorInfo) {
	// Make the HTTP Headers for this request
	headers := make(map[string]string)

	if (strings.HasSuffix(url, "Primary") || strings.HasSuffix(url, "Backdrop")) && method == "POST" {
		headers["Content-Type"] = "image/jpeg"
	}
	headers = AddEJAuthHeaders(msConfig, headers)

	// Make the HTTP request to EJ
	resp, respBody, Err = httpx.MakeHTTPRequest(ctx, url, method, headers, 60, body, msConfig.Type)
	if Err.Message != "" {
		return nil, nil, Err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, logging.LogErrorInfo{
			Message: fmt.Sprintf("%s Server returned a non-success status code", msConfig.Type),
			Help:    "Check the response from the server for more details",
			Detail:  map[string]any{"status_code": resp.StatusCode, "error_body": string(respBody)},
		}
	}

	return resp, respBody, logging.LogErrorInfo{}
}

func AddEJAuthHeaders(msConfig config.Config_MediaServer, headers map[string]string) map[string]string {
	headers["X-Emby-Token"] = msConfig.ApiToken
	if msConfig.Type == "Jellyfin" {
		headers["Authorization"] = fmt.Sprintf(`MediaBrowser Token="%s"`, msConfig.ApiToken)
	}
	headers["Accept"] = "application/json"
	headers["X-Emby-Client"] = "aura"
	headers["X-Emby-Device"] = "aura"
	headers["X-Emby-Device-Id"] = "aura"
	headers["X-Emby-Version"] = "1.0"
	return headers
}
