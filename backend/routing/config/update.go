package routes_config

import (
	"aura/config"
	autodownload "aura/download/auto"
	"aura/jobs"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/models"
	routes_auth "aura/routing/auth"
	sonarr_radarr "aura/sonarr-radarr"
	"aura/utils/httpx"
	"context"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"
)

type updateConfigRequest struct {
	Config config.Config `json:"config"`
}

type updateConfigResponse struct {
	Message string          `json:"message"`
	Status  AppConfigStatus `json:"status"`
}

// UpdateConfig godoc
// @Summary      Update Config
// @Description  Update the application configuration
// @Tags         Config
// @Accept       json
// @Produce      json
// @Param        newConfig  body      updateConfigRequest  true  "New Configuration"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200        {object}  httpx.JSONResponse{data=routes_config.updateConfigResponse}
// @Failure      500        {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/config [post]
func UpdateAppConfig(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Update Config", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req updateConfigRequest
	var response updateConfigResponse

	// Decode the incoming JSON request body
	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "New Config")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	newConfig := req.Config

	// Keep the current User Preferences, as they are saved separately
	response = updateFullConfig(ctx, ld, logAction, newConfig, true)
	httpx.SendResponse(w, ld, response)
}

// updateFullConfig checks, saves and applies a full config from onboarding or an imported file.
// If keepUserPreferences is true, the current User Preferences are kept.
func updateFullConfig(ctx context.Context, ld *logging.LogData, logAction *logging.LogAction, newConfig config.Config, keepUserPreferences bool) (response updateConfigResponse) {
	// Only one config update at a time, so updates can't overwrite each other
	config.UpdateLock.Lock()
	defer config.UpdateLock.Unlock()

	// Keep the current media servers if they weren't sent, as the setup page only has the main one
	if newConfig.MediaServers == nil {
		current := config.Current.Clone()
		newConfig.MediaServers, newConfig.MainMediaServer = current.MediaServers, current.MainMediaServer
	}

	authChanged, authValid := checkConfigDifferences_Auth(ctx, config.Current.Auth, &newConfig.Auth)
	loggingChanged, loggingValid := checkConfigDifferences_Logging(ctx, config.Current.Logging, &newConfig.Logging)
	mediaServerChanged, mediaServerValid, newMediaServerName := checkConfigDifferences_MediaServer(ctx, config.Current.MediaServer, &newConfig.MediaServer)
	mediaServersChanged, mediaServersValid := checkConfigDifferences_MediaServers(ctx, config.Current, &newConfig)
	mediuxChanged, mediuxValid := checkConfigDifferences_Mediux(ctx, config.Current.Mediux, &newConfig.Mediux)
	jobsChanged, jobsChangedKeys, jobsValid := checkConfigDifferences_Jobs(ctx, config.Current.Jobs, &newConfig.Jobs)
	backupsChanged, backupsValid := checkConfigDifferences_Backups(ctx, config.Current.Backups, &newConfig.Backups)
	imagesChanged, imagesValid := checkConfigDifferences_Images(ctx, config.Current.Images, &newConfig.Images, newConfig.MediaServer)
	tmdbChanged, tmdbValid := checkConfigDifferences_TMDB(ctx, config.Current.TMDB, &newConfig.TMDB)
	labelsAndTagsChanged, labelsAndTagsValid := checkConfigDifferences_LabelsAndTags(ctx, config.Current.LabelsAndTags, &newConfig.LabelsAndTags)
	notificationsChanged, notificationsValid := checkConfigDifferences_Notifications(ctx, config.Current.Notifications, &newConfig.Notifications)
	sonarrRadarrChanged, sonarrRadarrValid := checkConfigDifferences_SonarrRadarr(ctx, config.Current.SonarrRadarr, &newConfig.SonarrRadarr, newConfig.MediaServer)

	userPreferencesChanged, userPreferencesValid := false, true
	if keepUserPreferences {
		newConfig.UserPreferences = config.Current.UserPreferences
	} else {
		userPreferencesChanged, userPreferencesValid = checkConfigDifferences_UserPreferences(ctx, config.Current.UserPreferences, &newConfig.UserPreferences)
	}

	if !authValid || !loggingValid || !mediaServerValid || !mediaServersValid || !mediuxValid || !jobsValid || !backupsValid || !imagesValid || !tmdbValid || !labelsAndTagsValid || !notificationsValid || !sonarrRadarrValid || !userPreferencesValid {
		ld.Status = logging.StatusError
		message, help := "Invalid configuration", "The provided configuration is invalid. Check the results for details."
		// Use the first error as the message, if there is one
		if firstErr := firstSubActionError(logAction); firstErr.Message != "" {
			message = fmt.Sprintf("Invalid configuration: %s", firstErr.Message)
			if firstErr.Help != "" {
				help = firstErr.Help
			}
		}
		logAction.SetError(message, help, map[string]any{
			"auth_valid":             authValid,
			"logging_valid":          loggingValid,
			"media_server_valid":     mediaServerValid,
			"media_servers_valid":    mediaServersValid,
			"mediux_valid":           mediuxValid,
			"jobs_valid":             jobsValid,
			"backups_valid":          backupsValid,
			"images_valid":           imagesValid,
			"tmdb_valid":             tmdbValid,
			"labels_and_tags_valid":  labelsAndTagsValid,
			"notifications_valid":    notificationsValid,
			"sonarr_radarr_valid":    sonarrRadarrValid,
			"user_preferences_valid": userPreferencesValid,
		})
		response.Message = "Invalid configuration. Check the results for details."
		return response
	}

	if !authChanged && !loggingChanged && !mediaServerChanged && !mediaServersChanged && !mediuxChanged &&
		!jobsChanged && !backupsChanged && !imagesChanged && !tmdbChanged && !labelsAndTagsChanged &&
		!notificationsChanged && !sonarrRadarrChanged && !userPreferencesChanged {
		// If nothing has changed AND the config is valid, log a warning
		if config.Valid {
			ld.Status = logging.StatusWarn
			response.Message = "No changes detected in configuration"
			logging.LOGGER.Warn().Timestamp().Msg(response.Message)
			return response
		} else if !config.Valid {
			// If nothing has changed AND the config is invalid, re-validate
			newConfig.Validate(ctx)
			if !config.Valid {
				ld.Status = logging.StatusError
				response.Message = "Configuration is still invalid after update attempt"
				logging.LOGGER.Error().Timestamp().Msg(response.Message)
				return response
			} else {
				ld.Status = logging.StatusSuccess
				response.Message = "Configuration has been validated successfully"
				logging.LOGGER.Info().Timestamp().Msg(response.Message)
				return response
			}
		}
	}

	// Save the new config
	saveErr := newConfig.Save(ctx)
	if saveErr.Message != "" {
		logAction.SetErrorFromInfo(saveErr)
		return response
	}

	// Update the global config variable
	config.Current = newConfig
	config.Loaded = true
	config.Valid = true
	config.MediaServerValid = true
	config.MediaServerName = newMediaServerName
	config.MediuxValid = true

	var changedJobs []jobs.JobKey
	if jobsChanged {
		changedJobs = jobsChangedKeys
	}
	applyConfigChanges(ctx, logAction, configChanges{
		authChanged:         authChanged,
		mediaServerChanged:  mediaServerChanged,
		mediaServersChanged: mediaServersChanged,
		changedJobs:         changedJobs,
	})

	response.Message = "Configuration saved"
	response.Status = currentConfigStatus(ctx)
	return response
}

