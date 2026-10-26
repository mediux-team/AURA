package downloadqueue

import (
	"aura/database"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/models"
	sonarr_radarr "aura/sonarr-radarr"
	"aura/utils"
	"context"
	"fmt"
	"slices"
	"time"
)

// claimNextJob atomically claims the next pending Download Queue Job. Returns
// false if there was no pending job to claim.
func claimNextJob(ctx context.Context) (job database.DownloadQueueJob, found bool) {
	// The worker runs on a bare background context, unlike HTTP handlers, so
	// create a logging context here for the DB calls below to attach to.
	ctx, ld := logging.CreateLoggingContext(ctx, "Download Queue Worker")
	claimAction := ld.AddAction("Claim Next Download Queue Job", logging.LevelDebug)
	ctx = logging.WithCurrentAction(ctx, claimAction)

	job, found, claimErr := database.ClaimNextDownloadQueueJob(ctx, workerID)
	claimAction.Complete()
	if claimErr.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", claimErr.Message).Msg("Failed to claim next Download Queue Job")
		ld.Log()
		return job, false
	}
	if !found {
		return job, false
	}

	QueueBroadcaster.Publish(QueueEvent{Type: "job_started", Job: &job})
	return job, true
}

// jobProgress publishes how far a job is
type jobProgress struct {
	progress QueueProgress
}

func newJobProgress(jobID int64, title string, total int) *jobProgress {
	return &jobProgress{progress: QueueProgress{JobID: jobID, MediaItemTitle: title, Active: []ProgressImage{}, Total: total}}
}

// update adds an image that started, or counts one that finished
func (p *jobProgress) update(update mediaserver.ImageUpdate) {
	if !update.Finished {
		p.progress.Active = append(p.progress.Active, ProgressImage{
			ID:            update.Image.ID,
			ImageType:     update.Image.Type,
			SeasonNumber:  update.Image.SeasonNumber,
			EpisodeNumber: update.Image.EpisodeNumber,
		})
	} else {
		if i := slices.IndexFunc(p.progress.Active, func(image ProgressImage) bool { return image.ID == update.Image.ID }); i >= 0 {
			p.progress.Active = slices.Delete(p.progress.Active, i, i+1)
		}
		p.progress.Done++
		if update.Failed {
			p.progress.Failed++
		}
	}
	p.publish()
}

func (p *jobProgress) publish() {
	progress := p.progress
	progress.Active = slices.Clone(p.progress.Active)
	QueueBroadcaster.Publish(QueueEvent{Type: "job_progress", Progress: &progress})
}

