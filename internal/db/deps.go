package db

import (
	"fmt"
	"strings"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

// AddDep adds a dependency between items.
// If itemID is in_progress and dependsOnID is not done, itemID is reverted
// to open with a log entry — an in_progress task with unmet deps is invalid.
func (db *DB) AddDep(itemID, dependsOnID string) error {
	// Check for self-dependency first (before existence check, since IN clause fails for same ID twice)
	if itemID == dependsOnID {
		return fmt.Errorf("cannot create self-dependency: %s cannot depend on itself", itemID)
	}

	// Verify both items exist
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM items WHERE id IN (?, ?)`, itemID, dependsOnID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to verify items: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("one or both items not found: %s, %s (use 'tpg list' to see available items)", itemID, dependsOnID)
	}

	// Check for parent-child circular dependency (common mistake)
	if err := checkParentChildCycle(db, itemID, dependsOnID); err != nil {
		return err
	}

	// Check for general circular dependency
	if wouldCreateCycle(db, itemID, dependsOnID) {
		return fmt.Errorf("cannot add dependency: %s already depends on %s (would create cycle)", dependsOnID, itemID)
	}

	_, err = db.Exec(`
		INSERT OR IGNORE INTO deps (item_id, depends_on) VALUES (?, ?)`,
		itemID, dependsOnID)
	if err != nil {
		return fmt.Errorf("failed to add dependency: %w", err)
	}

	// If the dependent task is in_progress and the new dep is not done,
	// revert to open — it can't proceed until the dep is resolved.
	var itemStatus, depStatus string
	_ = db.QueryRow(`SELECT status FROM items WHERE id = ?`, itemID).Scan(&itemStatus)
	_ = db.QueryRow(`SELECT status FROM items WHERE id = ?`, dependsOnID).Scan(&depStatus)

	if itemStatus == "in_progress" && depStatus != "done" {
		_, _ = db.Exec(`UPDATE items SET status = 'open', agent_id = NULL, agent_last_active = NULL, updated_at = ? WHERE id = ?`,
			sqlTime(time.Now()), itemID)
		_ = db.AddLog(itemID, fmt.Sprintf("Reverted to open: dependency added on %s (not yet done)", dependsOnID))
	}

	return nil
}

// RemoveDep removes a dependency between items.
func (db *DB) RemoveDep(itemID, dependsOnID string) error {
	result, err := db.Exec(`DELETE FROM deps WHERE item_id = ? AND depends_on = ?`, itemID, dependsOnID)
	if err != nil {
		return fmt.Errorf("failed to remove dependency: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no dependency found: %s does not depend on %s", itemID, dependsOnID)
	}
	return nil
}

// GetBlockedBy returns the IDs and details of items that the given item blocks.
func (db *DB) GetBlockedBy(itemID string) ([]DepStatus, error) {
	rows, err := db.Query(`
		SELECT d.item_id, i.title, i.status
		FROM deps d
		JOIN items i ON d.item_id = i.id
		WHERE d.depends_on = ?`, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocked items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var deps []DepStatus
	for rows.Next() {
		var dep DepStatus
		if err := rows.Scan(&dep.ID, &dep.Title, &dep.Status); err != nil {
			return nil, fmt.Errorf("failed to scan blocked item: %w", err)
		}
		deps = append(deps, dep)
	}
	return deps, rows.Err()
}

// GetDeps returns the IDs of items that the given item depends on.
func (db *DB) GetDeps(itemID string) ([]string, error) {
	rows, err := db.Query(`SELECT depends_on FROM deps WHERE item_id = ?`, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependencies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var deps []string
	for rows.Next() {
		var depID string
		if err := rows.Scan(&depID); err != nil {
			return nil, fmt.Errorf("failed to scan dependency: %w", err)
		}
		deps = append(deps, depID)
	}
	return deps, rows.Err()
}

// HasUnmetDeps returns true if the item has dependencies that are not done.
func (db *DB) HasUnmetDeps(itemID string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM deps d
		JOIN items i ON d.depends_on = i.id
		WHERE d.item_id = ? AND i.status != 'done'`, itemID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check dependencies: %w", err)
	}
	return count > 0, nil
}

// DepEdge represents a dependency relationship with item details.
type DepEdge struct {
	ItemID          string
	ItemTitle       string
	ItemStatus      string
	DependsOnID     string
	DependsOnTitle  string
	DependsOnStatus string
}

