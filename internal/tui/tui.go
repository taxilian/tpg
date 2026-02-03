// Package tui provides an interactive terminal UI for tpg using Bubble Tea.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
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
	ViewGraph
	ViewTemplateList
	ViewTemplateDetail
	ViewConfig
	ViewCreateWizard
)

// InputMode represents what kind of text input is active.
type InputMode int

// DescViewMode represents which description view is active for templated items.
type DescViewMode int

const (
	DescViewRendered DescViewMode = iota // Show freshly rendered template (default)
	DescViewStored                       // Show stored description
	DescViewVars                         // Show raw variables (edit mode)
)

const (
	InputNone          InputMode = iota
	InputBlock                   // Entering block reason
	InputLog                     // Entering log message
	InputCancel                  // Entering cancel reason
	InputSearch                  // Entering search text
	InputProject                 // Entering project filter
	InputLabel                   // Entering label filter
	InputAddDep                  // Entering dependency ID to add
	InputCreate                  // Entering new item title
	InputCreateType              // Entering type for new item
	InputBatchStatus             // Entering status for batch change
	InputBatchPriority           // Entering priority for batch change
	InputTextarea                // Multi-line textarea editing
	InputStatusMenu              // Status change confirmation menu
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

	// Selection mode state
	selectMode    bool
	selectedItems map[string]bool // item ID -> selected

	// Graph view state
	graphNodes     []graphNode
	graphCursor    int
	graphCurrentID string // ID of the center task in graph view

	// Template variable expansion state (for detail view)
	varExpanded map[string]bool
	varCursor   int // which variable is selected for editing (-1 = none)

	// Template description view state
	descViewMode DescViewMode
	storedDesc   string // Cached stored description
	renderedDesc string // Cached rendered description

	// Textarea editing state
	textarea         textarea.Model
	textareaTarget   string // what we're editing: "description" or "var:<name>"
	textareaOriginal string // original value for cancel

	// Status menu state
	statusMenuCursor int // 0=start, 1=done, 2=block, 3=cancel

	// Config view state
	configFields  []db.ConfigField
	configCursor  int
	configEditing bool

	// Tree view state
	treeExpanded map[string]bool // item ID -> expanded state
	treeCursor   int             // cursor position in tree view

	// Create wizard state
	createWizardStep  int
	createWizardState CreateWizardState
}

// CreateWizardState holds all data during item creation
type CreateWizardState struct {
	// Step 1: Type
	SelectedType model.ItemType

	// Step 2: Basic Config
	Priority         int
	Project          string
	UseCustomProject bool

	// Step 3: Relationships
	ParentID     string
	Dependencies []string
	Blockers     []string

	// Step 4: Worktree (epics only)
	UseWorktree    bool
	WorktreeBranch string
	WorktreeBase   string

	// Step 5: Method
	UseTemplate      bool
	SelectedTemplate *templates.Template

	// Step 6: Title/Variables/Labels
	Title        string
	Labels       []string
	TemplateVars map[string]string

	// Step 7: Description
	Description string
}

// graphNode represents a task in the dependency graph view.
type graphNode struct {
	ID       string
	Title    string
	Status   string
	Column   int // 0 = blockers, 1 = current, 2 = blocked
	Position int // vertical position within column
}

// treeNode represents an item in the hierarchical tree view.
type treeNode struct {
	Item        model.Item
	Level       int  // Indentation level (0 = root, 1 = child, etc.)
	HasChildren bool // Whether this item has children
	IsLastChild bool // Whether this is the last child (for branch drawing)
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

	selectModeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
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

// statusText returns a short readable text for a status.
func statusText(s model.Status) string {
	switch s {
	case model.StatusOpen:
		return "open"
	case model.StatusInProgress:
		return "active"
	case model.StatusDone:
		return "done"
	case model.StatusBlocked:
		return "block"
	case model.StatusCanceled:
		return "cancel"
	default:
		return "?"
	}
}

// formatStatus returns "icon text" format for a status (e.g., "○ open").
func formatStatus(s model.Status) string {
	return statusIcon(s) + " " + statusText(s)
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

	// Initialize textarea for multi-line editing
	ta := textarea.New()
	ta.Placeholder = "Enter text..."
	ta.ShowLineNumbers = false
	ta.SetWidth(80)
	ta.SetHeight(10)

	return Model{
		db:               database,
		project:          project,
		viewMode:         ViewList,
		filterStatuses:   statuses,
		staleItems:       make(map[string]bool),
		selectedItems:    make(map[string]bool),
		varExpanded:      make(map[string]bool),
		textarea:         ta,
		treeExpanded:     make(map[string]bool),
		createWizardStep: 0, // 0 = not in wizard
		createWizardState: CreateWizardState{
			Priority:     2, // Default medium priority
			Project:      project,
			TemplateVars: make(map[string]string),
		},
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

// configMsg carries config data.
type configMsg struct {
	fields []db.ConfigField
	err    error
}

// editorFinishedMsg is sent when the external editor closes.
type editorFinishedMsg struct {
	itemID   string
	target   string // "description" or "var:<name>"
	tmpPath  string
	origTime time.Time
	err      error
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

	case configMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.configFields = msg.fields
		m.configCursor = 0
		return m, nil

	case editorFinishedMsg:
		return m.handleEditorFinished(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle textarea mode separately (it needs special key handling)
	if m.inputMode == InputTextarea {
		return m.handleTextareaKey(msg)
	}

	// Handle status menu mode
	if m.inputMode == InputStatusMenu {
		return m.handleStatusMenuKey(msg)
	}

	// Handle other input modes
	if m.inputMode != InputNone {
		return m.handleInputKey(msg)
	}

	switch m.viewMode {
	case ViewList:
		return m.handleListKey(msg)
	case ViewDetail:
		return m.handleDetailKey(msg)
	case ViewGraph:
		return m.handleGraphKey(msg)
	case ViewTemplateList:
		return m.handleTemplateListKey(msg)
	case ViewTemplateDetail:
		return m.handleTemplateDetailKey(msg)
	case ViewConfig:
		return m.handleConfigKey(msg)
	case ViewCreateWizard:
		return m.handleCreateWizardKey(msg)
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

// handleStatusMenuKey handles key events when the status menu is active.
func (m Model) handleStatusMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel menu, return to previous view
		m.inputMode = InputNone
		return m, nil

	case "up", "k":
		if m.statusMenuCursor > 0 {
			m.statusMenuCursor--
		}
		return m, nil

	case "down", "j":
		if m.statusMenuCursor < 3 {
			m.statusMenuCursor++
		}
		return m, nil

	case "enter":
		// Execute the selected action
		m.inputMode = InputNone
		switch m.statusMenuCursor {
		case 0: // Start
			return m.doStart()
		case 1: // Done
			return m.doDone()
		case 2: // Block
			return m.startInput(InputBlock, "Block reason: ")
		case 3: // Cancel
			return m.startInput(InputCancel, "Cancel reason (optional): ")
		}
		return m, nil

	case "s":
		// Quick key for Start
		m.inputMode = InputNone
		return m.doStart()

	case "d":
		// Quick key for Done
		m.inputMode = InputNone
		return m.doDone()

	case "b":
		// Quick key for Block (needs reason)
		m.inputMode = InputNone
		return m.startInput(InputBlock, "Block reason: ")

	case "c":
		// Quick key for Cancel (needs reason)
		m.inputMode = InputNone
		return m.startInput(InputCancel, "Cancel reason (optional): ")
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
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			project = treeNodes[m.cursor].Item.Project
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
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 {
		return m, nil
	}
	item := treeNodes[m.cursor].Item

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

	case InputBatchStatus:
		result, cmd := m.doBatchStatus(text)
		// Exit select mode after batch operation
		result.selectMode = false
		result.selectedItems = make(map[string]bool)
		return result, cmd

	case InputBatchPriority:
		result, cmd := m.doBatchPriority(text)
		// Exit select mode after batch operation
		result.selectMode = false
		result.selectedItems = make(map[string]bool)
		return result, cmd
	}

	return m, nil
}

func (m Model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "ctrl+v":
		// Toggle selection mode
		m.selectMode = !m.selectMode
		if !m.selectMode {
			// Clear selections when exiting select mode
			m.selectedItems = make(map[string]bool)
		}
		return m, nil

	case " ":
		// Space: toggle selection of current item (only in select mode)
		treeNodes := m.buildTree()
		if m.selectMode && len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			id := treeNodes[m.cursor].Item.ID
			if m.selectedItems[id] {
				delete(m.selectedItems, id)
			} else {
				m.selectedItems[id] = true
			}
		}
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		treeNodes := m.buildTree()
		if m.cursor < len(treeNodes)-1 {
			m.cursor++
		}

	case "g", "home":
		m.cursor = 0

	case "G", "end":
		treeNodes := m.buildTree()
		m.cursor = max(0, len(treeNodes)-1)

	case "pgup", "ctrl+b":
		// Page up - move cursor up by visible height
		pageSize := m.listVisibleHeight()
		m.cursor -= pageSize
		if m.cursor < 0 {
			m.cursor = 0
		}

	case "pgdown", "ctrl+f":
		// Page down - move cursor down by visible height
		treeNodes := m.buildTree()
		pageSize := m.listVisibleHeight()
		m.cursor += pageSize
		if m.cursor >= len(treeNodes) {
			m.cursor = max(0, len(treeNodes)-1)
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
		treeNodes := m.buildTree()
		pageSize := m.listVisibleHeight() / 2
		if pageSize < 1 {
			pageSize = 1
		}
		m.cursor += pageSize
		if m.cursor >= len(treeNodes) {
			m.cursor = max(0, len(treeNodes)-1)
		}

	case "right":
		// Expand current node if it has children
		treeNodes := m.buildTree()
		if m.cursor < len(treeNodes) {
			node := treeNodes[m.cursor]
			if node.HasChildren && !m.treeExpanded[node.Item.ID] {
				m.treeExpanded[node.Item.ID] = true
			}
		}

	case "left":
		// Collapse current node or jump to parent
		treeNodes := m.buildTree()
		if m.cursor < len(treeNodes) {
			node := treeNodes[m.cursor]
			if node.HasChildren && m.treeExpanded[node.Item.ID] {
				// Collapse if expanded
				delete(m.treeExpanded, node.Item.ID)
			} else if node.Level > 0 {
				// Jump to parent if collapsed and has a parent
				if node.Item.ParentID != nil {
					for i, n := range treeNodes {
						if n.Item.ID == *node.Item.ParentID {
							m.cursor = i
							break
						}
					}
				}
			}
		}

	case "enter", "l":
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 {
			m.viewMode = ViewDetail
			return m, m.loadDetail()
		}

	// Actions - show status menu for confirmation
	case "s":
		if m.selectMode && len(m.selectedItems) > 0 {
			return m.startInput(InputBatchStatus, "Batch status (o=open, i=in_progress, b=blocked, d=done, c=canceled): ")
		}
		return m.showStatusMenu(0) // Start selected
	case "p":
		if m.selectMode && len(m.selectedItems) > 0 {
			return m.startInput(InputBatchPriority, "Batch priority (1-5): ")
		}
		return m.startInput(InputProject, "Project: ")
	case "d":
		if m.selectMode && len(m.selectedItems) > 0 {
			return m.doBatchDone()
		}
		return m.showStatusMenu(1) // Done selected
	case "b":
		return m.showStatusMenu(2) // Block selected
	case "L":
		return m.startInput(InputLog, "Log message: ")
	case "c":
		return m.showStatusMenu(3) // Cancel selected
	case "D":
		return m.doDelete()

	// Filtering
	case "/":
		return m.startInput(InputSearch, "Search: ")
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
		// Start the creation wizard instead of simple input
		m.viewMode = ViewCreateWizard
		m.createWizardStep = 1
		m.createWizardState = CreateWizardState{
			Priority:     2,
			Project:      m.project,
			TemplateVars: make(map[string]string),
		}
		return m, nil

	// Templates
	case "T":
		m.viewMode = ViewTemplateList
		return m, m.loadTemplates()

	// Config
	case "C":
		m.viewMode = ViewConfig
		return m, m.loadConfig()
	}

	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "h", "backspace":
		if m.descViewMode == DescViewVars {
			// Exit variable edit mode, go back to rendered view
			m.descViewMode = DescViewRendered
			m.varCursor = -1
			return m, nil
		}
		if m.depNavActive {
			m.depNavActive = false
			return m, nil
		}
		m.viewMode = ViewList
		m.logsVisible = false
		m.varCursor = -1
		m.descViewMode = DescViewRendered // Reset to default view mode

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
		} else if m.varCursor >= 0 {
			// Navigate variables
			treeNodes := m.buildTree()
			if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
				item := treeNodes[m.cursor].Item
				varNames := m.getSortedVarNames(item)
				if m.varCursor < len(varNames)-1 {
					m.varCursor++
				}
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
		} else if m.varCursor >= 0 {
			// Navigate variables
			if m.varCursor > 0 {
				m.varCursor--
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
				// Find the target in tree nodes
				treeNodes := m.buildTree()
				for i, node := range treeNodes {
					if node.Item.ID == targetID {
						m.cursor = i
						m.depNavActive = false
						return m, m.loadDetail()
					}
				}
				m.message = fmt.Sprintf("Item %s not in current filter", targetID)
			}
		}

	// Actions work in detail view too - show status menu for confirmation
	case "s":
		return m.showStatusMenu(0) // Start selected
	case "d":
		return m.showStatusMenu(1) // Done selected
	case "b":
		return m.showStatusMenu(2) // Block selected
	case "L":
		return m.startInput(InputLog, "Log message: ")
	case "c":
		return m.showStatusMenu(3) // Cancel selected
	case "a":
		return m.startInput(InputAddDep, "Add blocker ID: ")
	case "e":
		// Open built-in textarea editor for description or selected variable
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			// If variable cursor is active, edit that variable
			if m.varCursor >= 0 && item.TemplateID != "" && len(item.TemplateVars) > 0 {
				varNames := m.getSortedVarNames(item)
				if m.varCursor < len(varNames) {
					varName := varNames[m.varCursor]
					return m.startTextareaEdit("var:"+varName, item.TemplateVars[varName])
				}
			}
			// Otherwise edit description
			return m.startTextareaEdit("description", item.Description)
		}
	case "E":
		// Toggle variable edit mode for templated items
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			if item.TemplateID != "" && len(item.TemplateVars) > 0 {
				if m.descViewMode == DescViewVars {
					// Exit variable edit mode, go back to rendered view
					m.descViewMode = DescViewRendered
					m.varCursor = -1
				} else {
					// Enter variable edit mode
					m.descViewMode = DescViewVars
					m.varCursor = 0
				}
			}
		}

	case "r":
		return m, m.loadDetail()

	case "g":
		// Enter graph view
		m.buildGraph()
		m.viewMode = ViewGraph
		return m, nil

	// Toggle template variable expansion
	case "x":
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			if item.TemplateID != "" && len(item.TemplateVars) > 0 {
				// Toggle all variables
				allExpanded := true
				for k := range item.TemplateVars {
					if !m.varExpanded[k] {
						allExpanded = false
						break
					}
				}
				for k := range item.TemplateVars {
					m.varExpanded[k] = !allExpanded
				}
			}
		}

	// Toggle description view mode (rendered/stored/vars)
	case "X":
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			if item.TemplateID != "" {
				// Cycle through view modes: rendered -> stored -> vars -> rendered
				m.descViewMode = (m.descViewMode + 1) % 3
				// Reset variable cursor when entering/exiting vars mode
				if m.descViewMode == DescViewVars {
					m.varCursor = 0
				} else {
					m.varCursor = -1
				}
			}
		}

	// Refresh stored description from rendered template
	case "R":
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			if item.TemplateID != "" {
				rendered := renderTemplateForItem(item)
				return m, func() tea.Msg {
					if err := m.db.SetDescription(item.ID, rendered); err != nil {
						return actionMsg{err: err}
					}
					return actionMsg{message: fmt.Sprintf("Updated description for %s from template", item.ID)}
				}
			}
		}
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