// Loads the libraries of the media servers that aren't the main one. Replaced in tests.
var refreshOtherMediaServers = mediaserver.RefreshOtherMediaServers

// Restarts the Plex event listener. Replaced in tests.
var restartPlexEventListener = autodownload.StartOrRestartPlexWebSocketClient

// applyConfigChanges applies the saved config changes to the running app
func applyConfigChanges(ctx context.Context, logAction *logging.LogAction, changes configChanges) {
	// Restart the jobs whose settings changed
	for _, key := range changes.changedJobs {
		if err := jobs.StartJob(key); err != nil {
			logging.LOGGER.Error().Timestamp().Err(err).Str("job", string(key)).Msg("Failed to restart job after config update")
		}
	}

	if changes.mediaServerChanged {
		restartPlexEventListener()
	}

	// Load the libraries of new or changed media servers
	if changes.mediaServersChanged {
		logging.Go("Refresh Other Media Servers", refreshOtherMediaServers)
	}

	// Restart OIDC with the new Auth settings, so the login page shows the correct login options
	if changes.authChanged {
		oidcCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		oidcErr := routes_auth.InitOIDC(oidcCtx)
		cancel()
		if oidcErr.Message != "" {
			logAction.SetError("Configuration saved, but Single Sign-On (OIDC) could not be started", oidcErr.Help, oidcErr.Detail)
		}
	}
}

// checkConfigDifferences_Auth compares old and new Auth configurations.
func checkConfigDifferences_Auth(ctx context.Context, oldAuth config.Config_Auth, newAuth *config.Config_Auth) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: Auth", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = false

	// APIKeyHash is never sent to the frontend (json:"-") and can only be changed via the
	// dedicated generate/regenerate endpoint - force-preserve it here so a generic config save
	// (which will always carry an empty APIKeyHash from the client) can never wipe it out.
	newAuth.APIKeyHash = oldAuth.APIKeyHash

	if !reflect.DeepEqual(oldAuth, newAuth) {
		if oldAuth.Enabled != newAuth.Enabled {
			logAction.AppendResult("Auth.Enabled changed", fmt.Sprintf("from '%v' to '%v'", oldAuth.Enabled, newAuth.Enabled))
			logging.LOGGER.Info().
				Timestamp().
				Bool("old_enabled", oldAuth.Enabled).
				Bool("new_enabled", newAuth.Enabled).
				Msg("Auth.Enabled changed")
			changed = true
		}

		// The password hash is masked on the way out - keep the old value unless a new one was entered
		if oldAuth.Password != newAuth.Password && strings.HasPrefix(newAuth.Password, "***") {
			newAuth.Password = oldAuth.Password
		}
		if oldAuth.Password != newAuth.Password {
			logAction.AppendResult("Auth.Password changed", "value redacted")
			logging.LOGGER.Info().
				Timestamp().
				Msg("Auth.Password changed")
			changed = true
		}

		if oldAuth.SessionCookieSecure != newAuth.SessionCookieSecure || oldAuth.TrustProxyForCookieSecure != newAuth.TrustProxyForCookieSecure {
			changed = true
		}

		if !reflect.DeepEqual(oldAuth.AllowedOrigins, newAuth.AllowedOrigins) {
			changed = true
		}

		if oldAuth.PasswordEnabled != newAuth.PasswordEnabled {
			logAction.AppendResult("Auth.PasswordEnabled changed", fmt.Sprintf("from '%v' to '%v'", oldAuth.PasswordEnabled, newAuth.PasswordEnabled))
			changed = true
		}

		if oldAuth.OIDC.Enabled != newAuth.OIDC.Enabled {
			logAction.AppendResult("Auth.OIDC.Enabled changed", fmt.Sprintf("from '%v' to '%v'", oldAuth.OIDC.Enabled, newAuth.OIDC.Enabled))
			changed = true
		}

		if oldAuth.OIDC.IssuerURL != newAuth.OIDC.IssuerURL ||
			oldAuth.OIDC.ClientID != newAuth.OIDC.ClientID ||
			!reflect.DeepEqual(oldAuth.OIDC.RedirectURLs, newAuth.OIDC.RedirectURLs) ||
			!reflect.DeepEqual(oldAuth.OIDC.AllowedEmails, newAuth.OIDC.AllowedEmails) ||
			!reflect.DeepEqual(oldAuth.OIDC.AllowedDomains, newAuth.OIDC.AllowedDomains) {
			changed = true
		}

		// ClientSecret is masked on the way out (MaskToken, "***" prefix) - preserve the old
		// value unless the admin actually typed a new one, matching the pattern used for every
		// other masked secret field in this file (see checkConfigDifferences_TMDB etc).
		if oldAuth.OIDC.ClientSecret != newAuth.OIDC.ClientSecret {
			if !strings.HasPrefix(newAuth.OIDC.ClientSecret, "***") {
				logAction.AppendResult("Auth.OIDC.ClientSecret changed", "value redacted")
				changed = true
			} else {
				newAuth.OIDC.ClientSecret = oldAuth.OIDC.ClientSecret
			}
		}
	}

	newValid = config.ValidateAuth(ctx, newAuth)
	// Catch any other changes not checked above
	if !changed && !reflect.DeepEqual(oldAuth, *newAuth) {
		changed = true
	}
	return changed, newValid
}

// checkConfigDifferences_Logging compares old and new Logging configurations.
func checkConfigDifferences_Logging(ctx context.Context, oldLogging config.Config_Logging, newLogging *config.Config_Logging) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: Logging", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = false
	if !reflect.DeepEqual(oldLogging, newLogging) {
		if oldLogging.Level != newLogging.Level {
			logAction.AppendResult("Logging.Level changed", fmt.Sprintf("from '%s' to '%s'", oldLogging.Level, newLogging.Level))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_level", oldLogging.Level).
				Str("new_level", newLogging.Level).
				Msg("Logging.Level changed")
			changed = true
		}

		if oldLogging.KeepDays != newLogging.KeepDays {
			logAction.AppendResult("Logging.KeepDays changed", fmt.Sprintf("from '%d' to '%d'", oldLogging.KeepDays, newLogging.KeepDays))
			logging.LOGGER.Info().
				Timestamp().
				Int("old_keep_days", oldLogging.KeepDays).
				Int("new_keep_days", newLogging.KeepDays).
				Msg("Logging.KeepDays changed")
			changed = true
		}
	}

	newValid = config.ValidateLogging(ctx, newLogging)
	// Catch any other changes not checked above
	if !changed && !reflect.DeepEqual(oldLogging, *newLogging) {
		changed = true
	}
	return changed, newValid
}

