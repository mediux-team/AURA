package routes_logging

import (
	"aura/logging"
	"aura/utils/httpx"
	"bufio"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type structActionLabelSection struct {
	Label   string `json:"label"`
	Section string `json:"section"`
}

var possibleActionsMutex sync.Mutex

// The label and section of each route, by method and path. Routes with {params} match by pattern.
var possible_actions_paths = map[string]structActionLabelSection{
	// Base Routes
	"GET:/api": {
		Label:   "Health Check (API Root)",
		Section: "BASE",
	},
	"GET:/api/health": {
		Label:   "Health Check",
		Section: "BASE",
	},

	// Login & Auth
	"POST:/api/login": {
		Label:   "User Login",
		Section: "AUTH",
	},
	"POST:/api/logout": {
		Label:   "User Logout",
		Section: "AUTH",
	},
	"GET:/api/auth/oidc/login": {
		Label:   "Start OIDC Login",
		Section: "AUTH",
	},
	"GET:/api/auth/oidc/callback": {
		Label:   "OIDC Login Callback",
		Section: "AUTH",
	},
	"GET:/api/config/auth-methods": {
		Label:   "Get Login Methods",
		Section: "AUTH",
	},
	"POST:/api/config/auth/api-key": {
		Label:   "Generate API Key",
		Section: "AUTH",
	},

	// Setup Routes
	"POST:/api/setup/verify": {
		Label:   "Verify Setup Code",
		Section: "SETUP",
	},
	"POST:/api/onboarding/finalize": {
		Label:   "Finish Setup",
		Section: "SETUP",
	},

	// Search
	"GET:/api/search": {
		Label:   "Handle Search",
		Section: "SEARCH",
	},

	// Sonarr/Radarr Webhook
	"POST:/api/sonarr/webhook": {
		Label:   "Handle Sonarr Webhook",
		Section: "SONARR/RADARR",
	},

	// Config Routes
	"GET:/api/config": {
		Label:   "Get Config",
		Section: "CONFIG",
	},
	"GET:/api/config/stream": {
		Label:   "Stream Config Status",
		Section: "CONFIG",
	},
	"GET:/api/config/template-variables": {
		Label:   "Get Notification Template Variables",
		Section: "CONFIG",
	},
	"POST:/api/config": {
		Label:   "Update Config",
		Section: "CONFIG",
	},
	"PATCH:/api/config/{section}": {
		Label:   "Update Config Section",
		Section: "CONFIG",
	},
	"GET:/api/config/export": {
		Label:   "Export Config",
		Section: "CONFIG",
	},
	"POST:/api/config/import": {
		Label:   "Import Config",
		Section: "CONFIG",
	},
	"POST:/api/config/main-media-server/preview": {
		Label:   "Preview Main Media Server Switch",
		Section: "CONFIG",
	},
	"POST:/api/config/main-media-server": {
		Label:   "Switch Main Media Server",
		Section: "CONFIG",
	},

	// Database Routes
	"GET:/api/db": {
		Label:   "Get All Saved Items",
		Section: "DATABASE",
	},
	"POST:/api/db": {
		Label:   "Add Saved Item",
		Section: "DATABASE",
	},
	"PATCH:/api/db": {
		Label:   "Update Saved Item",
		Section: "DATABASE",
	},
	"DELETE:/api/db": {
		Label:   "Delete Saved Item",
		Section: "DATABASE",
	},
	"PATCH:/api/db/ignore": {
		Label:   "Ignore Saved Item",
		Section: "DATABASE",
	},
	"PATCH:/api/db/ignore/stop": {
		Label:   "Stop Ignoring Saved Item",
		Section: "DATABASE",
	},
	"POST:/api/db/force-check": {
		Label:   "Force Check Saved Items",
		Section: "DATABASE",
	},
	"GET:/api/db/creators": {
		Label:   "Get Saved Set Creators",
		Section: "DATABASE",
	},
	"GET:/api/db/backups": {
		Label:   "Get Database Backups",
		Section: "DATABASE",
	},
	"POST:/api/db/backups": {
		Label:   "Create Database Backup",
		Section: "DATABASE",
	},

	// Jobs Routes
	"GET:/api/jobs": {
		Label:   "Get Jobs",
		Section: "JOBS",
	},
	"POST:/api/jobs": {
		Label:   "Run Job",
		Section: "JOBS",
	},

	// Download Routes
	"POST:/api/download/image/item": {
		Label:   "Download Media Item Image",
		Section: "DOWNLOAD",
	},
	"POST:/api/download/image/collection": {
		Label:   "Download Collection Image",
		Section: "DOWNLOAD",
	},
	"POST:/api/download/favorite-creators": {
		Label:   "Download From Favorite Creators",
		Section: "DOWNLOAD",
	},
	"POST:/api/download/upgrade-sets": {
		Label:   "Upgrade Saved Sets",
		Section: "DOWNLOAD",
	},

	// Download Queue Routes
	"GET:/api/download/queue": {
		Label:   "Get Download Queue Status",
		Section: "DOWNLOAD",
	},
	"DELETE:/api/download/queue": {
		Label:   "Clear Download Queue",
		Section: "DOWNLOAD",
	},
	"GET:/api/download/queue/stream": {
		Label:   "Stream Download Queue",
		Section: "DOWNLOAD",
	},
	"GET:/api/download/queue/item": {
		Label:   "Get Download Queue Items",
		Section: "DOWNLOAD",
	},
	"POST:/api/download/queue/item": {
		Label:   "Add Item to Download Queue",
		Section: "DOWNLOAD",
	},
	"DELETE:/api/download/queue/item": {
		Label:   "Remove Item from Download Queue",
		Section: "DOWNLOAD",
	},
	"POST:/api/download/queue/item/top": {
		Label:   "Move Item to Top of Download Queue",
		Section: "DOWNLOAD",
	},

	// Failed Download Routes
	"GET:/api/download/history": {
		Label:   "Get Failed Downloads",
		Section: "DOWNLOAD",
	},
	"DELETE:/api/download/history": {
		Label:   "Clear Failed Downloads",
		Section: "DOWNLOAD",
	},
	"DELETE:/api/download/history/item": {
		Label:   "Delete Failed Download",
		Section: "DOWNLOAD",
	},
	"POST:/api/download/history/retry": {
		Label:   "Retry Failed Download",
		Section: "DOWNLOAD",
	},

	// Image Routes
	"GET:/api/images/media/item": {
		Label:   "Get Media Item Image",
		Section: "IMAGES",
	},
	"GET:/api/images/media/collection": {
		Label:   "Get Collection Image",
		Section: "IMAGES",
	},
	"GET:/api/images/mediux/item": {
		Label:   "Get Mediux Item Image",
		Section: "IMAGES",
	},
	"GET:/api/images/mediux/avatar": {
		Label:   "Get Mediux Avatar Image",
		Section: "IMAGES",
	},
	"DELETE:/api/images/temp": {
		Label:   "Delete Temp Images",
		Section: "IMAGES",
	},

	// Labels & Tags Route
	"POST:/api/labels-tags": {
		Label:   "Apply Labels/Tags",
		Section: "LABELS/TAGS",
	},

	// Logging Routes
	"GET:/api/logs": {
		Label:   "Get Logs",
		Section: "LOGS",
	},
	"DELETE:/api/logs": {
		Label:   "Clear Logs",
		Section: "LOGS",
	},
	"GET:/api/logs/debug-bundle": {
		Label:   "Download Debug Bundle",
		Section: "LOGS",
	},

	// Statistics Route
	"GET:/api/statistics": {
		Label:   "Get Statistics",
		Section: "STATISTICS",
	},

	// Media Server Routes
	"GET:/api/mediaserver/libraries": {
		Label:   "Get Media Server Libraries",
		Section: "MEDIA SERVER",
	},
	"POST:/api/mediaserver/libraries/options": {
		Label:   "Get Media Server Library Options",
		Section: "MEDIA SERVER",
	},
	"GET:/api/mediaserver/library/items": {
		Label:   "Get Media Server Library Items",
		Section: "MEDIA SERVER",
	},
	"GET:/api/mediaserver/library/item/related": {
		Label:   "Get Related Library Items",
		Section: "MEDIA SERVER",
	},
	"POST:/api/mediaserver/library/refresh": {
		Label:   "Refresh Library",
		Section: "MEDIA SERVER",
	},
	"GET:/api/mediaserver/library/refresh/stream": {
		Label:   "Stream Library Refresh",
		Section: "MEDIA SERVER",
	},
	"GET:/api/mediaserver/item": {
		Label:   "Get Media Server Item Details",
		Section: "MEDIA SERVER",
	},
	"GET:/api/mediaserver/collections": {
		Label:   "Get Movie Collections",
		Section: "MEDIA SERVER",
	},
	"GET:/api/mediaserver/collections/item": {
		Label:   "Get All Collection Children Items",
		Section: "MEDIA SERVER",
	},
	"PATCH:/api/mediaserver/rate": {
		Label:   "Rate Media Item",
		Section: "MEDIA SERVER",
	},
	"POST:/api/mediaserver/refresh": {
		Label:   "Refresh Media Item Metadata",
		Section: "MEDIA SERVER",
	},
	"GET:/api/mediaserver/sync": {
		Label:   "Get Media Server Sync Progress",
		Section: "MEDIA SERVER",
	},
	"POST:/api/mediaserver/sync": {
		Label:   "Sync Saved Sets to a Media Server",
		Section: "MEDIA SERVER",
	},
	"DELETE:/api/mediaserver/sync": {
		Label:   "Stop Media Server Sync",
		Section: "MEDIA SERVER",
	},

	// Plex OAuth Routes
	"GET:/api/oauth/plex": {
		Label:   "Get Plex Pin and ID",
		Section: "MEDIA SERVER",
	},
	"POST:/api/oauth/plex": {
		Label:   "Check Plex Pin",
		Section: "MEDIA SERVER",
	},

	// Silo Plugin Routes
	"GET:/api/plugin/silo/status": {
		Label:   "Get Silo Plugin Status",
		Section: "SILO PLUGIN",
	},
	"GET:/api/plugin/silo/images": {
		Label:   "Get Images for the Silo Plugin",
		Section: "SILO PLUGIN",
	},
	"GET:/api/plugin/silo/download/{token}": {
		Label:   "Download Image for the Silo Plugin",
		Section: "SILO PLUGIN",
	},

	// MediUX Routes
	"GET:/api/mediux/user": {
		Label:   "Get Mediux User Following and Hiding",
		Section: "MEDIUX",
	},
	"GET:/api/mediux/set": {
		Label:   "Get Mediux Set By ID",
		Section: "MEDIUX",
	},
	"GET:/api/mediux/sets/item": {
		Label:   "Get Mediux Item Sets",
		Section: "MEDIUX",
	},
	"GET:/api/mediux/sets/user": {
		Label:   "Get Mediux User Sets",
		Section: "MEDIUX",
	},

	// Validation Routes
	"POST:/api/validate/mediux": {
		Label:   "Validate Mediux Info",
		Section: "VALIDATION",
	},
	"POST:/api/validate/mediaserver": {
		Label:   "Validate Media Server Info",
		Section: "VALIDATION",
	},
	"POST:/api/validate/sonarr": {
		Label:   "Validate Sonarr Info",
		Section: "VALIDATION",
	},
	"POST:/api/validate/radarr": {
		Label:   "Validate Radarr Info",
		Section: "VALIDATION",
	},
	"POST:/api/validate/notifications": {
		Label:   "Send Test Notification",
		Section: "VALIDATION",
	},
}

type GetLogContents_Response struct {
	PossibleActionsPaths map[string]structActionLabelSection `json:"possible_actions_paths"`
	LogEntries           []*logging.LogData                  `json:"log_entries"`
	TotalLogEntries      int                                 `json:"total_log_entries"`
}

// GetLogContents godoc
// @Summary      Get Log Entries
// @Description  Retrieve log entries from the server's log file with optional filtering by log level, status, and route/action. This endpoint allows clients to access and analyze logs for monitoring and debugging purposes.
// @Tags         Logging
// @Produce      json
// @Param        log_levels  query     string  false  "Comma-separated list of log levels to filter by (e.g. info,error,debug)"
// @Param        statuses    query     string  false  "Comma-separated list of statuses to filter by (e.g. success,error)"
// @Param        actions     query     string  false  "Comma-separated list of route paths or action names to filter by (e.g. GET:/api/db,User Login)"
// @Param        items_per_page query   int     false  "Number of log entries to return per page (default: 20)"
// @Param        page_number query     int     false  "Page number to return (default: 1)"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetLogContents_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/logs [get]
func GetLogContents(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Log Contents", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetLogContents_Response

	// Read the log file
	readContentsAction := logAction.AddSubAction("Read Log File", logging.LevelTrace)
	file, err := os.Open(logging.LogFilePath)
	if err != nil {
		readContentsAction.SetError("Failed to open log file", "Make sure the log file exists and is accessible", map[string]any{
			"path":  logging.LogFilePath,
			"error": err.Error()})
		httpx.SendResponse(w, ld, response)
		return
	}
	defer file.Close()
	readContentsAction.Complete()

	// Query Param - Log Level Filter
	filteredLogLevelsStr := r.URL.Query().Get("log_levels")
	var filteredLogLevels []string
	if filteredLogLevelsStr != "" {
		filteredLogLevels = strings.Split(filteredLogLevelsStr, ",")
	}

	// Query Param - Status Filter
	filteredStatusesStr := r.URL.Query().Get("statuses")
	var filteredStatuses []string
	if filteredStatusesStr != "" {
		filteredStatuses = strings.Split(filteredStatusesStr, ",")
	}

	// Query Param - Route/Action Filter
	filteredActionsStr := r.URL.Query().Get("actions")
	var filteredActions []string
	if filteredActionsStr != "" {
		filteredActions = strings.Split(filteredActionsStr, ",")
	}

	// Query Param - Pagination
	itemsPerPage := 20
	pageNumber := 1
	ippStr := r.URL.Query().Get("items_per_page")
	if ippStr != "" {
		if val, err := strconv.Atoi(ippStr); err == nil && val > 0 {
			itemsPerPage = val
		}
	}

	pnStr := r.URL.Query().Get("page_number")
	if pnStr != "" {
		if val, err := strconv.Atoi(pnStr); err == nil && val > 0 {
			pageNumber = val
		}
	}

	var logEntries []*logging.LogData
	parseLogAction := logAction.AddSubAction("Parse Log Entries", logging.LevelTrace)
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			parseLogAction.SetError("Failed to read log file line", err.Error(), nil)
			httpx.SendResponse(w, ld, response)
			return
		}
		var entry logging.LogData
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			// If there is no Route info and No Actions, skip this log entry
			if entry.Route == nil && len(entry.Actions) == 0 {
				continue
			}
			// If there is Route info and Actions are present, skip "Route Not Found" entries
			if entry.Route != nil && entry.Route.Path != "" && len(entry.Actions) != 0 && entry.Actions[0].Name == "Route Not Found" {
				continue
			}

			// Auto-register seen route paths so filters stay in sync with router changes
			if entry.Route != nil && entry.Route.Path != "" {
				path := entry.Route.Path
				key := actionKeyForRoute(entry.Route.Method, path)
				entry.Route.Action = key

				label := key
				if len(entry.Actions) > 0 && entry.Actions[0] != nil && strings.TrimSpace(entry.Actions[0].Name) != "" {
					label = entry.Actions[0].Name
				}

				possibleActionsMutex.Lock()
				if _, exists := possible_actions_paths[key]; !exists {
					possible_actions_paths[key] = structActionLabelSection{
						Label:   label,
						Section: inferRouteSection(path),
					}
				}
				possibleActionsMutex.Unlock()
			}

			if entry.Route == nil && len(entry.Actions) != 0 {
				// If there is no Route info but Actions are present, append Action Name to possible_actions_paths
				actionName := entry.Message
				possibleActionsMutex.Lock()
				if _, exists := possible_actions_paths[actionName]; !exists {
					possible_actions_paths[actionName] = structActionLabelSection{
						Label:   actionName,
						Section: "AURA BACKGROUND TASK",
					}
				}
				possibleActionsMutex.Unlock()
			}

			if entry.Timestamp.Equal((time.Time{})) && entry.Time != (time.Time{}) {
				entry.Timestamp = entry.Time
				entry.Time = time.Time{} // Clear the Time field so it doesn't show up in JSON
			}
			logEntries = append(logEntries, &entry)
		} else {
			continue
		}
	}
	parseLogAction.Complete()

	// Apply Log Level Filter
	if len(filteredLogLevels) > 0 {
		filterLogLevelAction := logAction.AddSubAction("Apply Log Level Filter", logging.LevelTrace)
		filteredEntries := make([]*logging.LogData, 0, len(logEntries))
		for _, entry := range logEntries {
			// If the level is error, always keep the entry
			if strings.EqualFold(entry.Level, "error") {
				filteredEntries = append(filteredEntries, entry)
				continue
			}
			filteredActions := make([]*logging.LogAction, 0, len(entry.Actions))
			for _, action := range entry.Actions {
				filtered := filterLogActionByLevels(action, filteredLogLevels)
				if filtered != nil {
					filteredActions = append(filteredActions, filtered)
				}
			}
			entry.Actions = filteredActions
			if len(entry.Actions) > 0 {
				filteredEntries = append(filteredEntries, entry)
			}
		}
		logEntries = filteredEntries
		filterLogLevelAction.Complete()
	}

	// Apply Status Filter
	if len(filteredStatuses) > 0 {
		filterStatusAction := logAction.AddSubAction("Apply Status Filter", logging.LevelTrace)
		filteredEntries := make([]*logging.LogData, 0, len(logEntries))
		for _, entry := range logEntries {
			// If the status is error, always keep the entry
			if strings.EqualFold(entry.Status, "error") {
				filteredEntries = append(filteredEntries, entry)
				continue
			}
			filteredActions := make([]*logging.LogAction, 0, len(entry.Actions))
			for _, action := range entry.Actions {
				filtered := filterLogActionByStatuses(action, filteredStatuses)
				if filtered != nil {
					filteredActions = append(filteredActions, filtered)
				}
			}
			entry.Actions = filteredActions
			if len(entry.Actions) > 0 {
				filteredEntries = append(filteredEntries, entry)
			}
		}
		logEntries = filteredEntries
		filterStatusAction.Complete()
	}

	// Apply Route/Action Filter
	if len(filteredActions) > 0 {
		filterRouteAction := logAction.AddSubAction("Apply Route/Action Filter", logging.LevelTrace)
		filteredEntries := make([]*logging.LogData, 0, len(logEntries))
		for _, entry := range logEntries {
			entryMatches := false

			if entry.Route != nil && entry.Route.Path != "" {
				key := entry.Route.Action
				for _, actionFilter := range filteredActions {
					f := strings.TrimSpace(actionFilter)
					if strings.EqualFold(key, f) || strings.EqualFold(entry.Route.Path, f) {
						entryMatches = true
						break
					}
				}
			} else if entry.Message != "" {
				// Background Task Name
				for _, actionFilter := range filteredActions {
					if strings.EqualFold(entry.Message, strings.TrimSpace(actionFilter)) {
						entryMatches = true
						break
					}
				}
			}

			if entryMatches {
				filteredEntries = append(filteredEntries, entry)
			}
		}
		logEntries = filteredEntries
		filterRouteAction.Complete()
	}

	// Sort log entries by timestamp descending
	for i, j := 0, len(logEntries)-1; i < j; i, j = i+1, j-1 {
		logEntries[i], logEntries[j] = logEntries[j], logEntries[i]
	}

	// Get the total number of log entries before pagination
	totalNumberOfLogEntries := len(logEntries)

	// Apply Pagination
	startIndex := (pageNumber - 1) * itemsPerPage
	if startIndex >= totalNumberOfLogEntries {
		logEntries = []*logging.LogData{}
	} else {
		endIndex := startIndex + itemsPerPage
		if endIndex > totalNumberOfLogEntries {
			endIndex = totalNumberOfLogEntries
		}
		logEntries = logEntries[startIndex:endIndex]
	}

	pageStart := 0
	pageEnd := 0
	if len(logEntries) > 0 {
		pageStart = startIndex + 1
		pageEnd = startIndex + len(logEntries)
	}

	logging.LOGGER.Debug().Timestamp().Msgf("Retrieved %d-%d of %d log entries after filtering and pagination",
		pageStart, pageEnd, totalNumberOfLogEntries)
	logAction.AppendResult("log_entries_total", totalNumberOfLogEntries)
	logAction.AppendResult("log_entries_returned", len(logEntries))
	logAction.AppendResult("log_entries_filtered", totalNumberOfLogEntries-len(logEntries))

	response.LogEntries = logEntries
	possibleActionsMutex.Lock()
	response.PossibleActionsPaths = maps.Clone(possible_actions_paths)
	possibleActionsMutex.Unlock()
	response.TotalLogEntries = totalNumberOfLogEntries
	httpx.SendResponse(w, ld, response)
}

