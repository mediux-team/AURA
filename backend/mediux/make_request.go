package mediux

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

var MediuxApiURL string = "https://images.mediux.io"
var MediuxPublicURL string = "https://api.mediux.pro"

// How long to wait before each retry, when MediUX gets too many requests
var tooManyRequestsWaits = []time.Duration{2 * time.Second, 5 * time.Second}

const maxTooManyRequestsWait = 15 * time.Second

var mediuxRestyClient = resty.New().
	SetHeader("Content-Type", "application/json").
	SetHeader("User-Agent", "aura/1.0").
	SetHeader("X-Request", "mediux-aura")

func makeRequest(ctx context.Context, url string, method string, body []byte, token string, isImageRequest bool) (resp *http.Response, respBody []byte, Err logging.LogErrorInfo) {
	// Make the HTTP Headers for this request
	headers := make(map[string]string)
	headers = AddMediuxAuthHeader(url, token, isImageRequest, headers)

	// Make the HTTP request to MediUX, waiting and trying again when MediUX gets too many requests
	for attempt := 0; ; attempt++ {
		resp, respBody, Err = httpx.MakeHTTPRequest(ctx, url, method, headers, 60, body, "MediUX")
		if Err.Message != "" {
			return nil, nil, Err
		}
		if resp.StatusCode != http.StatusTooManyRequests || attempt == len(tooManyRequestsWaits) {
			break
		}
		wait := tooManyRequestsWaits[attempt]
		if seconds, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && seconds > 0 {
			wait = min(time.Duration(seconds)*time.Second, maxTooManyRequestsWait)
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, nil, logging.LogErrorInfo{Message: "Stopped waiting for MediUX"}
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, logging.LogErrorInfo{
			Message: "MediUX Server returned a non-success status code",
			Help:    "Check the response from the server for more details",
			Detail:  map[string]any{"status_code": resp.StatusCode, "error_body": string(respBody)},
		}
	}

	return resp, respBody, logging.LogErrorInfo{}
}

type MediuxGraphQLQueryBody struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
	QueryName string         `json:"query_name,omitempty"`
}

func makeGraphQLRequest(ctx context.Context, queryBody MediuxGraphQLQueryBody) (resp *resty.Response, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Send GraphQL Request to MediUX", logging.LevelTrace)
	defer logAction.Complete()

	resp, err := mediuxRestyClient.R().
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", config.Current.Mediux.ApiToken)).
		SetBody(queryBody).
		Post(fmt.Sprintf("%s/graphql", MediuxApiURL))
	if err != nil {
		logAction.SetError("Failed to send GraphQL request to MediUX", "Ensure the MediUX API is reachable and the token is valid.",
			map[string]any{
				"error":         err.Error(),
				"status_code":   resp.StatusCode(),
				"response_body": resp.String(),
				"query_name":    queryBody.QueryName,
			})
		return nil, *logAction.Error
	}
	if resp.StatusCode() != http.StatusOK {
		logAction.SetError("MediUX GraphQL request returned non-200 status", "Check the MediUX API status or your request parameters.",
			map[string]any{
				"status_code":   resp.StatusCode(),
				"response_body": resp.String(),
				"query_name":    queryBody.QueryName,
			})
		return nil, *logAction.Error
	}
	return resp, logging.LogErrorInfo{}
}

func AddMediuxAuthHeader(url string, token string, isImageRequest bool, headers map[string]string) map[string]string {
	if token == "" {
		token = config.Current.Mediux.ApiToken
	}
	if strings.HasPrefix(url, MediuxApiURL) || strings.HasPrefix(url, "https://api.mediux.io/") {
		headers["Authorization"] = fmt.Sprintf("Bearer %s", token)
	}
	if isImageRequest {
		headers["Accept"] = "image/*"
	} else {
		headers["Accept"] = "application/json"
	}
	return headers
}