// buildGraph constructs the graph nodes from the current item's dependencies.
func (m *Model) buildGraph() {
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 || m.cursor >= len(treeNodes) {
		m.graphNodes = nil
		return
	}

	item := treeNodes[m.cursor].Item
	m.graphCurrentID = item.ID
	m.graphNodes = nil

	// Add blockers (column 0)
	for i, dep := range m.detailDeps {
		m.graphNodes = append(m.graphNodes, graphNode{
			ID:       dep.ID,
			Title:    dep.Title,
			Status:   dep.Status,
			Column:   0,
			Position: i,
		})
	}

	// Add current item (column 1)
	m.graphNodes = append(m.graphNodes, graphNode{
		ID:       item.ID,
		Title:    item.Title,
		Status:   string(item.Status),
		Column:   1,
		Position: 0,
	})

	// Add blocked items (column 2)
	for i, dep := range m.detailBlocks {
		m.graphNodes = append(m.graphNodes, graphNode{
			ID:       dep.ID,
			Title:    dep.Title,
			Status:   dep.Status,
			Column:   2,
			Position: i,
		})
	}

	m.graphCursor = len(m.detailDeps) // Start cursor on current item
}

// buildTree constructs a hierarchical tree from filtered items.
// Returns flattened list with level information.
func (m *Model) buildTree() []treeNode {
	// Create a map of all items for quick lookup
	itemMap := make(map[string]model.Item)
	for _, item := range m.filtered {
		itemMap[item.ID] = item
	}

	// Create a map of parent -> children relationships
	childrenMap := make(map[string][]model.Item)
	for _, item := range m.filtered {
		if item.ParentID != nil {
			childrenMap[*item.ParentID] = append(childrenMap[*item.ParentID], item)
		}
	}

	var nodes []treeNode

	// Find root items (no parent or parent not in filtered list)
	for _, item := range m.filtered {
		isRoot := item.ParentID == nil
		if item.ParentID != nil {
			if _, hasParent := itemMap[*item.ParentID]; !hasParent {
				isRoot = true // Parent not in filtered list, treat as root
			}
		}

		if isRoot {
			nodes = append(nodes, treeNode{
				Item:        item,
				Level:       0,
				HasChildren: len(childrenMap[item.ID]) > 0,
			})

			// Recursively add children if expanded
			if m.treeExpanded[item.ID] {
				nodes = append(nodes, m.getChildNodes(item.ID, 1, childrenMap)...)
			}
		}
	}

	return nodes
}

// getChildNodes recursively gets child nodes for a parent.
func (m *Model) getChildNodes(parentID string, level int, childrenMap map[string][]model.Item) []treeNode {
	var nodes []treeNode
	children := childrenMap[parentID]

	for i, child := range children {
		nodes = append(nodes, treeNode{
			Item:        child,
			Level:       level,
			HasChildren: len(childrenMap[child.ID]) > 0,
			IsLastChild: i == len(children)-1,
		})

		// Recursively add grandchildren if expanded
		if m.treeExpanded[child.ID] {
			nodes = append(nodes, m.getChildNodes(child.ID, level+1, childrenMap)...)
		}
	}

	return nodes
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

func (m Model) handleConfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle editing mode
	if m.configEditing {
		switch msg.String() {
		case "esc":
			m.configEditing = false
			m.inputText = ""
			return m, nil
		case "enter":
			// Save the value
			if m.configCursor < len(m.configFields) {
				field := m.configFields[m.configCursor]
				config, err := db.LoadConfig()
				if err != nil {
					m.err = err
					m.configEditing = false
					return m, nil
				}
				if err := db.SetConfigField(config, field.Path, m.inputText); err != nil {
					m.err = err
					m.configEditing = false
					return m, nil
				}
				if err := db.SaveConfig(config); err != nil {
					m.err = err
					m.configEditing = false
					return m, nil
				}
				m.message = fmt.Sprintf("Set %s = %s", field.Path, m.inputText)
				m.configEditing = false
				m.inputText = ""
				return m, m.loadConfig()
			}
			return m, nil
		case "backspace":
			if len(m.inputText) > 0 {
				m.inputText = m.inputText[:len(m.inputText)-1]
			}
			return m, nil
		default:
			if len(msg.String()) == 1 {
				m.inputText += msg.String()
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "h", "backspace":
		m.viewMode = ViewList
		m.configCursor = 0
		return m, nil

	case "up", "k":
		if m.configCursor > 0 {
			m.configCursor--
		}

	case "down", "j":
		if m.configCursor < len(m.configFields)-1 {
			m.configCursor++
		}

	case "g", "home":
		m.configCursor = 0

	case "G", "end":
		m.configCursor = max(0, len(m.configFields)-1)

	case "enter", "e":
		// Start editing the current field
		if m.configCursor < len(m.configFields) {
			field := m.configFields[m.configCursor]
			// Don't allow editing map fields directly
			if field.Type == "map" {
				m.message = "Cannot edit map fields directly; edit config.json"
				return m, nil
			}
			m.configEditing = true
			// Pre-fill with current value
			if field.Value != nil {
				m.inputText = fmt.Sprintf("%v", field.Value)
			} else {
				m.inputText = ""
			}
		}

	case "r":
		return m, m.loadConfig()
	}

	return m, nil
}

func (m Model) handleGraphKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "esc", "h", "backspace":
		m.viewMode = ViewDetail
		return m, nil

	case "j", "down":
		if m.graphCursor < len(m.graphNodes)-1 {
			m.graphCursor++
		}

	case "k", "up":
		if m.graphCursor > 0 {
			m.graphCursor--
		}

	case "enter":
		if m.graphCursor >= 0 && m.graphCursor < len(m.graphNodes) {
			targetID := m.graphNodes[m.graphCursor].ID
			// Find the target in filtered items
			for i, item := range m.filtered {
				if item.ID == targetID {
					m.cursor = i
					m.viewMode = ViewDetail
					return m, m.loadDetail()
				}
			}
			m.message = fmt.Sprintf("Item %s not in current filter", targetID)
		}
	}

	return m, nil
}

