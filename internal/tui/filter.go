package tui

import (
	"github.com/taxilian/tpg/internal/model"
	"sort"
	"strings"
)

// applyFilters filters items based on current filter state.
func (m *Model) applyFilters() {
	m.filtered = nil
	for _, item := range m.items {
		// Status filter
		if !m.filterStatuses[item.Status] {
			continue
		}
		// Project filter (partial match)
		if m.filterProject != "" && !strings.Contains(strings.ToLower(item.Project), strings.ToLower(m.filterProject)) {
			continue
		}
		// Search filter
		if m.filterSearch != "" {
			search := strings.ToLower(m.filterSearch)
			if !strings.Contains(strings.ToLower(item.Title), search) &&
				!strings.Contains(strings.ToLower(item.ID), search) &&
				!strings.Contains(strings.ToLower(item.Description), search) {
				continue
			}
		}
		// Label filter (partial match, like search)
		if m.filterLabel != "" {
			found := false
			filter := strings.ToLower(m.filterLabel)
			for _, itemLabel := range item.Labels {
				if strings.Contains(strings.ToLower(itemLabel), filter) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		// Ready filter (uses cached readyIDs)
		if m.filterReady && m.readyIDs != nil && !m.readyIDs[item.ID] {
			continue
		}
		m.filtered = append(m.filtered, item)
	}

	// Sort by priority first (lower = higher priority), then by ID for stability
	sort.Slice(m.filtered, func(i, j int) bool {
		if m.filtered[i].Priority != m.filtered[j].Priority {
			return m.filtered[i].Priority < m.filtered[j].Priority
		}
		return m.filtered[i].ID < m.filtered[j].ID
	})

	// Adjust cursor
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m Model) activeFiltersString() string {
	var parts []string

	// Status filter - iterate in consistent order
	var statuses []string
	statusOrder := []model.Status{
		model.StatusOpen,
		model.StatusInProgress,
		model.StatusBlocked,
		model.StatusDone,
		model.StatusCanceled,
	}
	for _, s := range statusOrder {
		if m.filterStatuses[s] {
			statuses = append(statuses, statusText(s))
		}
	}
	if len(statuses) < 5 {
		parts = append(parts, "status:"+strings.Join(statuses, ","))
	}

	if m.filterProject != "" {
		parts = append(parts, "project:"+m.filterProject)
	}

	if m.filterSearch != "" {
		parts = append(parts, "search:\""+m.filterSearch+"\"")
	}

	if m.filterLabel != "" {
		parts = append(parts, "label:\""+m.filterLabel+"\"")
	}

	if m.filterReady {
		parts = append(parts, "ready")
	}

	return strings.Join(parts, " ")
}
