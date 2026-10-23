package plex

import (
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"fmt"
	"net/url"
	"path"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func (p *Plex) ApplyCollectionImage(ctx context.Context, collectionItem *models.CollectionItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Plex: Applying %s Image to Collection '%s' | %s [%s]",
		cases.Title(language.English).String(imageFile.Type), collectionItem.Title, collectionItem.LibraryTitle, collectionItem.RatingKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	// Get the MediUX Image URL
	imageURL, Err := mediux.ConstructImageUrl(ctx, imageFile.ID, imageFile.Modified.String(), mediux.ImageQualityOriginal)
	if Err.Message != "" {
		return Err
	}

	// POST Method is used when not using a local image
	// POST Method requires the posterType to be plural (posters or arts)
	var imageType string
	if imageFile.Type == "collection_backdrop" {
		imageType = "arts"
	} else {
		imageType = "posters"
	}

	// Construct the URL for the Plex API request
	u, err := url.Parse(p.Config.URL)
	if err != nil {
		logAction.SetError("Failed to parse base URL", "Ensure the URL is valid", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	u.Path = path.Join(u.Path, "library", "metadata", collectionItem.RatingKey, imageType)
	query := u.Query()
	query.Set("url", imageURL)
	u.RawQuery = query.Encode()
	URL := u.String()

	// Make the HTTP Request to Plex
	resp, _, Err := makeRequest(ctx, p.Config, URL, "POST", nil)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	defer resp.Body.Close()

	return Err
}
