package autodownload

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	mediuxReconnectDelay = 5 * time.Second
)

func StartMediuxWebSocketClient() {
	go func() {
		defer logging.RecoverPanic("MediUX WebSocket")
		for {
			err := connectAndSubscribeMediux()
			if err != nil {
				logging.LOGGER.Error().Timestamp().Err(err).Msg("Mediux WebSocket connection error")
			}
			logging.LOGGER.Warn().Timestamp().Msgf("Reconnecting to WebSocket in %s...", mediuxReconnectDelay)
			// Wait before reconnecting
			time.Sleep(mediuxReconnectDelay)
		}
	}()
}

func connectAndSubscribeMediux() error {
	// Build WebSocket URL with token
	URL, err := buildMediuxWebSocketURL()
	if err != nil {
		return err
	}
	maskedToken := config.MaskToken(config.Current.Mediux.ApiToken)
	maskedURL := strings.ReplaceAll(URL, config.Current.Mediux.ApiToken, maskedToken)

	logging.LOGGER.Info().Timestamp().Str("url", maskedURL).
		Msg("Mediux Event Listener: Connecting to Mediux WebSocket")

	// Connect to WebSocket
	c, _, err := websocket.DefaultDialer.Dial(URL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Mediux WebSocket at %s: %w", maskedURL, err)
	}
	defer c.Close()

	collectionType := "show_sets"
	subscribeMsg := map[string]any{
		"type":       "subscribe",
		"collection": collectionType,
	}

	if err := c.WriteJSON(subscribeMsg); err != nil {
		return err
	}
	logging.LOGGER.Debug().Timestamp().Str("collection", collectionType).Msg("Subscribed to Mediux WebSocket collection")

	// Listen for messages until error/close
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			return err
		}

		// Decode message
		var msg MediuxWebSocketResponseMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to unmarshal WebSocket message")
			continue
		}

		// Handle Ping messages
		if msg.Type == "ping" {
			pongMsg := map[string]string{
				"type": "pong",
			}
			if err := c.WriteJSON(pongMsg); err != nil {
				logging.LOGGER.Error().Timestamp().Err(err).Msg("Failed to send pong message")
			}
			continue
		}

		// Handle message based on type/event as needed
		if msg.Event == "init" {
			continue
		} else if msg.Event == "update" {
			for set := range msg.UpdatedSets {
				ctx, ld := logging.CreateLoggingContext(context.Background(), "Mediux Update - Processing Set")
				logAction := ld.AddAction("Fetching All Items from DB", logging.LevelDebug)
				ctx = logging.WithCurrentAction(ctx, logAction)

				setID := msg.UpdatedSets[set].ID
				showID := msg.UpdatedSets[set].ShowID
				showTitle := msg.UpdatedSets[set].Title

				logAction.AppendResult("set_id", setID)
				logAction.AppendResult("set_show_id", showID)
				logAction.AppendResult("set_title", showTitle)

				logging.LOGGER.Info().Timestamp().Int("set_id", msg.UpdatedSets[set].ID).Str("set_title", msg.UpdatedSets[set].Title).Msg("Show set updated")

				// Get all items from the database that match this Set
				dbFilter := models.DBFilter{
					ItemTMDB_ID: showID,
					SetID:       strconv.Itoa(setID),
				}
				out, Err := database.GetAllSavedSets(ctx, dbFilter)
				if Err.Message != "" {
					logging.LOGGER.Error().Timestamp().Str("error", Err.Message).Msg("Failed to fetch items from DB for updated set")
					continue
				}
				if len(out.Items) == 0 {
					continue // No items found in database for this set, skip processing
				}

				logging.LOGGER.Debug().Timestamp().Int("num_items", len(out.Items)).Msg("Found Items in DB for updated set")

				// Process each item in the set
				for _, dbItem := range out.Items {
					logging.LOGGER.Debug().Timestamp().Msgf("Processing %s since a set it contains was updated", utils.MediaItemInfo(dbItem.MediaItem))
					result := CheckItem(ctx, dbItem)
					switch result.OverallResult {
					case "Success":
						logging.LOGGER.Info().Timestamp().Msgf("Auto-download success for item %s", utils.MediaItemInfo(dbItem.MediaItem))
						logAction.AppendResult("auto_download_result", result)
					case "Warn":
						logging.LOGGER.Warn().Timestamp().Msgf("Auto-download warning for item %s", utils.MediaItemInfo(dbItem.MediaItem))
						logAction.AppendResult("auto_download_result", result)
					case "Error":
						logging.LOGGER.Error().Timestamp().Msgf("Auto-download error for item %s", utils.MediaItemInfo(dbItem.MediaItem))
						logAction.AppendResult("auto_download_result", result)
					case "Skipped":
						logging.LOGGER.Debug().Timestamp().Msgf("Auto-download skipped for item %s", utils.MediaItemInfo(dbItem.MediaItem))
						logAction.AppendResult("auto_download_result", result)
					}
				}
				ld.Log()
			}
		} else {
			logging.LOGGER.Warn().Timestamp().
				Str("type", msg.Type).
				Str("event", msg.Event).
				Interface("full", msg).
				Msg("Unhandled WebSocket message type/event")
		}

	}
}

func buildMediuxWebSocketURL() (string, error) {
	u, err := url.Parse(mediux.MediuxApiURL)
	if err != nil {
		return "", err
	}
	u.Scheme = "wss"
	u.Path = "/websocket"
	q := u.Query()
	q.Set("access_token", config.Current.Mediux.ApiToken)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

type MediuxWebSocketResponseMessage struct {
	Type        string                      `json:"type"`  // Always "subscription" for responses
	Event       string                      `json:"event"` // Example: "init", "ping", "update"
	UpdatedSets []MediuxWebSocketUpdateData `json:"data"`
}

type MediuxWebSocketUpdateData struct {
	ID          int    `json:"id"`
	Title       string `json:"set_title"`
	ShowID      string `json:"show_id"`
	DateUpdated string `json:"date_updated"`
}