// Recursively filter sub-actions by log level
func filterLogActionByLevels(action *logging.LogAction, filteredLogLevels []string) *logging.LogAction {
	// If the level is error, always keep it
	if strings.EqualFold(action.Level, "error") {
		return action
	}

	// Filter sub-actions recursively
	filteredSubActions := make([]*logging.LogAction, 0, len(action.SubActions))
	for _, sub := range action.SubActions {
		filtered := filterLogActionByLevels(sub, filteredLogLevels)
		if filtered != nil {
			filteredSubActions = append(filteredSubActions, filtered)
		}
	}
	action.SubActions = filteredSubActions

	// Check if this action matches any log level
	actionMatches := false
	for _, lvl := range filteredLogLevels {
		if strings.EqualFold(action.Level, lvl) {
			actionMatches = true
			break
		}
	}

	// Keep this action if it matches or has any sub-actions left
	if actionMatches || len(action.SubActions) > 0 {
		return action
	}
	return nil
}

// Recursively filter log actions by status
func filterLogActionByStatuses(action *logging.LogAction, filteredStatuses []string) *logging.LogAction {
	// If the status is error, always keep it
	if strings.EqualFold(action.Status, "error") {
		return action
	}

	// Filter sub-actions recursively
	filteredSubActions := make([]*logging.LogAction, 0, len(action.SubActions))
	for _, sub := range action.SubActions {
		filtered := filterLogActionByStatuses(sub, filteredStatuses)
		if filtered != nil {
			filteredSubActions = append(filteredSubActions, filtered)
		}
	}
	action.SubActions = filteredSubActions

	// Check if this action matches any status
	actionMatches := false
	for _, status := range filteredStatuses {
		if strings.EqualFold(action.Status, status) {
			actionMatches = true
			break
		}
	}

	// Keep this action if it matches or has any sub-actions left
	if actionMatches || len(action.SubActions) > 0 {
		return action
	}
	return nil
}

