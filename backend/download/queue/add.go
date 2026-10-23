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
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Add Entry for %s",
			utils.MediaItemInfo(saveItem.MediaItem)),
		logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	jobID, addErr := database.EnqueueDownloadQueueJob(ctx, saveItem)
	if addErr.Message != "" {
		logAction.SetErrorFromInfo(addErr)
		return *logAction.Error
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

	SignalNewJob()

	return Err
}
