package notifications

import (
	"aura/internal/config"
	"aura/internal/logging"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func SendDiscordNotification(message string, imageURL string, title string) logging.ErrorLog {

	if !validNotificationProvider() || config.Global.Notification.Provider != "Discord" {
		return logging.ErrorLog{
			Err: fmt.Errorf("invalid notification provider"),
			Log: logging.Log{
				Message: fmt.Sprintf("Invalid notification provider: %s", config.Global.Notification.Provider),
			}}
	}

	webhookURL := config.Global.Notification.Webhook

	if webhookURL == "" {
		return logging.ErrorLog{
			Err: fmt.Errorf("webhook url is empty"),
			Log: logging.Log{
				Message: "Webhook URL is empty",
			}}
	}

	embed := map[string]any{
		"author": map[string]any{
			"name":     "MediUX AURA Bot",
			"url":      "https://github.com/mediux-team/aura",
			"icon_url": "https://raw.githubusercontent.com/mediux-team/aura/master/frontend/public/aura_logo.png",
		},
		"title":       title,
		"description": message,
		"color":       0x9B59B6, // purple color
	}
	if imageURL != "" {
		embed["image"] = map[string]any{
			"url": imageURL,
		}
	}

	webhookBody := map[string]any{
		"username":   "MediUX AURA Bot",
		"avatar_url": "https://raw.githubusercontent.com/mediux-team/aura/master/frontend/public/aura_logo.png",
		"embeds":     []map[string]any{embed},
	}

	bodyBytes, err := json.Marshal(webhookBody)
	if err != nil {
		return logging.ErrorLog{
			Err: err,
			Log: logging.Log{
				Message: "Failed to marshal webhook body",
			}}
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return logging.ErrorLog{
			Err: err,
			Log: logging.Log{
				Message: "Failed to send webhook request",
			}}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return logging.ErrorLog{
			Err: fmt.Errorf("failed to send webhook request"),
			Log: logging.Log{
				Message: fmt.Sprintf("Failed to send webhook request, status code: %d", resp.StatusCode),
			}}
	}

	return logging.ErrorLog{}
}

func SendDiscordAppStartNotification() logging.ErrorLog {
	if !validNotificationProvider() || config.Global.Notification.Provider != "Discord" {
		return logging.ErrorLog{
			Err: fmt.Errorf("invalid notification provider"),
			Log: logging.Log{
				Message: fmt.Sprintf("Invalid notification provider: %s", config.Global.Notification.Provider),
			}}
	}

	message := "MediUX AURA has started successfully!"
	return SendDiscordNotification(message, "", "MediUX AURA Notification")
}
