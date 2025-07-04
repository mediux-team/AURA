package config

import (
	"aura/internal/logging"
	"aura/internal/modals"
	"fmt"
	"os"
	"path"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// Global is a pointer to the global configuration instance.
// It is used throughout the application to access configuration settings.
var Global *modals.Config

// LoadYamlConfig loads the application configuration from a YAML file.
//
// Returns:
//   - An error if the configuration file is missing, unreadable, or invalid.
func LoadYamlConfig() logging.StandardError {
	Err := logging.NewStandardError()

	// Use an environment variable to determine the config path
	// By default, it will use /config
	// This is useful for testing and local development
	// In Docker, the config path is set to /config
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/config"
	}

	// Check for a config.yml or config.yaml file
	yamlFile := path.Join(configPath, "config.yml")
	if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
		yamlFile = path.Join(configPath, "config.yaml")
		if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
			Err.Message = fmt.Sprintf("config.yml or config.yaml file not found in %s", configPath)
			return Err
		}
	}

	// Read the YAML file
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		Err.Message = fmt.Sprintf("failed to read config.yml file: %s", err.Error())
		return Err
	}

	// Parse the YAML file into a Config struct
	var config modals.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		Err.Message = fmt.Sprintf("failed to parse config.yml file: %s", err.Error())
		return Err
	}

	Global = &config
	return logging.StandardError{}
}

func PrintConfig() {
	logging.LOG.NoTime("Current Configuration Settings\n")

	// Development Mode
	if Global.Dev.Enable {
		logging.LOG.NoTime("\tDevelopment Mode:\n")
		logging.LOG.NoTime("\t\tEnabled\n")
		logging.LOG.NoTime(fmt.Sprintf("\t\tDevelopment Local Path: %s\n", Global.Dev.LocalPath))
	}

	// Cache Images and Save Image Next To Content
	logging.LOG.NoTime(fmt.Sprintf("\tCache Images: %t\n", Global.CacheImages))
	logging.LOG.NoTime(fmt.Sprintf("\tSave Image Next To Content: %t\n", Global.SaveImageNextToContent))

	// Logging Configuration
	logging.LOG.NoTime(fmt.Sprintf("\tLogging Level: %s\n", Global.Logging.Level))

	// Media Server Configuration
	logging.LOG.NoTime("\tMedia Server\n")
	logging.LOG.NoTime(fmt.Sprintf("\t\tType: %s\n", Global.MediaServer.Type))
	logging.LOG.NoTime(fmt.Sprintf("\t\tURL: %s\n", Global.MediaServer.URL))
	logging.LOG.NoTime(fmt.Sprintf("\t\tToken: %s\n", "***"+Global.MediaServer.Token[len(Global.MediaServer.Token)-4:]))
	logging.LOG.NoTime(fmt.Sprintf("\t\tUserID: %s\n", Global.MediaServer.UserID))
	logging.LOG.NoTime("\t\tLibraries:\n")
	for _, library := range Global.MediaServer.Libraries {
		logging.LOG.NoTime(fmt.Sprintf("\t\t\tName: %s\n", library.Name))
	}
	logging.LOG.NoTime(fmt.Sprintf("\t\tSeason Naming Convention: %s\n", Global.MediaServer.SeasonNamingConvention))

	// Auto Download Configuration
	logging.LOG.NoTime("\tAuto Download\n")
	logging.LOG.NoTime(fmt.Sprintf("\t\tEnabled: %t\n", Global.AutoDownload.Enabled))
	logging.LOG.NoTime(fmt.Sprintf("\t\tCron: %s\n", Global.AutoDownload.Cron))

	// Notification Configuration
	logging.LOG.NoTime("\tNotification\n")
	logging.LOG.NoTime(fmt.Sprintf("\t\tProvider: %s\n", Global.Notification.Provider))
	logging.LOG.NoTime(fmt.Sprintf("\t\tWebhook: %s\n", MaskWebhookURL(Global.Notification.Webhook)))

	// Mediux Configuration
	logging.LOG.NoTime("\tMediux\n")
	logging.LOG.NoTime(fmt.Sprintf("\t\tToken: %s\n", "***"+Global.Mediux.Token[len(Global.Mediux.Token)-4:]))
	logging.LOG.NoTime(fmt.Sprintf("\t\tDownload Quality: %s\n", Global.Mediux.DownloadQuality))

	// TMDB Configuration
	if Global.TMDB.ApiKey != "" {
		logging.LOG.NoTime("\tTMDB\n")
		logging.LOG.NoTime(fmt.Sprintf("\t\tAPI Key: %s\n", "***"+Global.TMDB.ApiKey[len(Global.TMDB.ApiKey)-4:]))
	}

	// Kometa Configuration
	if len(Global.Kometa.Labels) > 0 {
		logging.LOG.NoTime("\tKometa\n")
		logging.LOG.NoTime(fmt.Sprintf("\t\tRemove Labels: %t\n", Global.Kometa.RemoveLabels))
		logging.LOG.NoTime("\t\tLabels:\n")
		for _, label := range Global.Kometa.Labels {
			logging.LOG.NoTime(fmt.Sprintf("\t\t\t%s\n", label))
		}
	}

}

