package config

import (
	"aura/logging"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/robfig/cron/v3"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func (config *Config) Validate(ctx context.Context) {
	_, logAction := logging.AddSubActionToContext(ctx, "Validating Config", logging.LevelInfo)
	defer logAction.Complete()

	// Top-level action
	// Sub-action: Auth Config
	isAuthValid := ValidateAuth(ctx, &config.Auth)

	// Sub-action: Logging Config
	isLoggingValid := ValidateLogging(ctx, &config.Logging)

	// // Sub-action: MediaServer Config
	isMediaServerValid := ValidateMediaServer(ctx, &config.MediaServer)

	// Sub-action: MediaServers Config
	isMediaServersValid := ValidateMediaServers(ctx, config.MediaServers, config.MainMediaServer)

	// Sub-action: MediUX Config
	isMediuxValid := ValidateMediux(ctx, &config.Mediux)

	// Sub-action: Jobs Config
	isJobsValid := ValidateJobs(ctx, &config.Jobs)

	// Sub-action: Backups Config
	isBackupsValid := ValidateBackups(ctx, &config.Backups)

	// Sub-action: Images Config
	isImagesValid := ValidateImages(ctx, &config.Images, config.MediaServer)

	// Sub-action: Notifications Config
	isNotificationsValid := ValidateNotifications(ctx, &config.Notifications)

	// Sub-action: SonarrRadarr Config
	isSonarrRadarrValid := ValidateSonarrRadarr(ctx, &config.SonarrRadarr, config.MediaServer)

	// Sub-action: LabelsAndTags Config
	isLabelsAndTagsValid := ValidateLabelsAndTags(ctx, &config.LabelsAndTags)

	// Sub-action: UserPreferences Config
	isUserPreferencesValid := ValidateUserPreferences(ctx, &config.UserPreferences)

	// If any validation failed, set status to error
	if !isAuthValid || !isLoggingValid || !isMediaServerValid || !isMediaServersValid || !isMediuxValid || !isJobsValid || !isBackupsValid || !isImagesValid || !isNotificationsValid || !isSonarrRadarrValid || !isLabelsAndTagsValid || !isUserPreferencesValid {
		logAction.SetError("Config validation failed", "One or more config sections are invalid", nil)
		Valid = false
	} else {
		logging.SetLogLevel(config.Logging.Level)
		Valid = true
	}

}

