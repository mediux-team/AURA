package config

func DefaultNotificationTemplates() Config_NotificationTemplate {
	return Config_NotificationTemplate{
		AppStartup: Config_CustomNotification{
			Enabled: true,
			Title:   "{{AppName}} | Start Up",
			Message: "{{AppName}} backend server API has started{{NewLine}}Version: v{{AppVersion}}{{NewLine}}Server Name: {{MediaServerName}}{{NewLine}}Port: {{AppPort}}{{NewLine}}{{Timestamp}}",
		},
		TestNotification: Config_CustomNotification{
			Enabled: true,
			Title:   "Test Notification",
			Message: "This is a test notification from {{AppName}}. If you received this, your notification settings are correctly configured!",
		},
		Autodownload: Config_CustomNotification{
			Enabled:      true,
			Title:        "Auto Download | {{ReasonTitle}}",
			Message:      "{{MediaItemTitle}} ({{MediaItemLibraryTitle}}){{NewLine}}{{ImageName}}{{NewLine}}Set ID: {{SetID}}{{NewLine}}{{NewLine}}Reason:{{NewLine}}{{Reason}}",
			IncludeImage: true,
		},
		DownloadQueue: Config_CustomNotification{
			Enabled:      true,
			Title:        "Download Queue | {{ReasonTitle}}",
			Message:      "{{MediaItemTitle}} ({{MediaItemLibraryTitle}}){{NewLine}}Set ID: {{SetID}}{{NewLine}}{{NewLine}}{{Reason}}",
			IncludeImage: true,
		},
		NewSetsAvailableForIgnoredItems: Config_CustomNotification{
			Enabled:      true,
			Title:        "New Sets Available for Ignored Item",
			Message:      "A new set has been detected for the previously ignored item {{MediaItemTitle}} ({{MediaItemLibraryTitle}}). It is now part of {{SetCount}} set(s) in MediUX, and will no longer be ignored in {{AppName}}.",
			IncludeImage: true,
		},
		CheckForMediaItemChangesJob: Config_CustomNotification{
			Enabled:      true,
			Title:        "Check For Media Item Changes Job",
			Message:      "The media item '{{MediaItemTitle}}' (TMDB ID: {{MediaItemTMDBID}}) in library '{{MediaItemLibraryTitle}}' could not be found in the media server cache.{{NewLine}}Reason:{{NewLine}}{{Reason}}{{NewLine}}{{NewLine}}{{Action}}{{NewLine}}{{MoreInfo}}",
			IncludeImage: false,
		},
		SonarrNotification: Config_CustomNotification{
			Enabled:      true,
			Title:        "Sonarr | {{ReasonTitle}}",
			Message:      "{{MediaItemTitle}}{{NewLine}}{{ImageName}}{{NewLine}}Set ID: {{SetID}}{{NewLine}}Reason:{{NewLine}}{{Reason}}{{NewLine}}{{Result}}",
			IncludeImage: true,
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func DefaultJobsConfig() Config_Jobs {
	return Config_Jobs{
		RefreshMediaItemsAndCollections: Config_JobSetting{
			Enabled: boolPtr(true),
			Cron:    "0 4 * * *",
		},
		CheckForMediaItemChanges: Config_JobSetting{
			Enabled: boolPtr(true),
			Cron:    "0 */6 * * *",
		},
		HandleTempIgnoredItems: Config_JobSetting{
			Enabled: boolPtr(true),
			Cron:    "0 */6 * * *",
		},
		RefreshMediuxUsers: Config_JobSetting{
			Enabled: boolPtr(true),
			Cron:    "0 */12 * * *",
		},
		CheckMediuxSiteLink: Config_JobSetting{
			Enabled: boolPtr(true),
			Cron:    "0 */1 * * *",
		},
	}
}

func DefaultConfig() Config {
	return Config{
		Auth: Config_Auth{
			Enabled:             false,
			SessionCookieSecure: "auto",
		},
		Logging: Config_Logging{
			Level: "INFO",
		},
		Mediux: Config_Mediux{
			DownloadQuality: "optimized",
		},
		AutoDownload: Config_AutoDownload{
			Enabled: false,
			Cron:    "0 0 * * *",
		},
		Jobs: DefaultJobsConfig(),
		Images: Config_Images{
			CacheImages: Config_CacheImages{
				Enabled: false,
			},
			SaveImagesLocally: Config_SaveImagesLocally{
				Enabled: false,
			},
		},
		Notifications: Config_Notifications{
			Enabled:              false,
			Providers:            []Config_Notification_Provider{},
			NotificationTemplate: DefaultNotificationTemplates(),
		},
	}
}