func ValidateConfig() bool {

	// Check if Global is nil
	if Global == nil {
		return false
	}

	// Validate Logging configuration
	isLoggingValid := ValidateLoggingConfig()

	// Check MediaServer configuration
	isMediaServerValid := ValidateMediaServerConfig()

	// Validate AutoDownload configuration
	isAutoDownloadValid := ValidateAutoDownloadConfig()

	// Validate Mediux configuration
	isMediuxValid := ValidateMediuxConfig()

	if !isLoggingValid || !isMediaServerValid || !isAutoDownloadValid || !isMediuxValid {
		logging.LOG.Error("Invalid configuration file. See errors above.")
		return false
	}

	return true
}

func ValidateLoggingConfig() bool {
	isValid := true

	if Global.Logging.Level == "" {
		logging.LOG.Warn("\tLogging.Level is not set in the configuration file, using default level: INFO")
		Global.Logging.Level = "INFO"
	}

	if Global.Logging.Level != "TRACE" && Global.Logging.Level != "DEBUG" && Global.Logging.Level != "INFO" && Global.Logging.Level != "WARNING" && Global.Logging.Level != "ERROR" {
		logging.LOG.Warn(fmt.Sprintf("\tLogging.Level: '%s'. Must be one of: TRACE, DEBUG, INFO, WARNING, ERROR", Global.Logging.Level))
		Global.Logging.Level = "INFO"
	}

	logging.SetLogLevel(Global.Logging.Level)

	return isValid
}

func ValidateMediaServerConfig() bool {
	isValid := true

	if Global.MediaServer.Type == "" {
		logging.LOG.Warn("\tMediaServer.Type is not set in the configuration file")
		isValid = false
	}

	if Global.MediaServer.URL == "" {
		logging.LOG.Warn("\tMediaServer.URL is not set in the configuration file")
		isValid = false
	} else if !strings.HasPrefix(Global.MediaServer.URL, "http") {
		logging.LOG.Warn(fmt.Sprintf("\tMediaServer.URL: '%s' must start with http:// or https:// ", Global.MediaServer.URL))
		isValid = false
	}

	if Global.MediaServer.Token == "" {
		logging.LOG.Warn("\tMediaServer.Token is not set in the configuration file")
		isValid = false
	}

	if len(Global.MediaServer.Libraries) == 0 {
		logging.LOG.Warn("\tMediaServer.Libraries are not set in the configuration file")
		isValid = false
	}

	if !isValid {
		logging.LOG.Error("MediaServer configuration is invalid. Fix the errors above.")
		return false
	}

	// Set the MediaServer Type to Title Case
	Global.MediaServer.Type = cases.Title(language.English).String(Global.MediaServer.Type)

	// If the MediaServer type is not Plex, Emby, or Jellyfin, return an error
	if Global.MediaServer.Type != "Plex" && Global.MediaServer.Type != "Emby" && Global.MediaServer.Type != "Jellyfin" {
		logging.LOG.Error(fmt.Sprintf("\tMediaServer.Type: '%s'. Must be one of: Plex, Emby, Jellyfin", Global.MediaServer.Type))
		return false
	}

	// If the MediaServer type is Plex, set the SeasonNamingConvention to 2 if not set
	if Global.MediaServer.Type == "Plex" && Global.MediaServer.SeasonNamingConvention == "" {
		logging.LOG.Warn("\tMediaServer.SeasonNamingConvention is not set in the configuration file, using default value: 2")
		Global.MediaServer.SeasonNamingConvention = "2"
	}
	// If the SeasonNamingConvention is not 1 or 2, return an error
	if Global.MediaServer.Type == "Plex" && Global.MediaServer.SeasonNamingConvention != "1" && Global.MediaServer.SeasonNamingConvention != "2" {
		logging.LOG.Error(fmt.Sprintf("\tBad MediaServer.SeasonNamingConvention: '%s'. Must be one of: 1, 2", Global.MediaServer.SeasonNamingConvention))
		return false
	}

	// Trim the trailing slash from the URL
	Global.MediaServer.URL = strings.TrimSuffix(Global.MediaServer.URL, "/")

	// Set the Global.Notification.Provider to Title Case
	if Global.Notification.Provider != "" {
		Global.Notification.Provider = cases.Title(language.English).String(Global.Notification.Provider)
	}

	return true
}

func ValidateMediuxConfig() bool {
	isValid := true

	if Global.Mediux.Token == "" {
		logging.LOG.Warn("\tMediux.Token is not set in the configuration file")
		return false
	}

	if Global.Mediux.DownloadQuality == "" {
		logging.LOG.Warn("\tMediux.DownloadQuality is not set in the configuration file, using default value: optimized")
		Global.Mediux.DownloadQuality = "optimized"
	}

	if Global.Mediux.DownloadQuality != "original" && Global.Mediux.DownloadQuality != "optimized" {
		logging.LOG.Error(fmt.Sprintf("\tBad Mediux.DownloadQuality: '%s'. Must be one of: original, optimized", Global.Mediux.DownloadQuality))
		isValid = false
	}

	return isValid
}

func ValidateAutoDownloadConfig() bool {
	isValid := true

	if Global.AutoDownload.Cron == "" {
		logging.LOG.Warn("\tAutoDownload.Cron is not set in the configuration file, using default value: 0 0 * * *")
		Global.AutoDownload.Cron = "0 0 * * *" // Default to daily at midnight
	}

	if !Global.AutoDownload.Enabled {
		logging.LOG.Warn("\tAutoDownload is disabled in the configuration file")
	}

	return isValid
}