func ValidateAuth(ctx context.Context, Auth *Config_Auth) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Auth Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	if Auth.Enabled && !Auth.PasswordEnabled && !Auth.OIDC.Enabled {
		logAction.SetError("Auth has no way to log in", "Turn on the password or Single Sign-On (OIDC) when auth is enabled", nil)
		isValid = false
	}

	if Auth.Enabled && Auth.PasswordEnabled {
		if Auth.Password == "" {
			logAction.SetError("Auth.Password is not set", "Password must be set when auth is enabled", nil)
			isValid = false
		} else {
			_, _, _, err := argon2id.DecodeHash(Auth.Password)
			if err != nil {
				logAction.SetError("Auth.Password is not a valid Argon2id hash", err.Error(), nil)

				isValid = false
			}
		}
	}

	if Auth.OIDC.Enabled {
		// The Issuer URL is the base URL of the identity provider, not its discovery document
		Auth.OIDC.IssuerURL = strings.TrimSpace(Auth.OIDC.IssuerURL)
		if trimmed := strings.TrimSuffix(strings.TrimRight(Auth.OIDC.IssuerURL, "/"), "/.well-known/openid-configuration"); trimmed != strings.TrimRight(Auth.OIDC.IssuerURL, "/") {
			logAction.AppendWarning("message", fmt.Sprintf("Auth.OIDC.IssuerURL should not include /.well-known/openid-configuration, using '%s'", trimmed))
			Auth.OIDC.IssuerURL = trimmed
		}
		redirectURLs := []string{}
		for _, redirectURL := range Auth.OIDC.RedirectURLs {
			withPath := WithOIDCCallbackPath(redirectURL)
			if withPath == "" {
				continue
			}
			if withPath != strings.TrimSpace(redirectURL) {
				logAction.AppendWarning("message", fmt.Sprintf("Auth.OIDC.RedirectURLs should end with %s, using '%s'", OIDCCallbackPath, withPath))
			}
			redirectURLs = append(redirectURLs, withPath)
		}
		Auth.OIDC.RedirectURLs = redirectURLs

		if Auth.OIDC.IssuerURL == "" || Auth.OIDC.ClientID == "" || Auth.OIDC.ClientSecret == "" || len(Auth.OIDC.RedirectURLs) == 0 {
			logAction.SetError("Auth.OIDC is missing required fields", "IssuerURL, ClientID, ClientSecret, and at least one RedirectURL are required when OIDC is enabled", nil)
			isValid = false
		} else {
			if _, err := url.ParseRequestURI(Auth.OIDC.IssuerURL); err != nil {
				logAction.SetError("Auth.OIDC.IssuerURL is not a valid URL", err.Error(), nil)
				isValid = false
			}
			for _, redirectURL := range Auth.OIDC.RedirectURLs {
				if _, err := url.ParseRequestURI(redirectURL); err != nil {
					logAction.SetError("Auth.OIDC.RedirectURLs has a URL that isn't valid", err.Error(), map[string]any{"redirect_url": redirectURL})
					isValid = false
				}
			}
		}
	}

	// Default Auth.SessionCookieSecure to "auto" if not set
	switch Auth.SessionCookieSecure {
	case "":
		Auth.SessionCookieSecure = "auto"
	case "auto", "always", "never":
	default:
		logAction.SetError("Auth.SessionCookieSecure is invalid", "Must be one of: auto, always, never", nil)
		isValid = false
	}

	return isValid
}

func ValidateLogging(ctx context.Context, Logging *Config_Logging) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Logging Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// Check if Logging.Level is set
	if Logging.Level == "" {
		Logging.Level = "INFO" // Default to INFO if not set
		logging.LOGGER.Warn().Timestamp().Msg("Logging.Level not set, defaulting to 'INFO'")
	}

	// Set Logging.Level to uppercase for comparison
	Logging.Level = strings.ToUpper(Logging.Level)
	if Logging.Level != "TRACE" && Logging.Level != "DEBUG" && Logging.Level != "INFO" && Logging.Level != "WARN" && Logging.Level != "ERROR" {
		logAction.SetError("Logging.Level is not a valid level", "Valid levels are TRACE, DEBUG, INFO, WARN, ERROR", nil)
		isValid = false
	} else {
		logging.SetLogLevel(Logging.Level)
	}

	// Logging.KeepDays: default to 7 days, at most 365 days
	if Logging.KeepDays <= 0 {
		Logging.KeepDays = 7
	} else if Logging.KeepDays > 365 {
		logAction.SetError("Logging.KeepDays is too large", "Use a value between 1 and 365", nil)
		isValid = false
	}

	// Logging.KeepFiles: default to 7 files, at most 100
	if Logging.KeepFiles <= 0 {
		Logging.KeepFiles = 7
	} else if Logging.KeepFiles > 100 {
		logAction.SetError("Logging.KeepFiles is too large", "Use a value between 1 and 100", nil)
		isValid = false
	}

	if isValid {
		logging.SetLogKeepDays(Logging.KeepDays)
		logging.SetLogKeepFiles(Logging.KeepFiles)
	}

	return isValid
}

func ValidateMediaServer(ctx context.Context, MediaServer *Config_MediaServer) bool {
	return validateMediaServer(ctx, MediaServer, "")
}

