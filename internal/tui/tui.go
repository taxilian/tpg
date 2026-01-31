// Package tui provides an interactive terminal UI for tpg using Bubble Tea.
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
	"github.com/taxilian/tpg/internal/templates"
)

// ViewMode represents the current view state.
type ViewMode int

const (
	ViewList ViewMode = iota
	ViewDetail
	ViewTemplateList
	ViewTemplateDetail
)

// InputMode represents what kind of text input is active.
type InputMode int

const (
	InputNone       InputMode = iota
	InputBlock                // Entering block reason
	InputLog                  // Entering log message
	InputCancel               // Entering cancel reason
	InputSearch               // Entering search text
	InputProject              // Entering project filter
	InputLabel                // Entering label filter
	InputAddDep               // Entering dependency ID to add
	InputCreate               // Entering new item title
	InputCreateType           // Entering type for new item
)

// Status icons
const (
	iconOpen       = "○"
	iconInProgress = "◐"
	iconDone       = "●"
	iconBlocked    = "⊘"
	iconCanceled   = "✗"
)

// Model is the main Bubble Tea model for the TUI.
type Model struct {
	db       *db.DB
	project  string       // current project (for default filtering)
	items    []model.Item // all items from db
	filtered []model.Item // items after filtering
	cursor   int
	viewMode ViewMode

	// Filter state
	filterProject  string
	filterStatuses map[model.Status]bool // which statuses to show
	filterSearch   string
	filterLabel    string // label filter (partial match, like search)

	// Input state
	inputMode    InputMode
	inputText    string
	inputLabel   string
	inputContext string // For multi-step inputs (e.g., storing title before asking for type)

	// UI state
	width   int
	height  int
	err     error
	message string // temporary status message

	// Detail view state
	detailLogs   []model.Log
	detailDeps   []db.DepStatus // "depends on" (blockers)
	detailBlocks []db.DepStatus // "blocks" (what this item blocks)
	logsVisible  bool
	logScroll    int // scroll offset for logs
	depCursor    int // cursor within deps for navigation
	depSection   int // 0 = "blocked by", 1 = "blocks"
	depNavActive bool

	// Stale tracking
	staleItems map[string]bool // item IDs that are stale (no updates > 5 min)

	// Template browser state
	templates        []*templates.Template
	templateCursor   int
	selectedTemplate *templates.Template
}

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57"))

	statusColors = map[model.Status]lipgloss.Color{
		model.StatusOpen:       lipgloss.Color("252"),
		model.StatusInProgress: lipgloss.Color("214"),
		model.StatusBlocked:    lipgloss.Color("196"),
		model.StatusDone:       lipgloss.Color("42"),
		model.StatusCanceled:   lipgloss.Color("245"),
	}

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57"))

	messageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	detailLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("147"))

	staleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Bold(true)

	// Content area padding
	contentPadding = 2
)

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// listVisibleHeight returns the number of items that can be displayed in the list view.
func (m Model) listVisibleHeight() int {
	// Header: 2 lines (title + blank), Footer: 3 lines (blank + 2 help lines), Padding: 1 line top
	visibleHeight := m.height - 6
	if visibleHeight < 3 {
		visibleHeight = 3
	}
	return visibleHeight
}

// templateVisibleHeight returns the number of templates that can be displayed in the template list view.
func (m Model) templateVisibleHeight() int {
	// Header: 2 lines (title + blank), Footer: 2 lines (blank + help), Padding: varies
	visibleHeight := m.height - 8
	if visibleHeight < 5 {
		visibleHeight = 5
	}
	return visibleHeight
}

// depStatusIcon returns a colored icon for a dependency's status string.
func depStatusIcon(status string) string {
	s := model.Status(status)
	icon := statusIcon(s)
	if c, ok := statusColors[s]; ok {
		return lipgloss.NewStyle().Foreground(c).Render(icon)
	}
	return icon
}

func statusIcon(s model.Status) string {
	switch s {
	case model.StatusOpen:
		return iconOpen
	case model.StatusInProgress:
		return iconInProgress
	case model.StatusDone:
		return iconDone
	case model.StatusBlocked:
		return iconBlocked
	case model.StatusCanceled:
		return iconCanceled
	default:
		return "?"
	}
}

// New creates a new TUI model with the given database connection and project.
func New(database *db.DB, project string) Model {
	// Default: show open, in_progress, blocked
	statuses := map[model.Status]bool{
		model.StatusOpen:       true,
		model.StatusInProgress: true,
		model.StatusBlocked:    true,
		model.StatusDone:       false,
		model.StatusCanceled:   false,
	}
	return Model{
		db:             database,
		project:        project,
		viewMode:       ViewList,
		filterStatuses: statuses,
		staleItems:     make(map[string]bool),
	}
}