// checkConfigDifferences_MediaServer compares old and new MediaServer configurations.
func checkConfigDifferences_MediaServer(ctx context.Context, oldMediaServer config.Config_MediaServer, newMediaServer *config.Config_MediaServer) (changed, newValid bool, serverName string) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: MediaServer", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = false
	serverName = ""

	// The User ID is not saved or exported, so keep the current one if it's missing
	if newMediaServer.UserID == "" {
		newMediaServer.UserID = oldMediaServer.UserID
	}

	if !reflect.DeepEqual(oldMediaServer, newMediaServer) {
		if oldMediaServer.Type != newMediaServer.Type {
			logAction.AppendResult("MediaServer.Type changed", fmt.Sprintf("from '%s' to '%s'", oldMediaServer.Type, newMediaServer.Type))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_type", oldMediaServer.Type).
				Str("new_type", newMediaServer.Type).
				Msg("MediaServer.Type changed")
			changed = true
		}

		if oldMediaServer.URL != newMediaServer.URL {
			logAction.AppendResult("MediaServer.URL changed", fmt.Sprintf("from '%s' to '%s'", oldMediaServer.URL, newMediaServer.URL))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_url", oldMediaServer.URL).
				Str("new_url", newMediaServer.URL).
				Msg("MediaServer.URL changed")
			changed = true
		}

		if oldMediaServer.ApiToken != newMediaServer.ApiToken {
			if !strings.HasPrefix(newMediaServer.ApiToken, "***") {
				logAction.AppendResult("MediaServer.ApiToken changed", "value redacted")
				logging.LOGGER.Info().
					Timestamp().
					Msg("MediaServer.ApiToken changed")
				changed = true
			} else {
				newMediaServer.ApiToken = oldMediaServer.ApiToken
			}
		}

		if !reflect.DeepEqual(oldMediaServer.Libraries, newMediaServer.Libraries) {
			oldNames := libraryNames(oldMediaServer.Libraries)
			newNames := libraryNames(newMediaServer.Libraries)
			logAction.AppendResult("MediaServer.Libraries changed", fmt.Sprintf("from '%s' to '%s'", oldNames, newNames))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_libraries", oldNames).
				Str("new_libraries", newNames).
				Msg("MediaServer.Libraries changed")
			changed = true
		}

		if oldMediaServer.UserID != newMediaServer.UserID {
			logAction.AppendResult("MediaServer.UserID changed", fmt.Sprintf("from '%v' to '%v'", oldMediaServer.UserID, newMediaServer.UserID))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_user_id", fmt.Sprintf("%v", oldMediaServer.UserID)).
				Str("new_user_id", fmt.Sprintf("%v", newMediaServer.UserID)).
				Msg("MediaServer.UserID changed")
			changed = true
		}

		if oldMediaServer.EnablePlexEventListener != newMediaServer.EnablePlexEventListener {
			logAction.AppendResult("MediaServer.EnablePlexEventListener changed", fmt.Sprintf("from '%v' to '%v'", oldMediaServer.EnablePlexEventListener, newMediaServer.EnablePlexEventListener))
			logging.LOGGER.Info().
				Timestamp().
				Bool("old_enabled", oldMediaServer.EnablePlexEventListener).
				Bool("new_enabled", newMediaServer.EnablePlexEventListener).
				Msg("MediaServer.PlexEventListener.Enabled changed")
			changed = true
		}
	}
	newValid = config.ValidateMediaServer(ctx, newMediaServer)
	// If the Media Server config doesn't pass validation, return early
	if !newValid {
		return changed, newValid, serverName
	}

	// Check to see if we can connect to the Media Server with the new config
	connectionOk, serverName, _, msErr := mediaserver.TestConnection(ctx, newMediaServer)
	if msErr.Message != "" || !connectionOk {
		newValid = false
		return changed, newValid, serverName
	}

	// Get the User ID from the media server when the connection settings change, or if there isn't one
	if newMediaServer.Type == "Emby" || newMediaServer.Type == "Jellyfin" || newMediaServer.Type == "Silo" {
		connectionChanged := oldMediaServer.Type != newMediaServer.Type ||
			oldMediaServer.URL != newMediaServer.URL ||
			oldMediaServer.ApiToken != newMediaServer.ApiToken
		if connectionChanged || newMediaServer.UserID == "" {
			userID, userErr := mediaserver.GetAdminUser(ctx, newMediaServer)
			if userErr.Message != "" || userID == "" {
				logAction.SetError(fmt.Sprintf("Failed to get the admin user from the %s server", newMediaServer.Type),
					"Make sure the token belongs to an admin account", map[string]any{"error": userErr.Message})
				newValid = false
				return changed, newValid, serverName
			}
			if userID != oldMediaServer.UserID {
				logAction.AppendResult("MediaServer.UserID changed", fmt.Sprintf("from '%v' to '%v'", oldMediaServer.UserID, userID))
				changed = true
			}
			newMediaServer.UserID = userID
		}
	}

	// Catch any other changes not checked above
	if !changed && !reflect.DeepEqual(oldMediaServer, *newMediaServer) {
		changed = true
	}
	return changed, newValid, serverName
}

// checkConfigDifferences_Mediux compares old and new MediUX configurations.
// checkConfigDifferences_MediaServers compares old and new MediaServers configurations.
// The main media server is checked in checkConfigDifferences_MediaServer, which runs first.
func checkConfigDifferences_MediaServers(ctx context.Context, oldConfig config.Config, newConfig *config.Config) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: MediaServers", logging.LevelTrace)
	defer logAction.Complete()

	keepMediaServerIDs(oldConfig.MediaServers, newConfig)

	// API tokens are masked on the way out - keep the old value unless a new one was entered
	config.RestoreMediaServerTokens(oldConfig.MediaServers, newConfig.MediaServers)

	// Copy the checked main media server into the list
	newConfig.SyncMediaServers()

	if !config.ValidateMediaServers(ctx, newConfig.MediaServers, newConfig.MainMediaServer) {
		return false, false
	}

	// Check the connection of the other media servers that are new or changed
	oldServers := map[string]config.Config_MediaServer{}
	for _, server := range oldConfig.MediaServers {
		oldServers[server.ID] = server
	}
	newValid = true
	for i := range newConfig.MediaServers {
		server := &newConfig.MediaServers[i]
		if server.ID == newConfig.MainMediaServer || !server.Enabled {
			continue
		}
		oldServer, found := oldServers[server.ID]
		var old *config.Config_MediaServer
		if found {
			old = &oldServer
		}
		if !checkMediaServerConnection(ctx, old, server) {
			newValid = false
		}
	}
	if !newValid {
		return false, false
	}

	if oldConfig.MainMediaServer != newConfig.MainMediaServer {
		logAction.AppendResult("MainMediaServer changed", fmt.Sprintf("from '%s' to '%s'", oldConfig.MainMediaServer, newConfig.MainMediaServer))
		logging.LOGGER.Info().
			Timestamp().
			Str("old_main_media_server", oldConfig.MainMediaServer).
			Str("new_main_media_server", newConfig.MainMediaServer).
			Msg("MainMediaServer changed")
		changed = true
	}
	if !reflect.DeepEqual(storedMediaServers(oldConfig.MediaServers), storedMediaServers(newConfig.MediaServers)) {
		logAction.AppendResult("MediaServers changed", fmt.Sprintf("%d media servers", len(newConfig.MediaServers)))
		logging.LOGGER.Info().
			Timestamp().
			Int("media_servers", len(newConfig.MediaServers)).
			Msg("MediaServers changed")
		changed = true
	}
	return changed, true
}

// checkMediaServerConnection checks the connection of a media server that isn't the main one, if it is new or its connection settings changed.
// It also gets the User ID from Emby, Jellyfin and Silo servers.
func checkMediaServerConnection(ctx context.Context, oldServer *config.Config_MediaServer, server *config.Config_MediaServer) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Check Media Server Connection: %s", server.Name), logging.LevelTrace)
	defer logAction.Complete()

	// The User ID comes from the media server, not the request
	connectionChanged := oldServer == nil || oldServer.Type != server.Type || oldServer.URL != server.URL || oldServer.ApiToken != server.ApiToken
	server.UserID = ""
	if !connectionChanged {
		server.UserID = oldServer.UserID
		if server.UserID != "" || server.Type == "Plex" {
			return true
		}
	}

	if connectionChanged {
		connectionOk, _, _, Err := mediaserver.TestConnection(ctx, server)
		if Err.Message != "" || !connectionOk {
			logAction.SetError(fmt.Sprintf("Could not connect to media server '%s'", server.Name),
				"Check its URL and API token", map[string]any{"error": Err.Message})
			return false
		}
	}

	if server.Type == "Emby" || server.Type == "Jellyfin" || server.Type == "Silo" {
		userID, Err := mediaserver.GetAdminUser(ctx, server)
		if Err.Message != "" || userID == "" {
			logAction.SetError(fmt.Sprintf("Failed to get the admin user from media server '%s'", server.Name),
				"Make sure the token belongs to an admin account", map[string]any{"error": Err.Message})
			return false
		}
		server.UserID = userID
	}
	return true
}

