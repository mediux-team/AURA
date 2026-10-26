package mediaserver

import (
	"aura/config"
	"aura/logging"
	"aura/models"
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// ImageResult is how an image's download went
type ImageResult struct {
	Image   models.ImageFile
	Err     logging.LogErrorInfo
	Stopped bool // Not downloaded, as the downloads were stopped first
}

// ImageUpdate is sent when an image starts downloading, and when it finishes
type ImageUpdate struct {
	Image    models.ImageFile
	Finished bool
	Failed   bool
}

type applyImageFunc func(ctx context.Context, item *models.MediaItem, image models.ImageFile) logging.LogErrorInfo

// ApplyImages downloads up to the simultaneous downloads of the images at once, starting in the order given, and returns a result for each in the same order.
// Images not started when stop ends aren't downloaded. updated is the item with any change a media server made to it, like a new rating key.
// onUpdate is called one at a time.
func ApplyImages(ctx, stop context.Context, item models.MediaItem, images []models.ImageFile, onUpdate func(ImageUpdate)) (results []ImageResult, updated models.MediaItem) {
	return applyImages(ctx, stop, item, images, downloadApplyImageToMediaItem, onUpdate)
}

// ApplyCollectionImages applies a collection's images one after another, and returns a result for each
func ApplyCollectionImages(ctx, stop context.Context, collection models.CollectionItem, images []models.ImageFile, onUpdate func(ImageUpdate)) (results []ImageResult) {
	for _, image := range images {
		result := ImageResult{Image: image}
		if stop.Err() != nil {
			result.Err = logging.LogErrorInfo{Message: StoppedMessage}
			result.Stopped = true
			results = append(results, result)
			continue
		}
		if onUpdate != nil {
			onUpdate(ImageUpdate{Image: image})
		}
		msClient, Err := NewMediaServerClient(&config.Current.MediaServer)
		if Err.Message == "" {
			Err = msClient.ApplyCollectionImage(ctx, &collection, image)
		}
		result.Err = Err
		if onUpdate != nil {
			onUpdate(ImageUpdate{Image: image, Finished: true, Failed: result.Err.Message != ""})
		}
		results = append(results, result)
	}
	return results
}

func applyImages(ctx, stop context.Context, item models.MediaItem, images []models.ImageFile, apply applyImageFunc, onUpdate func(ImageUpdate)) (results []ImageResult, updated models.MediaItem) {
	results = make([]ImageResult, len(images))
	updated = item
	if len(images) == 0 {
		return results, updated
	}

	// Images for the same item, season or episode are applied one after another, as media servers can lose one otherwise
	lanes := [][]int{}
	laneOf := map[string]int{}
	for i, image := range images {
		key := imageTarget(image)
		lane, ok := laneOf[key]
		if !ok {
			lane = len(lanes)
			laneOf[key] = lane
			lanes = append(lanes, nil)
		}
		lanes[lane] = append(lanes[lane], i)
	}

	var mu sync.Mutex
	update := func(u ImageUpdate) {
		if onUpdate == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		onUpdate(u)
	}

	next := make(chan []int, len(lanes))
	for _, lane := range lanes {
		next <- lane
	}
	close(next)

	var wg sync.WaitGroup
	for range min(config.SimultaneousDownloads(), len(lanes)) {
		wg.Go(func() {
			for lane := range next {
				for _, i := range lane {
					result, changed := applyOneImage(ctx, stop, item, images[i], apply, update)
					results[i] = result
					if changed != nil {
						mu.Lock()
						updated = *changed
						mu.Unlock()
					}
				}
			}
		})
	}
	wg.Wait()
	return results, updated
}

// applyOneImage applies one image to its own copy of the item, and returns the copy when a media server changed it
func applyOneImage(ctx, stop context.Context, item models.MediaItem, image models.ImageFile, apply applyImageFunc, update func(ImageUpdate)) (result ImageResult, changed *models.MediaItem) {
	result.Image = image
	if stop.Err() != nil {
		result.Err = logging.LogErrorInfo{Message: StoppedMessage}
		result.Stopped = true
		return result, nil
	}

	copy := item.Clone()
	update(ImageUpdate{Image: image})
	func() {
		defer func() {
			if r := recover(); r != nil {
				logging.LOGGER.Error().Timestamp().Interface("recover", r).Str("stack", string(debug.Stack())).Msg("PANIC: in Apply Images")
				result.Err = logging.LogErrorInfo{Message: fmt.Sprintf("Something went wrong applying the image: %v", r)}
			}
		}()
		result.Err = apply(ctx, &copy, image)
	}()
	update(ImageUpdate{Image: image, Finished: true, Failed: result.Err.Message != ""})

	if copy.RatingKey != item.RatingKey {
		return result, &copy
	}
	return result, nil
}

// imageTarget is what the image is applied to: the item, a season or an episode
func imageTarget(image models.ImageFile) string {
	number := func(n *int) int {
		if n == nil {
			return -1
		}
		return *n
	}
	switch image.Type {
	case "season_poster":
		return fmt.Sprintf("season %d", number(image.SeasonNumber))
	case "titlecard":
		return fmt.Sprintf("episode %d %d", number(image.SeasonNumber), number(image.EpisodeNumber))
	}
	return "item"
}
