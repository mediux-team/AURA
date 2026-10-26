package routes_download

import (
	"aura/database"
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

// StreamDownloadQueueEvents godoc
// @Summary      Download Queue - Stream Events
// @Description  Server-Sent Events stream of live download queue updates (job added/started/finished/removed). Sends a full snapshot immediately on connect.
// @Tags         Download
// @Produce      text/event-stream
// @Security     SessionCookie
// @Router       /api/download/queue/stream [get]
func StreamDownloadQueueEvents(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download Queue - Stream Events", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	stream, ok := httpx.StartSSE(w)
	if !ok {
		return
	}

	ch := downloadqueue.QueueBroadcaster.Subscribe()
	defer downloadqueue.QueueBroadcaster.Unsubscribe(ch)

	jobs, Err := database.GetAllDownloadQueueJobs(ctx)
	if Err.Message != "" {
		jobs = []database.DownloadQueueJob{}
	}
	stream.Send(downloadqueue.QueueEvent{Type: "snapshot", Jobs: jobs})
	// So the progress shows straight away while jobs are downloading
	for _, progress := range downloadqueue.QueueBroadcaster.AllProgress() {
		stream.Send(downloadqueue.QueueEvent{Type: "job_progress", Progress: &progress})
	}

	httpx.StreamEvents(ctx, stream, ch, func(event downloadqueue.QueueEvent) { stream.Send(event) })
}
