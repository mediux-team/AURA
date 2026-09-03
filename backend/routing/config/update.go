package routes_config

import (
	"aura/config"
	autodownload "aura/download/auto"
	"aura/jobs"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/models"
	sonarr_radarr "aura/sonarr-radarr"
	"aura/utils/httpx"
	"context"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"sort"
	"strings"
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

	authChanged, authValid := checkConfigDifferences_Auth(ctx, config.Current.Auth, &newConfig.Auth)
	loggingChanged, loggingValid := checkConfigDifferences_Logging(ctx, config.Current.Logging, &newConfig.Logging)
	mediaServerChanged, mediaServerValid, newMediaServerName := checkConfigDifferences_MediaServer(ctx, config.Current.MediaServer, &newConfig.MediaServer)
	mediuxChanged, mediuxValid := checkConfigDifferences_Mediux(ctx, config.Current.Mediux, &newConfig.Mediux)
	autoDownloadChanged, autoDownloadValid := checkConfigDifferences_Autodownload(ctx, config.Current.AutoDownload, &newConfig.AutoDownload)
	jobsChanged, jobsValid := checkConfigDifferences_Jobs(ctx, config.Current.Jobs, &newConfig.Jobs)
	imagesChanged, imagesValid := checkConfigDifferences_Images(ctx, config.Current.Images, &newConfig.Images, newConfig.MediaServer)
	tmdbChanged, tmdbValid := checkConfigDifferences_TMDB(ctx, config.Current.TMDB, &newConfig.TMDB)
	labelsAndTagsChanged, labelsAndTagsValid := checkConfigDifferences_LabelsAndTags(ctx, config.Current.LabelsAndTags, &newConfig.LabelsAndTags)
	notificationsChanged, notificationsValid := checkConfigDifferences_Notifications(ctx, config.Current.Notifications, &newConfig.Notifications)
	sonarrRadarrChanged, sonarrRadarrValid := checkConfigDifferences_SonarrRadarr(ctx, config.Current.SonarrRadarr, &newConfig.SonarrRadarr, newConfig.MediaServer)
	databaseChanged, databaseValid := checkConfigDifferences_Database(ctx, config.Current.Database, &newConfig.Database)

	if !authValid || !loggingValid || !mediaServerValid || !mediuxValid || !autoDownloadValid || !jobsValid || !imagesValid || !tmdbValid || !labelsAndTagsValid || !notificationsValid || !sonarrRadarrValid || !databaseValid {
		ld.Status = logging.StatusError
		logAction.SetError("Invalid configuration", "The provided configuration is invalid. Check the results for details.", map[string]any{
			"auth_valid":            authValid,
			"logging_valid":         loggingValid,
			"media_server_valid":    mediaServerValid,
			"mediux_valid":          mediuxValid,
			"auto_download_valid":   autoDownloadValid,
			"jobs_valid":            jobsValid,
			"images_valid":          imagesValid,
			"tmdb_valid":            tmdbValid,
			"labels_and_tags_valid": labelsAndTagsValid,
			"notifications_valid":   notificationsValid,
			"sonarr_radarr_valid":   sonarrRadarrValid,
			"database_valid":        databaseValid,
		})
		response.Message = "Invalid configuration. Check the results for details."
		httpx.SendResponse(w, ld, response)
		return
	}

	if !authChanged && !loggingChanged && !mediaServerChanged && !mediuxChanged &&
		!autoDownloadChanged && !jobsChanged && !imagesChanged && !tmdbChanged && !labelsAndTagsChanged &&
		!notificationsChanged && !sonarrRadarrChanged && !databaseChanged {
		// If nothing has changed AND the config is valid, log a warning
		if config.Valid {
			ld.Status = logging.StatusWarn
			response.Message = "No changes detected in configuration"
			logging.LOGGER.Warn().Timestamp().Msg(response.Message)
			httpx.SendResponse(w, ld, response)
			return
		} else if !config.Valid {
			// If nothing has changed AND the config is invalid, re-validate
			newConfig.Validate(ctx)
			if !config.Valid {
				ld.Status = logging.StatusError
				response.Message = "Configuration is still invalid after update attempt"
				logging.LOGGER.Error().Timestamp().Msg(response.Message)
				httpx.SendResponse(w, ld, response)
				return
			} else {
				ld.Status = logging.StatusSuccess
				response.Message = "Configuration has been validated successfully"
				logging.LOGGER.Info().Timestamp().Msg(response.Message)
				httpx.SendResponse(w, ld, response)
				return
			}
		}
	}

	// Save the new config
	saveErr := newConfig.Save(ctx)
	if saveErr.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// Update the global config variable
	config.Current = newConfig
	config.Loaded = true
	config.Valid = true
	config.MediaServerValid = true
	config.MediaServerName = newMediaServerName
	config.MediuxValid = true

	if autoDownloadChanged {
		jobs.StartAutoDownloadJob()
	}

	if jobsChanged {
		jobs.StartConfiguredJobs()
	}

	if mediaServerChanged {
		autodownload.StartOrRestartPlexWebSocketClient()
	}

	response.Status = AppConfigStatus{
		ConfigLoaded:     config.Loaded,
		ConfigValid:      (config.Valid && config.MediuxValid && config.MediaServerValid),
		NeedsSetup:       !(config.Loaded && config.Valid && config.MediuxValid && config.MediaServerValid),
		CurrentSetup:     *newConfig.SanitizeConfig(ctx),
		MediaServerName:  config.MediaServerName,
		APIKeyConfigured: config.Current.Auth.APIKeyHash != "",
	}

	httpx.SendResponse(w, ld, response)
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

		if oldAuth.Password != newAuth.Password {
			logAction.AppendResult("Auth.Password changed", fmt.Sprintf("from '%s' to '%s'", oldAuth.Password, newAuth.Password))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_password", fmt.Sprintf("%s", oldAuth.Password)).
				Str("new_password", fmt.Sprintf("%s", newAuth.Password)).
				Msg("Auth.Password changed")
			changed = true
		}

		if oldAuth.SessionCookieSecure != newAuth.SessionCookieSecure || oldAuth.TrustProxyForCookieSecure != newAuth.TrustProxyForCookieSecure {
			changed = true
		}

		if !reflect.DeepEqual(oldAuth.AllowedOrigins, newAuth.AllowedOrigins) {
			changed = true
		}

		if oldAuth.OIDC.Enabled != newAuth.OIDC.Enabled {
			logAction.AppendResult("Auth.OIDC.Enabled changed", fmt.Sprintf("from '%v' to '%v'", oldAuth.OIDC.Enabled, newAuth.OIDC.Enabled))
			changed = true
		}

		if oldAuth.OIDC.IssuerURL != newAuth.OIDC.IssuerURL ||
			oldAuth.OIDC.ClientID != newAuth.OIDC.ClientID ||
			oldAuth.OIDC.RedirectURL != newAuth.OIDC.RedirectURL ||
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
	}

	newValid = config.ValidateLogging(ctx, newLogging)
	return changed, newValid
}

