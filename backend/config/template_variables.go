package config

const (
	TemplateTypeAppStartup                      = "app_startup"
	TemplateTypeTestNotification                = "test_notification"
	TemplateTypeAutodownload                    = "autodownload"
	TemplateTypeDownloadQueue                   = "download_queue"
	TemplateTypeNewItemAutoDownload             = "new_item_auto_download"
	TemplateTypeNewSetsAvailableForIgnoredItems = "new_sets_available_for_ignored_items"
	TemplateTypeCheckForMediaItemChangesJob     = "check_for_media_item_changes_job"
	TemplateTypeSonarrNotification              = "sonarr_notification"
	TemplateTypeSiloPlugin                      = "silo_plugin"
)

var BaseTemplateVariables = []string{
	"{{AppName}}",
	"{{AppVersion}}",
	"{{AppPort}}",
	"{{AppAuthor}}",
	"{{AppLicense}}",
	"{{MediaServerName}}",
	"{{MediaServerType}}",
	"{{Timestamp}}",
	"{{NewLine}}",
	"{{Tab}}",
}

var MediaItemVariables = []string{
	"{{MediaItemTitle}}",
	"{{MediaItemYear}}",
	"{{MediaItemTMDBID}}",
	"{{MediaItemLibraryTitle}}",
	"{{MediaItemRatingKey}}",
	"{{MediaItemType}}",
}

var SetItemVariables = []string{
	"{{SetID}}",
	"{{SetTitle}}",
	"{{SetType}}",
	"{{SetCreator}}",
}

var ImageVariables = []string{
	"{{ImageName}}",
	"{{ImageType}}",
}

var DownloadReasonVariables = []string{
	"{{ReasonTitle}}",
	"{{Reason}}",
}

type NotificationTemplateVariableCatalog struct {
	TemplateVariables map[string][]string         `json:"template_variables"`
	DefaultTemplates  Config_NotificationTemplate `json:"default_templates"` // The built-in templates, for Restore Default
}

func AllowedTemplateVariables(templateType string) []string {
	switch templateType {
	case TemplateTypeAppStartup:
		return mergeTemplateVariableGroups(BaseTemplateVariables)
	case TemplateTypeTestNotification:
		return mergeTemplateVariableGroups(BaseTemplateVariables)
	case TemplateTypeAutodownload:
		return mergeTemplateVariableGroups(
			BaseTemplateVariables,
			MediaItemVariables,
			SetItemVariables,
			ImageVariables,
			DownloadReasonVariables,
		)
	case TemplateTypeDownloadQueue:
		return mergeTemplateVariableGroups(
			BaseTemplateVariables,
			MediaItemVariables,
			SetItemVariables,
			DownloadReasonVariables,
			[]string{
				"{{Source}}",
			},
		)
	case TemplateTypeNewItemAutoDownload, TemplateTypeSiloPlugin:
		return mergeTemplateVariableGroups(
			BaseTemplateVariables,
			MediaItemVariables,
			SetItemVariables,
			DownloadReasonVariables,
		)
	case TemplateTypeNewSetsAvailableForIgnoredItems:
		return mergeTemplateVariableGroups(
			BaseTemplateVariables,
			MediaItemVariables,
			[]string{
				"{{SetCount}}",
			},
		)
	case TemplateTypeCheckForMediaItemChangesJob:
		return mergeTemplateVariableGroups(
			BaseTemplateVariables,
			MediaItemVariables,
			[]string{
				"{{Reason}}",
				"{{Action}}",
				"{{MoreInfo}}",
			},
		)
	case TemplateTypeSonarrNotification:
		return mergeTemplateVariableGroups(
			BaseTemplateVariables,
			MediaItemVariables,
			SetItemVariables,
			ImageVariables,
			DownloadReasonVariables,
			[]string{
				"{{Result}}",
			},
		)
	default:
		return []string{}
	}
}

func GetNotificationTemplateVariableCatalog() NotificationTemplateVariableCatalog {
	return NotificationTemplateVariableCatalog{
		TemplateVariables: map[string][]string{
			TemplateTypeAppStartup:                      AllowedTemplateVariables(TemplateTypeAppStartup),
			TemplateTypeTestNotification:                AllowedTemplateVariables(TemplateTypeTestNotification),
			TemplateTypeAutodownload:                    AllowedTemplateVariables(TemplateTypeAutodownload),
			TemplateTypeDownloadQueue:                   AllowedTemplateVariables(TemplateTypeDownloadQueue),
			TemplateTypeNewItemAutoDownload:             AllowedTemplateVariables(TemplateTypeNewItemAutoDownload),
			TemplateTypeNewSetsAvailableForIgnoredItems: AllowedTemplateVariables(TemplateTypeNewSetsAvailableForIgnoredItems),
			TemplateTypeCheckForMediaItemChangesJob:     AllowedTemplateVariables(TemplateTypeCheckForMediaItemChangesJob),
			TemplateTypeSonarrNotification:              AllowedTemplateVariables(TemplateTypeSonarrNotification),
			TemplateTypeSiloPlugin:                      AllowedTemplateVariables(TemplateTypeSiloPlugin),
		},
		DefaultTemplates: DefaultNotificationTemplates(),
	}
}

func mergeTemplateVariableGroups(groups ...[]string) []string {
	seen := make(map[string]struct{})
	merged := make([]string, 0)

	for _, group := range groups {
		for _, variable := range group {
			if _, ok := seen[variable]; ok {
				continue
			}
			seen[variable] = struct{}{}
			merged = append(merged, variable)
		}
	}

	return merged
}
