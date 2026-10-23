package downloadqueue

import (
	"aura/models"
	"testing"
)

func TestCountJobImages(t *testing.T) {
	season := func(n int) *int { return &n }
	item := models.DBSavedItem{
		MediaItem: models.MediaItem{Series: &models.MediaItemSeries{Seasons: []models.MediaItemSeason{{SeasonNumber: 1}}}},
		PosterSets: []models.DBPosterSetDetail{
			{
				PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "1", Type: "show", Title: "Set"}, Images: []models.ImageFile{
					{Type: "poster"}, {Type: "backdrop"}, {Type: "season_poster", SeasonNumber: season(1)}, {Type: "season_poster", SeasonNumber: season(2)}, {Type: "logo"},
				}},
				SelectedTypes: models.SelectedTypes{Poster: true, SeasonPoster: true},
			},
			{
				// Skipped by the job, as it is missing its ID
				PosterSet:     models.PosterSet{BaseSetInfo: models.BaseSetInfo{Type: "show", Title: "No ID"}, Images: []models.ImageFile{{Type: "poster"}}},
				SelectedTypes: models.SelectedTypes{Poster: true},
			},
		},
	}
	// The poster and the season 1 poster. The backdrop isn't selected, there is no season 2 and logos aren't applied
	if got := countJobImages(item); got != 2 {
		t.Errorf("countJobImages = %d, want 2", got)
	}

	// A collection's poster and backdrop are applied too, with or without sets
	item.CollectionImages = &models.QueuedCollectionImages{
		Collection: models.CollectionItem{RatingKey: "c1", Title: "Toy Story", LibraryTitle: "Movies"},
		Images:     []models.ImageFile{{Type: "collection_poster"}, {Type: "collection_backdrop"}},
	}
	if got := countJobImages(item); got != 4 {
		t.Errorf("countJobImages with collection images = %d, want 4", got)
	}
	if got := countJobImages(models.DBSavedItem{CollectionImages: item.CollectionImages}); got != 2 {
		t.Errorf("countJobImages with only collection images = %d, want 2", got)
	}
}

func TestBroadcasterKeepsTheLatestProgress(t *testing.T) {
	b := &broadcaster{subs: map[chan QueueEvent]struct{}{}}
	if b.LatestProgress() != nil {
		t.Fatal("there should be no progress before a job starts")
	}

	b.Publish(QueueEvent{Type: "job_progress", Progress: &QueueProgress{JobID: 1, Done: 1, Total: 3}})
	b.Publish(QueueEvent{Type: "job_progress", Progress: &QueueProgress{JobID: 1, Done: 2, Total: 3}})
	if p := b.LatestProgress(); p == nil || p.Done != 2 {
		t.Errorf("LatestProgress = %+v, want done 2", p)
	}

	for _, eventType := range []string{"job_finished", "job_started", "queue_cleared"} {
		b.Publish(QueueEvent{Type: "job_progress", Progress: &QueueProgress{JobID: 1}})
		b.Publish(QueueEvent{Type: eventType})
		if b.LatestProgress() != nil {
			t.Errorf("the progress should be cleared by %s", eventType)
		}
	}
}
