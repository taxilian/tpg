package db

import (
	"fmt"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

// CleanupCounts holds counts of items that would be or were deleted.
type CleanupCounts struct {
	DoneItems     int
	CanceledItems int
	Logs          int
	Dependencies  int
	ItemLabels    int
}

// CountOldItems returns the count of items older than the given date with the given status.
func (db *DB) CountOldItems(before time.Time, status model.Status) (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM items 
		WHERE status = ? AND updated_at < ?`,
		status, sqlTime(before)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count old items: %w", err)
	}
	return count, nil
}

// GetOldItemIDs returns the IDs of items older than the given date with the given status.
func (db *DB) GetOldItemIDs(before time.Time, status model.Status) ([]string, error) {
	rows, err := db.Query(`
		SELECT id FROM items 
		WHERE status = ? AND updated_at < ?`,
		status, sqlTime(before))
	if err != nil {
		return nil, fmt.Errorf("failed to get old item IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan item ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteOldItems removes items older than the given date with the given status.
// Returns the number of items deleted and associated cleanup counts.
func (db *DB) DeleteOldItems(before time.Time, status model.Status) (int, error) {
	// Get the IDs first
	ids, err := db.GetOldItemIDs(before, status)
	if err != nil {
		return 0, err
	}

	if len(ids) == 0 {
		return 0, nil
	}

	// Delete in a transaction
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range ids {
		// Delete logs
		if _, err := tx.Exec(`DELETE FROM logs WHERE item_id = ?`, id); err != nil {
			return 0, fmt.Errorf("failed to delete logs for %s: %w", id, err)
		}

		// Delete dependencies (both directions)
		if _, err := tx.Exec(`DELETE FROM deps WHERE item_id = ? OR depends_on = ?`, id, id); err != nil {
			return 0, fmt.Errorf("failed to delete dependencies for %s: %w", id, err)
		}

		// Delete item labels
		if _, err := tx.Exec(`DELETE FROM item_labels WHERE item_id = ?`, id); err != nil {
			return 0, fmt.Errorf("failed to delete item labels for %s: %w", id, err)
		}

		// Delete the item
		if _, err := tx.Exec(`DELETE FROM items WHERE id = ?`, id); err != nil {
			return 0, fmt.Errorf("failed to delete item %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return len(ids), nil
}

// CountOrphanedLogs returns the count of logs that reference non-existent items.
func (db *DB) CountOrphanedLogs() (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM logs 
		WHERE item_id NOT IN (SELECT id FROM items)`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count orphaned logs: %w", err)
	}
	return count, nil
}

// DeleteOrphanedLogs removes logs that reference non-existent items.
func (db *DB) DeleteOrphanedLogs() (int, error) {
	result, err := db.Exec(`
		DELETE FROM logs 
		WHERE item_id NOT IN (SELECT id FROM items)`)
	if err != nil {
		return 0, fmt.Errorf("failed to delete orphaned logs: %w", err)
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// CountOldLogs returns the count of logs older than the given date.
func (db *DB) CountOldLogs(before time.Time) (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM logs 
		WHERE created_at < ?`,
		sqlTime(before)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count old logs: %w", err)
	}
	return count, nil
}

// DeleteOldLogs removes logs older than the given date.
func (db *DB) DeleteOldLogs(before time.Time) (int, error) {
	result, err := db.Exec(`
		DELETE FROM logs 
		WHERE created_at < ?`,
		sqlTime(before))
	if err != nil {
		return 0, fmt.Errorf("failed to delete old logs: %w", err)
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// Vacuum compacts the database by running SQLite VACUUM.
// Returns the size difference in bytes (positive means space saved).
func (db *DB) Vacuum() error {
	_, err := db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	return nil
}

// GetDatabaseSize returns the current database file size in bytes.
// This is an approximation using SQLite's page_count * page_size.
func (db *DB) GetDatabaseSize() (int64, error) {
	var pageCount, pageSize int64
	if err := db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return 0, fmt.Errorf("failed to get page count: %w", err)
	}
	if err := db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, fmt.Errorf("failed to get page size: %w", err)
	}
	return pageCount * pageSize, nil
}
