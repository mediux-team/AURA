package downloadqueue

import (
	"aura/database"
	"cmp"
	"slices"
	"sync"
)

// QueueEvent is broadcast to all subscribed SSE clients whenever the
// download queue changes.
type QueueEvent struct {
	Type     string                      `json:"type"` // "snapshot" | "job_added" | "job_started" | "job_progress" | "job_finished" | "job_removed" | "queue_cleared" | "queue_reordered"
	Job      *database.DownloadQueueJob  `json:"job,omitempty"`
	Jobs     []database.DownloadQueueJob `json:"jobs,omitempty"`
	Progress *QueueProgress              `json:"progress,omitempty"`
}

// QueueProgress is how far a running job is
type QueueProgress struct {
	JobID          int64           `json:"job_id"`
	MediaItemTitle string          `json:"media_item_title"`
	Active         []ProgressImage `json:"active"` // Images being downloaded
	Done           int             `json:"done"`   // Images of the job already downloaded
	Failed         int             `json:"failed"` // Images of the job that couldn't be downloaded
	Total          int             `json:"total"`  // Images the job downloads
}

// ProgressImage is an image being downloaded
type ProgressImage struct {
	ID            string `json:"id"`
	ImageType     string `json:"image_type"`
	SeasonNumber  *int   `json:"season_number,omitempty"`
	EpisodeNumber *int   `json:"episode_number,omitempty"`
}

type broadcaster struct {
	mu   sync.Mutex
	subs map[chan QueueEvent]struct{}

	// The latest progress of each running job, sent to new subscribers after the snapshot
	progress map[int64]QueueProgress
}

var QueueBroadcaster = &broadcaster{subs: map[chan QueueEvent]struct{}{}}

// Subscribe registers a new listener. The returned channel is buffered;
// slow consumers have events dropped rather than blocking publishers.
func (b *broadcaster) Subscribe() chan QueueEvent {
	ch := make(chan QueueEvent, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broadcaster) Unsubscribe(ch chan QueueEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
}

func (b *broadcaster) Publish(evt QueueEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch evt.Type {
	case "job_progress":
		if b.progress == nil {
			b.progress = map[int64]QueueProgress{}
		}
		b.progress[evt.Progress.JobID] = *evt.Progress
	case "job_started", "job_finished", "job_removed":
		if evt.Job != nil {
			delete(b.progress, evt.Job.ID)
		}
	case "queue_cleared":
		b.progress = nil
	}
	for ch := range b.subs {
		select {
		case ch <- evt:
		default:
			// Slow subscriber: drop the event, the client will get true
			// state from the next mutation or a fresh snapshot on reconnect.
		}
	}
}

// AllProgress returns the progress of each running job, by job ID
func (b *broadcaster) AllProgress() []QueueProgress {
	b.mu.Lock()
	defer b.mu.Unlock()
	all := make([]QueueProgress, 0, len(b.progress))
	for _, progress := range b.progress {
		all = append(all, progress)
	}
	slices.SortFunc(all, func(a, b QueueProgress) int { return cmp.Compare(a.JobID, b.JobID) })
	return all
}
