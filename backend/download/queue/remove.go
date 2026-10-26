package downloadqueue

import (
	"aura/database"
	"aura/logging"
	"context"
	"fmt"
)

// RemoveFromQueue removes a job. A running job is stopped, and removes itself once its images downloading finish.
func RemoveFromQueue(ctx context.Context, jobID int64) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Remove Download Queue Job %d", jobID),
		logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	if stopJob(jobID) {
		logAction.AppendResult("stopped", "the job was running")
	} else {
		deleteErr := database.DeleteDownloadQueueJob(ctx, jobID)
		if deleteErr.Message != "" {
			logAction.SetErrorFromInfo(deleteErr)
			return *logAction.Error
		}
	}

	QueueBroadcaster.Publish(QueueEvent{Type: "job_removed", Job: &database.DownloadQueueJob{ID: jobID}})

	return Err
}

// RemoveAllFromQueue removes the waiting jobs, and stops the running ones
func RemoveAllFromQueue(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Remove All Download Queue Jobs", logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	deleteErr := database.DeleteAllDownloadQueueJobs(ctx)
	if deleteErr.Message != "" {
		logAction.SetErrorFromInfo(deleteErr)
		return *logAction.Error
	}
	stopAllJobs()

	QueueBroadcaster.Publish(QueueEvent{Type: "queue_cleared"})

	return Err
}

// MoveToTopOfQueue makes a waiting job the next one to run
func MoveToTopOfQueue(ctx context.Context, jobID int64) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Move Download Queue Job %d to the Top", jobID), logging.LevelDebug)
	defer logAction.Complete()

	moved, Err := database.MoveDownloadQueueJobToTop(ctx, jobID)
	if Err.Message != "" {
		return Err
	}
	if !moved {
		logAction.SetError("This job isn't waiting in the Download Queue", "It may have started or finished already, refresh the page", map[string]any{"job_id": jobID})
		return *logAction.Error
	}

	jobs, Err := database.GetAllDownloadQueueJobs(ctx)
	if Err.Message != "" {
		return Err
	}
	QueueBroadcaster.Publish(QueueEvent{Type: "queue_reordered", Jobs: jobs})
	return logging.LogErrorInfo{}
}
