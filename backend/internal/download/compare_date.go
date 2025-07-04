package download

import (
	"aura/internal/logging"
	"time"
)

func compareLastUpdateToUpdateSetDateUpdated(lastUpdate string, dateUpdated time.Time) bool {
	// Convert the lastUpdate string to a time.Time object
	lastUpdateTime, err := time.Parse(time.RFC3339, lastUpdate)
	if err != nil {

		logging.LOG.Error("Failed to parse lastUpdate time: " + err.Error())

		// If parsing fails, we can't compare the dates, so return false
		// This means we will always download the poster set again
		return false
	}

	return lastUpdateTime.Before(dateUpdated)
}