// Messages
type itemsMsg struct {
	items []model.Item
	err   error
}

type detailMsg struct {
	logs   []model.Log
	deps   []db.DepStatus // "depends on" (what blocks this)
	blocks []db.DepStatus // "blocks" (what this blocks)
	err    error
}

type actionMsg struct {
	message string
	err     error
}

// templatesMsg carries template list data.
type templatesMsg struct {
	templates []*templates.Template
	err       error
}

// staleMsg carries stale items data.
type staleMsg struct {
	stale []model.Item
	err   error
}

// loadItems loads items from the database, filtered by the current project.
func (m Model) loadItems() tea.Cmd {
	return func() tea.Msg {
		items, err := m.db.ListItemsFiltered(db.ListFilter{Project: m.project})
		if err != nil {
			return itemsMsg{items: items, err: err}
		}
		// Populate labels for display
		if err := m.db.PopulateItemLabels(items); err != nil {
			return itemsMsg{items: items, err: err}
		}
		return itemsMsg{items: items, err: nil}
	}
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

// loadTemplates loads available templates.
func (m Model) loadTemplates() tea.Cmd {
	return func() tea.Msg {
		tmpls, err := templates.ListTemplates()
		return templatesMsg{templates: tmpls, err: err}
	}
}

// loadDetail loads logs and deps for current item.
func (m Model) loadDetail() tea.Cmd {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	id := m.filtered[m.cursor].ID
	return func() tea.Msg {
		logs, err := m.db.GetLogs(id)
		if err != nil {
			return detailMsg{err: err}
		}
		deps, err := m.db.GetDepStatuses(id)
		if err != nil {
			return detailMsg{err: err}
		}
		blocks, err := m.db.GetBlockedBy(id)
		if err != nil {
			return detailMsg{err: err}
		}
		return detailMsg{logs: logs, deps: deps, blocks: blocks}
	}
}

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
		m.filtered = append(m.filtered, item)
	}
	// Adjust cursor
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadItems(), m.loadStaleItems())
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear message on any key
		m.message = ""
		m.err = nil
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case itemsMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.items = msg.items
		m.applyFilters()
		return m, nil

	case detailMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.detailLogs = msg.logs
		m.detailDeps = msg.deps
		m.detailBlocks = msg.blocks
		m.logScroll = 0
		m.depCursor = 0
		m.depSection = 0
		m.depNavActive = false
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.message = msg.message
		}
		return m, tea.Batch(m.loadItems(), m.loadStaleItems())

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
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle input mode first
	if m.inputMode != InputNone {
		return m.handleInputKey(msg)
	}

	switch m.viewMode {
	case ViewList:
		return m.handleListKey(msg)
	case ViewDetail:
		return m.handleDetailKey(msg)
	case ViewTemplateList:
		return m.handleTemplateListKey(msg)
	case ViewTemplateDetail:
		return m.handleTemplateDetailKey(msg)
	}
	return m, nil
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = InputNone
		m.inputText = ""
		m.inputContext = ""
		return m, nil

	case "enter":
		return m.submitInput()

	case "backspace":
		if len(m.inputText) > 0 {
			m.inputText = m.inputText[:len(m.inputText)-1]
			// Live filter for search, project, and label
			switch m.inputMode {
			case InputSearch:
				m.filterSearch = m.inputText
				m.applyFilters()
			case InputProject:
				m.filterProject = m.inputText
				m.applyFilters()
			case InputLabel:
				m.filterLabel = m.inputText
				m.applyFilters()
			}
		}

	default:
		// Add character if printable
		if len(msg.String()) == 1 {
			m.inputText += msg.String()
			// Live filter for search, project, and label
			switch m.inputMode {
			case InputSearch:
				m.filterSearch = m.inputText
				m.applyFilters()
			case InputProject:
				m.filterProject = m.inputText
				m.applyFilters()
			case InputLabel:
				m.filterLabel = m.inputText
				m.applyFilters()
			}
		}
	}
	return m, nil
}