// validateMediaServer checks a media server. name is set for media servers that aren't the main one, so errors say which one.
func validateMediaServer(ctx context.Context, MediaServer *Config_MediaServer, name string) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating MediaServer Config", logging.LevelTrace)
	defer logAction.Complete()

	field := func(field string) string {
		if name == "" {
			return "MediaServer." + field
		}
		return fmt.Sprintf("Media server '%s': %s", name, field)
	}

	isValid := true

	// Title case the MediaServer.Type
	MediaServer.Type = cases.Title(language.English).String(MediaServer.Type)

	// Check if MediaServer.Type is set
	if MediaServer.Type == "" {
		logAction.SetError(field("Type")+" is not set", "Media server type must be specified", nil)
		isValid = false
	} else if MediaServer.Type != "Plex" && MediaServer.Type != "Emby" && MediaServer.Type != "Jellyfin" && MediaServer.Type != "Silo" {
		logAction.SetError(fmt.Sprintf("%s: '%s' is not a valid type", field("Type"), MediaServer.Type), "Valid types are Plex, Emby, Jellyfin, Silo", nil)
		isValid = false
	}

	// Check if MediaServer.URL is set
	if MediaServer.URL == "" {
		logAction.SetError(field("URL")+" is not set", "Media server URL must be specified", nil)
		isValid = false
	} else if !strings.HasPrefix(MediaServer.URL, "http://") && !strings.HasPrefix(MediaServer.URL, "https://") {
		logAction.SetError(fmt.Sprintf("%s: '%s' must start with http:// or https:// ", field("URL"), MediaServer.URL), "", nil)
		isValid = false
	}

	// Check if MediaServer.ApiToken is set
	if MediaServer.ApiToken == "" {
		logAction.SetError(field("ApiToken")+" is not set", "Media server ApiToken must be specified", nil)
		isValid = false
	}

	// Check if MediaServer.Libraries is set
	if len(MediaServer.Libraries) == 0 {
		logAction.SetError(field("Libraries")+" is not set", "At least one media server library must be specified", nil)
		isValid = false
	}

	// If we reach here, return if not valid
	if !isValid {
		return isValid
	}

	// Now that we know the Media Server config is valid, clean up fields and set defaults
	// Trim the trailing slash from the URL
	MediaServer.URL = strings.TrimSuffix(MediaServer.URL, "/")

	return isValid
}

func ValidateMediux(ctx context.Context, Mediux *Config_Mediux) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating MediUX Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// Check if MediUX.ApiToken is set
	if Mediux.ApiToken == "" {
		logAction.SetError("Mediux.ApiToken is not set", "MediUX ApiToken must be specified", nil)
		isValid = false
	}

	// Check if Mediux.DownloadQuality is set
	if Mediux.DownloadQuality == "" {
		Mediux.DownloadQuality = "optimized"
		logging.LOGGER.Warn().Timestamp().Msg("Mediux.DownloadQuality not set, defaulting to 'optimized'")
		logAction.AppendWarning("message", "Mediux.DownloadQuality not set, defaulting to 'optimized'")
	} else if Mediux.DownloadQuality != "original" && Mediux.DownloadQuality != "optimized" {
		Mediux.DownloadQuality = "optimized"
		logging.LOGGER.Warn().Timestamp().Msg("Mediux.DownloadQuality invalid, defaulting to 'optimized'")
		logAction.AppendWarning("message", "Mediux.DownloadQuality invalid, defaulting to 'optimized'")
	}

	return isValid
}

func ValidateJobs(ctx context.Context, Jobs *Config_Jobs) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Jobs Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// Check each job setting
	if !ValidateJobSetting(ctx, &Jobs.AutoDownload, JobDefaults.AutoDownload) {
		isValid = false
	}
	if !ValidateJobSetting(ctx, &Jobs.RefreshMediaItemsAndCollections, JobDefaults.RefreshMediaItemsAndCollections) {
		isValid = false
	}
	if !ValidateJobSetting(ctx, &Jobs.CheckForMediaItemChanges, JobDefaults.CheckForMediaItemChanges) {
		isValid = false
	}
	if !ValidateJobSetting(ctx, &Jobs.CheckForNewMediaItems, JobDefaults.CheckForNewMediaItems) {
		isValid = false
	}
	if !ValidateJobSetting(ctx, &Jobs.HandleTempIgnoredItems, JobDefaults.HandleTempIgnoredItems) {
		isValid = false
	}
	if !ValidateJobSetting(ctx, &Jobs.RefreshMediuxUsers, JobDefaults.RefreshMediuxUsers) {
		isValid = false
	}
	if !ValidateJobSetting(ctx, &Jobs.CheckMediuxSiteLink, JobDefaults.CheckMediuxSiteLink) {
		isValid = false
	}
	if !ValidateJobSetting(ctx, &Jobs.BackupDatabase, JobDefaults.BackupDatabase) {
		isValid = false
	}
	if !ValidateJobSetting(ctx, &Jobs.UpgradeSavedSets, JobDefaults.UpgradeSavedSets) {
		isValid = false
	}

	return isValid
}

