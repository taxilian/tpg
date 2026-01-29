package db

import (
	"fmt"
	"time"
)

// MergeItems merges sourceID into targetID, combining dependencies, logs, and labels.
// The source item is deleted after merging. This operation is not easily reversible.
//
// Merge semantics:
//   - All deps where source depends on X → target depends on X (unless target already does, or X == target)
//   - All deps where X depends on source → X depends on target (unless X already does, or X == target)
//   - Fails if the combined deps would create a cycle (target depending on itself)
//   - Source's logs are moved to target with a merge note
//   - Source's labels are copied to target
//   - Source's description is appended to target (if non-empty)
//   - Source is deleted
func (db *DB) MergeItems(sourceID, targetID string) error {
	// Verify both items exist
	srcItem, err := db.GetItem(sourceID)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	tgtItem, err := db.GetItem(targetID)
	if err != nil {
		return fmt.Errorf("target: %w", err)
	}

	// Cannot merge an item into itself
	if sourceID == targetID {
		return fmt.Errorf("cannot merge an item into itself")
	}

	// Collect source's deps in both directions
	// "source depends on X" rows
	srcDepsOn, err := db.getDepsRaw(sourceID)
	if err != nil {
		return fmt.Errorf("failed to read source deps: %w", err)
	}
	// "X depends on source" rows
	srcBlockedBy, err := db.getReverseDepsRaw(sourceID)
	if err != nil {
		return fmt.Errorf("failed to read source reverse deps: %w", err)
	}

	// Check for cycles: would target end up depending on itself?
	// After merge, target inherits source's "depends on" set.
	// Also, anything that depended on source now depends on target.
	// A cycle exists if target is in source's "depends on" set, or
	// source is in target's "depends on" set (since source's blockers
	// would become target's blockers, and target would block itself).
	for _, depID := range srcDepsOn {
		if depID == targetID {
			return fmt.Errorf("cycle: source %s depends on target %s — merging would create a self-dependency", sourceID, targetID)
		}
	}
	for _, depID := range srcBlockedBy {
		if depID == targetID {
			return fmt.Errorf("cycle: target %s depends on source %s — merging would create a self-dependency", targetID, sourceID)
		}
	}

	// Also check transitively: would target indirectly depend on itself?
	// Build the proposed dep set for target and walk it.
	tgtDepsOn, err := db.getDepsRaw(targetID)
	if err != nil {
		return fmt.Errorf("failed to read target deps: %w", err)
	}
	proposedDeps := make(map[string]bool)
	for _, d := range tgtDepsOn {
		proposedDeps[d] = true
	}
	for _, d := range srcDepsOn {
		if d != targetID {
			proposedDeps[d] = true
		}
	}
	if err := db.checkTransitiveCycle(targetID, proposedDeps); err != nil {
		return err
	}

	// --- All checks passed, perform the merge ---

	// 1. Move "source depends on X" → "target depends on X"
	for _, depID := range srcDepsOn {
		if depID == targetID {
			continue // skip self-ref
		}
		// INSERT OR IGNORE handles duplicates (target may already depend on X)
		_, err := db.Exec(`INSERT OR IGNORE INTO deps (item_id, depends_on) VALUES (?, ?)`, targetID, depID)
		if err != nil {
			return fmt.Errorf("failed to transfer dep %s: %w", depID, err)
		}
	}

	// 2. Move "X depends on source" → "X depends on target"
	for _, blockerID := range srcBlockedBy {
		if blockerID == targetID {
			continue // skip self-ref
		}
		_, err := db.Exec(`INSERT OR IGNORE INTO deps (item_id, depends_on) VALUES (?, ?)`, blockerID, targetID)
		if err != nil {
			return fmt.Errorf("failed to transfer reverse dep %s: %w", blockerID, err)
		}
	}

	// 3. Delete all source deps (both directions)
	_, err = db.Exec(`DELETE FROM deps WHERE item_id = ? OR depends_on = ?`, sourceID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to clean source deps: %w", err)
	}

	// 4. Move logs from source to target, prefixed with merge note
	_, err = db.Exec(`UPDATE logs SET item_id = ? WHERE item_id = ?`, targetID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to transfer logs: %w", err)
	}
	// Add a merge log entry
	db.Exec(`INSERT INTO logs (item_id, message, created_at) VALUES (?, ?, ?)`,
		targetID, fmt.Sprintf("Merged from %s: %s", sourceID, srcItem.Title), sqlTime(time.Now()))

	// 5. Copy labels from source to target
	_, err = db.Exec(`
		INSERT OR IGNORE INTO item_labels (item_id, label_id)
		SELECT ?, label_id FROM item_labels WHERE item_id = ?`, targetID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to transfer labels: %w", err)
	}
	_, _ = db.Exec(`DELETE FROM item_labels WHERE item_id = ?`, sourceID)

	// 6. Append source description to target if non-empty
	if srcItem.Description != "" {
		sep := ""
		if tgtItem.Description != "" {
			sep = "\n\n---\nMerged from " + sourceID + ":\n"
		}
		_, err = db.Exec(`UPDATE items SET description = description || ?, updated_at = ? WHERE id = ?`,
			sep+srcItem.Description, sqlTime(time.Now()), targetID)
		if err != nil {
			return fmt.Errorf("failed to append description: %w", err)
		}
	}

	// 7. Delete source item
	_, err = db.Exec(`DELETE FROM items WHERE id = ?`, sourceID)
	if err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	return nil
}

// getDepsRaw returns IDs that itemID depends on.
func (db *DB) getDepsRaw(itemID string) ([]string, error) {
	rows, err := db.Query(`SELECT depends_on FROM deps WHERE item_id = ?`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// getReverseDepsRaw returns IDs of items that depend on itemID.
func (db *DB) getReverseDepsRaw(itemID string) ([]string, error) {
	rows, err := db.Query(`SELECT item_id FROM deps WHERE depends_on = ?`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// checkTransitiveCycle walks the proposed dependency set to detect if targetID
// would transitively depend on itself.
func (db *DB) checkTransitiveCycle(targetID string, directDeps map[string]bool) error {
	visited := make(map[string]bool)
	queue := make([]string, 0, len(directDeps))
	for d := range directDeps {
		queue = append(queue, d)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == targetID {
			return fmt.Errorf("cycle: merging would create a transitive self-dependency on %s", targetID)
		}
		if visited[current] {
			continue
		}
		visited[current] = true

		// Get what current depends on
		next, err := db.getDepsRaw(current)
		if err != nil {
			return fmt.Errorf("failed to check transitive deps: %w", err)
		}
		queue = append(queue, next...)
	}
	return nil
}