func (m Model) handleCreateWizardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel wizard
		m.viewMode = ViewList
		m.createWizardStep = 0
		m.message = "Creation canceled"
		return m, nil

	case "enter":
		return m.advanceWizardStep()

	case "left", "h":
		// Go back
		if m.createWizardStep > 1 {
			m.createWizardStep--
			// Skip worktree step if not an epic
			if m.createWizardStep == 4 && m.createWizardState.SelectedType != model.ItemTypeEpic {
				m.createWizardStep = 3
			}
		}
		return m, nil

	case "tab":
		// Switch between fields in current step
		return m.handleWizardTab()

	case "up", "k":
		return m.handleWizardUp()

	case "down", "j":
		return m.handleWizardDown()
	}

	// Handle character input for text fields
	if len(msg.String()) == 1 {
		return m.handleWizardCharInput(msg.String())
	}

	return m, nil
}

func (m Model) handleWizardTab() (tea.Model, tea.Cmd) {
	// Tab handling depends on current step
	switch m.createWizardStep {
	case 1: // Type selection - no tab fields
	case 2: // Basic config - toggle between priority and project
		m.createWizardState.UseCustomProject = !m.createWizardState.UseCustomProject
	case 3: // Relationships - toggle between parent, dependencies, blockers
		// Cycle through relationship fields
	case 4: // Worktree - toggle between fields
	case 5: // Method - toggle between ad-hoc and template
		m.createWizardState.UseTemplate = !m.createWizardState.UseTemplate
	case 6: // Title/Variables
	case 7: // Description
	}
	return m, nil
}

func (m Model) handleWizardUp() (tea.Model, tea.Cmd) {
	switch m.createWizardStep {
	case 1: // Type selection
		types := []model.ItemType{
			model.ItemTypeTask,
			model.ItemTypeEpic,
		}
		// Find current index and move up
		for i, t := range types {
			if t == m.createWizardState.SelectedType && i > 0 {
				m.createWizardState.SelectedType = types[i-1]
				break
			}
		}
	case 2: // Priority selection
		if m.createWizardState.Priority > 1 {
			m.createWizardState.Priority--
		}
	case 5: // Template selection
		if m.createWizardState.UseTemplate && len(m.templates) > 0 {
			// Find current template index
			for i, t := range m.templates {
				if m.createWizardState.SelectedTemplate != nil && t.ID == m.createWizardState.SelectedTemplate.ID && i > 0 {
					m.createWizardState.SelectedTemplate = m.templates[i-1]
					break
				}
			}
		}
	}
	return m, nil
}

func (m Model) handleWizardDown() (tea.Model, tea.Cmd) {
	switch m.createWizardStep {
	case 1: // Type selection
		types := []model.ItemType{
			model.ItemTypeTask,
			model.ItemTypeEpic,
		}
		// Find current index and move down
		for i, t := range types {
			if t == m.createWizardState.SelectedType && i < len(types)-1 {
				m.createWizardState.SelectedType = types[i+1]
				break
			}
		}
	case 2: // Priority selection
		if m.createWizardState.Priority < 5 {
			m.createWizardState.Priority++
		}
	case 5: // Template selection
		if m.createWizardState.UseTemplate && len(m.templates) > 0 {
			// Find current template index
			for i, t := range m.templates {
				if m.createWizardState.SelectedTemplate != nil && t.ID == m.createWizardState.SelectedTemplate.ID && i < len(m.templates)-1 {
					m.createWizardState.SelectedTemplate = m.templates[i+1]
					break
				}
			}
			// If no template selected yet, select first
			if m.createWizardState.SelectedTemplate == nil {
				m.createWizardState.SelectedTemplate = m.templates[0]
			}
		}
	}
	return m, nil
}

func (m Model) handleWizardCharInput(char string) (tea.Model, tea.Cmd) {
	switch m.createWizardStep {
	case 2: // Basic config - project input
		if m.createWizardState.UseCustomProject {
			m.createWizardState.Project += char
		}
	case 3: // Relationships
	case 4: // Worktree
		if m.createWizardState.UseWorktree {
			m.createWizardState.WorktreeBranch += char
		}
	case 6: // Title/Variables
		m.createWizardState.Title += char
	case 7: // Description
		m.createWizardState.Description += char
	}
	return m, nil
}

func (m Model) advanceWizardStep() (tea.Model, tea.Cmd) {
	state := &m.createWizardState

	switch m.createWizardStep {
	case 1: // Type selected
		if state.SelectedType == "" {
			state.SelectedType = model.ItemTypeTask // Default
		}
		m.createWizardStep = 2

	case 2: // Basic config
		m.createWizardStep = 3

	case 3: // Relationships
		// Skip worktree if not epic
		if state.SelectedType == model.ItemTypeEpic {
			m.createWizardStep = 4
		} else {
			m.createWizardStep = 5
		}

	case 4: // Worktree (epics only)
		m.createWizardStep = 5

	case 5: // Method selection
		if state.UseTemplate && state.SelectedTemplate == nil {
			// Need to select template first
			return m, m.loadTemplates()
		}
		m.createWizardStep = 6

	case 6: // Title/Variables
		if state.Title == "" {
			m.err = fmt.Errorf("title is required")
			return m, nil
		}
		m.createWizardStep = 7

	case 7: // Description
		// Validate description
		if !validateDescription(state.Description) {
			m.err = fmt.Errorf("description must be at least 3 words or 20 characters")
			return m, nil
		}
		m.createWizardStep = 8

	case 8: // Confirmation
		return m.createItemFromWizard()
	}

	return m, nil
}

func validateDescription(desc string) bool {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return false
	}

	// Check word count
	words := strings.Fields(desc)
	if len(words) >= 3 {
		return true
	}

	// Check character count
	if len(desc) >= 20 {
		return true
	}

	return false
}

func (m Model) createItemFromWizard() (tea.Model, tea.Cmd) {
	state := m.createWizardState

	return m, func() tea.Msg {
		// Generate ID
		itemID, err := m.db.GenerateItemID(state.SelectedType)
		if err != nil {
			return actionMsg{err: err}
		}

		now := time.Now()
		newItem := &model.Item{
			ID:             itemID,
			Project:        state.Project,
			Type:           state.SelectedType,
			Title:          state.Title,
			Description:    state.Description,
			Status:         model.StatusOpen,
			Priority:       state.Priority,
			WorktreeBranch: state.WorktreeBranch,
			WorktreeBase:   state.WorktreeBase,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if err := m.db.CreateItem(newItem); err != nil {
			return actionMsg{err: err}
		}

		// Set parent if specified
		if state.ParentID != "" {
			if err := m.db.SetParent(itemID, state.ParentID); err != nil {
				return actionMsg{err: err}
			}
		}

		// Add dependencies
		for _, depID := range state.Dependencies {
			if err := m.db.AddDep(itemID, depID); err != nil {
				return actionMsg{err: err}
			}
		}

		// Add blockers (reverse dependencies)
		for _, blockerID := range state.Blockers {
			if err := m.db.AddDep(blockerID, itemID); err != nil {
				return actionMsg{err: err}
			}
		}

		// Add labels
		for _, label := range state.Labels {
			if err := m.db.AddLabelToItem(itemID, state.Project, label); err != nil {
				return actionMsg{err: err}
			}
		}

		return actionMsg{message: fmt.Sprintf("Created %s (%s)", itemID, state.SelectedType)}
	}
}

func (m Model) startInput(mode InputMode, label string) (Model, tea.Cmd) {
	m.inputMode = mode
	m.inputLabel = label
	m.inputText = ""
	return m, nil
}

// showStatusMenu opens the status change confirmation menu with the given option pre-selected.
func (m Model) showStatusMenu(cursor int) (Model, tea.Cmd) {
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 {
		return m, nil
	}
	m.inputMode = InputStatusMenu
	m.statusMenuCursor = cursor
	return m, nil
}

// startTextareaEdit initializes the textarea for multi-line editing.
func (m Model) startTextareaEdit(target, content string) (Model, tea.Cmd) {
	m.textarea.SetValue(content)
	m.textarea.Focus()
	m.textareaTarget = target
	m.textareaOriginal = content
	m.inputMode = InputTextarea
	// Resize textarea to fit available space
	width := m.width - (contentPadding * 2) - 4
	if width < 40 {
		width = 80
	}
	height := m.height - 10 // Leave room for header and help
	if height < 5 {
		height = 10
	}
	m.textarea.SetWidth(width)
	m.textarea.SetHeight(height)
	return m, textarea.Blink
}

// handleTextareaKey handles key events when in textarea editing mode.
func (m Model) handleTextareaKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel editing, restore original
		m.inputMode = InputNone
		m.textarea.Blur()
		m.message = "Edit canceled"
		return m, nil
	case "ctrl+s":
		// Save changes
		return m.saveTextareaEdit()
	case "ctrl+e":
		// Switch to external editor
		return m.switchToExternalEditor()
	}
	// Pass all other keys to textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// saveTextareaEdit saves the textarea content to the database.
