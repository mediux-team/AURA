package plex

import (
	"aura/models"
)

func getItemRatingKeyFromImageFile(item models.MediaItem, imageFile models.ImageFile) string {
	if item.Type == "movie" || imageFile.Type == "poster" || imageFile.Type == "backdrop" {
		return item.RatingKey
	}
	if imageFile.Type == "season_poster" || imageFile.Type == "special_season_poster" {
		return getSeasonRatingKeyFromImageFile(item, imageFile)
	}
	if imageFile.Type == "titlecard" {
		return getEpisodeRatingKeyFromImageFile(item, imageFile)
	}
	return ""
}

func getSeasonRatingKeyFromImageFile(item models.MediaItem, imageFile models.ImageFile) string {
	seasonNumberFromSet := imageFile.SeasonNumber

	// Find the matching season Number in the Plex.Series.Seasons array where SeasonNumber == seasonNumberFromSet
	for _, season := range item.Series.Seasons {
		if seasonNumberFromSet != nil && season.SeasonNumber == *seasonNumberFromSet {
			return season.RatingKey
		}
	}
	return ""
}

func getEpisodeRatingKeyFromImageFile(item models.MediaItem, imageFile models.ImageFile) string {
	seasonNumberFromSet := imageFile.SeasonNumber
	episodeNumberFromSet := imageFile.EpisodeNumber

	if seasonNumberFromSet == nil || episodeNumberFromSet == nil {
		return ""
	}

	for _, season := range item.Series.Seasons {
		if season.SeasonNumber == *seasonNumberFromSet {
			for _, episode := range season.Episodes {
				if episode.MediuxEpisodeNumber == *episodeNumberFromSet && episode.SeasonNumber == *seasonNumberFromSet {
					return episode.RatingKey
				}
			}
		}
	}
	return ""
}

func getEpisodePathFromImageFile(item models.MediaItem, imageFile models.ImageFile) string {
	seasonNumberFromSet := imageFile.SeasonNumber
	episodeNumberFromSet := imageFile.EpisodeNumber

	if seasonNumberFromSet == nil || episodeNumberFromSet == nil {
		return ""
	}

	for _, season := range item.Series.Seasons {
		if season.SeasonNumber == *seasonNumberFromSet {
			for _, episode := range season.Episodes {
				if episode.MediuxEpisodeNumber == *episodeNumberFromSet && episode.SeasonNumber == *seasonNumberFromSet {
					return episode.File.Path
				}
			}
		}
	}
	return ""
}