// actionKeyForRoute is the route's key in possible_actions_paths, matching routes with {params} by pattern
func actionKeyForRoute(method, path string) string {
	key := routeActionKey(method, path)
	possibleActionsMutex.Lock()
	defer possibleActionsMutex.Unlock()
	if _, ok := possible_actions_paths[key]; ok {
		return key
	}
	for pattern := range possible_actions_paths {
		if strings.Contains(pattern, "{") && routeMatchesPattern(key, pattern) {
			return pattern
		}
	}
	return key
}

// routeMatchesPattern reports whether a route key matches a key with {params}, like PATCH:/api/config/{section}
func routeMatchesPattern(key, pattern string) bool {
	keyParts, patternParts := strings.Split(key, "/"), strings.Split(pattern, "/")
	if len(keyParts) != len(patternParts) {
		return false
	}
	for i, part := range patternParts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && keyParts[i] != "" {
			continue
		}
		if part != keyParts[i] {
			return false
		}
	}
	return true
}

// HasActionLabel reports whether a registered route has a label on the Logs page
func HasActionLabel(method, route string) bool {
	possibleActionsMutex.Lock()
	defer possibleActionsMutex.Unlock()
	_, ok := possible_actions_paths[routeActionKey(method, route)]
	return ok
}

func inferRouteSection(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/config"):
		return "CONFIG"
	case strings.HasPrefix(path, "/api/validate"):
		return "VALIDATION"
	case strings.HasPrefix(path, "/api/logs"):
		return "LOGS"
	case strings.HasPrefix(path, "/api/mediaserver"), strings.HasPrefix(path, "/api/oauth/plex"):
		return "MEDIA SERVER"
	case strings.HasPrefix(path, "/api/download"):
		return "DOWNLOAD"
	case strings.HasPrefix(path, "/api/images"):
		return "IMAGES"
	case strings.HasPrefix(path, "/api/mediux"):
		return "MEDIUX"
	case strings.HasPrefix(path, "/api/db"):
		return "DATABASE"
	case strings.HasPrefix(path, "/api/labels-tags"):
		return "LABELS/TAGS"
	case strings.HasPrefix(path, "/api/sonarr"):
		return "SONARR/RADARR"
	case strings.HasPrefix(path, "/api/jobs"):
		return "JOBS"
	case strings.HasPrefix(path, "/api/search"):
		return "SEARCH"
	case strings.HasPrefix(path, "/api/login"), strings.HasPrefix(path, "/api/logout"), strings.HasPrefix(path, "/api/auth"):
		return "AUTH"
	case strings.HasPrefix(path, "/api/setup"), strings.HasPrefix(path, "/api/onboarding"):
		return "SETUP"
	case strings.HasPrefix(path, "/api/statistics"):
		return "STATISTICS"
	case strings.HasPrefix(path, "/api/plugin/silo"):
		return "SILO PLUGIN"
	case path == "/api" || path == "/api/" || strings.HasPrefix(path, "/api/health"):
		return "BASE"
	default:
		return "OTHER"
	}
}

func routeActionKey(method, path string) string {
	m := strings.ToUpper(strings.TrimSpace(method))
	// /api/db/ and /api/db are the same route
	p := strings.TrimSpace(path)
	if len(p) > 1 {
		p = strings.TrimSuffix(p, "/")
	}
	if m == "" || p == "" {
		return p
	}
	return m + ":" + p
}