func (m Model) saveTextareaEdit() (Model, tea.Cmd) {
	content := m.textarea.Value()
	target := m.textareaTarget

	// Exit textarea mode
	m.inputMode = InputNone
	m.textarea.Blur()

	treeNodes := m.buildTree()
	if len(treeNodes) == 0 || m.cursor >= len(treeNodes) {
		return m, nil
	}
	item := treeNodes[m.cursor].Item

	if target == "description" {
		return m, func() tea.Msg {
			if err := m.db.SetDescription(item.ID, content); err != nil {
				return actionMsg{err: fmt.Errorf("failed to save description: %w", err)}
			}
			return actionMsg{message: fmt.Sprintf("Updated description for %s", item.ID)}
		}
	}

	// Handle template variable editing (var:<name>)
	if strings.HasPrefix(target, "var:") {
		varName := strings.TrimPrefix(target, "var:")
		return m, func() tea.Msg {
			if err := m.db.SetTemplateVar(item.ID, varName, content); err != nil {
				return actionMsg{err: fmt.Errorf("failed to save variable %s: %w", varName, err)}
			}
			return actionMsg{message: fmt.Sprintf("Updated variable '%s' for %s", varName, item.ID)}
		}
	}

	return m, nil
}

// getSortedVarNames returns the template variable names sorted alphabetically.
func (m Model) getSortedVarNames(item model.Item) []string {
	varNames := make([]string, 0, len(item.TemplateVars))
	for k := range item.TemplateVars {
		varNames = append(varNames, k)
	}
	sort.Strings(varNames)
	return varNames
}

// switchToExternalEditor switches from textarea to external editor.
func (m Model) switchToExternalEditor() (Model, tea.Cmd) {
	// Get current textarea content
	content := m.textarea.Value()
	target := m.textareaTarget

	// Exit textarea mode
	m.inputMode = InputNone
	m.textarea.Blur()

	treeNodes := m.buildTree()
	if len(treeNodes) == 0 || m.cursor >= len(treeNodes) {
		return m, nil
	}
	item := treeNodes[m.cursor].Item

	// Create temp file with current content
	tmpfile, err := os.CreateTemp("", "tpg-edit-*.md")
	if err != nil {
		m.err = fmt.Errorf("failed to create temp file: %w", err)
		return m, nil
	}
	tmpPath := tmpfile.Name()

	// Write current textarea content (not original)
	if _, err := tmpfile.WriteString(content); err != nil {
		_ = tmpfile.Close()
		_ = os.Remove(tmpPath)
		m.err = fmt.Errorf("failed to write temp file: %w", err)
		return m, nil
	}
	if err := tmpfile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		m.err = fmt.Errorf("failed to close temp file: %w", err)
		return m, nil
	}

	// Get original mod time for comparison
	origStat, err := os.Stat(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		m.err = fmt.Errorf("failed to stat temp file: %w", err)
		return m, nil
	}

	editor := getEditor()
	c := exec.Command(editor, tmpPath)

	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{
			itemID:   item.ID,
			target:   target,
			tmpPath:  tmpPath,
			origTime: origStat.ModTime(),
			err:      err,
		}
	})
}

func (m Model) doStart() (Model, tea.Cmd) {
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 {
		return m, nil
	}
	item := treeNodes[m.cursor].Item
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
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 {
		return m, nil
	}
	item := treeNodes[m.cursor].Item
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
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 {
		return m, nil
	}
	item := treeNodes[m.cursor].Item
	return m, func() tea.Msg {
		if err := m.db.DeleteItem(item.ID); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{message: fmt.Sprintf("Deleted %s", item.ID)}
	}
}

func (m Model) doBatchDone() (Model, tea.Cmd) {
	if len(m.selectedItems) == 0 {
		return m, nil
	}
	selectedIDs := make([]string, 0, len(m.selectedItems))
	for id := range m.selectedItems {
		selectedIDs = append(selectedIDs, id)
	}
	return m, func() tea.Msg {
		count := 0
		for _, id := range selectedIDs {
			if err := m.db.UpdateStatus(id, model.StatusDone, db.AgentContext{}); err != nil {
				return actionMsg{err: fmt.Errorf("failed to complete %s: %w", id, err)}
			}
			count++
		}
		return actionMsg{message: fmt.Sprintf("Completed %d items", count)}
	}
}

func (m Model) doBatchStatus(statusChar string) (Model, tea.Cmd) {
	if len(m.selectedItems) == 0 {
		return m, nil
	}
	var status model.Status
	switch statusChar {
	case "o":
		status = model.StatusOpen
	case "i":
		status = model.StatusInProgress
	case "b":
		status = model.StatusBlocked
	case "d":
		status = model.StatusDone
	case "c":
		status = model.StatusCanceled
	default:
		m.message = "Invalid status: use o/i/b/d/c"
		return m, nil
	}
	selectedIDs := make([]string, 0, len(m.selectedItems))
	for id := range m.selectedItems {
		selectedIDs = append(selectedIDs, id)
	}
	return m, func() tea.Msg {
		count := 0
		for _, id := range selectedIDs {
			if err := m.db.UpdateStatus(id, status, db.AgentContext{}); err != nil {
				return actionMsg{err: fmt.Errorf("failed to update %s: %w", id, err)}
			}
			count++
		}
		return actionMsg{message: fmt.Sprintf("Updated %d items to %s", count, status)}
	}
}

func (m Model) doBatchPriority(priorityStr string) (Model, tea.Cmd) {
	if len(m.selectedItems) == 0 {
		return m, nil
	}
	var priority int
	switch priorityStr {
	case "1":
		priority = 1
	case "2":
		priority = 2
	case "3":
		priority = 3
	case "4":
		priority = 4
	case "5":
		priority = 5
	default:
		m.message = "Invalid priority: use 1-5"
		return m, nil
	}
	selectedIDs := make([]string, 0, len(m.selectedItems))
	for id := range m.selectedItems {
		selectedIDs = append(selectedIDs, id)
	}
	return m, func() tea.Msg {
		count := 0
		for _, id := range selectedIDs {
			if err := m.db.UpdatePriority(id, priority); err != nil {
				return actionMsg{err: fmt.Errorf("failed to update %s: %w", id, err)}
			}
			count++
		}
		return actionMsg{message: fmt.Sprintf("Set priority %d on %d items", priority, count)}
	}
}

// getEditor returns the editor command to use.
// Prefers $TPG_EDITOR, then $EDITOR, then nvim, nano, vi.
func getEditor() string {
	if editor := os.Getenv("TPG_EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if _, err := exec.LookPath("nvim"); err == nil {
		return "nvim"
	}
	if _, err := exec.LookPath("nano"); err == nil {
		return "nano"
	}
	return "vi"
}

// editDescription opens the current item's description in an external editor.
// Returns a tea.ExecProcess command that suspends the TUI while editing.
func (m Model) editDescription() (Model, tea.Cmd) {
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 || m.cursor >= len(treeNodes) {
		return m, nil
	}
	item := treeNodes[m.cursor].Item

	// Create temp file with current description
	tmpfile, err := os.CreateTemp("", "tpg-edit-*.md")
	if err != nil {
		m.err = fmt.Errorf("failed to create temp file: %w", err)
		return m, nil
	}
	tmpPath := tmpfile.Name()

	// Write current description
	if _, err := tmpfile.WriteString(item.Description); err != nil {
		_ = tmpfile.Close()
		_ = os.Remove(tmpPath)
		m.err = fmt.Errorf("failed to write temp file: %w", err)
		return m, nil
	}
	if err := tmpfile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		m.err = fmt.Errorf("failed to close temp file: %w", err)
		return m, nil
	}

	// Get original mod time for comparison
	origStat, err := os.Stat(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		m.err = fmt.Errorf("failed to stat temp file: %w", err)
		return m, nil
	}

	editor := getEditor()
	c := exec.Command(editor, tmpPath)

	// Use tea.ExecProcess to suspend TUI and run editor
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{
			itemID:   item.ID,
			target:   "description",
			tmpPath:  tmpPath,
			origTime: origStat.ModTime(),
			err:      err,
		}
	})
}

// handleEditorFinished processes the result of an external editor session.
func (m Model) handleEditorFinished(msg editorFinishedMsg) (Model, tea.Cmd) {
	// Always clean up temp file
	defer func() { _ = os.Remove(msg.tmpPath) }()

	if msg.err != nil {
		m.err = fmt.Errorf("editor failed: %w", msg.err)
		return m, nil
	}

	// Check if file was modified
	newStat, err := os.Stat(msg.tmpPath)
	if err != nil {
		m.err = fmt.Errorf("failed to stat temp file: %w", err)
		return m, nil
	}

	if newStat.ModTime().Equal(msg.origTime) {
		m.message = "No changes made"
		return m, nil
	}

	// Read new content
	newContent, err := os.ReadFile(msg.tmpPath)
	if err != nil {
		m.err = fmt.Errorf("failed to read temp file: %w", err)
		return m, nil
	}

	// Update description in database
	if err := m.db.SetDescription(msg.itemID, string(newContent)); err != nil {
		m.err = fmt.Errorf("failed to update description: %w", err)
		return m, nil
	}

	m.message = fmt.Sprintf("Updated description for %s", msg.itemID)
	// Reload items to reflect the change
	return m, tea.Batch(m.loadItems(), m.loadDetail())
}

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	// Show textarea view when in textarea editing mode
	if m.inputMode == InputTextarea {
		b.WriteString(m.textareaView())
	} else if m.inputMode == InputStatusMenu {
		// Show status menu as a centered overlay
		b.WriteString(m.statusMenuView())
	} else {
		switch m.viewMode {
		case ViewList:
			b.WriteString(m.listView())
		case ViewDetail:
			b.WriteString(m.detailView())
		case ViewGraph:
			b.WriteString(m.graphView())
		case ViewTemplateList:
			b.WriteString(m.templateListView())
		case ViewTemplateDetail:
			b.WriteString(m.templateDetailView())
		case ViewConfig:
			b.WriteString(m.configView())
		case ViewCreateWizard:
			b.WriteString(m.createWizardView())
		}

		// Input line (for non-textarea input modes)
		if m.inputMode != InputNone {
			b.WriteString("\n")
			b.WriteString(inputStyle.Render(m.inputLabel + m.inputText + "█"))
		}
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

// textareaView renders the textarea editing view.
func (m Model) textareaView() string {
	var b strings.Builder

	// Header showing what we're editing
	var title string
	treeNodes := m.buildTree()
	if m.textareaTarget == "description" {
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			title = fmt.Sprintf("Editing description for %s", item.ID)
		} else {
			title = "Editing description"
		}
	} else if strings.HasPrefix(m.textareaTarget, "var:") {
		varName := strings.TrimPrefix(m.textareaTarget, "var:")
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			title = fmt.Sprintf("Editing variable '%s' for %s", varName, item.ID)
		} else {
			title = fmt.Sprintf("Editing variable '%s'", varName)
		}
	}

	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Textarea
	b.WriteString(m.textarea.View())
	b.WriteString("\n\n")

	// Help text
	b.WriteString(helpStyle.Render("ctrl+s:save  esc:cancel  ctrl+e:external editor"))

	return b.String()
}

