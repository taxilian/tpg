// Package format provides display formatting helpers for task status.
package format

import (
	"time"

	"github.com/taxilian/tpg/internal/model"
)

// StaleThreshold is the duration after which an in_progress task is considered stale.
const StaleThreshold = 5 * time.Minute

// IsStale returns true if the item is in_progress and hasn't been updated
// within the StaleThreshold duration.
func IsStale(item model.Item, now time.Time) bool {
	if item.Status != model.StatusInProgress {
		return false
	}
	return now.Sub(item.UpdatedAt) > StaleThreshold
}

// StatusDisplay returns the display status for an item.
// Returns "stale" for stale in_progress items, otherwise returns the actual status.
func StatusDisplay(item model.Item, now time.Time) string {
	if IsStale(item, now) {
		return "stale"
	}
	return string(item.Status)
}
