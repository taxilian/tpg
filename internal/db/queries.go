package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/taxilian/tpg/internal/model"
)

const itemSelectColumns = "id, project, type, title, description, status, priority, parent_id, agent_id, agent_last_active, template_id, step_index, variables, template_hash, results, created_at, updated_at"

// ListFilter contains optional filters for listing items.
type ListFilter struct {
	Project     string        // Filter by project
	Status      *model.Status // Filter by status
	Parent      string        // Filter by parent epic ID
	Type        string        // Filter by item type (task, epic)
	Blocking    string        // Show items that block this ID
	BlockedBy   string        // Show items blocked by this ID
	HasBlockers bool          // Show only items with unresolved blockers
	NoBlockers  bool          // Show only items with no blockers
	Labels      []string      // Filter by label names (AND - items must have all)
}

// ListItems returns items filtered by project and/or status.
func (db *DB) ListItems(project string, status *model.Status) ([]model.Item, error) {
	return db.ListItemsFiltered(ListFilter{Project: project, Status: status})
}

// ListItemsFiltered returns items matching the given filters.
func (db *DB) ListItemsFiltered(filter ListFilter) ([]model.Item, error) {
	query := fmt.Sprintf("SELECT %s FROM items WHERE 1=1", itemSelectColumns)
	args := []any{}

	if filter.Project != "" {
		query += ` AND project = ?`
		args = append(args, filter.Project)
	}
	if filter.Status != nil {
		if !filter.Status.IsValid() {
			return nil, fmt.Errorf("invalid status: %s", *filter.Status)
		}
		query += ` AND status = ?`
		args = append(args, *filter.Status)
	}
	if filter.Parent != "" {
		query += ` AND parent_id = ?`
		args = append(args, filter.Parent)
	}
	if filter.Type != "" {
		itemType := model.ItemType(filter.Type)
		if !itemType.IsValid() {
			return nil, fmt.Errorf("invalid type: %s (type cannot be empty)", filter.Type)
		}
		query += ` AND type = ?`
		args = append(args, filter.Type)
	}
	if filter.Blocking != "" {
		// Items that block the given ID (i.e., items the given ID depends on)
		query += ` AND id IN (SELECT depends_on FROM deps WHERE item_id = ?)`
		args = append(args, filter.Blocking)
	}
	if filter.BlockedBy != "" {
		// Items blocked by the given ID (i.e., items that depend on the given ID)
		query += ` AND id IN (SELECT item_id FROM deps WHERE depends_on = ?)`
		args = append(args, filter.BlockedBy)
	}
	if filter.HasBlockers {
		// Items with unresolved blockers (dependencies that aren't done)
		query += ` AND id IN (SELECT d.item_id FROM deps d JOIN items i ON d.depends_on = i.id WHERE i.status != 'done')`
	}
	if filter.NoBlockers {
		// Items with no blockers (either no deps, or all deps are done)
		query += ` AND id NOT IN (SELECT d.item_id FROM deps d JOIN items i ON d.depends_on = i.id WHERE i.status != 'done')`
	}
	if len(filter.Labels) > 0 {
		// Items must have ALL specified labels (AND semantics)
		// Build placeholder list for IN clause
		placeholders := ""
		for i := range filter.Labels {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
		}
		query += fmt.Sprintf(` AND id IN (
			SELECT il.item_id FROM item_labels il
			JOIN labels l ON il.label_id = l.id
			WHERE l.name IN (%s)
			GROUP BY il.item_id
			HAVING COUNT(DISTINCT l.name) = ?
		)`, placeholders)
		for _, label := range filter.Labels {
			args = append(args, label)
		}
		args = append(args, len(filter.Labels))
	}
	query += ` ORDER BY priority ASC, created_at ASC`

	return db.queryItems(query, args...)
}

// ReadyItems returns items that are open and have no unmet dependencies.
func (db *DB) ReadyItems(project string) ([]model.Item, error) {
	return db.ReadyItemsFiltered(project, nil)
}