func ValidateJobSetting(ctx context.Context, job *Config_JobSetting, def JobSettingDefault) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Job Setting", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	enabled, _ := job.Resolve(def)

	// Check if the job is enabled
	if !enabled {
		return isValid
	}

	// Check if the job's cron expression is set
	if job.Cron == "" {
		job.Cron = def.Cron
		logging.LOGGER.Warn().Timestamp().Str("cron", def.Cron).Msg("Job Cron not set, defaulting to its built-in schedule")
		logAction.AppendWarning("message", fmt.Sprintf("Job Cron not set, defaulting to '%s'", def.Cron))
	}

	// Validate the cron expression
	if !ValidateCron(job.Cron) {
		logAction.SetError(fmt.Sprintf("Job.Cron: '%s' is not a valid cron expression", job.Cron), "Please provide a valid cron expression", nil)
		isValid = false
	}

	return isValid
}

func ValidateCron(cronExpression string) bool {
	_, err := cron.ParseStandard(cronExpression)
	return err == nil
}

func ValidateImages(ctx context.Context, Images *Config_Images, msConfig Config_MediaServer) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Images Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// An empty template, or one with unknown variables or .., uses the standard name
	templates := &Images.SaveImagesLocally.NameTemplates
	defaults := DefaultImageNameTemplates()
	for _, t := range []struct {
		name     string
		template *string
		fallback string
	}{
		{"Poster", &templates.Poster, defaults.Poster},
		{"Backdrop", &templates.Backdrop, defaults.Backdrop},
		{"SeasonPoster", &templates.SeasonPoster, defaults.SeasonPoster},
		{"SpecialSeasonPoster", &templates.SpecialSeasonPoster, defaults.SpecialSeasonPoster},
		{"Titlecard", &templates.Titlecard, defaults.Titlecard},
	} {
		*t.template = strings.TrimSpace(*t.template)
		if *t.template == "" {
			*t.template = t.fallback
			continue
		}
		if !validateTemplateVariables(*t.template, ImageNameTemplateVariables) || slices.Contains(strings.Split(*t.template, "/"), "..") {
			logAction.AppendWarning("message", fmt.Sprintf("Images.SaveImagesLocally.NameTemplates.%s has invalid variables or .., using the standard name", t.name))
			*t.template = t.fallback
		}
	}
	return isValid
}

func ValidateNotifications(ctx context.Context, Notifications *Config_Notifications) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Notifications Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true
	defaults := DefaultNotificationTemplates()

	// If the notifications are not enabled, skip validation
	if !Notifications.Enabled || len(Notifications.Providers) == 0 {
		return isValid
	}

	// If notifications are enabled, validate each provider
	for i, provider := range Notifications.Providers {
		providerIsValid := ValidateNotificationsProvider(ctx, &provider)
		if !providerIsValid {
			isValid = false
		}
		Notifications.Providers[i] = provider
	}

	// If notifications are enabled, validate the templates
	templates := &Notifications.NotificationTemplate
	for _, t := range []struct {
		name         string
		templateType string
		template     *Config_CustomNotification
		fallback     Config_CustomNotification
	}{
		{"AppStartup", TemplateTypeAppStartup, &templates.AppStartup, defaults.AppStartup},
		{"TestNotification", TemplateTypeTestNotification, &templates.TestNotification, defaults.TestNotification},
		{"Autodownload", TemplateTypeAutodownload, &templates.Autodownload, defaults.Autodownload},
		{"DownloadQueue", TemplateTypeDownloadQueue, &templates.DownloadQueue, defaults.DownloadQueue},
		{"NewItemAutoDownload", TemplateTypeNewItemAutoDownload, &templates.NewItemAutoDownload, defaults.NewItemAutoDownload},
		{"NewSetsAvailableForIgnoredItems", TemplateTypeNewSetsAvailableForIgnoredItems, &templates.NewSetsAvailableForIgnoredItems, defaults.NewSetsAvailableForIgnoredItems},
		{"CheckForMediaItemChangesJob", TemplateTypeCheckForMediaItemChangesJob, &templates.CheckForMediaItemChangesJob, defaults.CheckForMediaItemChangesJob},
		{"SonarrNotification", TemplateTypeSonarrNotification, &templates.SonarrNotification, defaults.SonarrNotification},
		{"SiloPlugin", TemplateTypeSiloPlugin, &templates.SiloPlugin, defaults.SiloPlugin},
	} {
		validateNotificationTemplate(logAction, t.name, t.templateType, t.template, t.fallback)
	}

	return isValid
}

