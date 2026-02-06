package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

// Event type constants for history tracking
const (
	EventTypeCreated            = "created"
	EventTypeStatusChanged      = "status_changed"
	EventTypeTitleChanged       = "title_changed"
	EventTypeDescriptionChanged = "description_changed"
	EventTypePriorityChanged    = "priority_changed"
	EventTypeParentChanged      = "parent_changed"
	EventTypeAssigned           = "assigned"
	EventTypeCompleted          = "completed"
	EventTypeCanceled           = "canceled"
	EventTypeReopened           = "reopened"
	EventTypeDependencyAdded    = "dependency_added"
	EventTypeDependencyRemoved  = "dependency_removed"
)

// HistoryEntry represents a single history event for an item.
type HistoryEntry struct {
	ID        int64
	ItemID    string
	EventType string
	ActorID   string
	ActorType string
	Changes   map[string]any // Parsed JSON
	CreatedAt time.Time
}

// HistoryQueryOptions configures history queries.
type HistoryQueryOptions struct {
	ItemID     string    // Filter by specific item
	ActorID    string    // Filter by actor/agent
	Since      time.Time // Filter by time (entries >= since)
	EventTypes []string  // Filter by event type(s)
	Limit      int       // Max results (default 50)
}

// defaultHistoryLimit is the default limit for history queries.
const defaultHistoryLimit = 50

// RecordHistory records a history event for an item.
// This is a non-blocking operation — if recording fails, it logs a warning but doesn't
// return an error. This ensures history recording doesn't break existing functionality.
// The changes parameter should contain "old" and "new" values for change events,
// or just "value" for creation events.
func (db *DB) RecordHistory(itemID, eventType string, changes map[string]any) error {
	// Get actor context from environment
	agentCtx := GetAgentContext()

	// Marshal changes to JSON
	var changesJSON []byte
	var err error
	if changes != nil {
		changesJSON, err = json.Marshal(changes)
		if err != nil {
			log.Printf("warning: failed to marshal history changes for %s: %v", itemID, err)
			return nil // Non-fatal, don't break the operation
		}
	}

	// Insert history entry
	_, err = db.Exec(`
		INSERT INTO history (item_id, event_type, actor_id, actor_type, changes, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, itemID, eventType, nullString(agentCtx.ID), nullString(agentCtx.Type), string(changesJSON), sqlTime(time.Now()))
	if err != nil {
		log.Printf("warning: failed to record history for %s: %v", itemID, err)
		return nil // Non-fatal, don't break the operation
	}

	return nil
}

// nullString returns a sql.NullString that is NULL if s is empty.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// GetHistory retrieves history entries with flexible filtering options.
// Results are ordered by created_at DESC (newest first).
// Uses the appropriate index based on provided filters.
func (db *DB) GetHistory(opts HistoryQueryOptions) ([]HistoryEntry, error) {
	// Apply default limit if not specified
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultHistoryLimit
	}

	// Build query dynamically based on filters
	query := `SELECT id, item_id, event_type, actor_id, actor_type, changes, created_at
		FROM history WHERE 1=1`
	args := []any{}

	// Filter by item ID (uses idx_history_item_time)
	if opts.ItemID != "" {
		query += ` AND item_id = ?`
		args = append(args, opts.ItemID)
	}

	// Filter by actor ID (uses idx_history_actor_time)
	if opts.ActorID != "" {
		query += ` AND actor_id = ?`
		args = append(args, opts.ActorID)
	}

	// Filter by time (since)
	if !opts.Since.IsZero() {
		query += ` AND created_at >= ?`
		args = append(args, sqlTime(opts.Since))
	}

	// Filter by event types (IN clause)
	if len(opts.EventTypes) > 0 {
		placeholders := make([]string, len(opts.EventTypes))
		for i := range opts.EventTypes {
			placeholders[i] = "?"
		}
		query += fmt.Sprintf(` AND event_type IN (%s)`, strings.Join(placeholders, ", "))
		for _, et := range opts.EventTypes {
			args = append(args, et)
		}
	}

	// Order by created_at DESC (uses idx_history_recent for general queries)
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	return db.queryHistoryEntries(query, args...)
}

// GetItemHistory is a convenience wrapper for getting history of a specific item.
// Returns up to `limit` entries, sorted by created_at DESC.
func (db *DB) GetItemHistory(itemID string, limit int) ([]HistoryEntry, error) {
	return db.GetHistory(HistoryQueryOptions{
		ItemID: itemID,
		Limit:  limit,
	})
}

// GetRecentlyClosed returns items that have been closed (done or canceled).
// Results are ordered by closed_at DESC (most recently closed first).
// If since is non-zero, only returns items closed after that time.
func (db *DB) GetRecentlyClosed(limit int, since time.Time) ([]model.Item, error) {
	// Apply reasonable default if limit not specified
	if limit <= 0 {
		limit = defaultHistoryLimit
	}

	// Build query for items with closed_at set
	query := fmt.Sprintf(`SELECT %s FROM items WHERE closed_at IS NOT NULL`, itemSelectColumns)
	args := []any{}

	// Filter by since time if specified
	if !since.IsZero() {
		query += ` AND closed_at >= ?`
		args = append(args, sqlTime(since))
	}

	// Order by closed_at DESC (most recently closed first) and apply limit
	query += ` ORDER BY closed_at DESC LIMIT ?`
	args = append(args, limit)

	return db.queryItems(query, args...)
}

// queryHistoryEntries is a helper to scan history entry rows.
func (db *DB) queryHistoryEntries(query string, args ...any) ([]HistoryEntry, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []HistoryEntry
	for rows.Next() {
		var entry HistoryEntry
		var actorID sql.NullString
		var actorType sql.NullString
		var changesJSON sql.NullString

		if err := rows.Scan(
			&entry.ID, &entry.ItemID, &entry.EventType,
			&actorID, &actorType, &changesJSON,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan history entry: %w", err)
		}

		// Handle nullable fields
		if actorID.Valid {
			entry.ActorID = actorID.String
		}
		if actorType.Valid {
			entry.ActorType = actorType.String
		}

		// Parse JSON changes gracefully
		if changesJSON.Valid && changesJSON.String != "" {
			var changes map[string]any
			if err := json.Unmarshal([]byte(changesJSON.String), &changes); err != nil {
				// Graceful degradation: malformed JSON results in nil Changes
				// rather than failing the entire query
				entry.Changes = nil
			} else {
				entry.Changes = changes
			}
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate history rows: %w", err)
	}

	return entries, nil
}
