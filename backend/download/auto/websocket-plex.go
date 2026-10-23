package autodownload

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"aura/utils"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	plexReconnectDelay = 10 * time.Second
	plexScanCoolDown   = 30 * time.Second
	plexDedupWindow    = 8 * time.Second
	plexItemCacheTTL   = 10 * time.Minute
)

var plexRefreshEventDeduper = struct {
	mu       sync.Mutex
	lastSeen map[string]time.Time
}{
	lastSeen: make(map[string]time.Time),
}

var plexRefreshedItemsCache = struct {
	mu    sync.Mutex
	items map[string]cachedPlexItem
}{
	items: make(map[string]cachedPlexItem),
}

type cachedPlexItem struct {
	mediaItem models.MediaItem
	storedAt  time.Time
}

var (
	plexWSControlMu sync.Mutex
	plexWSStopChan  chan struct{}
)

// StartOrRestartPlexWebSocketClient stops any running Plex WebSocket goroutine and starts a new one.
func StartOrRestartPlexWebSocketClient() {
	plexWSControlMu.Lock()
	defer plexWSControlMu.Unlock()

	// Stop previous goroutine if running
	if plexWSStopChan != nil {
		close(plexWSStopChan)
		plexWSStopChan = nil
	}

	stopChan := make(chan struct{})
	plexWSStopChan = stopChan

	go func(stop <-chan struct{}) {
		defer logging.RecoverPanic("Plex Event Listener")
		for {
			if config.Current.MediaServer.Type != "Plex" ||
				!config.Current.MediaServer.EnablePlexEventListener {
				select {
				case <-stop:
					return
				case <-time.After(plexScanCoolDown):
				}
				continue
			}

			err := connectAndListenPlexWithStop(stop)
			if err != nil {
				logging.LOGGER.Error().Timestamp().Err(err).Msg("Plex WebSocket connection error")
			}

			logging.LOGGER.Warn().Timestamp().Msgf("Reconnecting to Plex WebSocket in %s...", plexReconnectDelay)
			select {
			case <-stop:
				return
			case <-time.After(plexReconnectDelay):
			}
		}
	}(stopChan)
}

// connectAndListenPlexWithStop listens to the Plex websocket, and returns early if stop is closed.
func connectAndListenPlexWithStop(stop <-chan struct{}) (err error) {
	wsURL, wsURLForLog, err := buildPlexWebSocketURL()
	if err != nil {
		return err
	}

	logging.LOGGER.Info().Timestamp().Str("url", wsURLForLog).
		Msg("Plex Event Listener: Connecting to Plex WebSocket")

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Plex WebSocket at %s: %w", wsURLForLog, err)
	}
	defer conn.Close()

	logging.LOGGER.Info().Timestamp().
		Msg("Plex Event Listener: Connected — watching for metadata refresh events")

	for {
		select {
		case <-stop:
			return nil
		default:
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, message, err := conn.ReadMessage()
			if err != nil {
				return fmt.Errorf("error reading from Plex WebSocket: %w", err)
			}
			handleMessage(message)
		}
	}
}

func buildPlexWebSocketURL() (wsURL string, wsURLForLog string, err error) {
	base := config.Current.MediaServer.URL
	base = strings.TrimRight(base, "/")

	// Determine the ws/wss scheme from the http/https URL
	var wsScheme string
	if strings.HasPrefix(base, "https://") {
		wsScheme = "wss"
		base = strings.TrimPrefix(base, "https://")
	} else {
		wsScheme = "ws"
		base = strings.TrimPrefix(base, "http://")
	}

	token := config.Current.MediaServer.ApiToken
	wsURL = fmt.Sprintf("%s://%s/:/websockets/notifications?X-Plex-Token=%s", wsScheme, base, token)

	maskedToken := config.MaskToken(token)
	wsURLForLog = fmt.Sprintf("%s://%s/:/websockets/notifications?X-Plex-Token=%s", wsScheme, base, maskedToken)

	return wsURL, wsURLForLog, nil
}