// processQueueJob downloads a job's images. When stop ends, the images that haven't started aren't downloaded and the job is removed.
func processQueueJob(ctx, stop context.Context, job database.DownloadQueueJob) {
	ctx, ld := logging.CreateLoggingContext(ctx, "Download Queue Processing")
	subAction := ld.AddAction(fmt.Sprintf("Processing Job %d: %s", job.ID, job.MediaItemTitle), logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, subAction)
	defer ld.Log()
	defer subAction.Complete()

	queueItem := job.Payload

	fileErrors := []string{}
	fileWarnings := []string{}

	finalize := func() {
		status := "success"
		if len(fileErrors) > 0 {
			status = "error"
		} else if len(fileWarnings) > 0 {
			status = "warning"
		}

		_, libraryTitle, _, title := queueItem.QueueEntry()
		message := fmt.Sprintf("%s (%s)", title, libraryTitle)

		finishErr := database.FinishDownloadQueueJob(ctx, job.ID, status, message, fileErrors, fileWarnings)
		if finishErr.Message != "" {
			subAction.AppendWarning(fmt.Sprintf("job_%d", job.ID), "Failed to finalize Download Queue Job")
		}

		finishedJob := job
		finishedJob.Status = status
		finishedJob.ResultMessage = message
		finishedJob.ResultErrors = fileErrors
		finishedJob.ResultWarnings = fileWarnings
		QueueBroadcaster.Publish(QueueEvent{Type: "job_finished", Job: &finishedJob})

		// Clear the job from the queue regardless of outcome. Download History
		// is now the permanent record, so the queue only shows active jobs.
		_ = database.DeleteDownloadQueueJob(ctx, job.ID)
	}

	// The job was removed or the queue was cleared. The images already applied stay applied, but the sets aren't saved.
	stopped := func() {
		_, _, _, title := queueItem.QueueEntry()
		subAction.AppendResult("stopped", "The job was removed from the Download Queue")
		logging.LOGGER.Info().Timestamp().Int64("job_id", job.ID).Str("title", title).Msg("Stopped a Download Queue Job")
		_ = database.DeleteDownloadQueueJob(ctx, job.ID)
		QueueBroadcaster.Publish(QueueEvent{Type: "job_removed", Job: &database.DownloadQueueJob{ID: job.ID}})
	}

	// Only collection images, applied to their media server collection
	if queueItem.IsCollectionOnly() {
		progress := newJobProgress(job.ID, queueItem.CollectionImages.Collection.Title, len(queueItem.CollectionImages.Images))
		errors, wasStopped := applyCollectionImages(ctx, stop, queueItem.CollectionImages, queueItem.Source, progress)
		if wasStopped {
			stopped()
			return
		}
		fileErrors = append(fileErrors, errors...)
		finalize()
		return
	}

	if queueItem.MediaItem.RatingKey == "" || queueItem.MediaItem.Title == "" || queueItem.MediaItem.LibraryTitle == "" || queueItem.MediaItem.TMDB_ID == "" {
		reason := "Media Item missing required fields: ratingKey/title/libraryTitle/tmdbId"
		fileErrors = append(fileErrors, reason)
		recordDownloadHistoryFailure(ctx, queueItem.MediaItem, models.DBPosterSetDetail{}, reason)
		finalize()
		return
	}

	if len(queueItem.PosterSets) == 0 && !queueItem.HasCollectionImages() {
		reason := "No poster sets found"
		fileWarnings = append(fileWarnings, reason)
		recordDownloadHistoryFailure(ctx, queueItem.MediaItem, models.DBPosterSetDetail{}, reason)
		finalize()
		return
	}

	mediuxItemInfo, mErr := mediux.GetBaseItemInfoByTMDB_ID(queueItem.MediaItem.TMDB_ID, queueItem.MediaItem.Type)
	if mErr.Message != "" {
		fileWarnings = append(fileWarnings, fmt.Sprintf("MediUX lookup failed: %s", mErr.Message))
	}

	found, mediaErr := mediaserver.GetMediaItemDetails(ctx, &queueItem.MediaItem)
	if mediaErr.Message != "" || !found {
		fileErrors = append(fileErrors, mediaErr.Message)
		recordDownloadHistoryFailure(ctx, queueItem.MediaItem, models.DBPosterSetDetail{}, mediaErr.Message)
		finalize()
		return
	}

	// Images the job downloads, for the progress
	progress := newJobProgress(job.ID, queueItem.MediaItem.Title, countJobImages(queueItem))
	progress.publish()

	for _, posterSet := range queueItem.PosterSets {
		if stop.Err() != nil {
			stopped()
			return
		}

		setErrors := []string{}
		setWarnings := []string{}
		imageResults := []models.ImageDownloadResult{}

		if posterSet.ID == "" || posterSet.Type == "" || posterSet.Title == "" {
			setErrors = append(setErrors, "poster set missing required fields: id/type/title")
			fileErrors = append(fileErrors, setErrors...)
			recordDownloadHistoryFailure(ctx, queueItem.MediaItem, posterSet, setErrors[0])
			SendNotification(
				FileIssues{Errors: setErrors, Warnings: setWarnings},
				queueItem.MediaItem,
				posterSet,
				mediuxItemInfo.TMDB_PosterPath,
				mediuxItemInfo.TMDB_BackdropPath,
				queueItem.Source,
			)
			continue
		}

		if !posterSet.SelectedTypes.Poster &&
			!posterSet.SelectedTypes.Backdrop &&
			!posterSet.SelectedTypes.SeasonPoster &&
			!posterSet.SelectedTypes.SpecialSeasonPoster &&
			!posterSet.SelectedTypes.Titlecard {
			setWarnings = append(setWarnings, "poster set has no selected image types")
			fileWarnings = append(fileWarnings, setWarnings...)
			recordDownloadHistoryFailure(ctx, queueItem.MediaItem, posterSet, setWarnings[0])
			SendNotification(
				FileIssues{Errors: setErrors, Warnings: setWarnings},
				queueItem.MediaItem,
				posterSet,
				mediuxItemInfo.TMDB_PosterPath,
				mediuxItemInfo.TMDB_BackdropPath,
				queueItem.Source,
			)
			continue
		}

		images := []models.ImageFile{}
		for idx, image := range models.SortImagesForApply(posterSet.Images) {
			if !models.IsKnownImageType(image.Type) {
				subAction.AppendWarning(fmt.Sprintf("job_%d_image_%d", job.ID, idx), fmt.Sprintf("Image has unrecognized type '%s'", image.Type))
				fileWarnings = append(fileWarnings, fmt.Sprintf("Image '%s' has unrecognized type '%s'", image.Src, image.Type))
				continue
			}
			// Only the selected types, for seasons and episodes the item has
			if !models.ImageIsSelected(queueItem.MediaItem, posterSet.SelectedTypes, image) {
				continue
			}
			images = append(images, image)
		}

		results, updatedItem := mediaserver.ApplyImages(ctx, stop, queueItem.MediaItem, images, progress.update)
		queueItem.MediaItem = updatedItem

		event := models.NewDownloadEvent(queueItem.MediaItem, posterSet.PosterSet, queueItem.Source, models.DownloadKindDownload)
		wasStopped := false
		for _, r := range results {
			if r.Stopped {
				wasStopped = true
				continue
			}
			result := models.ImageDownloadResult{
				ImageType:     r.Image.Type,
				SeasonNumber:  r.Image.SeasonNumber,
				EpisodeNumber: r.Image.EpisodeNumber,
				Success:       r.Err.Message == "",
			}
			if r.Err.Message != "" {
				setErrors = append(setErrors, fmt.Sprintf("%s: %s", utils.GetFileDownloadName(queueItem.MediaItem.Title, r.Image), r.Err.Message))
				result.FailureReason = r.Err.Message
			}
			event.AddImage(r.Image, r.Err.Message == "")
			imageResults = append(imageResults, result)
		}
		database.RecordDownloadEvent(ctx, event)
		if wasStopped {
			stopped()
			return
		}

		// Per-set notification (success/warning/error)
		SendNotification(
			FileIssues{Errors: setErrors, Warnings: setWarnings},
			queueItem.MediaItem,
			posterSet,
			mediuxItemInfo.TMDB_PosterPath,
			mediuxItemInfo.TMDB_BackdropPath,
			queueItem.Source,
		)

		recordDownloadHistory(ctx, queueItem.MediaItem, posterSet, imageResults, len(setWarnings))

		fileErrors = append(fileErrors, setErrors...)
		fileWarnings = append(fileWarnings, setWarnings...)
	}

	// The collection's own poster and backdrop, applied to the media server collection. They aren't saved.
	if queueItem.HasCollectionImages() {
		errors, wasStopped := applyCollectionImages(ctx, stop, queueItem.CollectionImages, queueItem.Source, progress)
		if wasStopped {
			stopped()
			return
		}
		fileErrors = append(fileErrors, errors...)
	}

	// Only collection images, there are no sets to save
	if len(queueItem.PosterSets) == 0 {
		finalize()
		return
	}

	// The sets were just downloaded
	for i := range queueItem.PosterSets {
		queueItem.PosterSets[i].LastDownloaded = time.Now()
	}
	upsertErr := database.UpsertSavedItem(ctx, queueItem)
	if upsertErr.Message != "" {
		fileErrors = append(fileErrors, upsertErr.Message)
		recordDownloadHistoryFailure(ctx, queueItem.MediaItem, models.DBPosterSetDetail{}, upsertErr.Message)
		finalize()
		return
	}

	// The sets this replaces are removed now the new sets are saved
	for _, setID := range queueItem.ReplaceSetIDs {
		if slices.ContainsFunc(queueItem.PosterSets, func(set models.DBPosterSetDetail) bool { return set.ID == setID }) {
			continue
		}
		if Err := database.DeletePosterSetForMediaItem(ctx, queueItem.MediaItem.TMDB_ID, queueItem.MediaItem.LibraryTitle, queueItem.MediaItem.Edition, setID); Err.Message != "" {
			fileWarnings = append(fileWarnings, fmt.Sprintf("Failed to remove the replaced set %s: %s", setID, Err.Message))
		}
	}

	finalize()

	// Handle any labels and tags asynchronously
	go func() {
		defer logging.RecoverPanic("Download Queue Labels and Tags")
		ctx, ld := logging.CreateLoggingContext(context.Background(), "Download Queue - Labels and Tags Handling")
		logAction := ld.AddAction("Handle Labels and Tags for Added Item", logging.LevelInfo)
		ctx = logging.WithCurrentAction(ctx, logAction)
		defer ld.Log()
		selectedTypes := models.SelectedTypes{}
		for _, posterSet := range queueItem.PosterSets {
			selectedTypes.Poster = selectedTypes.Poster || posterSet.SelectedTypes.Poster
			selectedTypes.Backdrop = selectedTypes.Backdrop || posterSet.SelectedTypes.Backdrop
			selectedTypes.SeasonPoster = selectedTypes.SeasonPoster || posterSet.SelectedTypes.SeasonPoster
			selectedTypes.SpecialSeasonPoster = selectedTypes.SpecialSeasonPoster || posterSet.SelectedTypes.SpecialSeasonPoster
			selectedTypes.Titlecard = selectedTypes.Titlecard || posterSet.SelectedTypes.Titlecard
		}

		mediaserver.AddLabelToMediaItem(ctx, queueItem.MediaItem, selectedTypes)
		sonarr_radarr.HandleTags(ctx, queueItem.MediaItem, selectedTypes)
	}()
}