// statusMenuView renders the status change confirmation menu.
func (m Model) statusMenuView() string {
	var b strings.Builder

	// Get current item info for context
	var itemInfo string
	treeNodes := m.buildTree()
	if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
		item := treeNodes[m.cursor].Item
		itemInfo = fmt.Sprintf("%s: %s", item.ID, item.Title)
		if len(itemInfo) > 50 {
			itemInfo = itemInfo[:47] + "..."
		}
	}

	// Menu box style
	menuStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 2)

	// Menu options
	options := []struct {
		key    string
		label  string
		status string
	}{
		{"s", "Start", "in_progress"},
		{"d", "Done", "done"},
		{"b", "Block", "blocked"},
		{"c", "Cancel", "canceled"},
	}

	var menuContent strings.Builder
	menuContent.WriteString(titleStyle.Render("Change Status") + "\n")
	if itemInfo != "" {
		menuContent.WriteString(dimStyle.Render(itemInfo) + "\n")
	}
	menuContent.WriteString("\n")

	for i, opt := range options {
		prefix := "  "
		if i == m.statusMenuCursor {
			prefix = "▸ "
		}

		line := fmt.Sprintf("%s[%s] %s", prefix, opt.key, opt.label)
		if opt.status != "" {
			line += " " + dimStyle.Render("("+opt.status+")")
		}

		if i == m.statusMenuCursor {
			menuContent.WriteString(selectedRowStyle.Render(line) + "\n")
		} else {
			menuContent.WriteString(line + "\n")
		}
	}

	menuContent.WriteString("\n")
	menuContent.WriteString(helpStyle.Render("↑/↓:select  enter:confirm  esc:cancel"))

	b.WriteString(menuStyle.Render(menuContent.String()))

	return b.String()
}

