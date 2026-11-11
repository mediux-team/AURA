package downloadqueue

import (
	"aura/database"
	"aura/logging"
	"aura/models"
	"aura/utils"
	"context"
	"fmt"
	"time"
)

func AddToQueue(ctx context.Context, saveItem models.DBSavedItem) (Err logging.LogErrorInfo) {
	_, Err = AddToQueueJob(ctx, saveItem)
	return Err
}

// AddToQueueJob adds the item to the Download Queue and returns its job ID. Jobs from the download window go before the others.
func AddToQueueJob(ctx context.Context, saveItem models.DBSavedItem) (jobID int64, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Add Entry for %s",
			utils.MediaItemInfo(saveItem.MediaItem)),
		logging.LevelDebug)
	defer logAction.Complete()

	jobID, addErr := database.EnqueueDownloadQueueJob(ctx, saveItem)
	if addErr.Message != "" {
		logAction.SetErrorFromInfo(addErr)
		return 0, *logAction.Error
	}

	logAction.AppendResult("job_id", jobID)

	tmdbID, libraryTitle, edition, title := saveItem.QueueEntry()
	QueueBroadcaster.Publish(QueueEvent{
		Type: "job_added",
		Job: &database.DownloadQueueJob{
			ID:             jobID,
			TMDB_ID:        tmdbID,
			LibraryTitle:   libraryTitle,
			Edition:        edition,
			MediaItemTitle: title,
			Payload:        saveItem,
			Status:         "pending",
			CreatedAt:      time.Now(),
		},
	})

	// The download window's jobs go before the jobs that run in the background
	if saveItem.Source == models.SourceDownloadWindow {
		moved, moveErr := database.MoveDownloadQueueJobAheadOfOthers(ctx, jobID)
		if moveErr.Message != "" {
			logAction.AppendWarning("move_ahead", moveErr.Message)
		} else if moved {
			if jobs, jobsErr := database.GetAllDownloadQueueJobs(ctx); jobsErr.Message == "" {
				QueueBroadcaster.Publish(QueueEvent{Type: "queue_reordered", Jobs: jobs})
			}
		}
	}

	SignalNewJob()

	return jobID, logging.LogErrorInfo{}
}