// validateNotificationTemplate uses the built-in parts of a template that are missing or have unknown variables
func validateNotificationTemplate(logAction *logging.LogAction, name, templateType string, template *Config_CustomNotification, fallback Config_CustomNotification) {
	warn := func(message string) {
		logging.LOGGER.Warn().Timestamp().Msg(message)
		logAction.AppendWarning("message", message)
	}
	if *template == (Config_CustomNotification{}) {
		*template = fallback
		warn(fmt.Sprintf("Notifications.NotificationTemplate.%s not set, defaulting to built-in template", name))
		return
	}
	validVariables := AllowedTemplateVariables(templateType)
	if !validateTemplateVariables(template.Title, validVariables) {
		warn(fmt.Sprintf("Notifications.NotificationTemplate.%s.Title contains invalid variables, please check the config documentation for valid variables", name))
		template.Title = fallback.Title
	}
	if !validateTemplateVariables(template.Message, validVariables) {
		warn(fmt.Sprintf("Notifications.NotificationTemplate.%s.Message contains invalid variables, please check the config documentation for valid variables", name))
		template.Message = fallback.Message
	}
}

func validateTemplateVariables(userStr string, validVariables []string) bool {
	for _, variable := range validVariables {
		userStr = strings.ReplaceAll(userStr, variable, "")
	}

	// If there are any {{ or }} left in the string, then there are invalid variables
	if strings.Contains(userStr, "{{") || strings.Contains(userStr, "}}") {
		return false
	}

	return true
}

func ValidateNotificationsProvider(ctx context.Context, provider *Config_Notification_Provider) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Notification Provider Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// Set the provider name to Title Case
	provider.Provider = cases.Title(language.English).String(provider.Provider)

	// If the provider name is not set, return an error
	if provider.Provider == "" {
		logAction.SetError("Notification.Provider is not set", "Provider name must be specified", nil)
		isValid = false
		return isValid
	}

	// If the provider is not enabled, log a warning and continue to the next provider
	if !provider.Enabled {
		logAction.AppendWarning("message", fmt.Sprintf("Notification.Provider '%s' is disabled, skipping validation", provider.Provider))
		return isValid
	}

	validProviders := []string{"Discord", "Pushover", "Gotify", "Webhook"}

	// If the provider is not in the list of valid providers, return an error
	if !stringSliceContains(validProviders, provider.Provider) {
		logAction.SetError(fmt.Sprintf("Bad Notification.Provider: '%s'. Must be one of: %v", provider.Provider, validProviders), "Please provide a valid provider", nil)
		isValid = false
	}

	switch provider.Provider {
	case "Discord":
		if provider.Discord.Webhook == "" {
			logAction.SetError("Notification.Webhook is not set", "Discord webhook must be specified", nil)
			isValid = false
		}

	case "Pushover":
		if provider.Pushover.UserKey == "" {
			logAction.SetError("Notification.UserKey is not set", "Pushover UserKey must be specified", nil)
			isValid = false
		}
		if provider.Pushover.ApiToken == "" {
			logAction.SetError("Notification.ApiToken is not set", "Pushover ApiToken must be specified", nil)
			isValid = false
		}

	case "Gotify":
		if provider.Gotify.URL == "" {
			logAction.SetError("Notification.URL is not set", "Gotify URL must be specified", nil)
			isValid = false
		}
		if provider.Gotify.ApiToken == "" {
			logAction.SetError("Notification.ApiToken is not set", "Gotify ApiToken must be specified", nil)
			isValid = false
		}
	case "Webhook":
		if provider.Webhook.URL == "" {
			logAction.SetError("Notification.URL is not set", "Webhook URL must be specified", nil)
			isValid = false
		}
	}

	return isValid
}