func (m Model) listView() string {
	var b strings.Builder

	// Header
	title := "tpg"
	b.WriteString(titleStyle.Render(title))
	b.WriteString(fmt.Sprintf("  %d/%d items", len(m.filtered), len(m.items)))

	// Selection mode indicator
	if m.selectMode {
		b.WriteString("  ")
		b.WriteString(selectModeStyle.Render(fmt.Sprintf("[SELECT MODE] (%d selected)", len(m.selectedItems))))
	}

	// Active filters
	filters := m.activeFiltersString()
	if filters != "" {
		b.WriteString("  ")
		b.WriteString(filterStyle.Render(filters))
	}
	b.WriteString("\n\n")

	// Build tree from filtered items
	treeNodes := m.buildTree()

	// Items
	if len(treeNodes) == 0 {
		b.WriteString("No items match filters\n")
	} else {
		// Calculate visible height accounting for header, footer, and padding
		// Header: 3 lines (title + filters + blank), Footer: 3 lines (blank + 2 help lines)
		visibleHeight := m.height - 6
		if visibleHeight < 3 {
			visibleHeight = 3 // Minimum visible items
		}
		if visibleHeight > len(treeNodes) {
			visibleHeight = len(treeNodes)
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
		if start > len(treeNodes)-visibleHeight && len(treeNodes) >= visibleHeight {
			start = len(treeNodes) - visibleHeight
		}

		end := min(start+visibleHeight, len(treeNodes))

		rowWidth := m.width - (contentPadding * 2)
		if rowWidth < 60 {
			rowWidth = 80
		}

		for i := start; i < end; i++ {
			node := treeNodes[i]
			selected := i == m.cursor

			if selected {
				// For selected row: plain text, then apply highlight to full width
				line := m.formatTreeNodeLinePlain(node, rowWidth)
				b.WriteString(selectedRowStyle.Width(rowWidth).Render(line))
			} else {
				// For non-selected: use styled version
				line := m.formatTreeNodeLineStyled(node, rowWidth)
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	// Footer
	b.WriteString("\n")
	if m.selectMode {
		b.WriteString(helpStyle.Render("j/k:nav  space:toggle  s:batch-status  p:batch-priority  d:batch-done"))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("ctrl+v:exit-select  esc:clear-filters  q:quit"))
	} else {
		b.WriteString(helpStyle.Render("j/k:nav  ^u/^d:½pg  pgup/dn:pg  g/G:top/end  enter:detail  s:start d:done n:new"))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("/:search p:project t:label 1-5:status 0:all  b:block L:log c:cancel  T:templates C:config  r:refresh q:quit"))
	}

	return b.String()
}

// formatItemLinePlain returns a plain text line without any ANSI styling.
// Used for selected rows where we apply a single highlight style.
func (m Model) formatItemLinePlain(item model.Item, width int) string {
	status := formatStatus(item.Status)

	// Selection indicator
	selectPrefix := ""
	selectWidth := 0
	if m.selectMode {
		if m.selectedItems[item.ID] {
			selectPrefix = "✓ "
		} else {
			selectPrefix = "  "
		}
		selectWidth = 2
	}

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

	// Status width: icon (1-2) + space + text (up to 6) = 9 chars padded
	statusWidth := 9

	// Format: status type id title [label1] [label2] [project]
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
	fixedWidth := 10 + labelsWidth + projectWidth + staleWidth + agentWidth + typeWidth + statusWidth + selectWidth
	titleWidth := width - fixedWidth
	if titleWidth < 20 {
		titleWidth = 40
	}

	title := item.Title
	if len(title) > titleWidth {
		title = title[:titleWidth-3] + "..."
	}

	if agent != "" {
		return fmt.Sprintf("%s%s%-8s %-4s %s %s %-*s%s %s", selectPrefix, stale, status, itemType, item.ID, agent, titleWidth, title, labels, project)
	}
	return fmt.Sprintf("%s%s%-8s %-4s %s  %-*s%s %s", selectPrefix, stale, status, itemType, item.ID, titleWidth, title, labels, project)
}

// formatItemLineStyled returns a styled line with colors for non-selected rows.
func (m Model) formatItemLineStyled(item model.Item, width int) string {
	icon := statusIcon(item.Status)
	text := statusText(item.Status)
	color := statusColors[item.Status]
	statusStyled := lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%-8s", icon+" "+text))

	// Selection indicator
	selectPrefix := ""
	selectWidth := 0
	if m.selectMode {
		if m.selectedItems[item.ID] {
			selectPrefix = selectModeStyle.Render("✓") + " "
		} else {
			selectPrefix = "  "
		}
		selectWidth = 2
	}

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

	// Status width: icon (1-2) + space + text (up to 6) = 9 chars padded
	statusWidth := 9

	// Format: status type id title [label1] [label2] [project]
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
	fixedWidth := 10 + labelsWidth + projectWidth + staleWidth + agentWidth + typeWidth + statusWidth + selectWidth
	titleWidth := width - fixedWidth
	if titleWidth < 20 {
		titleWidth = 40
	}

	title := item.Title
	if len(title) > titleWidth {
		title = title[:titleWidth-3] + "..."
	}

	if agent != "" {
		return fmt.Sprintf("%s%s%s %s %s %s %-*s%s %s", selectPrefix, stale, statusStyled, typeStyled, id, agent, titleWidth, title, labels, project)
	}
	return fmt.Sprintf("%s%s%s %s %s  %-*s%s %s", selectPrefix, stale, statusStyled, typeStyled, id, titleWidth, title, labels, project)
}

// formatTreeNodeLinePlain returns a plain text line for a tree node without ANSI styling.
// Used for selected rows where we apply a single highlight style.
func (m Model) formatTreeNodeLinePlain(node treeNode, width int) string {
	item := node.Item
	status := formatStatus(item.Status)

	// Build tree prefix with indentation and expand/collapse indicators
	treePrefix := m.buildTreePrefix(node)
	treePrefixWidth := len(treePrefix)

	// Selection indicator
	selectPrefix := ""
	selectWidth := 0
	if m.selectMode {
		if m.selectedItems[item.ID] {
			selectPrefix = "✓ "
		} else {
			selectPrefix = "  "
		}
		selectWidth = 2
	}

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

	// Status width: icon (1-2) + space + text (up to 6) = 9 chars padded
	statusWidth := 9

	// Format: status type id title [label1] [label2] [project]
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
	fixedWidth := 10 + labelsWidth + projectWidth + staleWidth + agentWidth + typeWidth + statusWidth + selectWidth + treePrefixWidth
	titleWidth := width - fixedWidth
	if titleWidth < 20 {
		titleWidth = 40
	}

	title := item.Title
	if len(title) > titleWidth {
		title = title[:titleWidth-3] + "..."
	}

	if agent != "" {
		return fmt.Sprintf("%s%s%s%-8s %-4s %s %s %-*s%s %s", selectPrefix, stale, treePrefix, status, itemType, item.ID, agent, titleWidth, title, labels, project)
	}
	return fmt.Sprintf("%s%s%s%-8s %-4s %s  %-*s%s %s", selectPrefix, stale, treePrefix, status, itemType, item.ID, titleWidth, title, labels, project)
}

// formatTreeNodeLineStyled returns a styled line with colors for non-selected rows.
func (m Model) formatTreeNodeLineStyled(node treeNode, width int) string {
	item := node.Item
	icon := statusIcon(item.Status)
	text := statusText(item.Status)
	color := statusColors[item.Status]
	statusStyled := lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%-8s", icon+" "+text))

	// Build tree prefix with indentation and expand/collapse indicators
	treePrefix := m.buildTreePrefix(node)
	treePrefixWidth := len(treePrefix)

	// Selection indicator
	selectPrefix := ""
	selectWidth := 0
	if m.selectMode {
		if m.selectedItems[item.ID] {
			selectPrefix = selectModeStyle.Render("✓") + " "
		} else {
			selectPrefix = "  "
		}
		selectWidth = 2
	}

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

	// Status width: icon (1-2) + space + text (up to 6) = 9 chars padded
	statusWidth := 9

	// Format: status type id title [label1] [label2] [project]
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
	fixedWidth := 10 + labelsWidth + projectWidth + staleWidth + agentWidth + typeWidth + statusWidth + selectWidth + treePrefixWidth
	titleWidth := width - fixedWidth
	if titleWidth < 20 {
		titleWidth = 40
	}

	title := item.Title
	if len(title) > titleWidth {
		title = title[:titleWidth-3] + "..."
	}

	// Render tree prefix with appropriate styling
	styledTreePrefix := dimStyle.Render(treePrefix)

	if agent != "" {
		return fmt.Sprintf("%s%s%s%s %s %s %s %-*s%s %s", selectPrefix, stale, styledTreePrefix, statusStyled, typeStyled, id, agent, titleWidth, title, labels, project)
	}
	return fmt.Sprintf("%s%s%s%s %s %s  %-*s%s %s", selectPrefix, stale, styledTreePrefix, statusStyled, typeStyled, id, titleWidth, title, labels, project)
}

// buildTreePrefix creates the indentation and branch indicators for a tree node.
func (m Model) buildTreePrefix(node treeNode) string {
	if node.Level == 0 {
		// Root level - no indentation
		if node.HasChildren {
			if m.treeExpanded[node.Item.ID] {
				return "▼ "
			}
			return "▶ "
		}
		return "○ "
	}

	// Build indentation based on level
	prefix := ""
	for i := 0; i < node.Level-1; i++ {
		prefix += "│  "
	}

	// Add branch connector
	if node.IsLastChild {
		prefix += "└─ "
	} else {
		prefix += "├─ "
	}

	// Add expand/collapse indicator for nodes with children
	if node.HasChildren {
		if m.treeExpanded[node.Item.ID] {
			prefix += "▼ "
		} else {
			prefix += "▶ "
		}
	} else {
		prefix += "○ "
	}

	return prefix
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

	return strings.Join(parts, " ")
}

// templateInfo holds information about a template for display purposes.
type templateInfo struct {
	name        string
	stepNum     int                 // 1-based step number (0 if no step)
	totalSteps  int                 // total number of steps
	notFound    bool                // template doesn't exist
	invalidStep bool                // step index is out of range
	tmpl        *templates.Template // the loaded template (nil if not found)
}

// getTemplateInfo loads template information for an item.
func getTemplateInfo(item model.Item) templateInfo {
	info := templateInfo{
		name: item.TemplateID,
	}

	if item.TemplateID == "" {
		return info
	}

	tmpl, err := templates.LoadTemplate(item.TemplateID)
	if err != nil {
		info.notFound = true
		return info
	}

	info.tmpl = tmpl
	info.totalSteps = len(tmpl.Steps)

	if item.StepIndex != nil {
		stepIdx := *item.StepIndex
		info.stepNum = stepIdx + 1 // Convert to 1-based
		if stepIdx < 0 || stepIdx >= len(tmpl.Steps) {
			info.invalidStep = true
		}
	}

	return info
}

// getUnusedVariables returns variables that exist in item.TemplateVars but aren't used in the template.
func getUnusedVariables(tmpl *templates.Template, vars map[string]string, stepIndex *int) map[string]string {
	if tmpl == nil || vars == nil {
		return nil
	}

	// Collect all variable references from the template
	usedVars := make(map[string]bool)

	// Parse the template description for {{.varname}} patterns
	parseVarRefs := func(text string) {
		// Match {{.varname}} and {{ .varname }} patterns
		// Also match {{if .varname}}, {{with .varname}}, etc.
		for i := 0; i < len(text); i++ {
			if i+2 < len(text) && text[i:i+2] == "{{" {
				// Find the closing }}
				end := strings.Index(text[i:], "}}")
				if end == -1 {
					continue
				}
				content := text[i+2 : i+end]
				// Look for .varname patterns
				for j := 0; j < len(content); j++ {
					if content[j] == '.' && j+1 < len(content) {
						// Extract the variable name
						start := j + 1
						k := start
						for k < len(content) && (content[k] == '_' || (content[k] >= 'a' && content[k] <= 'z') || (content[k] >= 'A' && content[k] <= 'Z') || (content[k] >= '0' && content[k] <= '9')) {
							k++
						}
						if k > start {
							varName := content[start:k]
							usedVars[varName] = true
						}
					}
				}
				i += end + 1
			}
		}
	}

	// Parse template-level description
	parseVarRefs(tmpl.Description)

	// Parse the specific step if we have one
	if stepIndex != nil && *stepIndex >= 0 && *stepIndex < len(tmpl.Steps) {
		step := tmpl.Steps[*stepIndex]
		parseVarRefs(step.Title)
		parseVarRefs(step.Description)
	} else {
		// Parse all steps
		for _, step := range tmpl.Steps {
			parseVarRefs(step.Title)
			parseVarRefs(step.Description)
		}
	}

	// Also consider variables defined in the template as "used"
	for name := range tmpl.Variables {
		usedVars[name] = true
	}

	// Find unused variables
	unused := make(map[string]string)
	for name, value := range vars {
		if !usedVars[name] {
			unused[name] = value
		}
	}

	if len(unused) == 0 {
		return nil
	}
	return unused
}

// renderTemplateForItem renders the template description for an item using its stored variables.
// Returns the rendered description and a boolean indicating if rendering was successful.
func renderTemplateForItem(item model.Item) string {
	if item.TemplateID == "" {
		return item.Description
	}

	tmpl, err := templates.LoadTemplate(item.TemplateID)
	if err != nil {
		return item.Description
	}

	// Find the step for this item (if it has a step index)
	if item.StepIndex != nil && *item.StepIndex >= 0 && *item.StepIndex < len(tmpl.Steps) {
		step := tmpl.Steps[*item.StepIndex]
		return templates.RenderText(step.Description, item.TemplateVars)
	}

	// No step index, render the template description
	return templates.RenderText(tmpl.Description, item.TemplateVars)
}

func (m Model) detailView() string {
	treeNodes := m.buildTree()
	if len(treeNodes) == 0 || m.cursor >= len(treeNodes) {
		return "No item selected"
	}

	item := treeNodes[m.cursor].Item
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

	// Template information - always show if item has a template
	var tmplInfo templateInfo
	if item.TemplateID != "" {
		tmplInfo = getTemplateInfo(item)

		// Format: "Template: <name>" or "Template: <name>, step <n>"
		tmplLine := "Template: " + tmplInfo.name
		if tmplInfo.notFound {
			tmplLine += " " + errorStyle.Render("[NOT FOUND]")
		} else if tmplInfo.invalidStep {
			tmplLine += fmt.Sprintf(", step %d", tmplInfo.stepNum) + " " + errorStyle.Render("[INVALID STEP]")
		} else if tmplInfo.totalSteps > 1 && tmplInfo.stepNum > 0 {
			tmplLine += fmt.Sprintf(", step %d", tmplInfo.stepNum)
		}
		b.WriteString(detailLabelStyle.Render("Template: ") + tmplLine[10:] + "\n") // Skip "Template: " prefix since we use detailLabelStyle

		// Show warning messages for error cases
		if tmplInfo.notFound {
			b.WriteString(errorStyle.Render("  ⚠ Template not found - showing raw variables") + "\n")
		} else if tmplInfo.invalidStep {
			b.WriteString(errorStyle.Render(fmt.Sprintf("  ⚠ Step %d is out of range (template has %d steps) - showing raw variables", tmplInfo.stepNum, tmplInfo.totalSteps)) + "\n")
		} else if tmplInfo.tmpl != nil {
			// Check for hash mismatch
			if item.TemplateHash != "" && item.TemplateHash != tmplInfo.tmpl.Hash {
				b.WriteString("  " + errorStyle.Render("[Template has changed since instantiation]") + "\n")
			}
		}
	}

	// Dependencies — "Blocked by" (what this depends on)
	if len(m.detailDeps) > 0 {
		header := "Blocked by:"
		if m.depNavActive && m.depSection == 0 {
			header = "▸ Blocked by:"
		}
		b.WriteString("\n" + detailLabelStyle.Render(header) + "\n")
		for i, dep := range m.detailDeps {
			depIcon := depStatusIcon(dep.Status)
			selected := m.depNavActive && m.depSection == 0 && i == m.depCursor
			line := fmt.Sprintf("  %s %s %s", depIcon, dep.ID, dep.Title)
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
			depIcon := depStatusIcon(dep.Status)
			selected := m.depNavActive && m.depSection == 1 && i == m.depCursor
			line := fmt.Sprintf("  %s %s %s", depIcon, dep.ID, dep.Title)
			if selected {
				b.WriteString(selectedRowStyle.Render(line) + "\n")
			} else {
				b.WriteString(line + "\n")
			}
		}
	}

	// Description section - behavior depends on whether this is a templated item
	if item.TemplateID != "" {
		// Templated item: show based on view mode and template validity
		showVars := m.descViewMode == DescViewVars || tmplInfo.notFound || tmplInfo.invalidStep

		if showVars {
			// Show raw variables (either user requested via E key, or fallback due to error)
			if len(item.TemplateVars) > 0 {
				// Check if any variables need expansion hint
				hasExpandable := false
				for _, v := range item.TemplateVars {
					if strings.Contains(v, "\n") || len(v) > 60 {
						hasExpandable = true
						break
					}
				}

				varsHeader := "\nVariables:"
				if m.varCursor >= 0 {
					varsHeader = "\n▸ Variables:" + " " + dimStyle.Render("[j/k:nav e:edit esc:exit]")
				} else if hasExpandable {
					varsHeader += " " + dimStyle.Render("[x:expand E:exit]")
				} else {
					varsHeader += " " + dimStyle.Render("[E:exit]")
				}
				b.WriteString(detailLabelStyle.Render(varsHeader) + "\n")

				// Sort variable names for consistent display
				varNames := m.getSortedVarNames(item)

				for i, name := range varNames {
					value := item.TemplateVars[name]
					displayValue := value

					// Check if value needs truncation
					if m.varExpanded[name] {
						// Show full value, indented for multi-line
						if strings.Contains(value, "\n") {
							lines := strings.Split(value, "\n")
							displayValue = lines[0]
							for j := 1; j < len(lines); j++ {
								displayValue += "\n      " + lines[j]
							}
						}
					} else {
						// Truncate if needed
						if strings.Contains(value, "\n") {
							// Show first line only with indicator
							firstLine := strings.Split(value, "\n")[0]
							if len(firstLine) > 60 {
								firstLine = firstLine[:57] + "..."
							}
							displayValue = firstLine + " " + dimStyle.Render("(...)")
						} else if len(value) > 60 {
							displayValue = value[:57] + dimStyle.Render("(...)")
						}
					}

					// Highlight selected variable
					if m.varCursor == i {
						line := fmt.Sprintf("  ▸ %s: %s", name, displayValue)
						b.WriteString(selectedRowStyle.Render(line) + "\n")
					} else {
						b.WriteString("    " + labelStyle.Render(name) + ": " + displayValue + "\n")
					}
				}
			}
		} else {
			// Show rendered or stored description
			descLabel := "\nDescription"
			switch m.descViewMode {
			case DescViewRendered:
				descLabel += " " + dimStyle.Render("[rendered]")
			case DescViewStored:
				descLabel += " " + dimStyle.Render("[stored]")
			}
			descLabel += ":"
			b.WriteString(detailLabelStyle.Render(descLabel) + "\n")

			if m.descViewMode == DescViewStored {
				b.WriteString(item.Description + "\n")
			} else {
				// Default: show rendered description
				rendered := renderTemplateForItem(item)
				b.WriteString(rendered + "\n")
			}

			// Show unused variables at the end (only when showing rendered description)
			if m.descViewMode == DescViewRendered && tmplInfo.tmpl != nil {
				unused := getUnusedVariables(tmplInfo.tmpl, item.TemplateVars, item.StepIndex)
				if len(unused) > 0 {
					b.WriteString("\n" + detailLabelStyle.Render("Unused Variables:") + "\n")
					// Sort for consistent display
					unusedNames := make([]string, 0, len(unused))
					for name := range unused {
						unusedNames = append(unusedNames, name)
					}
					sort.Strings(unusedNames)
					for _, name := range unusedNames {
						value := unused[name]
						displayValue := value
						if len(displayValue) > 50 {
							displayValue = displayValue[:47] + "..."
						}
						if strings.Contains(displayValue, "\n") {
							displayValue = strings.Split(displayValue, "\n")[0] + "..."
						}
						b.WriteString("    " + dimStyle.Render(name+": "+displayValue) + "\n")
					}
				}
			}
		}
	} else {
		// Non-templated item: just show description
		if item.Description != "" {
			b.WriteString("\n" + detailLabelStyle.Render("Description:") + "\n")
			b.WriteString(item.Description + "\n")
		}
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
	help := "esc:back  s:start d:done b:block L:log c:cancel a:add-dep e:edit  v:logs  g:graph  q:quit"
	if len(m.detailDeps) > 0 || len(m.detailBlocks) > 0 {
		help += "  tab:deps enter:jump"
	}
	if item.TemplateID != "" {
		if len(item.TemplateVars) > 0 {
			help += "  x:expand"
		}
		help += "  E:vars X:view R:refresh"
	}
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

func (m Model) graphView() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("Dependency Graph"))
	b.WriteString("\n\n")

	if len(m.graphNodes) == 0 {
		b.WriteString("No dependencies to display\n")
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("esc:back  q:quit"))
		return b.String()
	}

	// Column headers
	b.WriteString(detailLabelStyle.Render("  Blockers"))
	b.WriteString("              ")
	b.WriteString(detailLabelStyle.Render("Current"))
	b.WriteString("               ")
	b.WriteString(detailLabelStyle.Render("Blocks"))
	b.WriteString("\n")

	// Collect nodes by column
	var blockers, current, blocked []graphNode
	for _, node := range m.graphNodes {
		switch node.Column {
		case 0:
			blockers = append(blockers, node)
		case 1:
			current = append(current, node)
		case 2:
			blocked = append(blocked, node)
		}
	}

	// Calculate max rows needed
	maxRows := max(len(blockers), max(len(current), len(blocked)))
	if maxRows == 0 {
		maxRows = 1
	}

	// Render each row
	colWidth := 20 // Width for each column
	for row := 0; row < maxRows; row++ {
		var leftPart, midPart, rightPart string
		var leftSelected, midSelected, rightSelected bool

		// Left column (blockers)
		if row < len(blockers) {
			node := blockers[row]
			icon := depStatusIcon(node.Status)
			title := node.ID
			if len(node.Title) > 0 {
				maxTitle := colWidth - len(node.ID) - 4
				if maxTitle > 0 && len(node.Title) > maxTitle {
					title = node.ID + " " + node.Title[:maxTitle-3] + "..."
				} else if maxTitle > 0 {
					title = node.ID + " " + node.Title
				}
			}
			leftPart = fmt.Sprintf("%s %s", icon, title)
			// Check if this node is selected
			for i, n := range m.graphNodes {
				if n.ID == node.ID && n.Column == 0 && i == m.graphCursor {
					leftSelected = true
					break
				}
			}
		}

		// Middle column (current)
		if row < len(current) {
			node := current[row]
			icon := depStatusIcon(node.Status)
			title := node.ID
			if len(node.Title) > 0 {
				maxTitle := colWidth - len(node.ID) - 4
				if maxTitle > 0 && len(node.Title) > maxTitle {
					title = node.ID + " " + node.Title[:maxTitle-3] + "..."
				} else if maxTitle > 0 {
					title = node.ID + " " + node.Title
				}
			}
			midPart = fmt.Sprintf("%s %s", icon, title)
			// Check if this node is selected
			for i, n := range m.graphNodes {
				if n.ID == node.ID && n.Column == 1 && i == m.graphCursor {
					midSelected = true
					break
				}
			}
		}

		// Right column (blocked)
		if row < len(blocked) {
			node := blocked[row]
			icon := depStatusIcon(node.Status)
			title := node.ID
			if len(node.Title) > 0 {
				maxTitle := colWidth - len(node.ID) - 4
				if maxTitle > 0 && len(node.Title) > maxTitle {
					title = node.ID + " " + node.Title[:maxTitle-3] + "..."
				} else if maxTitle > 0 {
					title = node.ID + " " + node.Title
				}
			}
			rightPart = fmt.Sprintf("%s %s", icon, title)
			// Check if this node is selected
			for i, n := range m.graphNodes {
				if n.ID == node.ID && n.Column == 2 && i == m.graphCursor {
					rightSelected = true
					break
				}
			}
		}

		// Render the row with connectors
		// Left part
		if leftSelected {
			b.WriteString(selectedRowStyle.Render(fmt.Sprintf("%-*s", colWidth, leftPart)))
		} else {
			b.WriteString(fmt.Sprintf("%-*s", colWidth, leftPart))
		}

		// Left connector
		if row < len(blockers) && len(current) > 0 {
			if row == 0 {
				b.WriteString(" ───▶ ")
			} else {
				b.WriteString(" ────┘")
			}
		} else {
			b.WriteString("      ")
		}

		// Middle part
		if midSelected {
			b.WriteString(selectedRowStyle.Render(fmt.Sprintf("%-*s", colWidth, midPart)))
		} else {
			b.WriteString(fmt.Sprintf("%-*s", colWidth, midPart))
		}

		// Right connector
		if row < len(blocked) && len(current) > 0 {
			if row == 0 {
				b.WriteString(" ───▶ ")
			} else {
				b.WriteString(" └────")
			}
		} else {
			b.WriteString("      ")
		}

		// Right part
		if rightSelected {
			b.WriteString(selectedRowStyle.Render(fmt.Sprintf("%-*s", colWidth, rightPart)))
		} else {
			b.WriteString(fmt.Sprintf("%-*s", colWidth, rightPart))
		}

		b.WriteString("\n")
	}

	// Legend
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Legend: "))
	b.WriteString(lipgloss.NewStyle().Foreground(statusColors[model.StatusOpen]).Render(iconOpen + " open"))
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Foreground(statusColors[model.StatusInProgress]).Render(iconInProgress + " in_progress"))
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Foreground(statusColors[model.StatusDone]).Render(iconDone + " done"))
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Foreground(statusColors[model.StatusBlocked]).Render(iconBlocked + " blocked"))
	b.WriteString("\n")

	// Help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k:nav  enter:jump to task  esc:back  q:quit"))

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