// DepStatus represents a dependency with status details.
type DepStatus struct {
	ID            string
	Title         string
	Status        string
	IsInherited   bool   // True if this dependency is inherited from an ancestor epic
	InheritedFrom string // The ancestor epic ID from which this dependency is inherited
}

// GetDepStatuses returns dependencies for a single item with their statuses.
func (db *DB) GetDepStatuses(itemID string) ([]DepStatus, error) {
	rows, err := db.Query(`
		SELECT d.depends_on, i.title, i.status
		FROM deps d
		JOIN items i ON d.depends_on = i.id
		WHERE d.item_id = ?`, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency statuses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var deps []DepStatus
	for rows.Next() {
		var dep DepStatus
		if err := rows.Scan(&dep.ID, &dep.Title, &dep.Status); err != nil {
			return nil, fmt.Errorf("failed to scan dependency status: %w", err)
		}
		deps = append(deps, dep)
	}
	return deps, rows.Err()
}

// GetAncestorDependencies returns all unmet dependencies from ancestor epics.
// These are dependencies that the item implicitly inherits from its parent epic chain.
func (db *DB) GetAncestorDependencies(itemID string) ([]DepStatus, error) {
	// Get parent chain (ordered by depth DESC, so root is first, immediate parent is last)
	ancestors, err := db.GetParentChain(itemID)
	if err != nil {
		return nil, err
	}

	var ancestorDeps []DepStatus

	// For each ancestor, get its dependencies
	for _, ancestor := range ancestors {
		// Only consider epics (not intermediate tasks if any)
		if ancestor.Type != model.ItemTypeEpic {
			continue
		}

		// Get direct dependencies of this ancestor
		rows, err := db.Query(`
			SELECT d.depends_on, i.title, i.status
			FROM deps d
			JOIN items i ON d.depends_on = i.id
			WHERE d.item_id = ? AND i.status != 'done'`, ancestor.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get ancestor dependencies: %w", err)
		}

		for rows.Next() {
			var dep DepStatus
			if err := rows.Scan(&dep.ID, &dep.Title, &dep.Status); err != nil {
				rows.Close()
				return nil, fmt.Errorf("failed to scan ancestor dependency: %w", err)
			}
			// Mark as inherited
			dep.IsInherited = true
			dep.InheritedFrom = ancestor.ID
			ancestorDeps = append(ancestorDeps, dep)
		}
		rows.Close()
	}

	return ancestorDeps, nil
}

// GetAllDepStatuses returns both direct and inherited dependencies for an item.
func (db *DB) GetAllDepStatuses(itemID string) ([]DepStatus, error) {
	// Get direct dependencies
	directDeps, err := db.GetDepStatuses(itemID)
	if err != nil {
		return nil, err
	}

	// Get inherited dependencies from ancestors
	inheritedDeps, err := db.GetAncestorDependencies(itemID)
	if err != nil {
		return nil, err
	}

	// Combine both
	allDeps := append(directDeps, inheritedDeps...)
	return allDeps, nil
}

// ImpactItem represents a task that would become ready if dependencies are resolved.
type ImpactItem struct {
	ID       string
	Title    string
	Priority int
	Depth    int // Distance from the original task
}

// GetImpact returns all tasks that would become ready if the given task is completed.
// It finds tasks that are currently blocked only by this task (or by other tasks that
// would also become unblocked).
func (db *DB) GetImpact(itemID string) ([]ImpactItem, error) {
	// First, verify the item exists
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM items WHERE id = ?`, itemID).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to verify item: %w", err)
	}
	if count == 0 {
		return nil, fmt.Errorf("item not found: %s", itemID)
	}

	// Use a recursive CTE to find all tasks that would become ready
	// A task becomes ready when:
	// 1. Status is 'open'
	// 2. All its dependencies are 'done'
	//
	// When task X is completed, any task that depends on X will have one less
	// unmet dependency. If that was their only unmet dependency, they become ready.
	//
	// We need to find tasks where ALL non-done dependencies are in the "impact chain"
	// (i.e., they would also become ready when their dependencies are done)
	//
	// Note: SQLite doesn't allow multiple references to a recursive CTE within
	// the recursive part, so we use a two-phase approach:
	// 1. First, find all potentially impacted tasks (all tasks downstream in the dep graph)
	// 2. Then, filter to only those where all non-done deps are in the impact set

	query := `
		WITH RECURSIVE 
		-- Phase 1: Find all tasks downstream from the given task (transitive closure)
		-- These are tasks that would become done if the given task is completed
		downstream(id, depth) AS (
			-- Base case: tasks directly depending on the given task
			SELECT d.item_id, 1
			FROM deps d
			JOIN items i ON d.item_id = i.id
			WHERE d.depends_on = ?
			  AND i.status = 'open'
			
			UNION ALL
			
			-- Recursive case: tasks depending on downstream tasks
			SELECT d.item_id, dws.depth + 1
			FROM deps d
			JOIN items i ON d.item_id = i.id
			JOIN downstream dws ON d.depends_on = dws.id
			WHERE i.status = 'open'
			  AND dws.depth < 100  -- Prevent infinite loops
		),
		-- Phase 2: Find tasks that would become ready
		-- A task becomes ready when all its non-done dependencies would be done
		-- (either already done, or in the downstream set)
		impact_candidates AS (
			SELECT 
				d.item_id,
				-- Depth is the task's depth in the downstream chain
				-- We use MIN to handle cases where a task has multiple paths
				COALESCE(MIN(dws_item.depth), 0) as depth,
				-- Count of non-done dependencies
				COUNT(CASE WHEN dep.status != 'done' THEN 1 END) as non_done_dep_count
			FROM deps d
			JOIN items i ON d.item_id = i.id
			JOIN items dep ON d.depends_on = dep.id
			-- Join to check if dependencies are in downstream
			LEFT JOIN downstream dws ON d.depends_on = dws.id
			-- Join to get the task's own depth in downstream
			LEFT JOIN downstream dws_item ON d.item_id = dws_item.id
			WHERE i.status = 'open'
			GROUP BY d.item_id
			HAVING 
				-- Task must have at least one non-done dependency (otherwise it's already ready)
				COUNT(CASE WHEN dep.status != 'done' THEN 1 END) > 0
				-- All non-done dependencies are in the downstream set (would become done)
				-- We check: count of non-done deps = count of non-done deps that are in downstream
				AND COUNT(CASE WHEN dep.status != 'done' THEN 1 END) = 
				COUNT(CASE WHEN dep.status != 'done' AND (d.depends_on = ? OR dws.id IS NOT NULL) THEN 1 END)
				-- And at least one dependency is in downstream (i.e., this task is actually affected)
				AND COUNT(CASE WHEN d.depends_on = ? OR dws.id IS NOT NULL THEN 1 END) > 0
		)
		SELECT i.id, i.title, i.priority, ic.depth
		FROM impact_candidates ic
		JOIN items i ON ic.item_id = i.id
		ORDER BY ic.depth, i.priority ASC, i.created_at ASC
	`

	rows, err := db.Query(query, itemID, itemID, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate impact: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []ImpactItem
	for rows.Next() {
		var item ImpactItem
		if err := rows.Scan(&item.ID, &item.Title, &item.Priority, &item.Depth); err != nil {
			return nil, fmt.Errorf("failed to scan impact item: %w", err)
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// GetAllDeps returns all dependency edges with item details, optionally filtered by project.
func (db *DB) GetAllDeps(project string) ([]DepEdge, error) {
	query := `
		SELECT
			d.item_id, i1.title, i1.status,
			d.depends_on, i2.title, i2.status
		FROM deps d
		JOIN items i1 ON d.item_id = i1.id
		JOIN items i2 ON d.depends_on = i2.id`
	args := []any{}

	if project != "" {
		query += ` WHERE i1.project = ?`
		args = append(args, project)
	}
	query += ` ORDER BY i1.priority, i1.id`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query deps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var edges []DepEdge
	for rows.Next() {
		var e DepEdge
		if err := rows.Scan(&e.ItemID, &e.ItemTitle, &e.ItemStatus,
			&e.DependsOnID, &e.DependsOnTitle, &e.DependsOnStatus); err != nil {
			return nil, fmt.Errorf("failed to scan dep edge: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// GetDependencyChain returns all transitive dependencies of an item (what it depends on).
func (db *DB) GetDependencyChain(itemID string) ([]DepEdge, error) {
	query := `
		WITH RECURSIVE dep_chain(item_id, depends_on, depth) AS (
			-- Base case: direct dependencies
			SELECT d.item_id, d.depends_on, 1
			FROM deps d
			WHERE d.item_id = ?
			UNION ALL
			-- Recursive case: dependencies of dependencies
			SELECT d.item_id, d.depends_on, dc.depth + 1
			FROM deps d
			JOIN dep_chain dc ON d.item_id = dc.depends_on
			WHERE dc.depth < 100  -- Prevent infinite loops
		)
		SELECT 
			dc.item_id, i1.title, i1.status,
			dc.depends_on, i2.title, i2.status
		FROM dep_chain dc
		JOIN items i1 ON dc.item_id = i1.id
		JOIN items i2 ON dc.depends_on = i2.id
		ORDER BY dc.depth, i1.priority
	`

	rows, err := db.Query(query, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dependency chain: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var edges []DepEdge
	for rows.Next() {
		var e DepEdge
		if err := rows.Scan(&e.ItemID, &e.ItemTitle, &e.ItemStatus,
			&e.DependsOnID, &e.DependsOnTitle, &e.DependsOnStatus); err != nil {
			return nil, fmt.Errorf("failed to scan dep edge: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// GetReverseDependencyChain returns all items that transitively depend on the given item.
func (db *DB) GetReverseDependencyChain(itemID string) ([]DepEdge, error) {
	query := `
		WITH RECURSIVE rev_dep_chain(item_id, depends_on, depth) AS (
			-- Base case: items that directly depend on the given item
			SELECT d.item_id, d.depends_on, 1
			FROM deps d
			WHERE d.depends_on = ?
			UNION ALL
			-- Recursive case: items that depend on items in the chain
			SELECT d.item_id, d.depends_on, rd.depth + 1
			FROM deps d
			JOIN rev_dep_chain rd ON d.depends_on = rd.item_id
			WHERE rd.depth < 100  -- Prevent infinite loops
		)
		SELECT 
			dc.item_id, i1.title, i1.status,
			dc.depends_on, i2.title, i2.status
		FROM rev_dep_chain dc
		JOIN items i1 ON dc.item_id = i1.id
		JOIN items i2 ON dc.depends_on = i2.id
		ORDER BY dc.depth, i1.priority
	`

	rows, err := db.Query(query, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reverse dependency chain: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var edges []DepEdge
	for rows.Next() {
		var e DepEdge
		if err := rows.Scan(&e.ItemID, &e.ItemTitle, &e.ItemStatus,
			&e.DependsOnID, &e.DependsOnTitle, &e.DependsOnStatus); err != nil {
			return nil, fmt.Errorf("failed to scan dep edge: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// wouldCreateCycle checks if adding itemID -> dependsOnID would create a cycle.
// A cycle exists if dependsOnID already depends on itemID (directly or transitively).
func wouldCreateCycle(db *DB, itemID, dependsOnID string) bool {
	// Use BFS to check if dependsOnID can reach itemID
	visited := make(map[string]bool)
	queue := []string{dependsOnID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		// If we can reach itemID from dependsOnID, adding itemID -> dependsOnID creates cycle
		if current == itemID {
			return true
		}

		// Get all items that current depends on
		deps, err := db.GetDeps(current)
		if err != nil {
			continue // Skip on error, conservative approach
		}
		queue = append(queue, deps...)
	}

	return false
}

// checkParentChildCycle specifically checks for parent-child circular dependencies.
// This detects when trying to create a dependency between a parent and its child.
func checkParentChildCycle(db *DB, itemID, dependsOnID string) error {
	// Get the item that would have the dependency added
	item, err := db.GetItem(itemID)
	if err != nil {
		return nil // Can't check, allow it
	}

	// Check if dependsOnID is a child of itemID (via ParentID)
	// This would create: Parent depends on Child, but Child has ParentID = Parent
	childrenOfItem, err := db.getChildIDs(itemID)
	if err != nil {
		return nil
	}

	for _, childID := range childrenOfItem {
		if childID == dependsOnID {
			return fmt.Errorf("cannot create dependency: %s is a child of %s. Parent-child relationships should not have dependencies", dependsOnID, itemID)
		}
	}

	// Check if itemID is a child of dependsOnID
	// This would create: Child depends on Parent, but Child has ParentID = Parent
	if item.ParentID != nil && *item.ParentID == dependsOnID {
		return fmt.Errorf("cannot create dependency: %s is the parent of %s. Parent-child relationships should not have dependencies", dependsOnID, itemID)
	}

	return nil
}

// getChildIDs returns all item IDs that have the given item as their parent.
func (db *DB) getChildIDs(parentID string) ([]string, error) {
	rows, err := db.Query(`SELECT id FROM items WHERE parent_id = ?`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var children []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		children = append(children, id)
	}
	return children, rows.Err()
}

// CircularDep represents a circular dependency found in the database
type CircularDep struct {
	ItemID      string
	DependsOnID string
	CyclePath   []string // Full cycle path for reporting
}

// FindCircularDeps scans all dependencies and returns any circular ones
func (db *DB) FindCircularDeps() ([]CircularDep, error) {
	// Get all dependencies
	rows, err := db.Query(`SELECT item_id, depends_on FROM deps`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Build adjacency list
	deps := make(map[string][]string)
	allItems := make(map[string]bool)
	for rows.Next() {
		var itemID, dependsOnID string
		if err := rows.Scan(&itemID, &dependsOnID); err != nil {
			continue
		}
		deps[itemID] = append(deps[itemID], dependsOnID)
		allItems[itemID] = true
		allItems[dependsOnID] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	var circularDeps []CircularDep
	foundCycles := make(map[string]bool) // Track found cycles to avoid duplicates

	// For each item, check if following its deps leads back to itself
	for itemID := range allItems {
		if cycle := findCycleDFS(itemID, deps, []string{}, make(map[string]bool)); cycle != nil {
			// Create a unique key for this cycle to avoid duplicates
			cycleKey := strings.Join(cycle, "->")
			if !foundCycles[cycleKey] {
				foundCycles[cycleKey] = true
				circularDeps = append(circularDeps, CircularDep{
					ItemID:      cycle[0],
					DependsOnID: cycle[len(cycle)-2], // The item that points back to start
					CyclePath:   cycle,
				})
			}
		}
	}

	return circularDeps, nil
}

// findCycleDFS uses DFS to detect cycles starting from the given node
func findCycleDFS(start string, deps map[string][]string, path []string, visitedInPath map[string]bool) []string {
	// If we've seen this node in the current path, we found a cycle
	if visitedInPath[start] {
		// Extract the cycle from the path
		for i, node := range path {
			if node == start {
				// Return cycle from first occurrence of start to end, plus start again
				cycle := append(path[i:], start)
				return cycle
			}
		}
		return nil
	}

	// Mark as visited in current path
	visitedInPath[start] = true
	path = append(path, start)

	// Check all dependencies of this node
	for _, next := range deps[start] {
		if cycle := findCycleDFS(next, deps, path, visitedInPath); cycle != nil {
			return cycle
		}
	}

	// Unmark when backtracking
	visitedInPath[start] = false
	return nil
}

// ParentChildDep represents a parent-child circular dependency
type ParentChildDep struct {
	ParentID string
	ChildID  string
}

// FindParentChildCircularDeps finds dependencies where a parent depends on its child
// This is the specific issue caused by the template bug
func (db *DB) FindParentChildCircularDeps() ([]ParentChildDep, error) {
	// Find all dependencies where:
	// 1. Item depends on another item that has it as parent (parent depends on child)
	// 2. Item depends on its own parent (child depends on parent)
	query := `
		SELECT d.item_id, d.depends_on
		FROM deps d
		JOIN items child ON child.id = d.depends_on
		WHERE child.parent_id = d.item_id

		UNION

		SELECT d.item_id, d.depends_on
		FROM deps d
		JOIN items item ON item.id = d.item_id
		WHERE item.parent_id = d.depends_on
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []ParentChildDep
	seen := make(map[string]bool)
	for rows.Next() {
		var parentID, childID string
		if err := rows.Scan(&parentID, &childID); err != nil {
			continue
		}
		// Use sorted key to avoid duplicates
		key := parentID + "->" + childID
		if !seen[key] {
			seen[key] = true
			deps = append(deps, ParentChildDep{ParentID: parentID, ChildID: childID})
		}
	}

	return deps, rows.Err()
}

// RemoveCircularDep removes a specific circular dependency
func (db *DB) RemoveCircularDep(itemID, dependsOnID string) error {
	_, err := db.Exec(`DELETE FROM deps WHERE item_id = ? AND depends_on = ?`, itemID, dependsOnID)
	return err
}

// FixAllParentChildCircularDeps removes all parent-child circular deps
func (db *DB) FixAllParentChildCircularDeps() (int, error) {
	deps, err := db.FindParentChildCircularDeps()
	if err != nil {
		return 0, err
	}

	fixed := 0
	for _, dep := range deps {
		if err := db.RemoveCircularDep(dep.ParentID, dep.ChildID); err != nil {
			continue
		}
		fixed++
	}

	return fixed, nil
}