// keepMediaServerIDs gives a new media server the ID of the current one with the same type and URL.
// Config files from before multiple media servers have no IDs, so importing one keeps the current IDs.
func keepMediaServerIDs(current []config.Config_MediaServer, newConfig *config.Config) {
	currentIDs := map[string]bool{}
	for _, server := range current {
		currentIDs[server.ID] = true
	}
	usedIDs := map[string]bool{}
	for _, server := range newConfig.MediaServers {
		usedIDs[server.ID] = true
	}
	for i := range newConfig.MediaServers {
		server := &newConfig.MediaServers[i]
		if currentIDs[server.ID] {
			continue
		}
		for _, currentServer := range current {
			if usedIDs[currentServer.ID] || currentServer.Type != server.Type ||
				strings.TrimSuffix(currentServer.URL, "/") != strings.TrimSuffix(server.URL, "/") {
				continue
			}
			if newConfig.MainMediaServer == server.ID {
				newConfig.MainMediaServer = currentServer.ID
			}
			if newConfig.MediaServer.ID == server.ID {
				newConfig.MediaServer.ID = currentServer.ID
			}
			if server.LibraryMap == nil {
				server.LibraryMap = currentServer.LibraryMap
			}
			usedIDs[currentServer.ID] = true
			server.ID = currentServer.ID
			break
		}
	}
}

// storedMediaServers returns the media servers as they are saved, without the User IDs
func storedMediaServers(servers []config.Config_MediaServer) []config.Config_MediaServer {
	if len(servers) == 0 {
		return nil
	}
	stored := make([]config.Config_MediaServer, len(servers))
	for i, server := range servers {
		server.UserID = ""
		stored[i] = server
	}
	return stored
}

func checkConfigDifferences_Mediux(ctx context.Context, oldMediux config.Config_Mediux, newMediux *config.Config_Mediux) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: MediUX", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = false
	if !reflect.DeepEqual(oldMediux, newMediux) {
		if oldMediux.ApiToken != newMediux.ApiToken {
			if !strings.HasPrefix(newMediux.ApiToken, "***") {
				logAction.AppendResult("Mediux.ApiToken changed", "value redacted")
				logging.LOGGER.Info().
					Timestamp().
					Msg("Mediux.ApiToken changed")
				changed = true
			} else {
				newMediux.ApiToken = oldMediux.ApiToken
			}
		}

		if oldMediux.DownloadQuality != newMediux.DownloadQuality {
			logAction.AppendResult("Mediux.DownloadQuality changed", fmt.Sprintf("from '%v' to '%v'", oldMediux.DownloadQuality, newMediux.DownloadQuality))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_download_quality", fmt.Sprintf("%v", oldMediux.DownloadQuality)).
				Str("new_download_quality", fmt.Sprintf("%v", newMediux.DownloadQuality)).
				Msg("Mediux.DownloadQuality changed")
			changed = true
		}

		if oldMediux.SimultaneousDownloads != newMediux.SimultaneousDownloads {
			logAction.AppendResult("Mediux.SimultaneousDownloads changed", fmt.Sprintf("from '%v' to '%v'", oldMediux.SimultaneousDownloads, newMediux.SimultaneousDownloads))
			logging.LOGGER.Info().
				Timestamp().
				Int("old_simultaneous_downloads", oldMediux.SimultaneousDownloads).
				Int("new_simultaneous_downloads", newMediux.SimultaneousDownloads).
				Msg("Mediux.SimultaneousDownloads changed")
			changed = true
		}
	}
	newValid = config.ValidateMediux(ctx, newMediux)
	// If the MediUX config doesn't pass validation, return early
	if !newValid {
		return changed, newValid
	}

	// Check to see if we can validate the MediUX token with the new config
	mediuxTokenValid, mediuxErr := mediux.ValidateToken(ctx, newMediux.ApiToken)
	if mediuxErr.Message != "" || !mediuxTokenValid {
		newValid = false
	}

	// Catch any other changes not checked above
	if !changed && !reflect.DeepEqual(oldMediux, *newMediux) {
		changed = true
	}
	return changed, newValid
}

// checkConfigDifferences_Jobs compares old and new Jobs configurations on a
// per-job basis, so that only the jobs whose settings actually changed get restarted.
func checkConfigDifferences_Jobs(ctx context.Context, oldJobs config.Config_Jobs, newJobs *config.Config_Jobs) (changed bool, changedKeys []jobs.JobKey, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: Jobs", logging.LevelTrace)
	defer logAction.Complete()

	type jobField struct {
		key   jobs.JobKey
		label string
		old   config.Config_JobSetting
		new   *config.Config_JobSetting
		def   config.JobSettingDefault
	}

	fields := []jobField{
		{jobs.JobKeyAutoDownload, "AutoDownload", oldJobs.AutoDownload, &newJobs.AutoDownload, config.JobDefaults.AutoDownload},
		{jobs.JobKeyRefreshMediaItemsAndCollections, "RefreshMediaItemsAndCollections", oldJobs.RefreshMediaItemsAndCollections, &newJobs.RefreshMediaItemsAndCollections, config.JobDefaults.RefreshMediaItemsAndCollections},
		{jobs.JobKeyCheckForMediaItemChanges, "CheckForMediaItemChanges", oldJobs.CheckForMediaItemChanges, &newJobs.CheckForMediaItemChanges, config.JobDefaults.CheckForMediaItemChanges},
		{jobs.JobKeyCheckForNewMediaItems, "CheckForNewMediaItems", oldJobs.CheckForNewMediaItems, &newJobs.CheckForNewMediaItems, config.JobDefaults.CheckForNewMediaItems},
		{jobs.JobKeyHandleTempIgnoredItems, "HandleTempIgnoredItems", oldJobs.HandleTempIgnoredItems, &newJobs.HandleTempIgnoredItems, config.JobDefaults.HandleTempIgnoredItems},
		{jobs.JobKeyRefreshMediuxUsers, "RefreshMediuxUsers", oldJobs.RefreshMediuxUsers, &newJobs.RefreshMediuxUsers, config.JobDefaults.RefreshMediuxUsers},
		{jobs.JobKeyCheckMediuxSiteLink, "CheckMediuxSiteLink", oldJobs.CheckMediuxSiteLink, &newJobs.CheckMediuxSiteLink, config.JobDefaults.CheckMediuxSiteLink},
		{jobs.JobKeyBackupDatabase, "BackupDatabase", oldJobs.BackupDatabase, &newJobs.BackupDatabase, config.JobDefaults.BackupDatabase},
		{jobs.JobKeyUpgradeSavedSets, "UpgradeSavedSets", oldJobs.UpgradeSavedSets, &newJobs.UpgradeSavedSets, config.JobDefaults.UpgradeSavedSets},
	}

	for _, f := range fields {
		if reflect.DeepEqual(f.old, *f.new) {
			continue
		}
		oldEnabled, oldCron := f.old.Resolve(f.def)
		newEnabled, newCron := f.new.Resolve(f.def)
		logAction.AppendResult(fmt.Sprintf("Jobs.%s changed", f.label), fmt.Sprintf("from %+v to %+v", f.old, *f.new))
		logging.LOGGER.Info().
			Timestamp().
			Str("job", f.label).
			Bool("old_enabled", oldEnabled).
			Bool("new_enabled", newEnabled).
			Str("old_cron", oldCron).
			Str("new_cron", newCron).
			Msg("Job config changed")
		changed = true
		changedKeys = append(changedKeys, f.key)
	}

	newValid = config.ValidateJobs(ctx, newJobs)
	return changed, changedKeys, newValid
}