func ValidateSonarrRadarr(ctx context.Context, apps *Config_SonarrRadarr_Apps, mediaServerConfig Config_MediaServer) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating SonarrRadarr Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	if len(apps.Applications) == 0 {
		return isValid
	}

	for i, app := range apps.Applications {

		// Set the app type to Title Case
		app.Type = cases.Title(language.English).String(app.Type)

		// If the app type is not set, return an error
		if app.Type == "" {
			logAction.SetError(fmt.Sprintf("\tSonarrRadarr[%d].Type is not set", i), "App type must be specified", nil)
			isValid = false
		} else if app.Type != "Sonarr" && app.Type != "Radarr" {
			// If the app type is not Sonarr or Radarr, return an error
			logAction.SetError(fmt.Sprintf("\tBad SonarrRadarr[%d].Type: '%s'. Must be one of: Sonarr, Radarr", i, app.Type), "Invalid app type", nil)
			isValid = false
		}

		libraryNames := make([]string, len(mediaServerConfig.Libraries))
		for idx, lib := range mediaServerConfig.Libraries {
			libraryNames[idx] = lib.Title // Replace 'Title' with the actual field name containing the library string
		}

		if app.Library == "" {
			logAction.SetError(fmt.Sprintf("\tSonarrRadarr[%d].Library is not set", i), "Library must be specified", nil)
			isValid = false
		} else if !stringSliceContains(libraryNames, app.Library) {
			// If the library is not in the list of MediaServer libraries, return an error
			logAction.SetError(fmt.Sprintf("\tBad SonarrRadarr[%d].Library: '%s'. Must be one of: %v", i, app.Library, libraryNames), "Invalid library", nil)
			isValid = false
		}

		if app.URL == "" {
			logAction.SetError(fmt.Sprintf("\tSonarrRadarr[%d].URL is not set", i), "App URL must be specified", nil)
			isValid = false
		} else if !strings.HasPrefix(app.URL, "http") {
			// If the URL does not start with http or https, return an error
			logAction.SetError(fmt.Sprintf("\tSonarrRadarr[%d].URL: '%s' must start with http:// or https:// ", i, app.URL), "", nil)
			isValid = false
		}

		if app.ApiToken == "" {
			logAction.SetError(fmt.Sprintf("\tSonarrRadarr[%d].ApiToken is not set", i), "App ApiToken must be specified", nil)
			isValid = false
		}

		apps.Applications[i] = app
	}

	return isValid
}