func (m Model) submitInput() (tea.Model, tea.Cmd) {
	text := m.inputText
	mode := m.inputMode
	m.inputMode = InputNone
	m.inputText = ""

	// Handle inputs that don't require an existing item
	switch mode {
	case InputSearch:
		m.filterSearch = text
		m.applyFilters()
		return m, nil

	case InputProject:
		m.filterProject = text
		m.applyFilters()
		return m, nil

	case InputLabel:
		m.filterLabel = text
		m.applyFilters()
		return m, nil

	case InputCreate:
		if text == "" {
			return m, nil
		}
		// Store the title and ask for type
		m.inputContext = text
		m.inputMode = InputCreateType
		m.inputLabel = "Type (default: task): "
		m.inputText = ""
		return m, nil

	case InputCreateType:
		// Use the selected item's project if available
		var project string
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			project = m.filtered[m.cursor].Project
		}
		// Default to "task" if no type specified
		itemType := model.ItemType(text)
		if text == "" {
			itemType = model.ItemTypeTask
		}
		title := m.inputContext
		m.inputContext = ""
		return m, func() tea.Msg {
			itemID, err := m.db.GenerateItemID(itemType)
			if err != nil {
				return actionMsg{err: err}
			}
			now := time.Now()
			newItem := &model.Item{
				ID:        itemID,
				Project:   project,
				Type:      itemType,
				Title:     title,
				Status:    model.StatusOpen,
				Priority:  2,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := m.db.CreateItem(newItem); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{message: fmt.Sprintf("Created %s (%s)", newItem.ID, newItem.Type)}
		}
	}

	// Remaining inputs require an existing item
	if len(m.filtered) == 0 {
		return m, nil
	}
	item := m.filtered[m.cursor]

	switch mode {
	case InputBlock:
		if text == "" {
			return m, nil
		}
		return m, func() tea.Msg {
			if err := m.db.UpdateStatus(item.ID, model.StatusBlocked, db.AgentContext{}); err != nil {
				return actionMsg{err: err}
			}
			if err := m.db.AddLog(item.ID, "Blocked: "+text); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{message: fmt.Sprintf("Blocked %s", item.ID)}
		}

	case InputLog:
		if text == "" {
			return m, nil
		}
		return m, func() tea.Msg {
			if err := m.db.AddLog(item.ID, text); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{message: fmt.Sprintf("Logged to %s", item.ID)}
		}

	case InputCancel:
		return m, func() tea.Msg {
			if err := m.db.UpdateStatus(item.ID, model.StatusCanceled, db.AgentContext{}); err != nil {
				return actionMsg{err: err}
			}
			if text != "" {
				if err := m.db.AddLog(item.ID, "Canceled: "+text); err != nil {
					return actionMsg{err: err}
				}
			}
			return actionMsg{message: fmt.Sprintf("Canceled %s", item.ID)}
		}

	case InputAddDep:
		if text == "" {
			return m, nil
		}
		return m, func() tea.Msg {
			// text blocks current item
			if err := m.db.AddDep(item.ID, text); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{message: fmt.Sprintf("%s now blocks %s", text, item.ID)}
		}
	}

	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}

	case "g", "home":
		m.cursor = 0

	case "G", "end":
		m.cursor = max(0, len(m.filtered)-1)

	case "pgup", "ctrl+b":
		// Page up - move cursor up by visible height
		pageSize := m.listVisibleHeight()
		m.cursor -= pageSize
		if m.cursor < 0 {
			m.cursor = 0
		}

	case "pgdown", "ctrl+f":
		// Page down - move cursor down by visible height
		pageSize := m.listVisibleHeight()
		m.cursor += pageSize
		if m.cursor >= len(m.filtered) {
			m.cursor = max(0, len(m.filtered)-1)
		}

	case "ctrl+u":
		// Half page up
		pageSize := m.listVisibleHeight() / 2
		if pageSize < 1 {
			pageSize = 1
		}
		m.cursor -= pageSize
		if m.cursor < 0 {
			m.cursor = 0
		}

	case "ctrl+d":
		// Half page down
		pageSize := m.listVisibleHeight() / 2
		if pageSize < 1 {
			pageSize = 1
		}
		m.cursor += pageSize
		if m.cursor >= len(m.filtered) {
			m.cursor = max(0, len(m.filtered)-1)
		}

	case "enter", "l":
		if len(m.filtered) > 0 {
			m.viewMode = ViewDetail
			return m, m.loadDetail()
		}

	// Actions
	case "s":
		return m.doStart()
	case "d":
		return m.doDone()
	case "b":
		return m.startInput(InputBlock, "Block reason: ")
	case "L":
		return m.startInput(InputLog, "Log message: ")
	case "c":
		return m.startInput(InputCancel, "Cancel reason (optional): ")
	case "D":
		return m.doDelete()

	// Filtering
	case "/":
		return m.startInput(InputSearch, "Search: ")
	case "p":
		return m.startInput(InputProject, "Project: ")
	case "t":
		return m.startInput(InputLabel, "Label: ")
	case "1":
		m.filterStatuses[model.StatusOpen] = !m.filterStatuses[model.StatusOpen]
		m.applyFilters()
	case "2":
		m.filterStatuses[model.StatusInProgress] = !m.filterStatuses[model.StatusInProgress]
		m.applyFilters()
	case "3":
		m.filterStatuses[model.StatusBlocked] = !m.filterStatuses[model.StatusBlocked]
		m.applyFilters()
	case "4":
		m.filterStatuses[model.StatusDone] = !m.filterStatuses[model.StatusDone]
		m.applyFilters()
	case "5":
		m.filterStatuses[model.StatusCanceled] = !m.filterStatuses[model.StatusCanceled]
		m.applyFilters()
	case "0":
		// Show all
		for s := range m.filterStatuses {
			m.filterStatuses[s] = true
		}
		m.applyFilters()

	case "esc":
		// If filters are set, clear them; otherwise quit
		if m.filterSearch != "" || m.filterProject != "" || m.filterLabel != "" {
			m.filterSearch = ""
			m.filterProject = ""
			m.filterLabel = ""
			m.applyFilters()
		} else {
			return m, tea.Quit
		}

	case "r":
		return m, m.loadItems()

	// Dependencies
	case "a":
		return m.startInput(InputAddDep, "Add blocker ID: ")

	// Create
	case "n":
		label := "New item: "
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			if proj := m.filtered[m.cursor].Project; proj != "" {
				label = fmt.Sprintf("New item [%s]: ", proj)
			}
		}
		return m.startInput(InputCreate, label)

	// Templates
	case "T":
		m.viewMode = ViewTemplateList
		return m, m.loadTemplates()
	}

	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "h", "backspace":
		if m.depNavActive {
			m.depNavActive = false
			return m, nil
		}
		m.viewMode = ViewList
		m.logsVisible = false

	// Log toggle and scroll
	case "v":
		m.logsVisible = !m.logsVisible
		m.logScroll = 0
	case "j", "down":
		if m.depNavActive {
			m.depCursor++
			section := m.currentDepSection()
			if m.depCursor >= len(section) {
				m.depCursor = len(section) - 1
			}
			if m.depCursor < 0 {
				m.depCursor = 0
			}
		} else if m.logsVisible {
			// maxVisible matches the display constant in detailView
			maxVisible := 20
			maxScroll := max(0, len(m.detailLogs)-maxVisible)
			if m.logScroll < maxScroll {
				m.logScroll++
			}
		}
	case "k", "up":
		if m.depNavActive {
			if m.depCursor > 0 {
				m.depCursor--
			}
		} else if m.logsVisible && m.logScroll > 0 {
			m.logScroll--
		}

	// Dependency navigation
	case "tab":
		if len(m.detailDeps) > 0 || len(m.detailBlocks) > 0 {
			m.depNavActive = true
			m.depSection = (m.depSection + 1) % 2
			m.depCursor = 0
			// If switching to empty section, switch back
			if len(m.currentDepSection()) == 0 {
				m.depSection = (m.depSection + 1) % 2
			}
		}
	case "enter":
		if m.depNavActive {
			section := m.currentDepSection()
			if m.depCursor < len(section) {
				targetID := section[m.depCursor].ID
				// Find the target in filtered items
				for i, item := range m.filtered {
					if item.ID == targetID {
						m.cursor = i
						m.depNavActive = false
						return m, m.loadDetail()
					}
				}
				m.message = fmt.Sprintf("Item %s not in current filter", targetID)
			}
		}

	// Actions work in detail view too
	case "s":
		return m.doStart()
	case "d":
		return m.doDone()
	case "b":
		return m.startInput(InputBlock, "Block reason: ")
	case "L":
		return m.startInput(InputLog, "Log message: ")
	case "c":
		return m.startInput(InputCancel, "Cancel reason (optional): ")
	case "a":
		return m.startInput(InputAddDep, "Add blocker ID: ")

	case "r":
		return m, m.loadDetail()
	}

	return m, nil
}

