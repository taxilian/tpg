package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

// CreateItem inserts a new item into the database.
// If the item has a project, it will be auto-created if it doesn't exist.
func (db *DB) CreateItem(item *model.Item) error {
	if !item.Type.IsValid() {
		return fmt.Errorf("invalid item type: %s", item.Type)
	}
	if !item.Status.IsValid() {
		return fmt.Errorf("invalid status: %s", item.Status)
	}

	// Check if parent is closed (cannot add child to closed parent)
	if item.ParentID != nil && *item.ParentID != "" {
		var parentStatus model.Status
		err := db.QueryRow(`SELECT status FROM items WHERE id = ?`, *item.ParentID).Scan(&parentStatus)
		if err != nil {
			return fmt.Errorf("parent not found: %s (use 'tpg list' to see available items)", *item.ParentID)
		}
		if parentStatus == model.StatusDone || parentStatus == model.StatusCanceled {
			return fmt.Errorf("cannot add child to closed parent %s", *item.ParentID)
		}
	}

	// Auto-create project if specified
	if item.Project != "" {
		if err := db.EnsureProject(item.Project); err != nil {
			return err
		}
	}

	varsJSON, err := marshalTemplateVars(item.TemplateVars)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT INTO items (
			id, project, type, title, description, status, priority, parent_id,
			template_id, step_index, variables, template_hash, results,
			worktree_branch, worktree_base,
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Project, item.Type, item.Title, item.Description,
		item.Status, item.Priority, item.ParentID,
		item.TemplateID, item.StepIndex, varsJSON, item.TemplateHash, item.Results,
		item.WorktreeBranch, item.WorktreeBase,
		sqlTime(item.CreatedAt), sqlTime(item.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}
	return nil
}

// GetItem retrieves an item by ID.
func (db *DB) GetItem(id string) (*model.Item, error) {
	row := db.QueryRow(`
		SELECT id, project, type, title, description, status, priority, parent_id,
			agent_id, agent_last_active,
			template_id, step_index, variables, template_hash, results,
			worktree_branch, worktree_base,
			created_at, updated_at
		FROM items WHERE id = ?`, id)

	item := &model.Item{}
	var parentID sql.NullString
	var agentID sql.NullString
	var agentLastActive sql.NullTime
	var templateID sql.NullString
	var stepIndex sql.NullInt64
	var variables sql.NullString
	var templateHash sql.NullString
	var results sql.NullString
	var worktreeBranch sql.NullString
	var worktreeBase sql.NullString
	err := row.Scan(
		&item.ID, &item.Project, &item.Type, &item.Title, &item.Description,
		&item.Status, &item.Priority, &parentID,
		&agentID, &agentLastActive,
		&templateID, &stepIndex, &variables, &templateHash, &results,
		&worktreeBranch, &worktreeBase,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	if parentID.Valid {
		item.ParentID = &parentID.String
	}
	if agentID.Valid {
		item.AgentID = &agentID.String
	}
	if agentLastActive.Valid {
		item.AgentLastActive = &agentLastActive.Time
	}
	if templateID.Valid {
		item.TemplateID = templateID.String
	}
	if stepIndex.Valid {
		idx := int(stepIndex.Int64)
		item.StepIndex = &idx
	}
	if variables.Valid {
		vars, err := unmarshalTemplateVars(variables.String)
		if err != nil {
			return nil, err
		}
		item.TemplateVars = vars
	}
	if templateHash.Valid {
		item.TemplateHash = templateHash.String
	}
	if results.Valid {
		item.Results = results.String
	}
	if worktreeBranch.Valid {
		item.WorktreeBranch = worktreeBranch.String
	}
	if worktreeBase.Valid {
		item.WorktreeBase = worktreeBase.String
	}
	return item, nil
}

// UpdateStatus changes an item's status.
// UpdateStatus changes an item's status and optionally assigns it to an agent.
func (db *DB) UpdateStatus(id string, status model.Status, agentCtx AgentContext) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %s", status)
	}

	// Cannot close parent with open children
	if status == model.StatusDone || status == model.StatusCanceled {
		var openChildren int
		err := db.QueryRow(`SELECT COUNT(*) FROM items WHERE parent_id = ? AND status NOT IN ('done', 'canceled')`, id).Scan(&openChildren)
		if err != nil {
			return fmt.Errorf("failed to check children: %w", err)
		}
		if openChildren > 0 {
			return fmt.Errorf("cannot close %s: has %d open children", id, openChildren)
		}
	}

	// Update status and timestamp
	result, err := db.Exec(`
		UPDATE items SET status = ?, updated_at = ? WHERE id = ?`,
		status, sqlTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", id)
	}

	// If status is in_progress and agent is active, claim it
	if status == model.StatusInProgress && agentCtx.IsActive() {
		_, err = db.Exec(`UPDATE items 
			SET agent_id = ?, agent_last_active = CURRENT_TIMESTAMP
			WHERE id = ?`, agentCtx.ID, id)
		if err != nil {
			return fmt.Errorf("failed to set agent: %w", err)
		}
	}

	// If status is done/blocked/canceled, release it
	if status == model.StatusDone || status == model.StatusBlocked || status == model.StatusCanceled {
		_, err = db.Exec(`UPDATE items 
			SET agent_id = NULL, agent_last_active = NULL
			WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("failed to clear agent: %w", err)
		}
	}

	return nil
}

// CompleteItem marks an item as done, records a results message, and releases agent assignment.
func (db *DB) CompleteItem(id, results string, agentCtx AgentContext) error {
	// Cannot close parent with open children
	var openChildren int
	err := db.QueryRow(`SELECT COUNT(*) FROM items WHERE parent_id = ? AND status NOT IN ('done', 'canceled')`, id).Scan(&openChildren)
	if err != nil {
		return fmt.Errorf("failed to check children: %w", err)
	}
	if openChildren > 0 {
		return fmt.Errorf("cannot close %s: has %d open children", id, openChildren)
	}

	result, err := db.Exec(`
		UPDATE items SET status = ?, results = ?, updated_at = ? WHERE id = ?`,
		model.StatusDone, results, sqlTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("failed to complete item: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", id)
	}

	// Release agent assignment when done
	_, err = db.Exec(`UPDATE items 
		SET agent_id = NULL, agent_last_active = NULL
		WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to clear agent: %w", err)
	}

	return nil
}

// AppendDescription appends text to an item's description.
func (db *DB) AppendDescription(id string, text string) error {
	result, err := db.Exec(`
		UPDATE items
		SET description = COALESCE(description, '') || ? || char(10) || ?,
		    updated_at = ?
		WHERE id = ?`,
		"\n", text, sqlTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("failed to append description: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", id)
	}
	return nil
}

// SetParent sets an item's parent.
func (db *DB) SetParent(itemID, parentID string) error {
	// Verify parent exists and check its status
	var itemType string
	var parentStatus model.Status
	err := db.QueryRow(`SELECT type, status FROM items WHERE id = ?`, parentID).Scan(&itemType, &parentStatus)
	if err != nil {
		return fmt.Errorf("parent not found: %s (use 'tpg list' to see available items)", parentID)
	}

	// Cannot add child to closed parent
	if parentStatus == model.StatusDone || parentStatus == model.StatusCanceled {
		return fmt.Errorf("cannot add child to closed parent %s", parentID)
	}

	// Update the item's parent
	result, err := db.Exec(`
		UPDATE items SET parent_id = ?, updated_at = ? WHERE id = ?`,
		parentID, sqlTime(time.Now()), itemID)
	if err != nil {
		return fmt.Errorf("failed to set parent: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", itemID)
	}
	return nil
}

// SetProject changes an item's project.
func (db *DB) SetProject(id string, project string) error {
	// Auto-create project if specified
	if project != "" {
		if err := db.EnsureProject(project); err != nil {
			return err
		}
	}

	result, err := db.Exec(`
		UPDATE items SET project = ?, updated_at = ? WHERE id = ?`,
		project, sqlTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("failed to set project: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", id)
	}
	return nil
}

// SetDescription replaces an item's description entirely.
func (db *DB) SetDescription(id string, text string) error {
	result, err := db.Exec(`
		UPDATE items
		SET description = ?,
		    updated_at = ?
		WHERE id = ?`,
		text, sqlTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("failed to set description: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", id)
	}
	return nil
}

// SetTitle replaces an item's title.
func (db *DB) SetTitle(id string, title string) error {
	result, err := db.Exec(`
		UPDATE items
		SET title = ?,
		    updated_at = ?
		WHERE id = ?`,
		title, sqlTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("failed to set title: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", id)
	}
	return nil
}

// SetTemplateVar updates a single template variable for an item.
// It reads the current variables, updates the specified one, and writes back.
func (db *DB) SetTemplateVar(id string, varName string, value string) error {
	// First, get the current template variables
	var varsJSON sql.NullString
	err := db.QueryRow(`SELECT template_variables FROM items WHERE id = ?`, id).Scan(&varsJSON)
	if err == sql.ErrNoRows {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", id)
	}
	if err != nil {
		return fmt.Errorf("failed to get template variables: %w", err)
	}

	// Parse existing variables
	vars, err := unmarshalTemplateVars(varsJSON.String)
	if err != nil {
		return fmt.Errorf("failed to parse template variables: %w", err)
	}
	if vars == nil {
		vars = make(map[string]string)
	}

	// Update the variable
	vars[varName] = value

	// Marshal back to JSON
	newVarsJSON, err := marshalTemplateVars(vars)
	if err != nil {
		return fmt.Errorf("failed to encode template variables: %w", err)
	}

	// Update the database
	_, err = db.Exec(`
		UPDATE items
		SET template_variables = ?,
		    updated_at = ?
		WHERE id = ?`,
		newVarsJSON, sqlTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("failed to update template variable: %w", err)
	}

	return nil
}

// UpdatePriority changes an item's priority.
func (db *DB) UpdatePriority(id string, priority int) error {
	if priority < 1 || priority > 5 {
		return fmt.Errorf("invalid priority: %d (must be 1-5)", priority)
	}

	result, err := db.Exec(`
		UPDATE items
		SET priority = ?,
		    updated_at = ?
		WHERE id = ?`,
		priority, sqlTime(time.Now()), id)
	if err != nil {
		return fmt.Errorf("failed to update priority: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", id)
	}
	return nil
}

// DeleteItem removes an item and its associated logs and dependencies.
func (db *DB) DeleteItem(id string) error {
	// Check if item exists first
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM items WHERE id = ?`, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check item: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", id)
	}

	// Delete logs
	_, err = db.Exec(`DELETE FROM logs WHERE item_id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete logs: %w", err)
	}

	// Delete dependencies (both directions)
	_, err = db.Exec(`DELETE FROM deps WHERE item_id = ? OR depends_on = ?`, id, id)
	if err != nil {
		return fmt.Errorf("failed to delete dependencies: %w", err)
	}

	// Delete the item
	_, err = db.Exec(`DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	return nil
}

// GetChildren returns all items that have the given ID as their parent.
func (db *DB) GetChildren(parentID string) ([]model.Item, error) {
	query := fmt.Sprintf("SELECT %s FROM items WHERE parent_id = ? ORDER BY priority ASC, created_at ASC", itemSelectColumns)
	return db.queryItems(query, parentID)
}

// GetEpics returns all epics for a project (or all projects if empty).
func (db *DB) GetEpics(project string) ([]model.Item, error) {
	query := fmt.Sprintf("SELECT %s FROM items WHERE type = 'epic'", itemSelectColumns)
	args := []any{}
	if project != "" {
		query += " AND project = ?"
		args = append(args, project)
	}
	query += " ORDER BY priority ASC, created_at ASC"
	return db.queryItems(query, args...)
}

// GetDescendants returns all descendants of an item (children, grandchildren, etc.)
func (db *DB) GetDescendants(itemID string) ([]model.Item, error) {
	query := fmt.Sprintf(`
		WITH RECURSIVE descendants(id) AS (
			-- Base case: direct children
			SELECT id FROM items WHERE parent_id = ?
			UNION ALL
			-- Recursive case: children of children
			SELECT i.id FROM items i
			JOIN descendants d ON i.parent_id = d.id
		)
		SELECT %s FROM items WHERE id IN (SELECT id FROM descendants)
		ORDER BY priority ASC, created_at ASC
	`, itemSelectColumns)
	return db.queryItems(query, itemID)
}

// SetWorktreeMetadata sets the worktree branch and base for an item.
func (db *DB) SetWorktreeMetadata(itemID, branch, base string) error {
	result, err := db.Exec(`
		UPDATE items
		SET worktree_branch = ?, worktree_base = ?, updated_at = ?
		WHERE id = ?`,
		branch, base, sqlTime(time.Now()), itemID)
	if err != nil {
		return fmt.Errorf("failed to set worktree metadata: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("item not found: %s (use 'tpg list' to see available items)", itemID)
	}
	return nil
}

// GetParentChain returns all ancestors of an item up to the root.
func (db *DB) GetParentChain(itemID string) ([]model.Item, error) {
	query := fmt.Sprintf(`
		WITH RECURSIVE ancestors(id, parent_id, depth) AS (
			-- Base case: the item itself
			SELECT id, parent_id, 0 FROM items WHERE id = ?
			UNION ALL
			-- Recursive case: parents of parents
			SELECT i.id, i.parent_id, a.depth + 1
			FROM items i
			JOIN ancestors a ON i.id = a.parent_id
			WHERE a.depth < 100  -- Prevent infinite loops
		)
		SELECT %s FROM items WHERE id IN (SELECT id FROM ancestors WHERE depth > 0)
		ORDER BY (SELECT depth FROM ancestors WHERE id = items.id) DESC
	`, itemSelectColumns)
	return db.queryItems(query, itemID)
}