// checkConfigDifferences_Backups compares old and new Backups configurations.
func checkConfigDifferences_Backups(ctx context.Context, oldBackups config.Config_Backups, newBackups *config.Config_Backups) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: Backups", logging.LevelTrace)
	defer logAction.Complete()

	newValid = config.ValidateBackups(ctx, newBackups)
	if oldBackups.Keep != newBackups.Keep {
		logAction.AppendResult("Backups.Keep changed", fmt.Sprintf("from '%d' to '%d'", oldBackups.Keep, newBackups.Keep))
		logging.LOGGER.Info().
			Timestamp().
			Int("old_keep", oldBackups.Keep).
			Int("new_keep", newBackups.Keep).
			Msg("Backups.Keep changed")
		changed = true
	}
	return changed, newValid
}

// checkConfigDifferences_Images compares old and new Images configurations.
func checkConfigDifferences_Images(ctx context.Context, oldImages config.Config_Images, newImages *config.Config_Images, msConfig config.Config_MediaServer) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: Images", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = true
	if !reflect.DeepEqual(oldImages, newImages) {
		if oldImages.CacheImages.Enabled != newImages.CacheImages.Enabled {
			logAction.AppendResult("Images.CacheImages.Enabled changed", fmt.Sprintf("from '%v' to '%v'", oldImages.CacheImages.Enabled, newImages.CacheImages.Enabled))
			logging.LOGGER.Info().
				Timestamp().
				Bool("old_enabled", oldImages.CacheImages.Enabled).
				Bool("new_enabled", newImages.CacheImages.Enabled).
				Msg("Images.CacheImages.Enabled changed")
			changed = true
		}

		if oldImages.SaveImagesLocally.Enabled != newImages.SaveImagesLocally.Enabled {
			logAction.AppendResult("Images.SaveImagesLocally.Enabled changed", fmt.Sprintf("from '%v' to '%v'", oldImages.SaveImagesLocally.Enabled, newImages.SaveImagesLocally.Enabled))
			logging.LOGGER.Info().
				Timestamp().
				Bool("old_enabled", oldImages.SaveImagesLocally.Enabled).
				Bool("new_enabled", newImages.SaveImagesLocally.Enabled).
				Msg("Images.SaveImagesLocally.Enabled changed")
			changed = true
		}

		if oldImages.SaveImagesLocally.Path != newImages.SaveImagesLocally.Path {
			logAction.AppendResult("Images.SaveImagesLocally.Path changed", fmt.Sprintf("from '%s' to '%s'", oldImages.SaveImagesLocally.Path, newImages.SaveImagesLocally.Path))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_path", oldImages.SaveImagesLocally.Path).
				Str("new_path", newImages.SaveImagesLocally.Path).
				Msg("Images.SaveImagesLocally.Path changed")
			changed = true
		}

	}
	newValid = config.ValidateImages(ctx, newImages, msConfig)
	// Catch any other changes not checked above
	if !changed && !reflect.DeepEqual(oldImages, *newImages) {
		changed = true
	}
	return changed, newValid
}

// checkConfigDifferences_TMDB compares old and new TMDB configurations.
func checkConfigDifferences_TMDB(ctx context.Context, oldTMDB config.Config_TMDB, newTMDB *config.Config_TMDB) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: TMDB", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = true
	if !reflect.DeepEqual(oldTMDB, newTMDB) {
		if oldTMDB.ApiToken != newTMDB.ApiToken {
			if !strings.HasPrefix(newTMDB.ApiToken, "***") {
				logAction.AppendResult("TMDB.ApiToken changed", "value redacted")
				logging.LOGGER.Info().
					Timestamp().
					Msg("TMDB.ApiToken changed")
				changed = true
			} else {
				newTMDB.ApiToken = oldTMDB.ApiToken
			}
		}
	}
	// Catch any other changes not checked above
	if !changed && !reflect.DeepEqual(oldTMDB, *newTMDB) {
		changed = true
	}
	return changed, newValid
}