// currentDepSection returns the deps for the active section.
func (m Model) currentDepSection() []db.DepStatus {
	if m.depSection == 0 {
		return m.detailDeps
	}
	return m.detailBlocks
}

func (m Model) handleTemplateListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.templateCursor > 0 {
			m.templateCursor--
		}

	case "down", "j":
		if m.templateCursor < len(m.templates)-1 {
			m.templateCursor++
		}

	case "g", "home":
		m.templateCursor = 0

	case "G", "end":
		m.templateCursor = max(0, len(m.templates)-1)

	case "pgup", "ctrl+b":
		pageSize := m.templateVisibleHeight()
		m.templateCursor -= pageSize
		if m.templateCursor < 0 {
			m.templateCursor = 0
		}

	case "pgdown", "ctrl+f":
		pageSize := m.templateVisibleHeight()
		m.templateCursor += pageSize
		if m.templateCursor >= len(m.templates) {
			m.templateCursor = max(0, len(m.templates)-1)
		}

	case "ctrl+u":
		pageSize := m.templateVisibleHeight() / 2
		if pageSize < 1 {
			pageSize = 1
		}
		m.templateCursor -= pageSize
		if m.templateCursor < 0 {
			m.templateCursor = 0
		}

	case "ctrl+d":
		pageSize := m.templateVisibleHeight() / 2
		if pageSize < 1 {
			pageSize = 1
		}
		m.templateCursor += pageSize
		if m.templateCursor >= len(m.templates) {
			m.templateCursor = max(0, len(m.templates)-1)
		}

	case "enter", "l":
		if len(m.templates) > 0 && m.templateCursor < len(m.templates) {
			m.selectedTemplate = m.templates[m.templateCursor]
			m.viewMode = ViewTemplateDetail
		}

	case "esc", "h", "backspace":
		m.viewMode = ViewList
		m.templateCursor = 0

	case "r":
		return m, m.loadTemplates()
	}

	return m, nil
}

