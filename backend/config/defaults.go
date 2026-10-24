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
		NewItemAutoDownload: Config_CustomNotification{
			Enabled:      true,
			Title:        "Download Queue | {{ReasonTitle}}",
			Message:      "{{MediaItemTitle}}{{NewLine}}Set ID: {{SetID}}{{NewLine}}{{NewLine}}{{Reason}}",
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
		SiloPlugin: Config_CustomNotification{
			Enabled:      true,
			Title:        "Silo Plugin | {{ReasonTitle}}",
			Message:      "{{MediaItemTitle}} ({{MediaItemLibraryTitle}}){{NewLine}}Set ID: {{SetID}}{{NewLine}}{{NewLine}}{{Reason}}",
			IncludeImage: true,
		},
	}
}

func DefaultConfig() Config {
	return Config{
		Auth: Config_Auth{
			Enabled:             false,
			PasswordEnabled:     true,
			SessionCookieSecure: "auto",
		},
		Logging: Config_Logging{
			Level:     "INFO",
			KeepDays:  7,
			KeepFiles: 7,
		},
		Mediux: Config_Mediux{
			DownloadQuality: "optimized",
		},
		Jobs: DefaultJobsConfig(),
		Backups: Config_Backups{
			Keep: 7,
		},
		Images: Config_Images{
			CacheImages: Config_CacheImages{
				Enabled: false,
			},
			SaveImagesLocally: Config_SaveImagesLocally{
				Enabled:       false,
				NameTemplates: DefaultImageNameTemplates(),
			},
		},
		Notifications: Config_Notifications{
			Enabled:              false,
			Providers:            []Config_Notification_Provider{},
			NotificationTemplate: DefaultNotificationTemplates(),
		},
		UserPreferences: DefaultUserPreferences(),
	}
}

// Image types that can be selected in the Download Modal
var DownloadImageTypes = []string{"poster", "backdrop", "season_poster", "special_season_poster", "titlecard", "collection_poster", "collection_backdrop"}

func DefaultUserPreferences() Config_UserPreferences {
	return Config_UserPreferences{
		DownloadDefaults:                 append([]string{}, DownloadImageTypes...),
		ShowOnlyDownloadDefaults:         false,
		ShowDateModified:                 false,
		DefaultAddToQueue:                false,
		DefaultAutoAddNewCollectionItems: false,
		DefaultAutoUpgrade:               true,
		DefaultAddToDBOnly:               false,
		AutoDownloadNewItems:             false,
		AutoUpgradeSets:                  false,
		FavoriteCreators:                 []string{},
		PageViews:                        map[string]map[string]any{},
	}
}

var JobDefaults = struct {
	AutoDownload                    JobSettingDefault
	RefreshMediaItemsAndCollections JobSettingDefault
	CheckForMediaItemChanges        JobSettingDefault
	CheckForNewMediaItems           JobSettingDefault
	HandleTempIgnoredItems          JobSettingDefault
	RefreshMediuxUsers              JobSettingDefault
	CheckMediuxSiteLink             JobSettingDefault
	BackupDatabase                  JobSettingDefault
	UpgradeSavedSets                JobSettingDefault
}{
	AutoDownload:                    JobSettingDefault{Enabled: true, Cron: "0 0 * * *"},
	RefreshMediaItemsAndCollections: JobSettingDefault{Enabled: true, Cron: "0 4 * * *"},
	CheckForMediaItemChanges:        JobSettingDefault{Enabled: true, Cron: "0 */6 * * *"},
	CheckForNewMediaItems:           JobSettingDefault{Enabled: true, Cron: "*/10 * * * *"},
	HandleTempIgnoredItems:          JobSettingDefault{Enabled: true, Cron: "0 */6 * * *"},
	RefreshMediuxUsers:              JobSettingDefault{Enabled: true, Cron: "0 */12 * * *"},
	CheckMediuxSiteLink:             JobSettingDefault{Enabled: true, Cron: "0 */1 * * *"},
	BackupDatabase:                  JobSettingDefault{Enabled: true, Cron: "0 3 * * *"},
	UpgradeSavedSets:                JobSettingDefault{Enabled: true, Cron: "0 4 * * *"},
}

func boolPtr(b bool) *bool { return new(b) }

func DefaultJobsConfig() Config_Jobs {
	return Config_Jobs{
		AutoDownload: Config_JobSetting{
			Enabled: boolPtr(JobDefaults.AutoDownload.Enabled),
			Cron:    JobDefaults.AutoDownload.Cron,
		},
		RefreshMediaItemsAndCollections: Config_JobSetting{
			Enabled: boolPtr(JobDefaults.RefreshMediaItemsAndCollections.Enabled),
			Cron:    JobDefaults.RefreshMediaItemsAndCollections.Cron,
		},
		CheckForMediaItemChanges: Config_JobSetting{
			Enabled: boolPtr(JobDefaults.CheckForMediaItemChanges.Enabled),
			Cron:    JobDefaults.CheckForMediaItemChanges.Cron,
		},
		CheckForNewMediaItems: Config_JobSetting{
			Enabled: boolPtr(JobDefaults.CheckForNewMediaItems.Enabled),
			Cron:    JobDefaults.CheckForNewMediaItems.Cron,
		},
		HandleTempIgnoredItems: Config_JobSetting{
			Enabled: boolPtr(JobDefaults.HandleTempIgnoredItems.Enabled),
			Cron:    JobDefaults.HandleTempIgnoredItems.Cron,
		},
		RefreshMediuxUsers: Config_JobSetting{
			Enabled: boolPtr(JobDefaults.RefreshMediuxUsers.Enabled),
			Cron:    JobDefaults.RefreshMediuxUsers.Cron,
		},
		CheckMediuxSiteLink: Config_JobSetting{
			Enabled: boolPtr(JobDefaults.CheckMediuxSiteLink.Enabled),
			Cron:    JobDefaults.CheckMediuxSiteLink.Cron,
		},
		BackupDatabase: Config_JobSetting{
			Enabled: boolPtr(JobDefaults.BackupDatabase.Enabled),
			Cron:    JobDefaults.BackupDatabase.Cron,
		},
		UpgradeSavedSets: Config_JobSetting{
			Enabled: boolPtr(JobDefaults.UpgradeSavedSets.Enabled),
			Cron:    JobDefaults.UpgradeSavedSets.Cron,
		},
	}
}