func ValidateUserPreferences(ctx context.Context, UserPreferences *Config_UserPreferences) bool {
	_, logAction := logging.AddSubActionToContext(ctx, "Validating UserPreferences Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// Remove any unknown or duplicate image types from DownloadDefaults
	downloadDefaults := []string{}
	for _, imageType := range UserPreferences.DownloadDefaults {
		if !stringSliceContains(DownloadImageTypes, imageType) {
			logAction.AppendWarning("message", fmt.Sprintf("UserPreferences.DownloadDefaults: '%s' is not a valid image type, removing it", imageType))
			continue
		}
		if !stringSliceContains(downloadDefaults, imageType) {
			downloadDefaults = append(downloadDefaults, imageType)
		}
	}
	UserPreferences.DownloadDefaults = downloadDefaults

	// Remove blank and repeated favorite creators, keeping the best rank of each
	favoriteCreators := []string{}
	seen := map[string]bool{}
	for _, username := range UserPreferences.FavoriteCreators {
		username = strings.TrimSpace(username)
		if username == "" || seen[strings.ToLower(username)] {
			continue
		}
		seen[strings.ToLower(username)] = true
		favoriteCreators = append(favoriteCreators, username)
	}
	UserPreferences.FavoriteCreators = favoriteCreators

	// Each page's saved sort and filter options, only for known pages and not too large
	pageViews := map[string]map[string]any{}
	for page, view := range UserPreferences.PageViews {
		if !slices.Contains(PageViewPages, page) {
			logAction.AppendWarning("message", fmt.Sprintf("UserPreferences.PageViews has an unknown page %q, removing it", page))
			continue
		}
		if encoded, err := json.Marshal(view); err != nil || len(encoded) > maxPageViewSize {
			logAction.AppendWarning("message", fmt.Sprintf("UserPreferences.PageViews.%s is too large, removing it", page))
			continue
		}
		pageViews[page] = view
	}
	UserPreferences.PageViews = pageViews

	return isValid
}

func ValidateLabelsAndTags(ctx context.Context, LabelsAndTags *Config_LabelsAndTags) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating LabelsAndTags Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	if len(LabelsAndTags.Applications) == 0 {
		return isValid
	}

	// If RemoveOverlayLabelOnlyOnPosterDownload is true, then we need to check if "Overlay" is in the remove list for the Plex application
	if LabelsAndTags.RemoveOverlayLabelOnlyOnPosterDownload {
		for _, app := range LabelsAndTags.Applications {
			if app.Application == "Plex" {
				if !stringSliceContains(app.Remove, "Overlay") {
					logging.LOGGER.Warn().Timestamp().Msg("LabelsAndTags.RemoveOverlayLabelOnlyOnPosterDownload is true, but 'Overlay' is not in the remove list for the Plex application. This setting will have no effect unless 'Overlay' is added to the remove list for the Plex application.")
					logAction.AppendWarning("message", "LabelsAndTags.RemoveOverlayLabelOnlyOnPosterDownload is true, but 'Overlay' is not in the remove list for the Plex application. This setting will have no effect unless 'Overlay' is added to the remove list for the Plex application.")
					LabelsAndTags.RemoveOverlayLabelOnlyOnPosterDownload = false
					break
				}
			}
		}
	}

	return isValid
}

// stringSliceContains checks if a string is present in a slice of strings.
func stringSliceContains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

// IsSetUp reports whether any of the required settings have been entered.
// A config that is not set up yet is not treated as having errors
func (config Config) IsSetUp() bool {
	return config.MediaServer.Type != "" ||
		config.MediaServer.URL != "" ||
		config.MediaServer.ApiToken != "" ||
		config.Mediux.ApiToken != ""
}

func ValidateBackups(ctx context.Context, Backups *Config_Backups) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Backups Config", logging.LevelTrace)
	defer logAction.Complete()

	// Backups.Keep: default to 7 backups, at most 100
	if Backups.Keep <= 0 {
		Backups.Keep = 7
	} else if Backups.Keep > 100 {
		logAction.SetError("Backups.Keep is too large", "Use a value between 1 and 100", nil)
		return false
	}
	return true
}

// OIDCCallbackPath is where the identity provider sends the browser back to after logging in
const OIDCCallbackPath = "/api/auth/oidc/callback"

// WithOIDCCallbackPath adds the callback path to an aura address, e.g. https://aura.example.com
func WithOIDCCallbackPath(redirectURL string) string {
	redirectURL = strings.TrimRight(strings.TrimSpace(redirectURL), "/")
	if redirectURL == "" || strings.HasSuffix(redirectURL, OIDCCallbackPath) {
		return redirectURL
	}
	// Finish a callback path that was only partly typed
	for _, partial := range []string{"/api/auth/oidc", "/api/auth", "/api"} {
		if strings.HasSuffix(redirectURL, partial) {
			return strings.TrimSuffix(redirectURL, partial) + OIDCCallbackPath
		}
	}
	return redirectURL + OIDCCallbackPath
}
