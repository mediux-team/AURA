package utils

import (
	"aura/internal/config"
	"aura/internal/logging"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MakeHTTPRequest function to handle HTTP requests
func MakeHTTPRequest(url, method string, headers map[string]string, timeout int, body []byte, tokenType string) (*http.Response, []byte, logging.ErrorLog) {
	startTime := time.Now()
	var urlTitle string
	if tokenType == "MediaServer" {
		urlTitle = config.Global.MediaServer.Type
	} else {
		urlTitle = getURLTitle(url)
	}

	// Create a context with a timeout
	timeoutInterval := time.Duration(timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeoutInterval)
	defer cancel()

	// Create a new request with context
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
	if err != nil {
		errorMsg := fmt.Sprintf("Error creating HTTP request (%s) [%s]", urlTitle, ElapsedTime(startTime))
		return nil, nil, logging.ErrorLog{Err: err, Log: logging.Log{Message: errorMsg}}
	}

	// Add a User-Agent header to the request
	req.Header.Set("User-Agent", "AURA/1.0")
	req.Header.Set("X-Request", "mediux-aura")

	// Add headers to the request
	if tokenType == "MediaServer" {
		if strings.ToLower(config.Global.MediaServer.Type) == "plex" {
			req.Header.Set("X-Plex-Token", config.Global.MediaServer.Token)
		} else if strings.ToLower(config.Global.MediaServer.Type) == "emby" {
			req.Header.Set("X-Emby-Token", config.Global.MediaServer.Token)
		} else if strings.ToLower(config.Global.MediaServer.Type) == "jellyfin" {
			req.Header.Set("X-Emby-Token", config.Global.MediaServer.Token)
		}
	} else if strings.ToLower(tokenType) == "tmdb" {
		req.Header.Set("Authorization", "Bearer "+config.Global.TMDB.ApiKey)
	} else if strings.ToLower(tokenType) == "mediux" {
		req.Header.Set("Authorization", "Bearer "+config.Global.Mediux.Token)
	} else if strings.ToLower(tokenType) == "plex" {
		req.Header.Set("X-Plex-Token", config.Global.MediaServer.Token)
	} else if strings.ToLower(tokenType) == "emby" {
		req.Header.Set("X-Emby-Token", config.Global.MediaServer.Token)
	} else if strings.ToLower(tokenType) == "jellyfin" {
		req.Header.Set("X-Emby-Token", config.Global.MediaServer.Token)
	}

	if headers != nil {
		for key, value := range headers {
			req.Header.Add(key, value)
		}
		logging.LOG.Trace("Added custom headers to request")
	}

	// Create a new HTTP client with both HTTP/1.1 and HTTP/2 support
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
			ForceAttemptHTTP2: true, // Try HTTP/2 but fallback to HTTP/1.1 if needed
		},
		Timeout: timeoutInterval,
	}

	// Add common headers
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Accept", "*/*")

	// Send the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		errorMsg := fmt.Sprintf("Error sending HTTP request (%s) [%s]", urlTitle, ElapsedTime(startTime))
		return nil, nil, logging.ErrorLog{Err: err, Log: logging.Log{Message: errorMsg}}
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		errorMsg := fmt.Sprintf("Error reading HTTP response body (%s) [%s]", urlTitle, ElapsedTime(startTime))
		return nil, nil, logging.ErrorLog{Err: err, Log: logging.Log{Message: errorMsg}}
	}

	// Defer closing the response body
	defer resp.Body.Close()
	logging.LOG.Trace(fmt.Sprintf("Sent HTTP request to %s [%s]", urlTitle, ElapsedTime(startTime)))
	// Return the response
	return resp, respBody, logging.ErrorLog{}
}

func DecodeJSONBody(w http.ResponseWriter, r *http.Request, v any, structName string, startTime time.Time) logging.ErrorLog {
	logging.LOG.Debug(fmt.Sprintf("Decoding the request body into the `%s` struct", structName))
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(v)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to decode the request body into the `%s` struct --- `%s`", structName, err.Error())
		SendJsonResponse(w, http.StatusBadRequest, JSONResponse{
			Status:  "error",
			Message: errorMsg,
			Elapsed: ElapsedTime(startTime),
		})
		return logging.ErrorLog{Err: err, Log: logging.Log{Message: errorMsg}}
	}
	logging.LOG.Trace(fmt.Sprintf("Decoded the request body into the `%s` struct", structName))
	return logging.ErrorLog{}
}

func getURLTitle(rawURL string) string {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return ""
	}
	return parsedURL.Host
}