// ReadyItemsFiltered returns ready items with optional label filtering.
func (db *DB) ReadyItemsFiltered(project string, labels []string) ([]model.Item, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM items
		WHERE status = 'open'
		  AND id NOT IN (
		    SELECT d.item_id FROM deps d
		    JOIN items i ON d.depends_on = i.id
		    WHERE i.status != 'done'
		  )`, itemSelectColumns)
	args := []any{}

	if project != "" {
		query += ` AND project = ?`
		args = append(args, project)
	}
	if len(labels) > 0 {
		// Items must have ALL specified labels (AND semantics)
		placeholders := ""
		for i := range labels {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
		}
		query += fmt.Sprintf(` AND id IN (
			SELECT il.item_id FROM item_labels il
			JOIN labels l ON il.label_id = l.id
			WHERE l.name IN (%s)
			GROUP BY il.item_id
			HAVING COUNT(DISTINCT l.name) = ?
		)`, placeholders)
		for _, label := range labels {
			args = append(args, label)
		}
		args = append(args, len(labels))
	}
	query += ` ORDER BY priority ASC, created_at ASC`

	return db.queryItems(query, args...)
}

// StaleItems returns in-progress items that haven't been updated since the cutoff.
func (db *DB) StaleItems(project string, cutoff time.Time) ([]model.Item, error) {
	// Compare using unix epoch to avoid timestamp format mismatches between
	// the Go driver's time.Time serialization and SQLite's strftime.
	query := fmt.Sprintf("SELECT %s FROM items WHERE status = 'in_progress' AND updated_at < ?", itemSelectColumns)
	args := []any{cutoff.UTC().Format("2006-01-02 15:04:05")}
	if project != "" {
		query += " AND project = ?"
		args = append(args, project)
	}
	query += " ORDER BY updated_at ASC"
	return db.queryItems(query, args...)
}

// InProgressItemsByAgent returns in-progress items assigned to a specific agent.
func (db *DB) InProgressItemsByAgent(agentID string) ([]model.Item, error) {
	query := fmt.Sprintf("SELECT %s FROM items WHERE status = 'in_progress' AND agent_id = ? ORDER BY updated_at DESC", itemSelectColumns)
	return db.queryItems(query, agentID)
}

// StatusReport contains aggregated project status.
type StatusReport struct {
	Project          string
	Open             int
	InProgress       int
	Blocked          int
	Done             int
	Canceled         int
	Ready            int
	RecentDone       []model.Item // last 3 completed
	InProgItems      []model.Item // current in-progress (all)
	BlockedItems     []model.Item // blocked with reasons
	ReadyItems       []model.Item // ready for work
	StaleItems       []model.Item // in-progress with no updates > 5 min
	AgentID          string
	MyInProgItems    []model.Item // this agent's in-progress tasks
	OtherInProgCount int          // count of other agents' tasks
}

// ProjectStatus returns an aggregated status report for a project.
func (db *DB) ProjectStatus(project string) (*StatusReport, error) {
	return db.ProjectStatusFiltered(project, nil, "")
}

// ProjectStatusFiltered returns an aggregated status report with optional label filtering and agent awareness.
func (db *DB) ProjectStatusFiltered(project string, labels []string, agentID string) (*StatusReport, error) {
	report := &StatusReport{Project: project, AgentID: agentID}

	// Build label subquery for reuse
	labelSubquery := ""
	labelArgs := []any{}
	if len(labels) > 0 {
		placeholders := ""
		for i := range labels {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
		}
		labelSubquery = fmt.Sprintf(` AND id IN (
			SELECT il.item_id FROM item_labels il
			JOIN labels l ON il.label_id = l.id
			WHERE l.name IN (%s)
			GROUP BY il.item_id
			HAVING COUNT(DISTINCT l.name) = ?
		)`, placeholders)
		for _, label := range labels {
			labelArgs = append(labelArgs, label)
		}
		labelArgs = append(labelArgs, len(labels))
	}

	// Count by status
	query := `SELECT status, COUNT(*) FROM items WHERE 1=1`
	args := []any{}
	if project != "" {
		query += ` AND project = ?`
		args = append(args, project)
	}
	if labelSubquery != "" {
		query += labelSubquery
		args = append(args, labelArgs...)
	}
	query += ` GROUP BY status`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to count statuses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan status count: %w", err)
		}
		switch model.Status(status) {
		case model.StatusOpen:
			report.Open = count
		case model.StatusInProgress:
			report.InProgress = count
		case model.StatusBlocked:
			report.Blocked = count
		case model.StatusDone:
			report.Done = count
		case model.StatusCanceled:
			report.Canceled = count
		}
	}

	// Get ready count and items
	readyItems, err := db.ReadyItemsFiltered(project, labels)
	if err != nil {
		return nil, err
	}
	report.Ready = len(readyItems)
	report.ReadyItems = readyItems

	// Get in-progress items
	inProgStatus := model.StatusInProgress
	report.InProgItems, err = db.ListItemsFiltered(ListFilter{Project: project, Status: &inProgStatus, Labels: labels})
	if err != nil {
		return nil, err
	}

	// If agent is active, separate agent's tasks from others
	if agentID != "" {
		myItems := []model.Item{}
		otherCount := 0

		for _, item := range report.InProgItems {
			if item.AgentID != nil && *item.AgentID == agentID {
				myItems = append(myItems, item)
			} else {
				otherCount++
			}
		}

		report.MyInProgItems = myItems
		report.OtherInProgCount = otherCount
	}

	// Get blocked items
	blockedStatus := model.StatusBlocked
	report.BlockedItems, err = db.ListItemsFiltered(ListFilter{Project: project, Status: &blockedStatus, Labels: labels})
	if err != nil {
		return nil, err
	}

	// Get recent done (last 3)
	recentQuery := fmt.Sprintf(`
		SELECT %s
		FROM items WHERE status = 'done'`, itemSelectColumns)
	recentArgs := []any{}
	if project != "" {
		recentQuery += ` AND project = ?`
		recentArgs = append(recentArgs, project)
	}
	if labelSubquery != "" {
		recentQuery += labelSubquery
		recentArgs = append(recentArgs, labelArgs...)
	}
	recentQuery += ` ORDER BY updated_at DESC LIMIT 3`
	report.RecentDone, err = db.queryItems(recentQuery, recentArgs...)
	if err != nil {
		return nil, err
	}

	// Get stale in-progress items (no updates in > 5 minutes)
	staleCutoff := time.Now().Add(-5 * time.Minute)
	report.StaleItems, err = db.StaleItems(project, staleCutoff)
	if err != nil {
		return nil, err
	}

	return report, nil
}

// SummaryStats contains aggregated statistics for the summary command.
type SummaryStats struct {
	Project         string
	Total           int
	Open            int
	InProgress      int
	Blocked         int
	Done            int
	Canceled        int
	Ready           int
	EpicsInProgress int
	Stale           int
}

// GetSummaryStats returns aggregated project health statistics.
func (db *DB) GetSummaryStats(project string) (*SummaryStats, error) {
	stats := &SummaryStats{Project: project}

	// Count by status
	query := `SELECT status, COUNT(*) FROM items WHERE 1=1`
	args := []any{}
	if project != "" {
		query += ` AND project = ?`
		args = append(args, project)
	}
	query += ` GROUP BY status`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to count statuses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan status count: %w", err)
		}
		stats.Total += count
		switch model.Status(status) {
		case model.StatusOpen:
			stats.Open = count
		case model.StatusInProgress:
			stats.InProgress = count
		case model.StatusBlocked:
			stats.Blocked = count
		case model.StatusDone:
			stats.Done = count
		case model.StatusCanceled:
			stats.Canceled = count
		}
	}

	// Get ready count
	readyItems, err := db.ReadyItemsFiltered(project, nil)
	if err != nil {
		return nil, err
	}
	stats.Ready = len(readyItems)

	// Get epics in progress count
	epicQuery := `SELECT COUNT(*) FROM items WHERE type = 'epic' AND status = 'in_progress'`
	epicArgs := []any{}
	if project != "" {
		epicQuery += ` AND project = ?`
		epicArgs = append(args, project)
	}
	var epicCount int
	if err := db.QueryRow(epicQuery, epicArgs...).Scan(&epicCount); err != nil {
		return nil, fmt.Errorf("failed to count epics in progress: %w", err)
	}
	stats.EpicsInProgress = epicCount

	// Get stale count (in-progress with no updates > 5 minutes)
	staleCutoff := time.Now().Add(-5 * time.Minute)
	staleItems, err := db.StaleItems(project, staleCutoff)
	if err != nil {
		return nil, err
	}
	stats.Stale = len(staleItems)

	return stats, nil
}

// ListProjects returns all project names from the projects table.
func (db *DB) ListProjects() ([]string, error) {
	rows, err := db.Query(`SELECT name FROM projects ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("failed to query projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var projects []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, name)
	}
	return projects, rows.Err()
}

// queryItems is a helper to scan item rows.
func (db *DB) queryItems(query string, args ...any) ([]model.Item, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []model.Item
	for rows.Next() {
		var item model.Item
		var parentID sql.NullString
		var agentID sql.NullString
		var agentLastActive sql.NullTime
		var templateID sql.NullString
		var stepIndex sql.NullInt64
		var variables sql.NullString
		var templateHash sql.NullString
		var results sql.NullString
		if err := rows.Scan(
			&item.ID, &item.Project, &item.Type, &item.Title, &item.Description,
			&item.Status, &item.Priority, &parentID,
			&agentID, &agentLastActive,
			&templateID, &stepIndex, &variables, &templateHash, &results,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
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
		items = append(items, item)
	}
	return items, rows.Err()
}