// checkConfigDifferences_LabelsAndTags compares old and new LabelsAndTags configurations.
func checkConfigDifferences_LabelsAndTags(ctx context.Context, oldLAT config.Config_LabelsAndTags, newLAT *config.Config_LabelsAndTags) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: LabelsAndTags", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = true
	if !reflect.DeepEqual(oldLAT, newLAT) {

		// Compare RemoveOverlayLabelOnlyOnPosterDownload
		if oldLAT.RemoveOverlayLabelOnlyOnPosterDownload != newLAT.RemoveOverlayLabelOnlyOnPosterDownload {
			logAction.AppendResult("LabelsAndTags.RemoveOverlayLabelOnlyOnPosterDownload changed", fmt.Sprintf("from '%v' to '%v'", oldLAT.RemoveOverlayLabelOnlyOnPosterDownload, newLAT.RemoveOverlayLabelOnlyOnPosterDownload))
			logging.LOGGER.Info().
				Timestamp().
				Bool("old_value", oldLAT.RemoveOverlayLabelOnlyOnPosterDownload).
				Bool("new_value", newLAT.RemoveOverlayLabelOnlyOnPosterDownload).
				Msg("LabelsAndTags.RemoveOverlayLabelOnlyOnPosterDownload changed")
			changed = true
		}

		// Applications diff
		oldMap := applicationMapLabelsAndTags(oldLAT.Applications)
		newMap := applicationMapLabelsAndTags(newLAT.Applications)

		// Added / removed apps
		var added, removed []string
		for k := range oldMap {
			if _, ok := newMap[k]; !ok {
				removed = append(removed, k)
			}
		}
		for k := range newMap {
			if _, ok := oldMap[k]; !ok {
				added = append(added, k)
			}
		}
		if len(added) > 0 {
			sort.Strings(added)
			logAction.AppendResult("LabelsAndTags.Applications added", fmt.Sprintf("%s", joinNonEmptyComma(added)))
			logging.LOGGER.Info().
				Timestamp().
				Str("added_apps", fmt.Sprintf("%s", joinNonEmptyComma(added))).
				Msg("LabelsAndTags.Applications added")
			changed = true
		}
		if len(removed) > 0 {
			sort.Strings(removed)
			logAction.AppendResult("LabelsAndTags.Applications removed", fmt.Sprintf("%s", joinNonEmptyComma(removed)))
			logging.LOGGER.Info().
				Timestamp().
				Str("removed_apps", fmt.Sprintf("%s", joinNonEmptyComma(removed))).
				Msg("LabelsAndTags.Applications removed")
			changed = true
		}

		// Compare applications present in both
		for name, oldApp := range oldMap {
			newApp, ok := newMap[name]
			if !ok {
				continue
			}

			// Per-application enabled
			if oldApp.Enabled != newApp.Enabled {
				logAction.AppendResult("LabelsAndTags.Application.Enabled changed", fmt.Sprintf("from '%v' to '%v'", oldApp.Enabled, newApp.Enabled))
				logging.LOGGER.Info().
					Timestamp().
					Str("application", name).
					Bool("old_enabled", oldApp.Enabled).
					Bool("new_enabled", newApp.Enabled).
					Msg("LabelsAndTags.Application.Enabled changed")
				changed = true
			}

			// Add Labels/Tags for Selected Types diff
			if oldApp.AddLabelTagForSelectedTypes != newApp.AddLabelTagForSelectedTypes {
				logAction.AppendResult("LabelsAndTags.Application.AddLabelTagForSelectedTypes changed", fmt.Sprintf("from '%v' to '%v'", oldApp.AddLabelTagForSelectedTypes, newApp.AddLabelTagForSelectedTypes))
				logging.LOGGER.Info().
					Timestamp().
					Str("application", name).
					Bool("old_value", oldApp.AddLabelTagForSelectedTypes).
					Bool("new_value", newApp.AddLabelTagForSelectedTypes).
					Msg("LabelsAndTags.Application.AddLabelTagForSelectedTypes changed")
				changed = true
			}

			// Add list diff
			var addAdded, addRemoved []string
			oldAddMap := make(map[string]bool)
			for _, v := range oldApp.Add {
				oldAddMap[v] = true
			}
			newAddMap := make(map[string]bool)
			for _, v := range newApp.Add {
				newAddMap[v] = true
			}
			for k := range oldAddMap {
				if _, ok := newAddMap[k]; !ok {
					addRemoved = append(addRemoved, k)
				}
			}
			for k := range newAddMap {
				if _, ok := oldAddMap[k]; !ok {
					addAdded = append(addAdded, k)
				}
			}
			if len(addAdded) > 0 {
				sort.Strings(addAdded)
				logAction.AppendResult("LabelsAndTags.Application.Add added", fmt.Sprintf("%s", joinNonEmptyComma(addAdded)))
				logging.LOGGER.Info().
					Timestamp().
					Str("application", name).
					Str("added_labels", fmt.Sprintf("%s", joinNonEmptyComma(addAdded))).
					Msg("LabelsAndTags.Application.Add added")
				changed = true
			}
			if len(addRemoved) > 0 {
				sort.Strings(addRemoved)
				logAction.AppendResult("LabelsAndTags.Application.Add removed", fmt.Sprintf("%s", joinNonEmptyComma(addRemoved)))
				logging.LOGGER.Info().
					Timestamp().
					Str("application", name).
					Str("removed_labels", fmt.Sprintf("%s", joinNonEmptyComma(addRemoved))).
					Msg("LabelsAndTags.Application.Add removed")
				changed = true
			}

			// Remove list diff
			var removeAdded, removeRemoved []string
			oldRemoveMap := make(map[string]bool)
			for _, v := range oldApp.Remove {
				oldRemoveMap[v] = true
			}
			newRemoveMap := make(map[string]bool)
			for _, v := range newApp.Remove {
				newRemoveMap[v] = true
			}
			for k := range oldRemoveMap {
				if _, ok := newRemoveMap[k]; !ok {
					removeRemoved = append(removeRemoved, k)
				}
			}
			for k := range newRemoveMap {
				if _, ok := oldRemoveMap[k]; !ok {
					removeAdded = append(removeAdded, k)
				}
			}
			if len(removeAdded) > 0 {
				sort.Strings(removeAdded)
				logAction.AppendResult("LabelsAndTags.Application.Remove added", fmt.Sprintf("%s", joinNonEmptyComma(removeAdded)))
				logging.LOGGER.Info().
					Timestamp().
					Str("application", name).
					Str("added_labels", fmt.Sprintf("%s", joinNonEmptyComma(removeAdded))).
					Msg("LabelsAndTags.Application.Remove added")
				changed = true
			}
			if len(removeRemoved) > 0 {
				sort.Strings(removeRemoved)
				logAction.AppendResult("LabelsAndTags.Application.Remove removed", fmt.Sprintf("%s", joinNonEmptyComma(removeRemoved)))
				logging.LOGGER.Info().
					Timestamp().
					Str("application", name).
					Str("removed_labels", fmt.Sprintf("%s", joinNonEmptyComma(removeRemoved))).
					Msg("LabelsAndTags.Application.Remove removed")
				changed = true
			}
		}
	}
	// Catch any other changes not checked above
	if !changed && !reflect.DeepEqual(oldLAT, *newLAT) {
		changed = true
	}
	return changed, newValid
}

