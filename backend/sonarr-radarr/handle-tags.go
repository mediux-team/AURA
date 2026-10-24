package sonarr_radarr

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"strconv"
)

func (s *SonarrApp) HandleTags(ctx context.Context, app config.Config_SonarrRadarrApp, item models.MediaItem, selectedTypes models.SelectedTypes) (Err logging.LogErrorInfo) {
	return srHandleTags(ctx, app, item, selectedTypes)
}

func (r *RadarrApp) HandleTags(ctx context.Context, app config.Config_SonarrRadarrApp, item models.MediaItem, selectedTypes models.SelectedTypes) (Err logging.LogErrorInfo) {
	return srHandleTags(ctx, app, item, selectedTypes)
}

func HandleTags(ctx context.Context, item models.MediaItem, selectedTypes models.SelectedTypes) (Err logging.LogErrorInfo) {
	Err = logging.LogErrorInfo{}
	if len(config.Current.SonarrRadarr.Applications) == 0 {
		return Err
	}

	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Handling Tags for %s", utils.MediaItemInfo(item)), logging.LevelInfo)
	defer logAction.Complete()

	// If the item type is not movie or show return
	if item.Type != "movie" && item.Type != "show" {
		logAction.SetError("Unsupported Media Item Type",
			"Only media items of type 'movie' or 'show' are supported for tag handling",
			map[string]any{
				"title":         item.Title,
				"library_title": item.LibraryTitle,
				"tmdb_id":       item.TMDB_ID,
				"item_type":     item.Type,
			})
		return *logAction.Error
	}

	// If the media item doesn't have a TMDB ID or Library Title return
	if item.TMDB_ID == "" || item.LibraryTitle == "" {
		logAction.SetError("Media Item Missing TMDB ID or Library Title",
			"Ensure the media item has both a TMDB ID and Library Title to proceed with tag handling",
			map[string]any{
				"title":         item.Title,
				"library_title": item.LibraryTitle,
				"tmdb_id":       item.TMDB_ID,
			})
		return *logAction.Error
	}

	for _, srApp := range config.Current.SonarrRadarr.Applications {
		// Make sure all the app info is present
		Err = MakeSureAllAppInfoPresent(ctx, &srApp)
		if Err.Message != "" {
			continue
		}

		// Only handle the app if it matches the media item type
		if (item.Type == "movie" && srApp.Type != "Radarr") || (item.Type == "show" && srApp.Type != "Sonarr") {
			continue
		}

		// If the library title doesn't match continue
		if srApp.Library != item.LibraryTitle {
			continue
		}

		ctx, actionApp := logging.AddSubActionToContext(ctx, fmt.Sprintf("Handling Tags for %s | %s", srApp.Type, srApp.Library), logging.LevelInfo)
		defer actionApp.Complete()

		interfaceSR, Err := NewSonarrRadarrInterface(ctx, srApp)
		if Err.Message != "" {
			actionApp.SetError("Failed to Create Sonarr/Radarr Interface",
				"Ensure the Sonarr/Radarr Type, Library, URL, and API Token are set correctly in the config file",
				map[string]any{
					"type":    srApp.Type,
					"library": srApp.Library,
					"url":     srApp.URL,
				})
			continue
		}

		// Handle updating the item tags
		Err = interfaceSR.HandleTags(ctx, srApp, item, selectedTypes)
		if Err.Message != "" {
			actionApp.SetError("Failed to Update Sonarr/Radarr Item Tags",
				"Ensure the Sonarr/Radarr server is reachable and the media item exists in the specified library",
				map[string]any{
					"title":         item.Title,
					"library_title": item.LibraryTitle,
					"tmdb_id":       item.TMDB_ID,
				})
			continue
		}

		actionApp.AppendResult("status", "Successfully updated item tags")
	}

	return Err
}