func (m Model) configView() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("Configuration"))
	b.WriteString(fmt.Sprintf("  %d settings\n\n", len(m.configFields)))

	// Config fields
	if len(m.configFields) == 0 {
		b.WriteString("No configuration found\n")
		b.WriteString(dimStyle.Render("Run 'tpg init' to create a config file\n"))
	} else {
		visibleHeight := m.height - 8
		if visibleHeight < 5 {
			visibleHeight = 15
		}
		start := 0
		if m.configCursor >= visibleHeight {
			start = m.configCursor - visibleHeight + 1
		}
		end := min(start+visibleHeight, len(m.configFields))

		rowWidth := m.width - (contentPadding * 2)
		if rowWidth < 60 {
			rowWidth = 80
		}

		for i := start; i < end; i++ {
			field := m.configFields[i]
			selected := i == m.configCursor

			// Format value for display
			valueStr := db.FormatConfigValue(field.Value)

			// Format: path = value (type)
			line := fmt.Sprintf("%-35s = %-20s", field.Path, valueStr)
			if field.Type != "" && field.Type != "string" {
				line += " " + dimStyle.Render("("+field.Type+")")
			}

			// Truncate to fit width
			if len(line) > rowWidth {
				line = line[:rowWidth-3] + "..."
			}

			if selected {
				if m.configEditing {
					// Show edit mode
					editLine := fmt.Sprintf("%-35s = ", field.Path)
					b.WriteString(editLine)
					b.WriteString(inputStyle.Render(m.inputText + "█"))
					b.WriteString("\n")
				} else {
					b.WriteString(selectedRowStyle.Width(rowWidth).Render(line))
					b.WriteString("\n")
				}
			} else {
				b.WriteString(line)
				b.WriteString("\n")
			}
		}
	}

	// Footer
	b.WriteString("\n")
	if m.configEditing {
		b.WriteString(helpStyle.Render("enter:save  esc:cancel"))
	} else {
		b.WriteString(helpStyle.Render("j/k:nav  enter/e:edit  r:refresh  esc:back  q:quit"))
	}

	return b.String()
}

