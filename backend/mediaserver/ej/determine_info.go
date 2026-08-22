package ej

import "aura/models"

func getItemRatingKeyFromImageFile(embyJellyItem models.MediaItem, imageFile models.ImageFile) string {
	if embyJellyItem.Type == "movie" || imageFile.Type == "poster" || imageFile.Type == "backdrop" {
		return embyJellyItem.RatingKey
	}
	if imageFile.Type == "season_poster" || imageFile.Type == "special_season_poster" {
		return getSeasonRatingKeyFromImageFile(embyJellyItem, imageFile)
	}
	if imageFile.Type == "titlecard" {
		return getEpisodeRatingKeyFromImageFile(embyJellyItem, imageFile)
	}
	return ""
}

func getSeasonRatingKeyFromImageFile(embyJellyItem models.MediaItem, imageFile models.ImageFile) string {
	seasonNumberFromSet := imageFile.SeasonNumber
	if seasonNumberFromSet == nil {
		return ""
	}
	for _, season := range embyJellyItem.Series.Seasons {
		if season.SeasonNumber == *seasonNumberFromSet {
			return season.RatingKey
		}
	}
	return ""
}

func getEpisodeRatingKeyFromImageFile(embyJellyItem models.MediaItem, imageFile models.ImageFile) string {
	seasonNumberFromSet := imageFile.SeasonNumber
	episodeNumberFromSet := imageFile.EpisodeNumber
	if seasonNumberFromSet == nil || episodeNumberFromSet == nil {
		return ""
	}
	for _, season := range embyJellyItem.Series.Seasons {
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