func (m Model) handleTemplateDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "h", "backspace":
		m.viewMode = ViewTemplateList
		m.selectedTemplate = nil
	}

	return m, nil
}

func (m Model) startInput(mode InputMode, label string) (Model, tea.Cmd) {
	m.inputMode = mode
	m.inputLabel = label
	m.inputText = ""
	return m, nil
}

func (m Model) doStart() (Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	item := m.filtered[m.cursor]
	if item.Status != model.StatusOpen && item.Status != model.StatusBlocked {
		m.message = "Can only start open or blocked items"
		return m, nil
	}
	return m, func() tea.Msg {
		if err := m.db.UpdateStatus(item.ID, model.StatusInProgress, db.AgentContext{}); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{message: fmt.Sprintf("Started %s", item.ID)}
	}
}

func (m Model) doDone() (Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	item := m.filtered[m.cursor]
	if item.Status != model.StatusInProgress {
		m.message = "Can only complete in_progress items"
		return m, nil
	}
	return m, func() tea.Msg {
		if err := m.db.UpdateStatus(item.ID, model.StatusDone, db.AgentContext{}); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{message: fmt.Sprintf("Completed %s", item.ID)}
	}
}

func (m Model) doDelete() (Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	item := m.filtered[m.cursor]
	return m, func() tea.Msg {
		if err := m.db.DeleteItem(item.ID); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{message: fmt.Sprintf("Deleted %s", item.ID)}
	}
}

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	switch m.viewMode {
	case ViewList:
		b.WriteString(m.listView())
	case ViewDetail:
		b.WriteString(m.detailView())
	case ViewTemplateList:
		b.WriteString(m.templateListView())
	case ViewTemplateDetail:
		b.WriteString(m.templateDetailView())
	}

	// Input line
	if m.inputMode != InputNone {
		b.WriteString("\n")
		b.WriteString(inputStyle.Render(m.inputLabel + m.inputText + "█"))
	}

	// Status message
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	} else if m.message != "" {
		b.WriteString("\n")
		b.WriteString(messageStyle.Render(m.message))
	}

	// Apply padding to entire content
	padStyle := lipgloss.NewStyle().
		PaddingLeft(contentPadding).
		PaddingRight(contentPadding).
		PaddingTop(1)

	return padStyle.Render(b.String())
}

