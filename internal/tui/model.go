// Package tui provides an interactive terminal UI for tpg using Bubble Tea.
package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
	"github.com/taxilian/tpg/internal/templates"
	"time"
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
	ViewVariablePicker
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
	db                     *db.DB
	project                string       // current project (for default filtering)
	items                  []model.Item // all items from db
	filtered               []model.Item // items after filtering
	cursor                 int
	detailViewport         viewport.Model
	templateDetailViewport viewport.Model
	configViewport         viewport.Model
	varPickerViewport      viewport.Model
	listScroll             int // scroll position for list view
	viewMode               ViewMode

	// Auto-refresh tracking: skip scroll sync to preserve manual scroll position
	skipScrollSync bool
	prevCursor     int // Track if cursor position changed to avoid unnecessary scroll recalc

	// Filter state
	filterProject  string
	filterStatuses map[model.Status]bool // which statuses to show
	filterSearch   string
	filterLabel    string // label filter (partial match, like search)

	// Input state
	inputMode    InputMode
	inputLabel   string
	inputContext string // For multi-step inputs (e.g., storing title before asking for type)

	// UI state
	width   int
	height  int
	err     error
	message string // temporary status message

	// Detail view state
	detailID     string
	detailLogs   []model.Log
	detailDeps   []db.DepStatus // "depends on" (blockers)
	detailBlocks []db.DepStatus // "blocks" (what this item blocks)
	logsVisible  bool
	depCursor    int // cursor within deps for navigation
	depSection   int // 0 = "blocked by", 1 = "blocks"
	depNavActive bool

	// Stale tracking
	staleItems map[string]bool // item IDs that are stale (no updates > 5 min)

	// Ready filter cache
	filterReady bool            // whether ready filter is active
	readyIDs    map[string]bool // cached set of ready item IDs

	// Template browser state
	templates        []*templates.Template
	templateCursor   int
	templateScroll   int
	selectedTemplate *templates.Template

	// Selection mode state
	selectMode    bool
	selectedItems map[string]bool // item ID -> selected

	// Graph view state
	graphNodes     []graphNode
	graphCursor    int
	graphCurrentID string // ID of the center task in graph view

	// Template variable expansion state (for detail view)
	varExpanded     map[string]bool
	varCursor       int // which variable is selected for editing (-1 = none)
	varPickerScroll int // scroll position for variable picker

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
	configScroll  int
	configEditing bool

	// Tree view state
	treeExpanded map[string]bool // item ID -> expanded state

	// Create wizard state
	createWizardStep  int
	createWizardState CreateWizardState

	promptInput       textinput.Model
	searchInput       textinput.Model
	projectInput      textinput.Model
	labelInput        textinput.Model
	configInput       textinput.Model
	wizardTitleInput  textinput.Model
	wizardBranchInput textinput.Model
	wizardBaseInput   textinput.Model
	inputOriginal     string
}

// CreateWizardState holds all data during item creation
type CreateWizardState struct {
	// Step 1: Type (popup)
	SelectedType model.ItemType
	TypeCursor   int

	// Step 2: Title
	Title string

	// Step 3 (epic only): Worktree
	UseWorktree        bool
	WorktreeBranch     string
	WorktreeBase       string
	WorktreeField      int  // 0 = branch, 1 = base
	WorktreeBranchAuto bool // true when auto-generated from title

	// Step 3/4: Description
	Description string
}

// TypeOption represents an available item type with metadata.
type TypeOption struct {
	Type   model.ItemType
	Prefix string // e.g., "ts", "ep", "bg"
	Desc   string // optional description
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

	promptInput := newTextInput("")
	searchInput := newTextInput("Search")
	projectInput := newTextInput("Project")
	labelInput := newTextInput("Label")
	configInput := newTextInput("")
	wizardTitleInput := newTextInput("Enter title")
	wizardBranchInput := newTextInput("feature/ep-xxx-title")
	wizardBaseInput := newTextInput("main")

	return Model{
		db:                     database,
		project:                project,
		viewMode:               ViewList,
		filterStatuses:         statuses,
		staleItems:             make(map[string]bool),
		selectedItems:          make(map[string]bool),
		varExpanded:            make(map[string]bool),
		detailViewport:         newViewportModel(),
		templateDetailViewport: newViewportModel(),
		configViewport:         newViewportModel(),
		varPickerViewport:      newViewportModel(),
		textarea:               ta,
		treeExpanded:           make(map[string]bool),
		createWizardStep:       0, // 0 = not in wizard
		createWizardState: CreateWizardState{
			SelectedType: model.ItemTypeTask,
			TypeCursor:   0,
		},
		promptInput:       promptInput,
		searchInput:       searchInput,
		projectInput:      projectInput,
		labelInput:        labelInput,
		configInput:       configInput,
		wizardTitleInput:  wizardTitleInput,
		wizardBranchInput: wizardBranchInput,
		wizardBaseInput:   wizardBaseInput,
	}
}

// Refresh interval for auto-refresh
const refreshInterval = 5 * time.Second

// Messages
type tickMsg time.Time

type itemsMsg struct {
	items      []model.Item
	err        error
	preserveID string // ID to preserve cursor position on
}

type detailMsg struct {
	itemID string
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

type readyIDsMsg struct {
	ids map[string]bool
	err error
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