func handleMessage(message []byte) {
	// Preferred path: strongly-typed parsing for expected Plex payloads.
	var typedPayload PlexNotificationContainer
	if err := json.Unmarshal(message, &typedPayload); err == nil {
		if typedPayload.NotificationContainer.Type == "timeline" && len(typedPayload.NotificationContainer.TimelineEntry) > 0 {
			for _, entry := range typedPayload.NotificationContainer.TimelineEntry {
				messageInfo, ok := buildPlexRefreshMessageFromTyped(entry)
				if !ok {
					continue
				}
				processPlexRefreshMessage(messageInfo)
			}
			return
		}

		// If it is a known non-timeline typed payload (e.g. activity), ignore it.
		if typedPayload.NotificationContainer.Type != "" {
			return
		}
	}

	// Fallback path: generic map parsing for shape variations.
	var root map[string]any
	if err := json.Unmarshal(message, &root); err != nil {
		return
	}

	notificationContainer, ok := asMap(root["NotificationContainer"])
	if !ok {
		return
	}

	entries := getTimelineEntries(root)
	containerType := asString(notificationContainer["type"])
	if len(entries) == 0 && containerType == "" {
		return
	}

	if containerType != "timeline" {
		return
	}

	for _, entryAny := range entries {
		entry, isMap := asMap(entryAny)
		if !isMap {
			continue
		}

		messageInfo, ok := buildPlexRefreshMessage(entry)
		if !ok {
			continue
		}

		processPlexRefreshMessage(messageInfo)
	}
}

