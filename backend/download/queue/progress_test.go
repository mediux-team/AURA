package downloadqueue

import (
	"aura/database"
	"aura/mediaserver"
	"aura/models"
	"slices"
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

func TestBroadcasterKeepsEachJobsProgress(t *testing.T) {
	b := &broadcaster{subs: map[chan QueueEvent]struct{}{}}
	if len(b.AllProgress()) != 0 {
		t.Fatal("there should be no progress before a job starts")
	}

	b.Publish(QueueEvent{Type: "job_progress", Progress: &QueueProgress{JobID: 2, Done: 1, Total: 3}})
	b.Publish(QueueEvent{Type: "job_progress", Progress: &QueueProgress{JobID: 1, Done: 1, Total: 3}})
	b.Publish(QueueEvent{Type: "job_progress", Progress: &QueueProgress{JobID: 1, Done: 2, Total: 3}})
	all := b.AllProgress()
	if len(all) != 2 || all[0].JobID != 1 || all[0].Done != 2 || all[1].JobID != 2 {
		t.Errorf("AllProgress = %+v, want job 1 at 2 then job 2", all)
	}

	// A finished job's progress is removed, the other job's stays
	for _, eventType := range []string{"job_finished", "job_started", "job_removed"} {
		b.Publish(QueueEvent{Type: "job_progress", Progress: &QueueProgress{JobID: 1}})
		b.Publish(QueueEvent{Type: eventType, Job: &database.DownloadQueueJob{ID: 1}})
		if all := b.AllProgress(); len(all) != 1 || all[0].JobID != 2 {
			t.Errorf("after %s AllProgress = %+v, want only job 2", eventType, all)
		}
	}

	b.Publish(QueueEvent{Type: "queue_cleared"})
	if len(b.AllProgress()) != 0 {
		t.Error("clearing the queue should clear the progress")
	}
}

func TestJobProgressCountsTheImagesDownloading(t *testing.T) {
	season := func(n int) *int { return &n }
	poster := models.ImageFile{ID: "a", Type: "poster"}
	seasonPoster := models.ImageFile{ID: "b", Type: "season_poster", SeasonNumber: season(1)}
	titlecard := models.ImageFile{ID: "c", Type: "titlecard", SeasonNumber: season(1), EpisodeNumber: season(1)}
	p := newJobProgress(1, "Severance", []models.ImageFile{poster, seasonPoster, titlecard})
	if p.progress.Total != 3 || len(p.progress.Images) != 3 || p.progress.Images[0].Status != ImageWaiting {
		t.Fatalf("progress = %+v, want 3 images waiting", p.progress)
	}

	p.update(mediaserver.ImageUpdate{Image: poster})
	p.update(mediaserver.ImageUpdate{Image: seasonPoster})
	if len(p.progress.Active) != 2 {
		t.Fatalf("Active = %+v, want 2 images", p.progress.Active)
	}

	p.update(mediaserver.ImageUpdate{Image: poster, Finished: true, Failed: true})
	if len(p.progress.Active) != 1 || p.progress.Active[0].ID != "b" || p.progress.Done != 1 || p.progress.Failed != 1 {
		t.Errorf("progress = %+v, want the season poster downloading, 1 done and 1 failed", p.progress)
	}
	statuses := []string{}
	for _, image := range p.progress.Images {
		statuses = append(statuses, image.Status)
	}
	if want := []string{ImageFailed, ImageDownloading, ImageWaiting}; !slices.Equal(statuses, want) {
		t.Errorf("statuses = %v, want %v", statuses, want)
	}
}

func TestBroadcasterKeepsEveryImagesStatus(t *testing.T) {
	b := &broadcaster{subs: map[chan QueueEvent]struct{}{}}
	images := []ProgressImage{{ID: "a", ImageType: "poster", Status: ImageWaiting}, {ID: "b", ImageType: "backdrop", Status: ImageWaiting}}
	b.Publish(QueueEvent{Type: "job_progress", Progress: &QueueProgress{JobID: 1, Total: 2, Images: images}})

	// An update only has the image that changed
	b.Publish(QueueEvent{Type: "job_progress", Progress: &QueueProgress{JobID: 1, Done: 1, Total: 2, Changed: &ProgressImage{ID: "b", ImageType: "backdrop", Status: ImageDone}}})

	all := b.AllProgress()
	if len(all) != 1 || all[0].Done != 1 || len(all[0].Images) != 2 || all[0].Images[0].Status != ImageWaiting || all[0].Images[1].Status != ImageDone {
		t.Errorf("AllProgress = %+v, want every image with the backdrop done", all)
	}
	if all[0].Changed != nil {
		t.Error("the stored progress shouldn't have a changed image")
	}
	// The list that was published isn't changed
	if images[1].Status != ImageWaiting {
		t.Error("the published images were changed")
	}
}

func TestCountJobImagesForAnUpdate(t *testing.T) {
	season := func(n int) *int { return &n }
	item := models.DBSavedItem{
		MediaItem: models.MediaItem{Series: &models.MediaItemSeries{Seasons: []models.MediaItemSeason{{SeasonNumber: 1, Episodes: []models.MediaItemEpisode{{SeasonNumber: 1, EpisodeNumber: 1}, {SeasonNumber: 1, EpisodeNumber: 2}}}}}},
		PosterSets: []models.DBPosterSetDetail{{
			PosterSet: models.PosterSet{BaseSetInfo: models.BaseSetInfo{ID: "1", Type: "show", Title: "Set"}, Images: []models.ImageFile{
				{ID: "p", Type: "poster"},
				{ID: "e1", Type: "titlecard", SeasonNumber: season(1), EpisodeNumber: season(1)},
				{ID: "e2", Type: "titlecard", SeasonNumber: season(1), EpisodeNumber: season(2)},
			}},
			SelectedTypes: models.SelectedTypes{Poster: true, Titlecard: true},
			// The job updates only the second episode's title card
			OnlyImages: []models.QueuedImage{{ID: "e2", Type: "titlecard", ReasonTitle: "Episode Changed"}},
		}},
	}
	if got := countJobImages(item); got != 1 {
		t.Errorf("countJobImages = %d, want 1", got)
	}
}
