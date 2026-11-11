package utils

import (
	"aura/models"
	"strings"
	"testing"
)

func TestTemplateVarsAutodownloadSet(t *testing.T) {
	item := models.MediaItem{Title: "Show", LibraryTitle: "Series", Type: "show"}
	season := func(n int) *int { return &n }
	poster := models.ImageFile{ID: "p", Type: "poster"}
	titlecard := models.ImageFile{ID: "e1", Type: "titlecard", SeasonNumber: season(1), EpisodeNumber: season(1)}
	set := models.DBPosterSetDetail{
		PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "set"}},
		OnlyImages: []models.QueuedImage{
			{ID: "p", Type: "poster", ReasonTitle: "Image Updated", Reason: "Newer image"},
			{ID: "e1", Type: "titlecard", ReasonTitle: "Episode Added", Reason: "Season 01 Episode 01 added"},
		},
	}

	vars := TemplateVars_AutodownloadSet(item, set, []models.ImageFile{poster, titlecard}, nil, nil)
	if vars["ReasonTitle"] != "Image Updated, Episode Added" {
		t.Errorf("ReasonTitle = %q", vars["ReasonTitle"])
	}
	if vars["ImageType"] != "poster, titlecard" || strings.Count(vars["ImageName"], ",") != 1 {
		t.Errorf("ImageType = %q, ImageName = %q, want both images", vars["ImageType"], vars["ImageName"])
	}
	if !strings.Contains(vars["Reason"], "Newer image") || !strings.Contains(vars["Reason"], "Season 01 Episode 01 added") {
		t.Errorf("Reason = %q, want each image's reason", vars["Reason"])
	}

	// Nothing downloaded and an error
	vars = TemplateVars_SonarrSet(item, set, nil, []string{"Show S01E01: failed"}, nil)
	if vars["ReasonTitle"] != "Error" || !strings.Contains(vars["Reason"], "failed") || !strings.HasPrefix(vars["Result"], "Error") {
		t.Errorf("vars = %+v, want the error", vars)
	}
}