func processPlexRefreshMessage(messageInfo PlexRefreshMessage) {
	if !shouldEmitPlexRefreshEvent(messageInfo.SectionID, messageInfo.Subtitle, messageInfo.ItemRatingKey, messageInfo.ItemTypeID) {
		return
	}

	refreshedItem, ok := resolveUpdatedItemFromCache(messageInfo)
	if !ok || refreshedItem.MediaItem.RatingKey == "" {
		logging.LOGGER.Warn().Timestamp().
			Str("subtitle", messageInfo.Subtitle).
			Int("section_id", messageInfo.SectionID).
			Str("item_rating_key", messageInfo.ItemRatingKey).
			Msg("Plex Event Listener: No matching items found in cache")
		return
	}

	if cachedMediaItem, found := getCachedRefreshedMediaItem(refreshedItem.MediaItem.RatingKey); found {
		refreshedItem.MediaItem = cachedMediaItem
		logging.Go("Re-apply Saved Images", func() { reApplySavedImages(refreshedItem) })
		return
	}

	ctx, ld := logging.CreateLoggingContext(context.Background(), "Plex Event Listener")
	logAction := ld.AddAction("Refresh Item on Plex Metadata Update", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	_, Err := mediaserver.GetMediaItemDetails(ctx, &refreshedItem.MediaItem)
	if Err.Message != "" {
		logAction.SetError(
			"Failed to refresh item details",
			"Review errors for more details",
			map[string]any{
				"title":      refreshedItem.MediaItem.Title,
				"rating_key": refreshedItem.MediaItem.RatingKey,
				"error":      Err.Message,
			},
		)
		ld.Log()
		return
	}

	// Persist refreshed details back into the cache
	cache.LibraryStore.UpdateMediaItem(refreshedItem.MediaItem.LibraryTitle, &refreshedItem.MediaItem)
	setCachedRefreshedMediaItem(refreshedItem.MediaItem)

	logging.LOGGER.Info().Timestamp().
		Str("subtitle", messageInfo.Subtitle).
		Int("section_id", messageInfo.SectionID).
		Str("item_rating_key", messageInfo.ItemRatingKey).
		Str("item_title", refreshedItem.MediaItem.Title).
		Str("item_type", refreshedItem.ItemType).
		Msgf("Plex Event Listener: Detected metadata refresh for item %s", utils.MediaItemInfo(refreshedItem.MediaItem))

	logging.Go("Re-apply Saved Images", func() { reApplySavedImages(refreshedItem) })
}

func getCachedRefreshedMediaItem(ratingKey string) (models.MediaItem, bool) {
	if strings.TrimSpace(ratingKey) == "" {
		return models.MediaItem{}, false
	}

	now := time.Now()

	plexRefreshedItemsCache.mu.Lock()
	defer plexRefreshedItemsCache.mu.Unlock()

	for key, item := range plexRefreshedItemsCache.items {
		if now.Sub(item.storedAt) > plexItemCacheTTL {
			delete(plexRefreshedItemsCache.items, key)
		}
	}

	cached, exists := plexRefreshedItemsCache.items[ratingKey]
	if !exists {
		return models.MediaItem{}, false
	}

	return cached.mediaItem, true
}

func setCachedRefreshedMediaItem(item models.MediaItem) {
	if strings.TrimSpace(item.RatingKey) == "" {
		return
	}

	plexRefreshedItemsCache.mu.Lock()
	defer plexRefreshedItemsCache.mu.Unlock()

	plexRefreshedItemsCache.items[item.RatingKey] = cachedPlexItem{
		mediaItem: item,
		storedAt:  time.Now(),
	}
}

func reApplySavedImages(item PlexRefreshedItem) {
	logCtx, ld := logging.CreateLoggingContext(context.Background(), "Plex Event Listener")
	logAction := ld.AddAction("Re-Apply Saved Images After Metadata Refresh", logging.LevelInfo)
	logCtx = logging.WithCurrentAction(logCtx, logAction)
	//defer ld.Log()

	savedItems, Err := database.GetAllSavedSets(logCtx, models.DBFilter{
		ItemTMDB_ID:      item.MediaItem.TMDB_ID,
		ItemLibraryTitle: item.MediaItem.LibraryTitle,
	})
	if Err.Message != "" {
		logAction.SetError("Failed to fetch saved sets", Err.Message, map[string]any{
			"title":      item.MediaItem.Title,
			"rating_key": item.MediaItem.RatingKey,
		})
		return
	}

	if len(savedItems.Items) == 0 {
		logAction.AppendResult("result", "no saved sets found — nothing to re-apply")
		logging.LOGGER.Info().Timestamp().
			Str("item_title", item.MediaItem.Title).
			Str("item_rating_key", item.MediaItem.RatingKey).
			Msg("Plex Event Listener: No saved sets found for refreshed item, skipping image re-application")
		return
	}

	applied := 0
	matchedImages := make([]models.ImageFile, 0)

	for _, savedItem := range savedItems.Items {
		for _, posterSet := range savedItem.PosterSets {
			if !posterSet.AutoDownload {
				continue
			}
			for _, image := range posterSet.Images {
				switch image.Type {
				case "poster":
					if !posterSet.SelectedTypes.Poster {
						continue
					}
				case "backdrop":
					if !posterSet.SelectedTypes.Backdrop {
						continue
					}
				case "season_poster":
					if !posterSet.SelectedTypes.SeasonPoster && !posterSet.SelectedTypes.SpecialSeasonPoster {
						continue
					}
					// If the season number is 0, and we dont have special season poster selected, then skip
					if image.SeasonNumber != nil && *image.SeasonNumber == 0 && !posterSet.SelectedTypes.SpecialSeasonPoster {
						continue
					}
					// If the season number is greater than 0, and we dont have regular season poster selected, then skip
					if image.SeasonNumber != nil && *image.SeasonNumber > 0 && !posterSet.SelectedTypes.SeasonPoster {
						continue
					}
				case "titlecard":
					if !posterSet.SelectedTypes.Titlecard {
						continue
					}
				default:
					continue
				}

				if !shouldReApplyImage(item, image) {
					continue
				}

				matchedImages = append(matchedImages, image)
				applyErr := mediaserver.DownloadApplyImageToMainMediaServer(logCtx, &item.MediaItem, image)
				if applyErr.Message != "" {
					logging.LOGGER.Error().Timestamp().
						Str("error", applyErr.Message).
						Str("image_id", image.ID).
						Str("image_type", image.Type).
						Str("item_title", item.MediaItem.Title).
						Str("item_rating_key", item.MediaItem.RatingKey).
						Msg("Plex Event Listener: Failed to re-apply saved image to refreshed item")
				} else {
					applied++
					logging.LOGGER.Info().Timestamp().
						Str("image_type", image.Type).
						Str("item_title", item.MediaItem.Title).
						Str("item_rating_key", item.MediaItem.RatingKey).
						Msg("Plex Event Listener: Successfully re-applied saved image to refreshed item")
				}
			}
		}
	}

	if applied == 0 {
		logging.DevMsgf("Identified %d saved %s images for refreshed item %s, but none matched the criteria for re-application", len(matchedImages), item.ItemType, utils.MediaItemInfo(item.MediaItem))
		logAction.AppendResult("result", "no matching images found for refreshed item")
		logAction.Complete()
		return
	}

	logAction.AppendResult("matched_images", matchedImages)
	logAction.AppendResult("images_reapplied", applied)
	logAction.Complete()
}

func shouldReApplyImage(item PlexRefreshedItem, image models.ImageFile) bool {
	if item.MediaItem.RatingKey == "" || item.MediaItem.LibraryTitle == "" {
		return false
	}

	switch item.ItemType {
	case "movie":
		return image.Type == "poster" || image.Type == "backdrop"
	case "show":
		return image.Type == "poster" || image.Type == "backdrop"
	case "season":
		if image.Type != "season_poster" {
			return false
		}
		seasonNumber, ok := getSeasonNumberForRefreshedItem(item)
		if !ok || image.SeasonNumber == nil {
			return false
		}
		return *image.SeasonNumber == seasonNumber
	case "episode":
		if image.Type != "titlecard" {
			return false
		}
		seasonNumber, episodeNumber, ok := getEpisodeNumbersForRefreshedItem(item)
		if !ok || image.SeasonNumber == nil || image.EpisodeNumber == nil {
			return false
		}
		return *image.SeasonNumber == seasonNumber && *image.EpisodeNumber == episodeNumber
	default:
		return false
	}
}

func getSeasonNumberForRefreshedItem(item PlexRefreshedItem) (int, bool) {
	if item.MediaItem.Series == nil {
		return 0, false
	}
	for _, season := range item.MediaItem.Series.Seasons {
		if season.RatingKey == item.ItemRatingKey {
			return season.SeasonNumber, true
		}
	}
	return 0, false
}

func getEpisodeNumbersForRefreshedItem(item PlexRefreshedItem) (int, int, bool) {
	if item.MediaItem.Series == nil {
		return 0, 0, false
	}
	for _, season := range item.MediaItem.Series.Seasons {
		for _, episode := range season.Episodes {
			if episode.RatingKey == item.ItemRatingKey {
				return episode.SeasonNumber, episode.EpisodeNumber, true
			}
		}
	}
	return 0, 0, false
}

var episodeSuffixPattern = regexp.MustCompile(`(?i)\s+s\d{1,2}\s+e\d{1,3}$`)
var seasonSuffixPattern = regexp.MustCompile(`(?i)\s+s\d{1,2}$`)
var trailingParenYearPattern = regexp.MustCompile(`\s*\(\d{4}\)$`)
var trailingYearPattern = regexp.MustCompile(`\s+\d{4}$`)

func shouldEmitPlexRefreshEvent(sectionID int, subtitle, episodeRatingKey string, itemType int) bool {
	normalizedSubtitle := strings.ToLower(strings.TrimSpace(subtitle))
	normalizedEpisodeKey := strings.TrimSpace(episodeRatingKey)
	if normalizedEpisodeKey == "" {
		normalizedEpisodeKey = "none"
	}

	dedupKey := fmt.Sprintf("%d|%s|%s|%d", sectionID, normalizedEpisodeKey, normalizedSubtitle, itemType)
	now := time.Now()

	plexRefreshEventDeduper.mu.Lock()
	defer plexRefreshEventDeduper.mu.Unlock()

	for key, ts := range plexRefreshEventDeduper.lastSeen {
		if now.Sub(ts) > plexDedupWindow {
			delete(plexRefreshEventDeduper.lastSeen, key)
		}
	}

	if lastSeen, exists := plexRefreshEventDeduper.lastSeen[dedupKey]; exists {
		if now.Sub(lastSeen) <= plexDedupWindow {
			return false
		}
	}

	plexRefreshEventDeduper.lastSeen[dedupKey] = now
	return true
}

func buildPlexRefreshMessage(entry map[string]any) (PlexRefreshMessage, bool) {
	metadataState := strings.ToLower(asString(entry["metadataState"]))
	timelineState := asInt(firstNonNil(entry["state"], entry["timelineState"]))
	if !isCompletedMetadataState(metadataState, timelineState) {
		return PlexRefreshMessage{}, false
	}

	subtitle := asString(entry["title"])
	sectionID := asInt(firstNonNil(entry["sectionID"], entry["librarySectionID"]))
	if sectionID == 0 {
		return PlexRefreshMessage{}, false
	}

	itemTypeID := asInt(entry["type"])
	itemType := ItemTypeOptions[itemTypeID]
	if itemType == "" {
		itemType = "unknown"
	}

	itemRatingKey := asString(firstNonNil(entry["itemID"], entry["ratingKey"]))
	if itemRatingKey == "" {
		itemRatingKey = parseRatingKeyFromPath(asString(entry["key"]))
	}

	return PlexRefreshMessage{
		Subtitle:      subtitle,
		SectionID:     sectionID,
		ItemRatingKey: itemRatingKey,
		ItemType:      itemType,
		ItemTypeID:    itemTypeID,
		MetadataState: metadataState,
		TimelineState: timelineState,
	}, true
}

func buildPlexRefreshMessageFromTyped(entry PlexTimelineEntry) (PlexRefreshMessage, bool) {
	metadataState := strings.ToLower(strings.TrimSpace(entry.MetadataState))
	if !isCompletedMetadataState(metadataState, entry.State) {
		return PlexRefreshMessage{}, false
	}

	sectionID := asInt(entry.SectionID)
	if sectionID == 0 {
		return PlexRefreshMessage{}, false
	}

	itemType := ItemTypeOptions[entry.Type]
	if itemType == "" {
		itemType = "unknown"
	}

	itemRatingKey := strings.TrimSpace(entry.ItemID)

	return PlexRefreshMessage{
		Subtitle:      strings.TrimSpace(entry.Title),
		SectionID:     sectionID,
		ItemRatingKey: itemRatingKey,
		ItemType:      itemType,
		ItemTypeID:    entry.Type,
		MetadataState: metadataState,
		TimelineState: entry.State,
	}, true
}

func resolveUpdatedItemFromCache(msg PlexRefreshMessage) (updated PlexRefreshedItem, ok bool) {
	if msg.SectionID == 0 {
		return PlexRefreshedItem{}, false
	}

	section, ok := getSectionByID(msg.SectionID)
	if !ok || section == nil {
		return PlexRefreshedItem{}, false
	}

	if msg.ItemRatingKey != "" {
		for _, item := range section.MediaItems {
			if item.RatingKey == msg.ItemRatingKey {
				return PlexRefreshedItem{
					MediaItem:     item,
					ItemRatingKey: msg.ItemRatingKey,
					ItemType:      msg.ItemType,
					ItemTypeID:    msg.ItemTypeID,
				}, true
			}

			if item.Series == nil {
				continue
			}

			for _, season := range item.Series.Seasons {
				if season.RatingKey == msg.ItemRatingKey {
					return PlexRefreshedItem{
						MediaItem:     item,
						ItemRatingKey: msg.ItemRatingKey,
						ItemType:      msg.ItemType,
						ItemTypeID:    msg.ItemTypeID,
					}, true
				}
				for _, episode := range season.Episodes {
					if episode.RatingKey == msg.ItemRatingKey {
						return PlexRefreshedItem{
							MediaItem:     item,
							ItemRatingKey: msg.ItemRatingKey,
							ItemType:      msg.ItemType,
							ItemTypeID:    msg.ItemTypeID,
						}, true
					}
				}
			}
		}
	}

	normalizedShowTitle := normalizeTitle(extractShowTitle(msg.Subtitle))
	if normalizedShowTitle == "" {
		return PlexRefreshedItem{}, false
	}

	for _, item := range section.MediaItems {
		if normalizeTitle(item.Title) == normalizedShowTitle {
			return PlexRefreshedItem{
				MediaItem:     item,
				ItemRatingKey: msg.ItemRatingKey,
				ItemType:      msg.ItemType,
				ItemTypeID:    msg.ItemTypeID,
			}, true
		}
	}

	return PlexRefreshedItem{}, false
}

func getSectionByID(sectionID int) (*models.LibrarySection, bool) {
	sectionIDStr := strconv.Itoa(sectionID)
	for _, section := range cache.LibraryStore.GetAllSectionsSortedByTitle() {
		if section != nil && section.ID == sectionIDStr {
			return section, true
		}
	}
	return nil, false
}

func extractShowTitle(subtitle string) string {
	cleaned := strings.TrimSpace(subtitle)
	if cleaned == "" {
		return ""
	}

	cleaned = episodeSuffixPattern.ReplaceAllString(cleaned, "")
	cleaned = seasonSuffixPattern.ReplaceAllString(cleaned, "")
	cleaned = trailingParenYearPattern.ReplaceAllString(cleaned, "")
	cleaned = trailingYearPattern.ReplaceAllString(cleaned, "")

	return strings.TrimSpace(cleaned)
}

func normalizeTitle(title string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	if t == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(t))
	for _, r := range t {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune(r)
		}
	}

	return strings.Join(strings.Fields(b.String()), " ")
}