func srHandleTags(ctx context.Context, app config.Config_SonarrRadarrApp, item models.MediaItem, selectedTypes models.SelectedTypes) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Handling tags for %s in %s", utils.MediaItemInfo(item), app.Type), logging.LevelInfo)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	// Get the full item info from Sonarr/Radarr using the TMDB ID
	tmdbIDInt, _ := strconv.Atoi(item.TMDB_ID)
	logAction.AppendResult("tmdb_id", tmdbIDInt)
	srItem, Err := GetItemInfoFromTMDBID(ctx, app, tmdbIDInt)
	if Err.Message != "" {
		return Err
	}

	// Get all configured tags from Sonarr/Radarr
	allAvailableTags, Err := GetAllTags(ctx, app)
	if Err.Message != "" {
		return Err
	}

	// Get all of the applications configured for labels and tags
	for _, labelApp := range config.Current.LabelsAndTags.Applications {
		// If it is not Sonarr/Radarr, continue
		if labelApp.Application != "Sonarr" && labelApp.Application != "Radarr" {
			continue
		}

		// If the application is not enabled, continue
		if !labelApp.Enabled {
			continue
		}

		// Check to see if there is atleast one tag to add or remove for this application
		if len(labelApp.Add) == 0 && len(labelApp.Remove) == 0 {
			continue
		}

		// If the application is configured with add label tags for selected types, add them
		if labelApp.AddLabelTagForSelectedTypes {
			if selectedTypes.Poster {
				labelApp.Add = append(labelApp.Add, "aura-poster")
			}
			if selectedTypes.Backdrop {
				labelApp.Add = append(labelApp.Add, "aura-backdrop")
			}
			if selectedTypes.SeasonPoster {
				labelApp.Add = append(labelApp.Add, "aura-season-poster")
			}
			if selectedTypes.SpecialSeasonPoster {
				labelApp.Add = append(labelApp.Add, "aura-special-season-poster")
			}
			if selectedTypes.Titlecard {
				labelApp.Add = append(labelApp.Add, "aura-titlecard")
			}
		}

		// Build a map for quick lookup
		availableTagMap := make(map[string]SonarrRadarrTag)
		for _, tag := range allAvailableTags {
			availableTagMap[tag.Label] = tag
		}

		// Add any missing tags from app.Add
		tagsToAdd := make([]string, 0)
		for _, tagToAdd := range labelApp.Add {
			if _, exists := availableTagMap[tagToAdd]; !exists {
				tagsToAdd = append(tagsToAdd, tagToAdd)
			}
		}

		// If there are tags to add, add them
		if len(tagsToAdd) > 0 {
			newTags, Err := AddNewTags(ctx, app, tagsToAdd)
			if Err.Message != "" {
				continue
			}
			// Add new tags to availableTagMap
			for _, newTag := range newTags {
				availableTagMap[newTag.Label] = newTag
			}
		}

		// Build finalTags: start with current tags from srItem, add all tags from app.Add
		item, ok := srItem.(SR_Item)
		if !ok {
			logAction.SetError("Type Assertion Failed", "Failed to assert type to SR_Item", nil)
			return *logAction.Error
		}
		currentTagIDs := item.Tags

		// Build a set of tag IDs to avoid duplicates
		finalTagIDSet := make(map[int64]struct{})
		for _, id := range currentTagIDs {
			finalTagIDSet[id] = struct{}{}
		}

		// Add all tags from app.Add
		for _, tagLabel := range labelApp.Add {
			if tag, exists := availableTagMap[tagLabel]; exists {
				finalTagIDSet[int64(tag.ID)] = struct{}{}
			}
		}

		if labelApp.AddLabelTagForSelectedTypes {
			if selectedTypes.Poster {
				if tag, exists := availableTagMap["aura-poster"]; exists {
					finalTagIDSet[int64(tag.ID)] = struct{}{}
				}
			}
			if selectedTypes.Backdrop {
				if tag, exists := availableTagMap["aura-backdrop"]; exists {
					finalTagIDSet[int64(tag.ID)] = struct{}{}
				}
			}
			if selectedTypes.SeasonPoster {
				if tag, exists := availableTagMap["aura-season-poster"]; exists {
					finalTagIDSet[int64(tag.ID)] = struct{}{}
				}
			}
			if selectedTypes.SpecialSeasonPoster {
				if tag, exists := availableTagMap["aura-special-season-poster"]; exists {
					finalTagIDSet[int64(tag.ID)] = struct{}{}
				}
			}
			if selectedTypes.Titlecard {
				if tag, exists := availableTagMap["aura-titlecard"]; exists {
					finalTagIDSet[int64(tag.ID)] = struct{}{}
				}
			}
		}

		// Remove tags in app.Remove
		for _, tagLabel := range labelApp.Remove {
			if tag, exists := availableTagMap[tagLabel]; exists {
				delete(finalTagIDSet, int64(tag.ID))
			}
		}

		// Convert set to slice
		finalTagIDs := make([]int64, 0, len(finalTagIDSet))
		for id := range finalTagIDSet {
			finalTagIDs = append(finalTagIDs, id)
		}

		// Get tag labels for finalTagIDs
		finalTagLabels := make([]string, 0, len(finalTagIDs))
		for _, id := range finalTagIDs {
			for label, tag := range availableTagMap {
				if int64(tag.ID) == id {
					finalTagLabels = append(finalTagLabels, label)
					break
				}
			}
		}

		// Update the item in Sonarr/Radarr with the new tags
		item.Tags = finalTagIDs
		Err = UpdateItemInfo(ctx, app, item)
		if Err.Message != "" {
			return Err
		}

		logAction.AppendResult("final_tags", finalTagLabels)
	}

	return logging.LogErrorInfo{}
}
