package db

import "fmt"

// CleanupHistoryOptions configures history cleanup behavior.
type CleanupHistoryOptions struct {
	DryRun bool // If true, count but don't delete
}

// CleanupHistoryResult contains the results of a history cleanup operation.
type CleanupHistoryResult struct {
	TotalBefore   int // Total history entries before cleanup
	DeletedCount  int // Number of entries deleted (or would be deleted in dry run)
	DeletedRecent int // <24h entries deleted (should always be 0)
	DeletedStatus int // status events >30 days deleted
	DeletedOther  int // other events >7 days deleted
}

// CleanupHistory removes old history entries based on retention rules:
// - Keep ALL history for last 24 hours
// - Keep status_changed, completed, canceled, reopened for 30 days
// - Delete other event types after 7 days
//
// In dry-run mode, calculates what WOULD be deleted without actually deleting.
func (db *DB) CleanupHistory(opts CleanupHistoryOptions) (CleanupHistoryResult, error) {
	var result CleanupHistoryResult

	// Get total count before cleanup
	if err := db.QueryRow("SELECT COUNT(*) FROM history").Scan(&result.TotalBefore); err != nil {
		return result, fmt.Errorf("failed to count history: %w", err)
	}

	// The WHERE clause for deletable entries:
	// - More than 24 hours old AND
	// - Either:
	//   - Status events older than 30 days, OR
	//   - Non-status events older than 7 days
	whereClause := `
		created_at < datetime('now', '-24 hours') AND
		(
			(event_type IN ('status_changed', 'completed', 'canceled', 'reopened') 
			 AND created_at < datetime('now', '-30 days'))
			OR
			(event_type NOT IN ('status_changed', 'completed', 'canceled', 'reopened')
			 AND created_at < datetime('now', '-7 days'))
		)
	`

	// Count entries to delete by category
	// Status events >30 days
	err := db.QueryRow(`
		SELECT COUNT(*) FROM history WHERE
			created_at < datetime('now', '-24 hours') AND
			event_type IN ('status_changed', 'completed', 'canceled', 'reopened') AND
			created_at < datetime('now', '-30 days')
	`).Scan(&result.DeletedStatus)
	if err != nil {
		return result, fmt.Errorf("failed to count old status events: %w", err)
	}

	// Non-status events >7 days (but at least 24h old)
	err = db.QueryRow(`
		SELECT COUNT(*) FROM history WHERE
			created_at < datetime('now', '-24 hours') AND
			event_type NOT IN ('status_changed', 'completed', 'canceled', 'reopened') AND
			created_at < datetime('now', '-7 days')
	`).Scan(&result.DeletedOther)
	if err != nil {
		return result, fmt.Errorf("failed to count old other events: %w", err)
	}

	// DeletedRecent is always 0 (we never delete entries <24h old)
	result.DeletedRecent = 0
	result.DeletedCount = result.DeletedStatus + result.DeletedOther

	// If dry run, return counts without deleting
	if opts.DryRun {
		return result, nil
	}

	// Perform actual deletion
	_, err = db.Exec("DELETE FROM history WHERE " + whereClause)
	if err != nil {
		return result, fmt.Errorf("failed to delete history: %w", err)
	}

	return result, nil
}