func isCompletedMetadataState(metadataState string, timelineState int) bool {
	switch metadataState {
	case "processed", "finished", "complete", "completed", "loaded":
		return true
	}

	// Plex timeline messages often use numeric state values.
	// state=5 is commonly emitted when processing has finished.
	return timelineState == 5
}

func getTimelineEntries(root map[string]any) []any {
	notificationContainer, ok := asMap(root["NotificationContainer"])
	if !ok {
		return nil
	}

	for _, key := range []string{"TimelineEntry", "timelineEntry", "TimelineEntries", "timelineEntries"} {
		if entries, ok := asSlice(notificationContainer[key]); ok {
			return entries
		}
		if entry, ok := asMap(notificationContainer[key]); ok {
			return []any{entry}
		}
	}

	return nil
}

func parseRatingKeyFromPath(keyPath string) string {
	trimmed := strings.TrimSpace(keyPath)
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "metadata" && i+1 < len(parts) {
			return strings.TrimSpace(parts[i+1])
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asSlice(v any) ([]any, bool) {
	s, ok := v.([]any)
	return s, ok
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return fmt.Sprintf("%.0f", x)
	default:
		return ""
	}
}

func asInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		var out int
		_, _ = fmt.Sscanf(strings.TrimSpace(x), "%d", &out)
		return out
	default:
		return 0
	}
}