// checkConfigDifferences_MediaServer compares old and new MediaServer configurations.
func checkConfigDifferences_MediaServer(ctx context.Context, oldMediaServer config.Config_MediaServer, newMediaServer *config.Config_MediaServer) (changed, newValid bool, serverName string) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: MediaServer", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = false
	serverName = ""
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
				logAction.AppendResult("MediaServer.ApiToken changed", fmt.Sprintf("from '%s' to '%s'", oldMediaServer.ApiToken, newMediaServer.ApiToken))
				logging.LOGGER.Info().
					Timestamp().
					Str("old_api_token", fmt.Sprintf("%s", oldMediaServer.ApiToken)).
					Str("new_api_token", fmt.Sprintf("%s", newMediaServer.ApiToken)).
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
	}

	return changed, newValid, serverName
}

// checkConfigDifferences_Mediux compares old and new MediUX configurations.
func checkConfigDifferences_Mediux(ctx context.Context, oldMediux config.Config_Mediux, newMediux *config.Config_Mediux) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: MediUX", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = false
	if !reflect.DeepEqual(oldMediux, newMediux) {
		if oldMediux.ApiToken != newMediux.ApiToken {
			if !strings.HasPrefix(newMediux.ApiToken, "***") {
				logAction.AppendResult("Mediux.ApiToken changed", fmt.Sprintf("from '%s' to '%s'", oldMediux.ApiToken, newMediux.ApiToken))
				logging.LOGGER.Info().
					Timestamp().
					Str("old_api_token", fmt.Sprintf("%v", oldMediux.ApiToken)).
					Str("new_api_token", fmt.Sprintf("%v", newMediux.ApiToken)).
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

	return changed, newValid
}

// checkConfigDifferences_Autodownload compares old and new AutoDownload configurations.
func checkConfigDifferences_Autodownload(ctx context.Context, oldAutoDownload config.Config_AutoDownload, newAutoDownload *config.Config_AutoDownload) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: Autodownload", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = false
	if !reflect.DeepEqual(oldAutoDownload, newAutoDownload) {
		if oldAutoDownload.Enabled != newAutoDownload.Enabled {
			logAction.AppendResult("Autodownload.Enabled changed", fmt.Sprintf("from '%v' to '%v'", oldAutoDownload.Enabled, newAutoDownload.Enabled))
			logging.LOGGER.Info().
				Timestamp().
				Bool("old_enabled", oldAutoDownload.Enabled).
				Bool("new_enabled", newAutoDownload.Enabled).
				Msg("Autodownload.Enabled changed")
			changed = true
		}

		if oldAutoDownload.Cron != newAutoDownload.Cron {
			logAction.AppendResult("Autodownload.Cron changed", fmt.Sprintf("from '%s' to '%s'", oldAutoDownload.Cron, newAutoDownload.Cron))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_cron", oldAutoDownload.Cron).
				Str("new_cron", newAutoDownload.Cron).
				Msg("Autodownload.Cron changed")
			changed = true
		}
	}
	newValid = config.ValidateAutoDownload(ctx, newAutoDownload)
	return changed, newValid
}

// checkConfigDifferences_Jobs compares old and new Jobs configurations.
func checkConfigDifferences_Jobs(ctx context.Context, oldJobs config.Config_Jobs, newJobs *config.Config_Jobs) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: Jobs", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = config.ValidateJobs(ctx, newJobs)
	if !reflect.DeepEqual(oldJobs, *newJobs) {
		changed = true
		logAction.AppendResult("Jobs configuration changed", "Jobs schedule or enabled state updated")
		logging.LOGGER.Info().Timestamp().Msg("Jobs configuration changed")
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

		if oldImages.SaveImagesLocally.EpisodeNamingConvention != newImages.SaveImagesLocally.EpisodeNamingConvention {
			logAction.AppendResult("Images.SaveImagesLocally.EpisodeNamingConvention changed", fmt.Sprintf("from '%s' to '%s'", oldImages.SaveImagesLocally.EpisodeNamingConvention, newImages.SaveImagesLocally.EpisodeNamingConvention))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_episode_naming_convention", oldImages.SaveImagesLocally.EpisodeNamingConvention).
				Str("new_episode_naming_convention", newImages.SaveImagesLocally.EpisodeNamingConvention).
				Msg("Images.SaveImagesLocally.EpisodeNamingConvention changed")
			changed = true
		}
	}
	newValid = config.ValidateImages(ctx, newImages, msConfig)
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
				logAction.AppendResult("TMDB.ApiToken changed", fmt.Sprintf("from '%s' to '%s'", oldTMDB.ApiToken, newTMDB.ApiToken))
				logging.LOGGER.Info().
					Timestamp().
					Str("old_api_token", fmt.Sprintf("%v", oldTMDB.ApiToken)).
					Str("new_api_token", fmt.Sprintf("%v", newTMDB.ApiToken)).
					Msg("TMDB.ApiToken changed")
				changed = true
			} else {
				newTMDB.ApiToken = oldTMDB.ApiToken
			}
		}
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
						logAction.AppendResult("Notifications.Discord.Webhook changed", fmt.Sprintf("from '%v' to '%v'", oldWebhook, newWebhook))
						logging.LOGGER.Info().
							Timestamp().
							Str("old_webhook", oldWebhook).
							Str("new_webhook", newWebhook).
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
						logAction.AppendResult("Notifications.Pushover.UserKey changed", fmt.Sprintf("from '%v' to '%v'", oldUserKey, newUserKey))
						logging.LOGGER.Info().
							Timestamp().
							Str("old_user_key", oldUserKey).
							Str("new_user_key", newUserKey).
							Msg("Notifications.Pushover.UserKey changed")
						changed = true
					} else {
						newProv.Pushover.UserKey = oldProv.Pushover.UserKey
					}

				}
				if oldToken != newToken {
					if !strings.HasPrefix(newToken, "***") {
						logAction.AppendResult("Notifications.Pushover.ApiToken changed", fmt.Sprintf("from '%v' to '%v'", oldToken, newToken))
						logging.LOGGER.Info().
							Timestamp().
							Str("old_api_token", oldToken).
							Str("new_api_token", newToken).
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
							logAction.AppendResult("Notifications.Gotify.ApiToken changed", fmt.Sprintf("from '%v' to '%v'", oldProv.Gotify.ApiToken, newProv.Gotify.ApiToken))
							logging.LOGGER.Info().
								Timestamp().
								Str("old_api_token", fmt.Sprintf("%s", oldProv.Gotify.ApiToken)).
								Str("new_api_token", fmt.Sprintf("%s", newProv.Gotify.ApiToken)).
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
					logAction.AppendResult("Notifications.Webhook.URL changed", fmt.Sprintf("from '%v' to '%v'", oldURL, newURL))
					logging.LOGGER.Info().
						Timestamp().
						Str("old_webhook_url", oldURL).
						Str("new_webhook_url", newURL).
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
					maps.Copy(newHeaders, newProv.Webhook.Headers)
				}
				// Check for changes
				for k, oldV := range oldHeaders {
					newV, ok := newHeaders[k]
					if !ok || oldV != newV {
						logAction.AppendResult("Notifications.Webhook.Header changed", fmt.Sprintf("Header '%s' changed from '%s' to '%s'", k, oldV, newV))
						logging.LOGGER.Info().
							Timestamp().
							Str("header_key", k).
							Str("old_value", oldV).
							Str("new_value", newV).
							Msg("Notifications.Webhook.Header changed")
						changed = true
					}
				}
				for k, newV := range newHeaders {
					if _, ok := oldHeaders[k]; !ok {
						logAction.AppendResult("Notifications.Webhook.Header added", fmt.Sprintf("Header '%s' added with value '%s'", k, newV))
						logging.LOGGER.Info().
							Timestamp().
							Str("header_key", k).
							Str("new_value", newV).
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
		"new_sets_available_for_ignored_items": oldT.NewSetsAvailableForIgnoredItems,
		"check_for_media_item_changes_job":     oldT.CheckForMediaItemChangesJob,
		"sonarr_notification":                  oldT.SonarrNotification,
	}
	newMap := map[string]config.Config_CustomNotification{
		"app_startup":                          newT.AppStartup,
		"test_notification":                    newT.TestNotification,
		"autodownload":                         newT.Autodownload,
		"download_queue":                       newT.DownloadQueue,
		"new_sets_available_for_ignored_items": newT.NewSetsAvailableForIgnoredItems,
		"check_for_media_item_changes_job":     newT.CheckForMediaItemChangesJob,
		"sonarr_notification":                  newT.SonarrNotification,
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
					logAction.AppendResult("SonarrRadarr.Application.ApiToken changed", fmt.Sprintf("from '%s' to '%s'", oldProv.ApiToken, newProv.ApiToken))
					logging.LOGGER.Info().
						Timestamp().
						Str("application", name).
						Str("old_api_token", oldProv.ApiToken).
						Str("new_api_token", newProv.ApiToken).
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

	return changed, newValid
}

func checkConfigDifferences_Database(ctx context.Context, oldDB config.Config_Database, newDB *config.Config_Database) (changed, newValid bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Check Config Differences: Database", logging.LevelTrace)
	defer logAction.Complete()
	changed = false
	newValid = false
	if !reflect.DeepEqual(oldDB, newDB) {
		if oldDB.Type != newDB.Type {
			logAction.AppendResult("Database.Type changed", fmt.Sprintf("from '%s' to '%s'", oldDB.Type, newDB.Type))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_type", oldDB.Type).
				Str("new_type", newDB.Type).
				Msg("Database.Type changed")
			changed = true
		}

		if oldDB.DSN != newDB.DSN {
			logAction.AppendResult("Database.ConnectionString changed", fmt.Sprintf("from '%s' to '%s'", oldDB.DSN, newDB.DSN))
			logging.LOGGER.Info().
				Timestamp().
				Str("old_connection_string", oldDB.DSN).
				Str("new_connection_string", newDB.DSN).
				Msg("Database.ConnectionString changed")
			changed = true
		}
	}
	newValid = config.ValidateDatabase(ctx, newDB)
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