func (m Model) listView() string {
	var b strings.Builder

	// Header
	title := "tpg"
	b.WriteString(titleStyle.Render(title))
	b.WriteString(fmt.Sprintf("  %d/%d items", len(m.filtered), len(m.items)))

	// Active filters
	filters := m.activeFiltersString()
	if filters != "" {
		b.WriteString("  ")
		b.WriteString(filterStyle.Render(filters))
	}
	b.WriteString("\n\n")

	// Items
	if len(m.filtered) == 0 {
		b.WriteString("No items match filters\n")
	} else {
		// Calculate visible height accounting for header, footer, and padding
		// Header: 3 lines (title + filters + blank), Footer: 3 lines (blank + 2 help lines)
		visibleHeight := m.height - 6
		if visibleHeight < 3 {
			visibleHeight = 3 // Minimum visible items
		}
		if visibleHeight > len(m.filtered) {
			visibleHeight = len(m.filtered)
		}

		// Calculate start position to keep cursor visible
		// When cursor is near the bottom, scroll up
		// When cursor is near the top, scroll down
		start := 0
		if m.cursor >= visibleHeight {
			start = m.cursor - visibleHeight + 1
		}
		// Ensure start doesn't go beyond valid range
		if start < 0 {
			start = 0
		}
		if start > len(m.filtered)-visibleHeight && len(m.filtered) >= visibleHeight {
			start = len(m.filtered) - visibleHeight
		}

		end := min(start+visibleHeight, len(m.filtered))

		rowWidth := m.width - (contentPadding * 2)
		if rowWidth < 60 {
			rowWidth = 80
		}

		for i := start; i < end; i++ {
			item := m.filtered[i]
			selected := i == m.cursor

			if selected {
				// For selected row: plain text, then apply highlight to full width
				line := m.formatItemLinePlain(item, rowWidth)
				b.WriteString(selectedRowStyle.Width(rowWidth).Render(line))
			} else {
				// For non-selected: use styled version
				line := m.formatItemLineStyled(item, rowWidth)
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k:nav  ^u/^d:½pg  pgup/dn:pg  g/G:top/end  enter:detail  s:start d:done n:new"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("/:search p:project t:label 1-5:status 0:all  b:block L:log c:cancel  r:refresh q:quit"))

	return b.String()
}

// formatItemLinePlain returns a plain text line without any ANSI styling.
// Used for selected rows where we apply a single highlight style.
func (m Model) formatItemLinePlain(item model.Item, width int) string {
	icon := statusIcon(item.Status)

	// Stale indicator
	stale := ""
	staleWidth := 0
	if m.staleItems[item.ID] {
		stale = "⚠"
		staleWidth = 2 // ⚠ + space
	}

	// Agent indicator
	agent := ""
	agentWidth := 0
	if item.AgentID != nil && *item.AgentID != "" {
		agent = "◈"
		agentWidth = 2
	}

	// Type indicator (abbreviated)
	itemType := string(item.Type)
	if len(itemType) > 4 {
		itemType = itemType[:4]
	}
	typeWidth := 5 // 4 chars + space

	// Format: icon type id title [label1] [label2] [project]
	project := ""
	projectWidth := 0
	if item.Project != "" {
		project = "[" + item.Project + "]"
		projectWidth = len(project) + 1
	}

	// Build labels string
	labels := ""
	labelsWidth := 0
	for _, lbl := range item.Labels {
		labels += " [" + lbl + "]"
		labelsWidth += len(lbl) + 3 // brackets + space
	}

	// Calculate available space for title
	fixedWidth := 16 + labelsWidth + projectWidth + staleWidth + agentWidth + typeWidth
	titleWidth := width - fixedWidth
	if titleWidth < 20 {
		titleWidth = 40
	}

	title := item.Title
	if len(title) > titleWidth {
		title = title[:titleWidth-3] + "..."
	}

	if agent != "" {
		return fmt.Sprintf("%s%s %-4s %s %s %-*s%s %s", stale, icon, itemType, item.ID, agent, titleWidth, title, labels, project)
	}
	return fmt.Sprintf("%s%s %-4s %s  %-*s%s %s", stale, icon, itemType, item.ID, titleWidth, title, labels, project)
}

// formatItemLineStyled returns a styled line with colors for non-selected rows.
func (m Model) formatItemLineStyled(item model.Item, width int) string {
	icon := statusIcon(item.Status)
	color := statusColors[item.Status]
	iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)

	// Stale indicator
	stale := ""
	staleWidth := 0
	if m.staleItems[item.ID] {
		stale = staleStyle.Render("⚠ ")
		staleWidth = 2 // ⚠ + space
	}

	// Agent indicator
	agent := ""
	agentWidth := 0
	if item.AgentID != nil && *item.AgentID != "" {
		agent = dimStyle.Render("◈")
		agentWidth = 2
	}

	id := dimStyle.Render(item.ID)

	// Type indicator (abbreviated, dimmed)
	itemType := string(item.Type)
	if len(itemType) > 4 {
		itemType = itemType[:4]
	}
	typeStyled := dimStyle.Render(fmt.Sprintf("%-4s", itemType))
	typeWidth := 5 // 4 chars + space

	// Format: icon type id title [label1] [label2] [project]
	project := ""
	projectWidth := 0
	if item.Project != "" {
		project = dimStyle.Render("[" + item.Project + "]")
		projectWidth = len(item.Project) + 3 // brackets + space
	}

	// Build labels string
	labels := ""
	labelsWidth := 0
	for _, lbl := range item.Labels {
		labels += " " + labelStyle.Render("["+lbl+"]")
		labelsWidth += len(lbl) + 3 // brackets + space
	}

	// Calculate available space for title
	fixedWidth := 16 + labelsWidth + projectWidth + staleWidth + agentWidth + typeWidth
	titleWidth := width - fixedWidth
	if titleWidth < 20 {
		titleWidth = 40
	}

	title := item.Title
	if len(title) > titleWidth {
		title = title[:titleWidth-3] + "..."
	}

	if agent != "" {
		return fmt.Sprintf("%s%s %s %s %s %-*s%s %s", stale, iconStyled, typeStyled, id, agent, titleWidth, title, labels, project)
	}
	return fmt.Sprintf("%s%s %s %s  %-*s%s %s", stale, iconStyled, typeStyled, id, titleWidth, title, labels, project)
}

func (m Model) activeFiltersString() string {
	var parts []string

	// Status filter
	var statuses []string
	for s, active := range m.filterStatuses {
		if active {
			statuses = append(statuses, string(s)[:1]) // First char: o/i/b/d/c
		}
	}
	if len(statuses) < 5 {
		parts = append(parts, "status:"+strings.Join(statuses, ""))
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

	return strings.Join(parts, " ")
}

func (m Model) detailView() string {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return "No item selected"
	}

	item := m.filtered[m.cursor]
	var b strings.Builder

	// Title with status icon
	icon := statusIcon(item.Status)
	color := statusColors[item.Status]
	iconStyled := lipgloss.NewStyle().Foreground(color).Render(icon)

	// Add stale indicator to title if stale
	title := item.Title
	if m.staleItems[item.ID] {
		title = staleStyle.Render("⚠ ") + title
	}

	b.WriteString(iconStyled + " " + titleStyle.Render(title) + "\n\n")

	b.WriteString(detailLabelStyle.Render("ID:       ") + item.ID + "\n")
	b.WriteString(detailLabelStyle.Render("Type:     ") + string(item.Type) + "\n")
	b.WriteString(detailLabelStyle.Render("Project:  ") + item.Project + "\n")

	statusStyled := lipgloss.NewStyle().Foreground(color).Render(string(item.Status))
	b.WriteString(detailLabelStyle.Render("Status:   ") + statusStyled)

	// Add stale badge in detail view
	if m.staleItems[item.ID] {
		b.WriteString(" " + staleStyle.Render("[STALE]"))
	}
	b.WriteString("\n")

	b.WriteString(detailLabelStyle.Render("Priority: ") + fmt.Sprintf("%d", item.Priority) + "\n")

	if item.ParentID != nil {
		b.WriteString(detailLabelStyle.Render("Parent:   ") + *item.ParentID + "\n")
	}

	// Agent assignment
	if item.AgentID != nil && *item.AgentID != "" {
		b.WriteString(detailLabelStyle.Render("Agent:    ") + dimStyle.Render(*item.AgentID) + "\n")
	}

	// Labels
	if len(item.Labels) > 0 {
		labelsStr := ""
		for i, lbl := range item.Labels {
			if i > 0 {
				labelsStr += " "
			}
			labelsStr += labelStyle.Render("[" + lbl + "]")
		}
		b.WriteString(detailLabelStyle.Render("Labels:   ") + labelsStr + "\n")
	}

	// Dependencies — "Blocked by" (what this depends on)
	if len(m.detailDeps) > 0 {
		header := "Blocked by:"
		if m.depNavActive && m.depSection == 0 {
			header = "▸ Blocked by:"
		}
		b.WriteString("\n" + detailLabelStyle.Render(header) + "\n")
		for i, dep := range m.detailDeps {
			icon := depStatusIcon(dep.Status)
			selected := m.depNavActive && m.depSection == 0 && i == m.depCursor
			line := fmt.Sprintf("  %s %s %s", icon, dep.ID, dep.Title)
			if selected {
				b.WriteString(selectedRowStyle.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}
	}

	// Dependencies — "Blocks" (what this item blocks)
	if len(m.detailBlocks) > 0 {
		header := "Blocks:"
		if m.depNavActive && m.depSection == 1 {
			header = "▸ Blocks:"
		}
		b.WriteString("\n" + detailLabelStyle.Render(header) + "\n")
		for i, dep := range m.detailBlocks {
			icon := depStatusIcon(dep.Status)
			selected := m.depNavActive && m.depSection == 1 && i == m.depCursor
			line := fmt.Sprintf("  %s %s %s", icon, dep.ID, dep.Title)
			if selected {
				b.WriteString(selectedRowStyle.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}
	}

	// Description
	if item.Description != "" {
		b.WriteString("\n" + detailLabelStyle.Render("Description:") + "\n")
		b.WriteString(item.Description + "\n")
	}

	// Logs (toggle with v, scrollable)
	logCount := len(m.detailLogs)
	if logCount > 0 {
		if m.logsVisible {
			maxVisible := 20
			b.WriteString("\n" + detailLabelStyle.Render(fmt.Sprintf("Logs (%d):", logCount)) + " " + dimStyle.Render("v:hide j/k:scroll") + "\n")
			end := min(m.logScroll+maxVisible, logCount)
			for i := m.logScroll; i < end; i++ {
				log := m.detailLogs[i]
				ts := dimStyle.Render(log.CreatedAt.Format("2006-01-02 15:04"))
				b.WriteString("  " + ts + " " + log.Message + "\n")
			}
			if end < logCount {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  ... %d more (scroll with j/k)", logCount-end)) + "\n")
			}
		} else {
			b.WriteString("\n" + dimStyle.Render(fmt.Sprintf("Logs: %d entries (v to show)", logCount)) + "\n")
		}
	}

	b.WriteString("\n")
	help := "esc:back  s:start d:done b:block L:log c:cancel a:add-dep  v:logs  q:quit"
	if len(m.detailDeps) > 0 || len(m.detailBlocks) > 0 {
		help += "  tab:deps enter:jump"
	}
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

func (m Model) templateListView() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("Templates"))
	b.WriteString(fmt.Sprintf("  %d templates\n\n", len(m.templates)))

	// Templates
	if len(m.templates) == 0 {
		b.WriteString("No templates found\n")
		b.WriteString(dimStyle.Render("Templates are loaded from .tpg/templates/\n"))
	} else {
		visibleHeight := m.height - 8
		if visibleHeight < 5 {
			visibleHeight = 15
		}
		start := 0
		if m.templateCursor >= visibleHeight {
			start = m.templateCursor - visibleHeight + 1
		}
		end := min(start+visibleHeight, len(m.templates))

		rowWidth := m.width - (contentPadding * 2)
		if rowWidth < 60 {
			rowWidth = 80
		}

		for i := start; i < end; i++ {
			tmpl := m.templates[i]
			selected := i == m.templateCursor

			// Format: id - title (description preview)
			line := fmt.Sprintf("%s  %s", tmpl.ID, tmpl.Title)
			if len(tmpl.Description) > 0 {
				desc := tmpl.Description
				if len(desc) > 50 {
					desc = desc[:47] + "..."
				}
				line += " - " + desc
			}

			// Truncate to fit width
			if len(line) > rowWidth {
				line = line[:rowWidth-3] + "..."
			}

			if selected {
				b.WriteString(selectedRowStyle.Width(rowWidth).Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k:nav  enter:detail  r:refresh  esc:back  q:quit"))

	return b.String()
}

func (m Model) templateDetailView() string {
	if m.selectedTemplate == nil {
		return "No template selected"
	}

	tmpl := m.selectedTemplate
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render(tmpl.Title) + "\n\n")

	// Basic info
	b.WriteString(detailLabelStyle.Render("ID:          ") + tmpl.ID + "\n")
	if tmpl.Source != "" {
		b.WriteString(detailLabelStyle.Render("Source:      ") + tmpl.Source + "\n")
	}
	b.WriteString("\n")

	// Description
	if tmpl.Description != "" {
		b.WriteString(detailLabelStyle.Render("Description:") + "\n")
		b.WriteString(tmpl.Description + "\n\n")
	}

	// Variables
	if len(tmpl.Variables) > 0 {
		b.WriteString(detailLabelStyle.Render("Variables:") + "\n")
		for name, v := range tmpl.Variables {
			optional := ""
			if v.Optional {
				optional = dimStyle.Render(" (optional)")
			}
			b.WriteString("  " + labelStyle.Render(name) + optional + "\n")
			if v.Description != "" {
				b.WriteString("    " + dimStyle.Render(v.Description) + "\n")
			}
			if v.Default != "" {
				b.WriteString("    " + dimStyle.Render("Default: "+v.Default) + "\n")
			}
		}
		b.WriteString("\n")
	}

	// Steps
	if len(tmpl.Steps) > 0 {
		b.WriteString(detailLabelStyle.Render("Steps:") + "\n")
		for i, step := range tmpl.Steps {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, step.Title))
			if len(step.Depends) > 0 {
				b.WriteString("     " + dimStyle.Render("Depends: "+strings.Join(step.Depends, ", ")) + "\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("esc:back  q:quit"))

	return b.String()
}

// Run starts the TUI with the given project filter.
func Run(database *db.DB, project string) error {
	m := New(database, project)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