func (m Model) createWizardView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf(" Create New Item - Step %d/8 ", m.createWizardStep)))
	b.WriteString("\n\n")

	switch m.createWizardStep {
	case 1:
		b.WriteString(m.wizardTypeView())
	case 2:
		b.WriteString(m.wizardBasicConfigView())
	case 3:
		b.WriteString(m.wizardRelationshipsView())
	case 4:
		b.WriteString(m.wizardWorktreeView())
	case 5:
		b.WriteString(m.wizardMethodView())
	case 6:
		if m.createWizardState.UseTemplate {
			b.WriteString(m.wizardTemplateVarsView())
		} else {
			b.WriteString(m.wizardTitleView())
		}
	case 7:
		b.WriteString(m.wizardDescriptionView())
	case 8:
		b.WriteString(m.wizardConfirmationView())
	}

	// Help footer
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(" [←] Back  [Enter] Continue  [Esc] Cancel"))

	return b.String()
}

func (m Model) wizardTypeView() string {
	var b strings.Builder

	b.WriteString("What type of item would you like to create?\n\n")

	types := []struct {
		name string
		desc string
	}{
		{"task", "Standard task (default)"},
		{"epic", "Large body of work with child tasks"},
	}

	for _, t := range types {
		cursor := "  "
		if m.createWizardState.SelectedType == model.ItemType(t.name) {
			cursor = "● "
		}
		b.WriteString(fmt.Sprintf("%s%s - %s\n", cursor, t.name, t.desc))
	}

	return b.String()
}

func (m Model) wizardBasicConfigView() string {
	var b strings.Builder

	b.WriteString("Configure basic settings:\n\n")

	// Priority selection
	b.WriteString("Priority:\n")
	priorities := []struct {
		level int
		desc  string
	}{
		{1, "High (P0)"},
		{2, "Medium (P1)"},
		{3, "Low (P2)"},
		{4, "Very Low"},
		{5, "Lowest"},
	}
	for _, p := range priorities {
		cursor := "  "
		if m.createWizardState.Priority == p.level {
			cursor = "● "
		}
		b.WriteString(fmt.Sprintf("%s%d - %s\n", cursor, p.level, p.desc))
	}

	b.WriteString("\n")

	// Project selection
	b.WriteString("Project:\n")
	if m.createWizardState.UseCustomProject {
		b.WriteString("  ○ Use current project\n")
		b.WriteString(fmt.Sprintf("  ● Custom: %s█\n", m.createWizardState.Project))
	} else {
		b.WriteString(fmt.Sprintf("  ● Use current project (%s)\n", m.project))
		b.WriteString("  ○ Enter custom project\n")
	}

	return b.String()
}

func (m Model) wizardRelationshipsView() string {
	var b strings.Builder

	b.WriteString("Set relationships (optional):\n\n")

	// Parent
	b.WriteString("Parent Epic:\n")
	if m.createWizardState.ParentID == "" {
		b.WriteString("  ● No parent (top-level item)\n")
	} else {
		b.WriteString(fmt.Sprintf("  ● Parent: %s\n", m.createWizardState.ParentID))
	}

	b.WriteString("\n")

	// Dependencies
	b.WriteString("Dependencies:\n")
	if len(m.createWizardState.Dependencies) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, dep := range m.createWizardState.Dependencies {
			b.WriteString(fmt.Sprintf("  • %s\n", dep))
		}
	}

	b.WriteString("\n")

	// Blockers
	b.WriteString("Blocks:\n")
	if len(m.createWizardState.Blockers) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, blocker := range m.createWizardState.Blockers {
			b.WriteString(fmt.Sprintf("  • %s\n", blocker))
		}
	}

	return b.String()
}

func (m Model) wizardWorktreeView() string {
	var b strings.Builder

	b.WriteString("Worktree Configuration (Epic only):\n\n")

	// Use worktree toggle
	if m.createWizardState.UseWorktree {
		b.WriteString("  ● Create worktree for this epic\n")
		b.WriteString("  ○ No worktree\n")
	} else {
		b.WriteString("  ○ Create worktree for this epic\n")
		b.WriteString("  ● No worktree\n")
	}

	b.WriteString("\n")

	if m.createWizardState.UseWorktree {
		b.WriteString(fmt.Sprintf("Branch: %s\n", m.createWizardState.WorktreeBranch))
		b.WriteString(fmt.Sprintf("Base:   %s\n", m.createWizardState.WorktreeBase))
	}

	return b.String()
}

func (m Model) wizardMethodView() string {
	var b strings.Builder

	b.WriteString("Choose creation method:\n\n")

	if m.createWizardState.UseTemplate {
		b.WriteString("  ○ From Scratch (Ad-hoc)\n")
		b.WriteString("  ● From Template\n")
	} else {
		b.WriteString("  ● From Scratch (Ad-hoc)\n")
		b.WriteString("  ○ From Template\n")
	}

	if m.createWizardState.UseTemplate {
		b.WriteString("\nAvailable Templates:\n")
		if len(m.templates) == 0 {
			b.WriteString("  (no templates loaded - press Enter to load)\n")
		} else {
			for _, t := range m.templates {
				cursor := "  "
				if m.createWizardState.SelectedTemplate != nil && m.createWizardState.SelectedTemplate.ID == t.ID {
					cursor = "● "
				}
				b.WriteString(fmt.Sprintf("%s%s - %s\n", cursor, t.ID, t.Title))
			}
		}
	}

	return b.String()
}

func (m Model) wizardTitleView() string {
	var b strings.Builder

	b.WriteString("Enter item details:\n\n")

	// Title
	b.WriteString("Title (required):\n")
	if m.createWizardState.Title == "" {
		b.WriteString("  [Enter title]█\n")
	} else {
		b.WriteString(fmt.Sprintf("  %s█\n", m.createWizardState.Title))
	}

	b.WriteString("\n")

	// Labels
	b.WriteString("Labels (comma-separated, optional):\n")
	if len(m.createWizardState.Labels) == 0 {
		b.WriteString("  [none]\n")
	} else {
		b.WriteString(fmt.Sprintf("  %s\n", strings.Join(m.createWizardState.Labels, ", ")))
	}

	return b.String()
}

func (m Model) wizardTemplateVarsView() string {
	var b strings.Builder

	if m.createWizardState.SelectedTemplate == nil {
		b.WriteString("No template selected. Press Enter to go back.\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("Template: %s\n\n", m.createWizardState.SelectedTemplate.Title))

	// Show template variables
	if len(m.createWizardState.SelectedTemplate.Variables) > 0 {
		b.WriteString("Template Variables:\n")
		for name, v := range m.createWizardState.SelectedTemplate.Variables {
			optional := ""
			if v.Optional {
				optional = " (optional)"
			}
			value := m.createWizardState.TemplateVars[name]
			if value == "" {
				value = "[not set]"
			}
			b.WriteString(fmt.Sprintf("  %s%s: %s\n", name, optional, value))
		}
	}

	return b.String()
}

func (m Model) wizardDescriptionView() string {
	var b strings.Builder

	b.WriteString("Enter description (required):\n\n")

	if m.createWizardState.Description == "" {
		b.WriteString("  [Enter detailed description...]\n")
	} else {
		// Show first few lines of description
		lines := strings.Split(m.createWizardState.Description, "\n")
		for i, line := range lines {
			if i >= 10 {
				b.WriteString("  ...\n")
				break
			}
			b.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Description must be at least 3 words or 20 characters.\n"))

	return b.String()
}

func (m Model) wizardConfirmationView() string {
	var b strings.Builder

	state := m.createWizardState

	b.WriteString("Review and confirm:\n\n")

	// Basic info
	b.WriteString("Basic Info:\n")
	b.WriteString(fmt.Sprintf("  Type:     %s\n", state.SelectedType))
	b.WriteString(fmt.Sprintf("  Title:    %s\n", state.Title))
	b.WriteString(fmt.Sprintf("  Priority: %d\n", state.Priority))
	b.WriteString(fmt.Sprintf("  Project:  %s\n", state.Project))

	if len(state.Labels) > 0 {
		b.WriteString(fmt.Sprintf("  Labels:   %s\n", strings.Join(state.Labels, ", ")))
	}

	b.WriteString("\n")

	// Relationships
	if state.ParentID != "" || len(state.Dependencies) > 0 || len(state.Blockers) > 0 {
		b.WriteString("Relationships:\n")
		if state.ParentID != "" {
			b.WriteString(fmt.Sprintf("  Parent: %s\n", state.ParentID))
		}
		if len(state.Dependencies) > 0 {
			b.WriteString(fmt.Sprintf("  Depends on: %s\n", strings.Join(state.Dependencies, ", ")))
		}
		if len(state.Blockers) > 0 {
			b.WriteString(fmt.Sprintf("  Blocks: %s\n", strings.Join(state.Blockers, ", ")))
		}
		b.WriteString("\n")
	}

	// Worktree
	if state.SelectedType == model.ItemTypeEpic && state.UseWorktree {
		b.WriteString("Worktree:\n")
		b.WriteString(fmt.Sprintf("  Branch: %s\n", state.WorktreeBranch))
		b.WriteString(fmt.Sprintf("  Base:   %s\n", state.WorktreeBase))
		b.WriteString("\n")
	}

	// Description preview
	desc := state.Description
	if len(desc) > 200 {
		desc = desc[:200] + "..."
	}
	b.WriteString("Description Preview:\n")
	b.WriteString(fmt.Sprintf("  %s\n", desc))

	b.WriteString("\n")
	b.WriteString("Press Enter to create this item.\n")

	return b.String()
}

// Run starts the TUI with the given project filter.
func Run(database *db.DB, project string) error {
	m := New(database, project)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
