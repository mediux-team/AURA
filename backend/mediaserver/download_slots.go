package mediaserver

import (
	"aura/config"
	"aura/logging"
	"context"
	"sync"
)

// StoppedMessage is the error of an image that wasn't downloaded, as its downloads were stopped first
const StoppedMessage = "The download was stopped"

// downloadSlots limits how many single images are downloaded at once, like the download window's, which asks for one image at a time.
// A list of images, like a Download Queue job's, gets its own simultaneous downloads instead.
var downloadSlots = &slots{changed: make(chan struct{})}

type slots struct {
	mu      sync.Mutex
	active  int
	changed chan struct{} // Closed when a slot is freed
}

// acquire waits for a free slot, or returns false when ctx ends first
func (s *slots) acquire(ctx context.Context) bool {
	for {
		if ctx.Err() != nil {
			return false
		}
		// The limit is read each time, so a new setting is used straight away
		s.mu.Lock()
		if s.active < config.SimultaneousDownloads() {
			s.active++
			s.mu.Unlock()
			return true
		}
		changed := s.changed
		s.mu.Unlock()

		select {
		case <-changed:
		case <-ctx.Done():
			return false
		}
	}
}

func (s *slots) release() {
	s.mu.Lock()
	s.active--
	close(s.changed)
	s.changed = make(chan struct{})
	s.mu.Unlock()
}

// withDownloadSlot runs download once a slot is free
func withDownloadSlot(ctx context.Context, download func() logging.LogErrorInfo) logging.LogErrorInfo {
	if !downloadSlots.acquire(ctx) {
		return logging.LogErrorInfo{Message: StoppedMessage}
	}
	defer downloadSlots.release()
	return download()
}
