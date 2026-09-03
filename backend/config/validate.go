package config

import (
	"aura/logging"
	"context"
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

	// Sub-action: MediUX Config
	isMediuxValid := ValidateMediux(ctx, &config.Mediux)

	// Sub-action: AutoDownload Config
	isAutoDownloadValid := ValidateAutoDownload(ctx, &config.AutoDownload)

	// Sub-action: Jobs Config
	isJobsValid := ValidateJobs(ctx, &config.Jobs)

	// Sub-action: Images Config
	isImagesValid := ValidateImages(ctx, &config.Images, config.MediaServer)

	// Sub-action: Notifications Config
	isNotificationsValid := ValidateNotifications(ctx, &config.Notifications)

	// Sub-action: SonarrRadarr Config
	isSonarrRadarrValid := ValidateSonarrRadarr(ctx, &config.SonarrRadarr, config.MediaServer)

	// Sub-action: Database Config
	isDatabaseValid := ValidateDatabase(ctx, &config.Database)

	// Sub-action: LabelsAndTags Config
	isLabelsAndTagsValid := ValidateLabelsAndTags(ctx, &config.LabelsAndTags)

	// If any validation failed, set status to error
	if !isAuthValid || !isLoggingValid || !isMediaServerValid ||
		!isMediuxValid || !isAutoDownloadValid || !isJobsValid ||
		!isImagesValid || !isNotificationsValid || !isSonarrRadarrValid || !isDatabaseValid || !isLabelsAndTagsValid {
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

	if Auth.Enabled {
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
		if Auth.OIDC.IssuerURL == "" || Auth.OIDC.ClientID == "" || Auth.OIDC.ClientSecret == "" || Auth.OIDC.RedirectURL == "" {
			logAction.SetError("Auth.OIDC is missing required fields", "IssuerURL, ClientID, ClientSecret, and RedirectURL are all required when OIDC is enabled", nil)
			isValid = false
		} else {
			if _, err := url.ParseRequestURI(Auth.OIDC.IssuerURL); err != nil {
				logAction.SetError("Auth.OIDC.IssuerURL is not a valid URL", err.Error(), nil)
				isValid = false
			}
			if _, err := url.ParseRequestURI(Auth.OIDC.RedirectURL); err != nil {
				logAction.SetError("Auth.OIDC.RedirectURL is not a valid URL", err.Error(), nil)
				isValid = false
			}
		}
	}

	switch Auth.SessionCookieSecure {
	case "", "auto", "always", "never":
		// valid
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

	return isValid
}

func ValidateMediaServer(ctx context.Context, MediaServer *Config_MediaServer) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating MediaServer Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// Title case the MediaServer.Type
	MediaServer.Type = cases.Title(language.English).String(MediaServer.Type)

	// Check if MediaServer.Type is set
	if MediaServer.Type == "" {
		logAction.SetError("MediaServer.Type is not set", "Media server type must be specified", nil)
		isValid = false
	} else if MediaServer.Type != "Plex" && MediaServer.Type != "Emby" && MediaServer.Type != "Jellyfin" {
		logAction.SetError(fmt.Sprintf("MediaServer.Type: '%s' is not a valid type", MediaServer.Type), "Valid types are Plex, Emby, Jellyfin", nil)
		isValid = false
	}

	// Check if MediaServer.URL is set
	if MediaServer.URL == "" {
		logAction.SetError("MediaServer.URL is not set", "Media server URL must be specified", nil)
		isValid = false
	} else if !strings.HasPrefix(MediaServer.URL, "http://") && !strings.HasPrefix(MediaServer.URL, "https://") {
		logAction.SetError(fmt.Sprintf("MediaServer.URL: '%s' must start with http:// or https:// ", MediaServer.URL), "", nil)
		isValid = false
	}

	// Check if MediaServer.ApiToken is set
	if MediaServer.ApiToken == "" {
		logAction.SetError("MediaServer.ApiToken is not set", "Media server ApiToken must be specified", nil)
		isValid = false
	}

	// Check if MediaServer.Libraries is set
	if len(MediaServer.Libraries) == 0 {
		logAction.SetError("MediaServer.Libraries is not set", "At least one media server library must be specified", nil)
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

func ValidateAutoDownload(ctx context.Context, AutoDownload *Config_AutoDownload) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating AutoDownload Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// Check if AutoDownload is enabled
	if !AutoDownload.Enabled {
		return isValid
	}

	// Check if AutoDownload.Cron is set
	if AutoDownload.Cron == "" {
		AutoDownload.Cron = "0 0 * * *"
		logging.LOGGER.Warn().Timestamp().Msg("AutoDownload.Cron not set, defaulting to '0 0 * * *' (every day at midnight)")
		logAction.AppendWarning("message", "AutoDownload.Cron not set, defaulting to '0 0 * * *' (every day at midnight)")
	}

	// Validate the cron expression
	if !ValidateCron(AutoDownload.Cron) {
		logAction.SetError(fmt.Sprintf("AutoDownload.Cron: '%s' is not a valid cron expression", AutoDownload.Cron), "Please provide a valid cron expression", nil)
		isValid = false
	}

	return isValid
}

func ValidateCron(cronExpression string) bool {
	_, err := cron.ParseStandard(cronExpression)
	return err == nil
}

func ValidateJobs(ctx context.Context, Jobs *Config_Jobs) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Jobs Config", logging.LevelTrace)
	if logAction != nil {
		defer logAction.Complete()
	}

	isValid := true
	defaults := DefaultJobsConfig()

	validateJob := func(name string, setting *Config_JobSetting, defaultSetting Config_JobSetting) {
		if setting.Enabled == nil {
			setting.Enabled = defaultSetting.Enabled
		}
		if setting.Cron == "" {
			setting.Cron = defaultSetting.Cron
			logging.LOGGER.Warn().Timestamp().Msgf("Jobs.%s.Cron not set, defaulting to '%s'", name, defaultSetting.Cron)
			if logAction != nil {
				logAction.AppendWarning("message", fmt.Sprintf("Jobs.%s.Cron not set, defaulting to '%s'", name, defaultSetting.Cron))
			}
		}
		if setting.IsEnabled() && !ValidateCron(setting.Cron) {
			if logAction != nil {
				logAction.SetError(fmt.Sprintf("Jobs.%s.Cron: '%s' is not a valid cron expression", name, setting.Cron), "Please provide a valid cron expression", nil)
			}
			isValid = false
		}
	}

	validateJob("RefreshMediaItemsAndCollections", &Jobs.RefreshMediaItemsAndCollections, defaults.RefreshMediaItemsAndCollections)
	validateJob("CheckForMediaItemChanges", &Jobs.CheckForMediaItemChanges, defaults.CheckForMediaItemChanges)
	validateJob("HandleTempIgnoredItems", &Jobs.HandleTempIgnoredItems, defaults.HandleTempIgnoredItems)
	validateJob("RefreshMediuxUsers", &Jobs.RefreshMediuxUsers, defaults.RefreshMediuxUsers)
	validateJob("CheckMediuxSiteLink", &Jobs.CheckMediuxSiteLink, defaults.CheckMediuxSiteLink)

	return isValid
}

func ValidateImages(ctx context.Context, Images *Config_Images, msConfig Config_MediaServer) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Images Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// If Images.SaveImagesLocally.Enabled is true, validate the EpisodeNamingConvention
	if Images.SaveImagesLocally.Enabled {
		if msConfig.Type != "Plex" {
			return isValid
		}

		validEpisodeNamingConventions := []string{"match", "static"}

		Images.SaveImagesLocally.EpisodeNamingConvention = strings.ToLower(Images.SaveImagesLocally.EpisodeNamingConvention)
		if !stringSliceContains(validEpisodeNamingConventions, Images.SaveImagesLocally.EpisodeNamingConvention) {
			if msConfig.Type == "Plex" && Images.SaveImagesLocally.EpisodeNamingConvention == "" {
				Images.SaveImagesLocally.EpisodeNamingConvention = "match"
				logAction.AppendWarning("message", "Images.SaveImagesLocally.EpisodeNamingConvention not set, defaulting to 'match'")
			} else if msConfig.Type == "Plex" && Images.SaveImagesLocally.EpisodeNamingConvention != "match" && Images.SaveImagesLocally.EpisodeNamingConvention != "static" {
				Images.SaveImagesLocally.EpisodeNamingConvention = "match"
				logAction.AppendWarning("message", "Images.SaveImagesLocally.EpisodeNamingConvention invalid, defaulting to 'match'")
			}
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

	// If notifications are enabled, valiate the templates
	if Notifications.NotificationTemplate.AppStartup == (Config_CustomNotification{}) {
		Notifications.NotificationTemplate.AppStartup = defaults.AppStartup
		logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.AppStartup not set, defaulting to built-in template")
		logAction.AppendWarning("message", "Notifications.NotificationTemplate.AppStartup not set, defaulting to built-in template")
	} else {
		validAppStartupVariables := AllowedTemplateVariables(TemplateTypeAppStartup)
		if !validateTemplateVariables(Notifications.NotificationTemplate.AppStartup.Title, validAppStartupVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.AppStartup.Title contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.AppStartup.Title contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.AppStartup.Title = defaults.AppStartup.Title
		}
		if !validateTemplateVariables(Notifications.NotificationTemplate.AppStartup.Message, validAppStartupVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.AppStartup.Message contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.AppStartup.Message contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.AppStartup.Message = defaults.AppStartup.Message
		}
	}

	if Notifications.NotificationTemplate.TestNotification == (Config_CustomNotification{}) {
		Notifications.NotificationTemplate.TestNotification = defaults.TestNotification
		logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.TestNotification not set, defaulting to built-in template")
		logAction.AppendWarning("message", "Notifications.NotificationTemplate.TestNotification not set, defaulting to built-in template")
	} else {
		validTestNotificationVariables := AllowedTemplateVariables(TemplateTypeTestNotification)
		if !validateTemplateVariables(Notifications.NotificationTemplate.TestNotification.Title, validTestNotificationVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.TestNotification.Title contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.TestNotification.Title contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.TestNotification.Title = defaults.TestNotification.Title
		}
		if !validateTemplateVariables(Notifications.NotificationTemplate.TestNotification.Message, validTestNotificationVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.TestNotification.Message contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.TestNotification.Message contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.TestNotification.Message = defaults.TestNotification.Message
		}
	}

	if Notifications.NotificationTemplate.Autodownload == (Config_CustomNotification{}) {
		Notifications.NotificationTemplate.Autodownload = defaults.Autodownload
		logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.Autodownload not set, defaulting to built-in template")
		logAction.AppendWarning("message", "Notifications.NotificationTemplate.Autodownload not set, defaulting to built-in template")
	} else {
		validAutodownloadVariables := AllowedTemplateVariables(TemplateTypeAutodownload)
		if !validateTemplateVariables(Notifications.NotificationTemplate.Autodownload.Title, validAutodownloadVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.Autodownload.Title contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.Autodownload.Title contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.Autodownload.Title = defaults.Autodownload.Title
		}
		if !validateTemplateVariables(Notifications.NotificationTemplate.Autodownload.Message, validAutodownloadVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.Autodownload.Message contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.Autodownload.Message contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.Autodownload.Message = defaults.Autodownload.Message
		}
	}

	if Notifications.NotificationTemplate.DownloadQueue == (Config_CustomNotification{}) {
		Notifications.NotificationTemplate.DownloadQueue = defaults.DownloadQueue
		logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.DownloadQueue not set, defaulting to built-in template")
		logAction.AppendWarning("message", "Notifications.NotificationTemplate.DownloadQueue not set, defaulting to built-in template")
	} else {
		validDownloadQueueVariables := AllowedTemplateVariables(TemplateTypeDownloadQueue)
		if !validateTemplateVariables(Notifications.NotificationTemplate.DownloadQueue.Title, validDownloadQueueVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.DownloadQueue.Title contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.DownloadQueue.Title contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.DownloadQueue.Title = defaults.DownloadQueue.Title
		}
		if !validateTemplateVariables(Notifications.NotificationTemplate.DownloadQueue.Message, validDownloadQueueVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.DownloadQueue.Message contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.DownloadQueue.Message contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.DownloadQueue.Message = defaults.DownloadQueue.Message
		}
	}

	if Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems == (Config_CustomNotification{}) {
		Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems = defaults.NewSetsAvailableForIgnoredItems
		logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems not set, defaulting to built-in template")
		logAction.AppendWarning("message", "Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems not set, defaulting to built-in template")
	} else {
		validNewSetsAvailableForIgnoredItemsVariables := AllowedTemplateVariables(TemplateTypeNewSetsAvailableForIgnoredItems)
		if !validateTemplateVariables(Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Title, validNewSetsAvailableForIgnoredItemsVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Title contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Title contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Title = defaults.NewSetsAvailableForIgnoredItems.Title
		}
		if !validateTemplateVariables(Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Message, validNewSetsAvailableForIgnoredItemsVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Message contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Message contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.NewSetsAvailableForIgnoredItems.Message = defaults.NewSetsAvailableForIgnoredItems.Message
		}
	}

	if Notifications.NotificationTemplate.CheckForMediaItemChangesJob == (Config_CustomNotification{}) {
		Notifications.NotificationTemplate.CheckForMediaItemChangesJob = defaults.CheckForMediaItemChangesJob
		logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.CheckForMediaItemChangesJob not set, defaulting to built-in template")
		logAction.AppendWarning("message", "Notifications.NotificationTemplate.CheckForMediaItemChangesJob not set, defaulting to built-in template")
	} else {
		validCheckForMediaItemChangesJobVariables := AllowedTemplateVariables(TemplateTypeCheckForMediaItemChangesJob)
		if !validateTemplateVariables(Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Title, validCheckForMediaItemChangesJobVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Title contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Title contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Title = defaults.CheckForMediaItemChangesJob.Title
		}
		if !validateTemplateVariables(Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Message, validCheckForMediaItemChangesJobVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Message contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Message contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.CheckForMediaItemChangesJob.Message = defaults.CheckForMediaItemChangesJob.Message
		}
	}

	if Notifications.NotificationTemplate.SonarrNotification == (Config_CustomNotification{}) {
		Notifications.NotificationTemplate.SonarrNotification = defaults.SonarrNotification
		logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.SonarrNotification not set, defaulting to built-in template")
		logAction.AppendWarning("message", "Notifications.NotificationTemplate.SonarrNotification not set, defaulting to built-in template")
	} else {
		validSonarrNotificationVariables := AllowedTemplateVariables(TemplateTypeSonarrNotification)
		if !validateTemplateVariables(Notifications.NotificationTemplate.SonarrNotification.Title, validSonarrNotificationVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.SonarrNotification.Title contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.SonarrNotification.Title contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.SonarrNotification.Title = defaults.SonarrNotification.Title
		}
		if !validateTemplateVariables(Notifications.NotificationTemplate.SonarrNotification.Message, validSonarrNotificationVariables) {
			logging.LOGGER.Warn().Timestamp().Msg("Notifications.NotificationTemplate.SonarrNotification.Message contains invalid variables, please check the config documentation for valid variables")
			logAction.AppendWarning("message", "Notifications.NotificationTemplate.SonarrNotification.Message contains invalid variables, please check the config documentation for valid variables")
			Notifications.NotificationTemplate.SonarrNotification.Message = defaults.SonarrNotification.Message
		}
	}

	return isValid
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

func ValidateDatabase(ctx context.Context, Database *Config_Database) bool {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Validating Database Config", logging.LevelTrace)
	defer logAction.Complete()

	isValid := true

	// If Database.Type is not set, set to default "sqlite3"
	if Database.Type == "" {
		Database.Type = "sqlite3"
		logAction.AppendWarning("message", "Database.Type not set, defaulting to 'sqlite3'")
	} else if Database.Type != "sqlite3" && Database.Type != "mysql" && Database.Type != "postgresql" {
		logAction.AppendWarning("message", "Database.Type invalid, defaulting to 'sqlite3'")
		Database.Type = "sqlite3"
		Database.Path = "AURA.db"
	}

	switch Database.Type {
	case "sqlite3":
		if Database.Path == "" {
			Database.Path = "AURA.db"
		}
	}

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