// checkConfigDifferences_Notifications compares old and new Notifications configurations.
func checkConfigDifferences_Notifications(ctx context.Context, oldNotifications config.Config_Notifications, newNotifications *config.Config_Notifications) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: Notifications", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = false
	if !reflect.DeepEqual(oldNotifications, newNotifications) {
		// Global toggle
		if oldNotifications.Enabled != newNotifications.Enabled {
			logAction.AppendResult("Notifications.Enabled changed", fmt.Sprintf("from '%v' to '%v'", oldNotifications.Enabled, newNotifications.Enabled))
			logging.LOGGER.Info().
				Timestamp().
				Bool("old_enabled", oldNotifications.Enabled).
				Bool("new_enabled", newNotifications.Enabled).
				Msg("Notifications.Enabled changed")
			changed = true
		}

		// Providers diff
		oldMap := providerMapNotifications(oldNotifications.Providers)
		newMap := providerMapNotifications(newNotifications.Providers)

		// Added / removed types
		var added, removed []string
		for k := range oldMap {
			if _, ok := newMap[k]; !ok {
				removed = append(removed, k)
			}
		}
		for k := range newMap {
			if _, ok := oldMap[k]; !ok {
				added = append(added, k)
			}
		}
		if len(added) > 0 {
			sort.Strings(added)
			logAction.AppendResult("Notifications.Providers added", fmt.Sprintf("%s", joinNonEmptyComma(added)))
			logging.LOGGER.Info().
				Timestamp().
				Str("added_providers", fmt.Sprintf("%s", joinNonEmptyComma(added))).
				Msg("Notifications.Providers added")
			changed = true
		}
		if len(removed) > 0 {
			sort.Strings(removed)
			logAction.AppendResult("Notifications.Providers removed", fmt.Sprintf("%s", joinNonEmptyComma(removed)))
			logging.LOGGER.Info().
				Timestamp().
				Str("removed_providers", fmt.Sprintf("%s", joinNonEmptyComma(removed))).
				Msg("Notifications.Providers removed")
			changed = true
		}

		// Compare providers present in both
		for name, oldProv := range oldMap {
			newProv, ok := newMap[name]
			if !ok {
				continue
			}

			// Per-provider enabled
			if oldProv.Enabled != newProv.Enabled {
				logAction.AppendResult("Notifications.Provider.Enabled changed", fmt.Sprintf("from '%v' to '%v'", oldProv.Enabled, newProv.Enabled))
				logging.LOGGER.Info().
					Timestamp().
					Str("provider", name).
					Bool("old_enabled", oldProv.Enabled).
					Bool("new_enabled", newProv.Enabled).
					Msg("Notifications.Provider.Enabled changed")
				changed = true
			}

			switch name {
			case "Discord":
				var oldWebhook, newWebhook string
				if oldProv.Discord != nil {
					oldWebhook = strings.TrimSpace(oldProv.Discord.Webhook)
				}
				if newProv.Discord != nil {
					newWebhook = strings.TrimSpace(newProv.Discord.Webhook)
				}
				if oldWebhook != newWebhook {
					if !config.IsMaskedWebhook(newWebhook) {
						logAction.AppendResult("Notifications.Discord.Webhook changed", "value redacted")
						logging.LOGGER.Info().
							Timestamp().
							Msg("Notifications.Discord.Webhook changed")
						changed = true
					} else {
						newProv.Discord.Webhook = oldProv.Discord.Webhook
					}
				}

			case "Pushover":
				var oldToken, oldUserKey, newToken, newUserKey string
				if oldProv.Pushover != nil {
					oldToken = strings.TrimSpace(oldProv.Pushover.ApiToken)
					oldUserKey = strings.TrimSpace(oldProv.Pushover.UserKey)
				}
				if newProv.Pushover != nil {
					newToken = strings.TrimSpace(newProv.Pushover.ApiToken)
					newUserKey = strings.TrimSpace(newProv.Pushover.UserKey)
				}
				if oldUserKey != newUserKey {
					if !strings.HasPrefix(newUserKey, "***") {
						logAction.AppendResult("Notifications.Pushover.UserKey changed", "value redacted")
						logging.LOGGER.Info().
							Timestamp().
							Msg("Notifications.Pushover.UserKey changed")
						changed = true
					} else {
						newProv.Pushover.UserKey = oldProv.Pushover.UserKey
					}

				}
				if oldToken != newToken {
					if !strings.HasPrefix(newToken, "***") {
						logAction.AppendResult("Notifications.Pushover.ApiToken changed", "value redacted")
						logging.LOGGER.Info().
							Timestamp().
							Msg("Notifications.Pushover.ApiToken changed")
						changed = true
					} else {
						newProv.Pushover.ApiToken = oldProv.Pushover.ApiToken
					}

				}

			case "Gotify":
				var oldGotifyURL, newGotifyURL string
				if oldProv.Gotify != nil {
					oldGotifyURL = strings.TrimSpace(oldProv.Gotify.URL)
				}
				if newProv.Gotify != nil {
					newGotifyURL = strings.TrimSpace(newProv.Gotify.URL)
				}
				// URL is never masked; any difference is a real change
				if oldGotifyURL != newGotifyURL {
					logAction.AppendResult("Notifications.Gotify.URL changed", fmt.Sprintf("from '%v' to '%v'", oldGotifyURL, newGotifyURL))
					logging.LOGGER.Info().
						Timestamp().
						Str("old_gotify_url", oldGotifyURL).
						Str("new_gotify_url", newGotifyURL).
						Msg("Notifications.Gotify.URL changed")
					changed = true
				}
				// Token may still arrive masked; keep existing mask logic
				if oldProv.Gotify != nil && newProv.Gotify != nil {
					if oldProv.Gotify.ApiToken != newProv.Gotify.ApiToken {
						if !strings.HasPrefix(newProv.Gotify.ApiToken, "***") {
							logAction.AppendResult("Notifications.Gotify.ApiToken changed", "value redacted")
							logging.LOGGER.Info().
								Timestamp().
								Msg("Notifications.Gotify.ApiToken changed")
							changed = true
						} else {
							newProv.Gotify.ApiToken = oldProv.Gotify.ApiToken
						}
					}
				}

			case "Webhook":
				var oldURL, newURL string
				if oldProv.Webhook != nil {
					oldURL = strings.TrimSpace(oldProv.Webhook.URL)
				}
				if newProv.Webhook != nil {
					newURL = strings.TrimSpace(newProv.Webhook.URL)
				}
				if oldURL != newURL {
					logAction.AppendResult("Notifications.Webhook.URL changed", "value redacted")
					logging.LOGGER.Info().
						Timestamp().
						Msg("Notifications.Webhook.URL changed")
					changed = true
				}

				// Custom Headers
				oldHeaders := make(map[string]string)
				newHeaders := make(map[string]string)
				if oldProv.Webhook != nil {
					maps.Copy(oldHeaders, oldProv.Webhook.Headers)
				}
				if newProv.Webhook != nil {
					// Header values are masked on the way out - keep the old value of an unchanged header
					for k, newV := range newProv.Webhook.Headers {
						if oldV, ok := oldHeaders[k]; ok && newV != oldV && strings.HasPrefix(newV, "***") {
							newProv.Webhook.Headers[k] = oldV
						}
					}
					maps.Copy(newHeaders, newProv.Webhook.Headers)
				}
				// Check for changes
				for k, oldV := range oldHeaders {
					newV, ok := newHeaders[k]
					if !ok || oldV != newV {
						logAction.AppendResult("Notifications.Webhook.Header changed", fmt.Sprintf("Header '%s' changed (value redacted)", k))
						logging.LOGGER.Info().
							Timestamp().
							Str("header_key", k).
							Msg("Notifications.Webhook.Header changed")
						changed = true
					}
				}
				for k := range newHeaders {
					if _, ok := oldHeaders[k]; !ok {
						logAction.AppendResult("Notifications.Webhook.Header added", fmt.Sprintf("Header '%s' added (value redacted)", k))
						logging.LOGGER.Info().
							Timestamp().
							Str("header_key", k).
							Msg("Notifications.Webhook.Header added")
						changed = true
					}
				}
			default:
				// Unknown provider type: nothing more to compare
			}
		}

		// Compare template diffs
		templateDiffs := diffNotificationTemplates(oldNotifications.NotificationTemplate, newNotifications.NotificationTemplate)
		for _, d := range templateDiffs {
			logAction.AppendResult(
				fmt.Sprintf("Notifications.Templates.%s.%s changed", d.Event, d.Field),
				fmt.Sprintf("from '%v' to '%v'", d.Old, d.New),
			)
			logging.LOGGER.Info().
				Timestamp().
				Str("template_event", d.Event).
				Str("template_field", d.Field).
				Str("old_value", fmt.Sprintf("%v", d.Old)).
				Str("new_value", fmt.Sprintf("%v", d.New)).
				Msg("Notifications template changed")
			changed = true
		}

	}
	newValid = config.ValidateNotifications(ctx, newNotifications)
	// Catch any other changes not checked above
	if !changed && !reflect.DeepEqual(oldNotifications, *newNotifications) {
		changed = true
	}
	return changed, newValid
}

type notificationTemplateDiff struct {
	Event string
	Field string
	Old   any
	New   any
}