// applyCollectionImages applies a collection's own images to the media server collection, and returns the errors
func applyCollectionImages(ctx, stop context.Context, images *models.QueuedCollectionImages, source string, progress *jobProgress) (errors []string, stopped bool) {
	collection := images.Collection
	event := models.NewCollectionDownloadEvent(collection, images.Set, source)
	defer func() { database.RecordDownloadEvent(ctx, event) }()
	for _, r := range mediaserver.ApplyCollectionImages(ctx, stop, collection, images.Images, progress.update) {
		if r.Stopped {
			stopped = true
			continue
		}
		if r.Err.Message != "" {
			errors = append(errors, fmt.Sprintf("%s %s: %s", collection.Title, r.Image.Type, r.Err.Message))
		}
		event.AddImage(r.Image, r.Err.Message == "")
	}
	return errors, stopped
}

// recordDownloadHistory keeps a set's failed download in the history. A set that downloaded without failures clears its earlier ones instead.
func recordDownloadHistory(ctx context.Context, mediaItem models.MediaItem, posterSet models.DBPosterSetDetail, imageResults []models.ImageDownloadResult, warningCount int) {
	succeeded := 0
	failedImages := []models.ImageDownloadResult{}
	for _, r := range imageResults {
		if r.Success {
			succeeded++
		} else {
			failedImages = append(failedImages, r)
		}
	}

	if succeeded == 0 && len(failedImages) == 0 {
		// Nothing was actually attempted for this set (e.g. all images filtered out); skip history.
		return
	}

	// Saved Sets shows the successful downloads, the history only keeps failures
	if len(failedImages) == 0 {
		if deleteErr := database.DeleteDownloadHistoryForSet(ctx, mediaItem.TMDB_ID, mediaItem.LibraryTitle, mediaItem.Edition, posterSet.ID); deleteErr.Message != "" {
			logging.LOGGER.Error().Timestamp().Str("error", deleteErr.Message).Msg("Failed to clear earlier Download History failures")
		}
		return
	}

	entry := database.DownloadHistoryEntry{
		TMDB_ID:         mediaItem.TMDB_ID,
		LibraryTitle:    mediaItem.LibraryTitle,
		Edition:         mediaItem.Edition,
		MediaItemTitle:  mediaItem.Title,
		MediaItemYear:   mediaItem.Year,
		SetID:           posterSet.ID,
		SetTitle:        posterSet.Title,
		SetCreator:      posterSet.UserCreated,
		SelectedTypes:   posterSet.SelectedTypes,
		WarningCount:    warningCount,
		ImagesSucceeded: succeeded,
		ImagesFailed:    len(failedImages),
		FailedImages:    failedImages,
	}

	if insertErr := database.InsertDownloadHistoryEntry(ctx, entry); insertErr.Message != "" {
		logging.LOGGER.Error().Timestamp().Str("error", insertErr.Message).Msg("Failed to insert Download History entry")
	}
}

func recordDownloadHistoryFailure(ctx context.Context, mediaItem models.MediaItem, posterSet models.DBPosterSetDetail, reason string) {
	recordDownloadHistory(ctx, mediaItem, posterSet, []models.ImageDownloadResult{
		{FailureReason: reason},
	}, 0)
}

// countJobImages counts the images a job downloads, skipping the same sets and images as processQueueJob
func countJobImages(queueItem models.DBSavedItem) (total int) {
	for _, posterSet := range queueItem.PosterSets {
		if posterSet.ID == "" || posterSet.Type == "" || posterSet.Title == "" {
			continue
		}
		for _, image := range posterSet.Images {
			if models.IsKnownImageType(image.Type) && models.ImageIsSelected(queueItem.MediaItem, posterSet.SelectedTypes, image) {
				total++
			}
		}
	}
	if queueItem.HasCollectionImages() {
		total += len(queueItem.CollectionImages.Images)
	}
	return total
}
