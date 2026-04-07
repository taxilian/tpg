package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/templates"
	"time"
)

// loadItems loads items from the database, filtered by the current project.
func (m Model) loadItems() tea.Cmd {
	return m.loadItemsPreserving("")
}

// loadItemsPreserving loads items and tries to preserve cursor on the given ID.
func (m Model) loadItemsPreserving(preserveID string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.db.ListItemsFiltered(db.ListFilter{Project: m.project})
		if err != nil {
			return itemsMsg{items: items, err: err, preserveID: preserveID}
		}
		// Populate labels for display
		if err := m.db.PopulateItemLabels(items); err != nil {
			return itemsMsg{items: items, err: err, preserveID: preserveID}
		}
		return itemsMsg{items: items, err: nil, preserveID: preserveID}
	}
}

// tickCmd returns a command that sends a tick after the refresh interval.
func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// loadStaleItems loads stale items and returns a command.
func (m Model) loadStaleItems() tea.Cmd {
	return func() tea.Msg {
		cutoff := time.Now().Add(-5 * time.Minute)
		stale, err := m.db.StaleItems(m.project, cutoff)
		if err != nil {
			return staleMsg{err: err}
		}
		return staleMsg{stale: stale}
	}
}

func (m Model) loadReadyIDs() tea.Cmd {
	return func() tea.Msg {
		items, err := m.db.ReadyItems(m.project)
		if err != nil {
			return readyIDsMsg{err: err}
		}
		ids := make(map[string]bool, len(items))
		for _, item := range items {
			ids[item.ID] = true
		}
		return readyIDsMsg{ids: ids}
	}
}

// loadTemplates loads available templates.
func (m Model) loadTemplates() tea.Cmd {
	return func() tea.Msg {
		tmpls, err := templates.ListTemplates()
		return templatesMsg{templates: tmpls, err: err}
	}
}

// loadConfig loads config fields.
func (m Model) loadConfig() tea.Cmd {
	return func() tea.Msg {
		config, err := db.LoadConfig()
		if err != nil {
			return configMsg{err: err}
		}
		fields := db.GetConfigFields(config)
		return configMsg{fields: fields}
	}
}

// loadDetail loads logs and deps for current item.
func (m Model) loadDetail() tea.Cmd {
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 || m.cursor >= len(treeNodes) {
		return nil
	}
	id := treeNodes[m.cursor].Item.ID
	return func() tea.Msg {
		logs, err := m.db.GetLogs(id)
		if err != nil {
			return detailMsg{itemID: id, err: err}
		}
		deps, err := m.db.GetDepStatuses(id)
		if err != nil {
			return detailMsg{itemID: id, err: err}
		}
		blocks, err := m.db.GetBlockedBy(id)
		if err != nil {
			return detailMsg{itemID: id, err: err}
		}
		return detailMsg{itemID: id, logs: logs, deps: deps, blocks: blocks}
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadItems(), m.loadStaleItems(), m.loadReadyIDs(), tickCmd())
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.message = ""
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		configureViewport(&m.detailViewport, m.width, detailViewportHeight(m.height))
		configureViewport(&m.templateDetailViewport, m.width, templateDetailViewportHeight(m.height))
		configureViewport(&m.configViewport, m.width, configViewportHeight(m.height))
		configureViewport(&m.varPickerViewport, m.width, varPickerViewportHeight(m.height))
		switch m.viewMode {
		case ViewDetail:
			(&m).syncDetailViewport()
		case ViewTemplateDetail:
			(&m).syncTemplateDetailViewport()
		case ViewConfig:
			(&m).syncConfigViewport()
		case ViewVariablePicker:
			treeNodes := m.buildTree()
			if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
				item := treeNodes[m.cursor].Item
				if item.TemplateID != "" && len(item.TemplateVars) > 0 {
					(&m).syncVarPickerViewport(item, m.getSortedVarNames(item))
				}
			}
		}
		return m, nil

	case tickMsg:
		// Auto-refresh: skip if user is in input mode, textarea, or certain views
		if m.inputMode != InputNone {
			return m, tickCmd() // Just reschedule, don't refresh
		}
		if m.viewMode == ViewCreateWizard || m.viewMode == ViewConfig {
			return m, tickCmd() // Skip refresh in wizard/config views
		}
		// Get current item ID to preserve selection
		var preserveID string
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			preserveID = treeNodes[m.cursor].Item.ID
		}
		m.skipScrollSync = true
		return m, tea.Batch(m.loadItemsPreserving(preserveID), m.loadStaleItems(), m.loadReadyIDs(), tickCmd())

	case itemsMsg:
		if msg.err != nil {
			m.err = msg.err
			m.skipScrollSync = false
			return m, nil
		}
		// Save current ID before applying filters
		var currentID string
		if msg.preserveID != "" {
			currentID = msg.preserveID
		}
		m.items = msg.items
		m.applyFilters()
		// Restore cursor position if we have a preserved ID
		if currentID != "" {
			treeNodes := m.buildTree()
			for i, node := range treeNodes {
				if node.Item.ID == currentID {
					m.cursor = i
					break
				}
			}
		}
		// Only sync scroll on user-initiated loads, not auto-refresh
		if !m.skipScrollSync {
			m.syncListScroll()
		}
		m.skipScrollSync = false
		m.prevCursor = m.cursor
		if m.viewMode == ViewDetail {
			return m, m.loadDetail()
		}
		return m, nil

	case detailMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.detailLogs = msg.logs
		m.detailDeps = msg.deps
		m.detailBlocks = msg.blocks
		if msg.itemID != m.detailID {
			m.detailViewport.GotoTop()
			m.detailID = msg.itemID
		}
		m.depCursor = 0
		m.depSection = 0
		m.depNavActive = false
		(&m).syncDetailViewport()
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.err = nil
			m.message = msg.message
		}
		// Preserve cursor position after status changes (start/done/delete)
		var preserveID string
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			preserveID = treeNodes[m.cursor].Item.ID
		}
		m.skipScrollSync = true
		return m, tea.Batch(m.loadItemsPreserving(preserveID), m.loadStaleItems())

	case templatesMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.templates = msg.templates
		m.templateCursor = 0
		return m, nil

	case staleMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Rebuild stale items map
		m.staleItems = make(map[string]bool)
		for _, item := range msg.stale {
			m.staleItems[item.ID] = true
		}
		return m, nil

	case readyIDsMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.readyIDs = msg.ids
		if m.filterReady && !m.skipScrollSync {
			m.applyFilters()
			m.syncListScroll()
		}
		return m, nil

	case configMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.configFields = msg.fields
		m.configCursor = 0
		m.configViewport.GotoTop()
		(&m).syncConfigViewport()
		return m, nil

	case editorFinishedMsg:
		return m.handleEditorFinished(msg)
	}

	return m, nil
}