func diffNotificationTemplates(oldT, newT config.Config_NotificationTemplate) []notificationTemplateDiff {
	oldMap := map[string]config.Config_CustomNotification{
		"app_startup":                          oldT.AppStartup,
		"test_notification":                    oldT.TestNotification,
		"autodownload":                         oldT.Autodownload,
		"download_queue":                       oldT.DownloadQueue,
		"new_item_auto_download":               oldT.NewItemAutoDownload,
		"new_sets_available_for_ignored_items": oldT.NewSetsAvailableForIgnoredItems,
		"check_for_media_item_changes_job":     oldT.CheckForMediaItemChangesJob,
		"sonarr_notification":                  oldT.SonarrNotification,
		"silo_plugin":                          oldT.SiloPlugin,
	}
	newMap := map[string]config.Config_CustomNotification{
		"app_startup":                          newT.AppStartup,
		"test_notification":                    newT.TestNotification,
		"autodownload":                         newT.Autodownload,
		"download_queue":                       newT.DownloadQueue,
		"new_item_auto_download":               newT.NewItemAutoDownload,
		"new_sets_available_for_ignored_items": newT.NewSetsAvailableForIgnoredItems,
		"check_for_media_item_changes_job":     newT.CheckForMediaItemChangesJob,
		"sonarr_notification":                  newT.SonarrNotification,
		"silo_plugin":                          newT.SiloPlugin,
	}

	diffs := make([]notificationTemplateDiff, 0)
	for event, o := range oldMap {
		n := newMap[event]
		if o.Enabled != n.Enabled {
			diffs = append(diffs, notificationTemplateDiff{Event: event, Field: "enabled", Old: o.Enabled, New: n.Enabled})
		}
		if o.Title != n.Title {
			diffs = append(diffs, notificationTemplateDiff{Event: event, Field: "title", Old: o.Title, New: n.Title})
		}
		if o.Message != n.Message {
			diffs = append(diffs, notificationTemplateDiff{Event: event, Field: "message", Old: o.Message, New: n.Message})
		}
		if o.IncludeImage != n.IncludeImage {
			diffs = append(diffs, notificationTemplateDiff{Event: event, Field: "include_image", Old: o.IncludeImage, New: n.IncludeImage})
		}
	}
	return diffs
}

// checkConfigDifferences_SonarrRadarr compares old and new Sonarr/Radarr configurations.
func checkConfigDifferences_SonarrRadarr(ctx context.Context, oldSR config.Config_SonarrRadarr_Apps, newSR *config.Config_SonarrRadarr_Apps, msConfig config.Config_MediaServer) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: SonarrRadarr", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = false
	if !reflect.DeepEqual(oldSR, newSR) {

		// Providers diff
		oldMap := applicationSonarrRadarr(oldSR.Applications)
		newMap := applicationSonarrRadarr(newSR.Applications)

		// Added / removed types
		var added, removed []string
		for k := range oldMap {
			if _, ok := newMap[k]; !ok {
				removed = append(removed, k)
			}
		}
		for k := range newMap {
			if _, ok := oldMap[k]; !ok {
				added = append(added, k)
			}
		}
		if len(added) > 0 {
			sort.Strings(added)
			logAction.AppendResult("SonarrRadarr.Applications added", fmt.Sprintf("%s", joinNonEmptyComma(added)))
			logging.LOGGER.Info().
				Timestamp().
				Str("added_applications", fmt.Sprintf("%s", joinNonEmptyComma(added))).
				Msg("SonarrRadarr.Applications added")
			changed = true
		}
		if len(removed) > 0 {
			sort.Strings(removed)
			logAction.AppendResult("SonarrRadarr.Applications removed", fmt.Sprintf("%s", joinNonEmptyComma(removed)))
			logging.LOGGER.Info().
				Timestamp().
				Str("removed_applications", fmt.Sprintf("%s", joinNonEmptyComma(removed))).
				Msg("SonarrRadarr.Applications removed")
			changed = true
		}

		// Compare providers present in both
		for name, oldProv := range oldMap {
			newProv, ok := newMap[name]
			if !ok {
				continue
			}

			// Per App - ApiToken
			if oldProv.ApiToken != newProv.ApiToken {
				if !strings.HasPrefix(newProv.ApiToken, "***") {
					logAction.AppendResult("SonarrRadarr.Application.ApiToken changed", "value redacted")
					logging.LOGGER.Info().
						Timestamp().
						Str("application", name).
						Msg("SonarrRadarr.Application ApiToken changed")
					changed = true
				} else {

					newProv.ApiToken = oldProv.ApiToken
				}
			}

			// Per App - URL
			if oldProv.URL != newProv.URL {
				logAction.AppendResult("SonarrRadarr.Application.URL changed", fmt.Sprintf("from '%s' to '%s'", oldProv.URL, newProv.URL))
				logging.LOGGER.Info().
					Timestamp().
					Str("application", name).
					Str("old_url", oldProv.URL).
					Str("new_url", newProv.URL).
					Msg("SonarrRadarr.Application URL changed")
				changed = true
			}

			// Per App - Type
			if oldProv.Type != newProv.Type {
				logAction.AppendResult("SonarrRadarr.Application.Type changed", fmt.Sprintf("from '%s' to '%s'", oldProv.Type, newProv.Type))
				logging.LOGGER.Info().
					Timestamp().
					Str("application", name).
					Str("old_type", oldProv.Type).
					Str("new_type", newProv.Type).
					Msg("SonarrRadarr.Application Type changed")
				changed = true
			}

			// Per App - Library
			if oldProv.Library != newProv.Library {
				logAction.AppendResult("SonarrRadarr.Application.Library changed", fmt.Sprintf("from '%s' to '%s'", oldProv.Library, newProv.Library))
				logging.LOGGER.Info().
					Timestamp().
					Str("application", name).
					Str("old_library", oldProv.Library).
					Str("new_library", newProv.Library).
					Msg("SonarrRadarr.Application Library changed")
				changed = true
			}

		}
	}

	// Restore ApiTokens
	for i, app := range newSR.Applications {
		if strings.HasPrefix(app.ApiToken, "***") {
			// Find matching old app by Library and Type
			for _, oldApp := range oldSR.Applications {
				if app.Library == oldApp.Library && app.Type == oldApp.Type {
					newSR.Applications[i].ApiToken = oldApp.ApiToken
					break
				}
			}
		}
	}

	newValid = config.ValidateSonarrRadarr(ctx, newSR, msConfig)
	if !newValid {
		return changed, newValid
	}

	// Test connections for each Sonarr/Radarr app
	for _, app := range newSR.Applications {
		connectionOk, srErr := sonarr_radarr.TestConnection(ctx, app)
		if srErr.Message != "" || !connectionOk {
			newValid = false
		}
	}

	// Catch any other changes not checked above
	if !changed && !reflect.DeepEqual(oldSR, *newSR) {
		changed = true
	}
	return changed, newValid
}

// libraryNames returns a comma-separated string of library names from the given slice.
func libraryNames(libs []models.LibrarySection) string {
	names := make([]string, 0, len(libs))
	for _, l := range libs {
		n := strings.TrimSpace(l.Title)
		if n != "" {
			names = append(names, n)
		}
	}
	return joinNonEmptyComma(names)
}

// joinNonEmptyComma joins non-empty trimmed strings with a comma, or returns "(none)" if all are empty.
func joinNonEmptyComma(items []string) string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return "(none)"
	}
	return strings.Join(out, ", ")
}

// providerMapNotifications creates a map of notification providers keyed by provider name.
func providerMapNotifications(items []config.Config_Notification_Provider) map[string]config.Config_Notification_Provider {
	m := make(map[string]config.Config_Notification_Provider, len(items))
	for _, p := range items {
		if p.Provider == "" {
			continue
		}
		m[p.Provider] = p
	}
	return m
}

// applicationMapLabelsAndTags creates a map of LabelsAndTags providers keyed by application name.
func applicationMapLabelsAndTags(items []config.Config_LabelsAndTagsProvider) map[string]config.Config_LabelsAndTagsProvider {
	m := make(map[string]config.Config_LabelsAndTagsProvider, len(items))
	for _, p := range items {
		if p.Application == "" {
			continue
		}
		m[p.Application] = p
	}
	return m
}

func applicationSonarrRadarr(apps []config.Config_SonarrRadarrApp) map[string]config.Config_SonarrRadarrApp {
	m := make(map[string]config.Config_SonarrRadarrApp, len(apps))
	for _, a := range apps {
		if a.Library == "" {
			continue
		}
		m[a.Library] = a
	}
	return m
}