type PlexRefreshMessage struct {
	Subtitle      string `json:"subtitle"`
	SectionID     int    `json:"section_id"`
	ItemRatingKey string `json:"item_rating_key"`
	ItemType      string `json:"item_type"`
	ItemTypeID    int    `json:"item_type_id"`
	MetadataState string `json:"metadata_state"`
	TimelineState int    `json:"timeline_state"`
}

type PlexRefreshedItem struct {
	MediaItem     models.MediaItem `json:"media_item"`
	ItemRatingKey string           `json:"item_rating_key"`
	ItemType      string           `json:"item_type"`
	ItemTypeID    int              `json:"item_type_id"`
}

var ItemTypeOptions = map[int]string{
	1: "movie",
	2: "show",
	3: "season",
	4: "episode",
}

type PlexNotificationContainer struct {
	NotificationContainer PlexNotificationBody `json:"NotificationContainer"`
}

type PlexNotificationBody struct {
	Type                 string                     `json:"type"`
	Size                 int                        `json:"size"`
	ActivityNotification []PlexActivityNotification `json:"ActivityNotification,omitempty"`
	TimelineEntry        []PlexTimelineEntry        `json:"TimelineEntry,omitempty"`
}

type PlexActivityNotification struct {
	Event    string       `json:"event"`
	UUID     string       `json:"uuid"`
	Activity PlexActivity `json:"Activity"`
}

type PlexActivity struct {
	UUID        string `json:"uuid"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Progress    int    `json:"progress"`
	Cancellable bool   `json:"cancellable,omitempty"`
	UserID      int    `json:"userID,omitempty"`
}

type PlexTimelineEntry struct {
	Identifier    string `json:"identifier"`
	SectionID     string `json:"sectionID"`
	ItemID        string `json:"itemID"`
	Type          int    `json:"type"` // 1 movie, 2 show, 3 season, 4 episode
	Title         string `json:"title"`
	State         int    `json:"state"`
	MetadataState string `json:"metadataState,omitempty"`
	MediaState    string `json:"mediaState,omitempty"`
	UpdatedAt     int64  `json:"updatedAt"`
}
