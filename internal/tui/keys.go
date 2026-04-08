package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
)

type helpKeyMap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (k helpKeyMap) ShortHelp() []key.Binding  { return k.short }
func (k helpKeyMap) FullHelp() [][]key.Binding { return k.full }

var appBindings = struct {
	Quit       key.Binding
	Back       key.Binding
	ToggleHelp key.Binding
}{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "h", "backspace"),
		key.WithHelp("esc", "back"),
	),
	ToggleHelp: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}

var listBindings = struct {
	Up             key.Binding
	Down           key.Binding
	HalfPageUp     key.Binding
	HalfPageDown   key.Binding
	PageUp         key.Binding
	PageDown       key.Binding
	Top            key.Binding
	End            key.Binding
	Expand         key.Binding
	Collapse       key.Binding
	Detail         key.Binding
	SelectMode     key.Binding
	ToggleSelected key.Binding
	BatchStatus    key.Binding
	BatchPriority  key.Binding
	BatchDone      key.Binding
	Start          key.Binding
	Done           key.Binding
	Project        key.Binding
	Block          key.Binding
	Log            key.Binding
	Cancel         key.Binding
	Delete         key.Binding
	Search         key.Binding
	Label          key.Binding
	Ready          key.Binding
	StatusOpen     key.Binding
	StatusProgress key.Binding
	StatusBlocked  key.Binding
	StatusDone     key.Binding
	StatusCanceled key.Binding
	StatusAll      key.Binding
	ClearFilters   key.Binding
	Refresh        key.Binding
	AddDep         key.Binding
	New            key.Binding
	Templates      key.Binding
	Config         key.Binding
}{
	Up:             key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:           key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	HalfPageUp:     key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half up")),
	HalfPageDown:   key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half down")),
	PageUp:         key.NewBinding(key.WithKeys("pgup", "ctrl+b"), key.WithHelp("pgup", "page up")),
	PageDown:       key.NewBinding(key.WithKeys("pgdown", "ctrl+f"), key.WithHelp("pgdn", "page down")),
	Top:            key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	End:            key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "end")),
	Expand:         key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "expand")),
	Collapse:       key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "collapse")),
	Detail:         key.NewBinding(key.WithKeys("enter", "l"), key.WithHelp("enter", "detail")),
	SelectMode:     key.NewBinding(key.WithKeys("ctrl+v"), key.WithHelp("ctrl+v", "select")),
	ToggleSelected: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
	BatchStatus:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "batch status")),
	BatchPriority:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "batch priority")),
	BatchDone:      key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "batch done")),
	Start:          key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
	Done:           key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "done")),
	Project:        key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "project")),
	Block:          key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "block")),
	Log:            key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "log")),
	Cancel:         key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "cancel")),
	Delete:         key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete")),
	Search:         key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Label:          key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "label")),
	Ready:          key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "ready")),
	StatusOpen:     key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "open")),
	StatusProgress: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "in progress")),
	StatusBlocked:  key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "blocked")),
	StatusDone:     key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "done")),
	StatusCanceled: key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "canceled")),
	StatusAll:      key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "all")),
	ClearFilters:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear filters")),
	Refresh:        key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	AddDep:         key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add blocker")),
	New:            key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	Templates:      key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "templates")),
	Config:         key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "config")),
}

var detailBindings = struct {
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Top          key.Binding
	End          key.Binding
	ToggleLogs   key.Binding
	DepNav       key.Binding
	Jump         key.Binding
	Start        key.Binding
	Done         key.Binding
	Block        key.Binding
	Log          key.Binding
	Cancel       key.Binding
	AddDep       key.Binding
	Edit         key.Binding
	Variables    key.Binding
	Refresh      key.Binding
	Graph        key.Binding
	Rerender     key.Binding
}{
	Up:           key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "scroll up")),
	Down:         key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "scroll down")),
	PageUp:       key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
	PageDown:     key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
	HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half up")),
	HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half down")),
	Top:          key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
	End:          key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
	ToggleLogs:   key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "logs")),
	DepNav:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "deps")),
	Jump:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "jump")),
	Start:        key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
	Done:         key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "done")),
	Block:        key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "block")),
	Log:          key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "log")),
	Cancel:       key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "cancel")),
	AddDep:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add blocker")),
	Edit:         key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Variables:    key.NewBinding(key.WithKeys("V"), key.WithHelp("V", "toggle")),
	Refresh:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Graph:        key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "graph")),
	Rerender:     key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "rerender")),
}

var graphBindings = struct {
	Up   key.Binding
	Down key.Binding
	Jump key.Binding
}{
	Up:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	Jump: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "jump")),
}

var templateListBindings = struct {
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Top          key.Binding
	End          key.Binding
	Detail       key.Binding
	Refresh      key.Binding
}{
	Up:           key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down:         key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	PageUp:       key.NewBinding(key.WithKeys("pgup", "ctrl+b"), key.WithHelp("pgup", "page up")),
	PageDown:     key.NewBinding(key.WithKeys("pgdown", "ctrl+f"), key.WithHelp("pgdn", "page down")),
	HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half up")),
	HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half down")),
	Top:          key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	End:          key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "end")),
	Detail:       key.NewBinding(key.WithKeys("enter", "l"), key.WithHelp("enter", "detail")),
	Refresh:      key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
}

var templateDetailBindings = struct {
	Up           key.Binding
	Down         key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Top          key.Binding
	End          key.Binding
}{
	Up:           key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down:         key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	PageUp:       key.NewBinding(key.WithKeys("pgup", "ctrl+b"), key.WithHelp("pgup", "page up")),
	PageDown:     key.NewBinding(key.WithKeys("pgdown", "ctrl+f"), key.WithHelp("pgdn", "page down")),
	HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u"), key.WithHelp("ctrl+u", "half up")),
	HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "half down")),
	Top:          key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
	End:          key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
}

var configBindings = struct {
	Up      key.Binding
	Down    key.Binding
	Top     key.Binding
	End     key.Binding
	Edit    key.Binding
	Refresh key.Binding
}{
	Up:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	Top:     key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	End:     key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "end")),
	Edit:    key.NewBinding(key.WithKeys("enter", "e"), key.WithHelp("enter", "edit")),
	Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
}

var configEditBindings = struct {
	Save   key.Binding
	Cancel key.Binding
}{
	Save:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "save")),
	Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}

var textareaBindings = struct {
	Save           key.Binding
	Cancel         key.Binding
	ExternalEditor key.Binding
}{
	Save:           key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
	Cancel:         key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	ExternalEditor: key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "external editor")),
}

var statusMenuBindings = struct {
	Up      key.Binding
	Down    key.Binding
	Confirm key.Binding
	Cancel  key.Binding
	Start   key.Binding
	Done    key.Binding
	Block   key.Binding
	Stop    key.Binding
}{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Confirm: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
	Cancel:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
	Start:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start")),
	Done:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "done")),
	Block:   key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "block")),
	Stop:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "cancel item")),
}

var variablePickerBindings = struct {
	Up   key.Binding
	Down key.Binding
	Edit key.Binding
}{
	Up:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
	Down: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
	Edit: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit")),
}

var wizardTypeBindings = struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Cancel key.Binding
}{
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Cancel: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}

var wizardTitleBindings = struct {
	Continue key.Binding
	Cancel   key.Binding
}{
	Continue: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "continue")),
	Cancel:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}

var wizardWorktreeBindings = struct {
	Up       key.Binding
	Down     key.Binding
	Switch   key.Binding
	Continue key.Binding
	Cancel   key.Binding
}{
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "toggle")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "toggle")),
	Switch:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "field")),
	Continue: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "continue")),
	Cancel:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
}

var wizardDescriptionBindings = struct {
	Continue key.Binding
	Cancel   key.Binding
}{
	Continue: key.NewBinding(key.WithKeys("ctrl+s", "ctrl+enter"), key.WithHelp("ctrl+s/ctrl+enter", "save")),
	Cancel:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
}

func (m Model) canToggleHelp() bool {
	if m.inputMode == InputTextarea {
		return false
	}
	if m.inputMode != InputNone && m.inputMode != InputStatusMenu {
		return false
	}
	if m.configEditing {
		return false
	}
	if m.viewMode == ViewCreateWizard {
		if m.createWizardStep == 2 || m.isWizardDescriptionStep() {
			return false
		}
		if m.createWizardStep == 3 && m.createWizardState.SelectedType == model.ItemTypeEpic && m.createWizardState.UseWorktree {
			return false
		}
	}
	return true
}

func (m Model) toggleHelpBinding() key.Binding {
	b := appBindings.ToggleHelp
	b.SetEnabled(m.canToggleHelp())
	return b
}

func enabledBinding(binding key.Binding, enabled bool) key.Binding {
	b := binding
	b.SetEnabled(enabled)
	return b
}

func (m Model) currentHelpKeyMap() help.KeyMap {
	if m.inputMode == InputTextarea {
		return helpKeyMap{
			short: []key.Binding{textareaBindings.Save, textareaBindings.Cancel, textareaBindings.ExternalEditor},
			full:  [][]key.Binding{{textareaBindings.Save, textareaBindings.ExternalEditor}, {textareaBindings.Cancel}},
		}
	}
	if m.inputMode == InputStatusMenu {
		return helpKeyMap{
			short: []key.Binding{statusMenuBindings.Up, statusMenuBindings.Down, statusMenuBindings.Confirm, statusMenuBindings.Cancel, m.toggleHelpBinding()},
			full:  [][]key.Binding{{statusMenuBindings.Up, statusMenuBindings.Down, statusMenuBindings.Confirm}, {statusMenuBindings.Start, statusMenuBindings.Done, statusMenuBindings.Block, statusMenuBindings.Stop}, {statusMenuBindings.Cancel, m.toggleHelpBinding()}},
		}
	}
	if m.configEditing {
		return helpKeyMap{
			short: []key.Binding{configEditBindings.Save, configEditBindings.Cancel},
			full:  [][]key.Binding{{configEditBindings.Save, configEditBindings.Cancel}},
		}
	}
	if m.viewMode == ViewCreateWizard {
		switch {
		case m.createWizardStep == 1:
			return helpKeyMap{
				short: []key.Binding{wizardTypeBindings.Up, wizardTypeBindings.Down, wizardTypeBindings.Select, wizardTypeBindings.Cancel, m.toggleHelpBinding()},
				full:  [][]key.Binding{{wizardTypeBindings.Up, wizardTypeBindings.Down, wizardTypeBindings.Select}, {wizardTypeBindings.Cancel, m.toggleHelpBinding()}},
			}
		case m.createWizardStep == 2:
			return helpKeyMap{
				short: []key.Binding{wizardTitleBindings.Continue, wizardTitleBindings.Cancel},
				full:  [][]key.Binding{{wizardTitleBindings.Continue, wizardTitleBindings.Cancel}},
			}
		case m.isWizardDescriptionStep():
			return helpKeyMap{
				short: []key.Binding{wizardDescriptionBindings.Continue, wizardDescriptionBindings.Cancel},
				full:  [][]key.Binding{{wizardDescriptionBindings.Continue, wizardDescriptionBindings.Cancel}},
			}
		default:
			switchField := enabledBinding(wizardWorktreeBindings.Switch, m.createWizardState.UseWorktree)
			return helpKeyMap{
				short: []key.Binding{wizardWorktreeBindings.Up, wizardWorktreeBindings.Down, switchField, wizardWorktreeBindings.Continue, wizardWorktreeBindings.Cancel, m.toggleHelpBinding()},
				full:  [][]key.Binding{{wizardWorktreeBindings.Up, wizardWorktreeBindings.Down, switchField}, {wizardWorktreeBindings.Continue, wizardWorktreeBindings.Cancel, m.toggleHelpBinding()}},
			}
		}
	}

	switch m.viewMode {
	case ViewList:
		if m.selectMode {
			return helpKeyMap{
				short: []key.Binding{listBindings.Up, listBindings.Down, listBindings.ToggleSelected, listBindings.BatchStatus, listBindings.BatchPriority, listBindings.BatchDone, listBindings.SelectMode, listBindings.ClearFilters, appBindings.Quit, m.toggleHelpBinding()},
				full: [][]key.Binding{
					{listBindings.Up, listBindings.Down, listBindings.HalfPageUp, listBindings.HalfPageDown, listBindings.PageUp, listBindings.PageDown, listBindings.Top, listBindings.End},
					{listBindings.ToggleSelected, listBindings.BatchStatus, listBindings.BatchPriority, listBindings.BatchDone, listBindings.SelectMode},
					{listBindings.Search, listBindings.Label, listBindings.Ready, listBindings.StatusOpen, listBindings.StatusProgress, listBindings.StatusBlocked, listBindings.StatusDone, listBindings.StatusCanceled, listBindings.StatusAll, listBindings.ClearFilters},
					{listBindings.Refresh, appBindings.Quit, m.toggleHelpBinding()},
				},
			}
		}
		return helpKeyMap{
			short: []key.Binding{listBindings.Up, listBindings.Down, listBindings.HalfPageUp, listBindings.HalfPageDown, listBindings.PageUp, listBindings.PageDown, listBindings.Top, listBindings.End, listBindings.Detail, listBindings.Start, listBindings.Done, listBindings.New, listBindings.Search, listBindings.Project, listBindings.Label, listBindings.Ready, listBindings.Block, listBindings.Log, listBindings.Cancel, listBindings.Delete, listBindings.Templates, listBindings.Config, listBindings.Refresh, appBindings.Quit, m.toggleHelpBinding()},
			full: [][]key.Binding{
				{listBindings.Up, listBindings.Down, listBindings.HalfPageUp, listBindings.HalfPageDown, listBindings.PageUp, listBindings.PageDown, listBindings.Top, listBindings.End, listBindings.Expand, listBindings.Collapse, listBindings.Detail},
				{listBindings.Start, listBindings.Done, listBindings.Block, listBindings.Log, listBindings.Cancel, listBindings.Delete, listBindings.AddDep, listBindings.New, listBindings.SelectMode},
				{listBindings.Search, listBindings.Project, listBindings.Label, listBindings.Ready, listBindings.StatusOpen, listBindings.StatusProgress, listBindings.StatusBlocked, listBindings.StatusDone, listBindings.StatusCanceled, listBindings.StatusAll, listBindings.ClearFilters},
				{listBindings.Templates, listBindings.Config, listBindings.Refresh, appBindings.Quit, m.toggleHelpBinding()},
			},
		}
	case ViewDetail:
		hasDeps := len(m.detailDeps) > 0 || len(m.detailBlocks) > 0
		hasVars := false
		hasTemplate := false
		treeNodes := m.buildTree()
		if len(treeNodes) > 0 && m.cursor < len(treeNodes) {
			item := treeNodes[m.cursor].Item
			hasTemplate = item.TemplateID != ""
			hasVars = item.TemplateID != "" && len(item.TemplateVars) > 0
		}
		return helpKeyMap{
			short: []key.Binding{appBindings.Back, detailBindings.Start, detailBindings.Done, detailBindings.Block, detailBindings.Log, detailBindings.Cancel, detailBindings.AddDep, detailBindings.Edit, detailBindings.ToggleLogs, detailBindings.Down, detailBindings.Up, detailBindings.PageDown, detailBindings.PageUp, detailBindings.Top, detailBindings.End, enabledBinding(detailBindings.DepNav, hasDeps), enabledBinding(detailBindings.Variables, hasVars), detailBindings.Graph, appBindings.Quit, m.toggleHelpBinding()},
			full: [][]key.Binding{
				{detailBindings.Up, detailBindings.Down, detailBindings.PageUp, detailBindings.PageDown, detailBindings.HalfPageUp, detailBindings.HalfPageDown, detailBindings.Top, detailBindings.End},
				{detailBindings.Start, detailBindings.Done, detailBindings.Block, detailBindings.Log, detailBindings.Cancel, detailBindings.AddDep, detailBindings.Edit, detailBindings.ToggleLogs, detailBindings.Graph, detailBindings.Refresh, enabledBinding(detailBindings.Rerender, hasTemplate)},
				{enabledBinding(detailBindings.DepNav, hasDeps), enabledBinding(detailBindings.Jump, hasDeps), enabledBinding(detailBindings.Variables, hasVars)},
				{appBindings.Back, appBindings.Quit, m.toggleHelpBinding()},
			},
		}
	case ViewGraph:
		return helpKeyMap{
			short: []key.Binding{graphBindings.Up, graphBindings.Down, graphBindings.Jump, appBindings.Back, appBindings.Quit, m.toggleHelpBinding()},
			full:  [][]key.Binding{{graphBindings.Up, graphBindings.Down, graphBindings.Jump}, {appBindings.Back, appBindings.Quit, m.toggleHelpBinding()}},
		}
	case ViewTemplateList:
		return helpKeyMap{
			short: []key.Binding{templateListBindings.Up, templateListBindings.Down, templateListBindings.Detail, templateListBindings.Refresh, appBindings.Back, appBindings.Quit, m.toggleHelpBinding()},
			full:  [][]key.Binding{{templateListBindings.Up, templateListBindings.Down, templateListBindings.PageUp, templateListBindings.PageDown, templateListBindings.HalfPageUp, templateListBindings.HalfPageDown, templateListBindings.Top, templateListBindings.End}, {templateListBindings.Detail, templateListBindings.Refresh}, {appBindings.Back, appBindings.Quit, m.toggleHelpBinding()}},
		}
	case ViewTemplateDetail:
		return helpKeyMap{
			short: []key.Binding{templateDetailBindings.Up, templateDetailBindings.Down, templateDetailBindings.PageUp, templateDetailBindings.PageDown, templateDetailBindings.Top, templateDetailBindings.End, appBindings.Back, appBindings.Quit, m.toggleHelpBinding()},
			full:  [][]key.Binding{{templateDetailBindings.Up, templateDetailBindings.Down, templateDetailBindings.PageUp, templateDetailBindings.PageDown, templateDetailBindings.HalfPageUp, templateDetailBindings.HalfPageDown, templateDetailBindings.Top, templateDetailBindings.End}, {appBindings.Back, appBindings.Quit, m.toggleHelpBinding()}},
		}
	case ViewConfig:
		return helpKeyMap{
			short: []key.Binding{configBindings.Up, configBindings.Down, configBindings.Edit, configBindings.Refresh, appBindings.Back, appBindings.Quit, m.toggleHelpBinding()},
			full:  [][]key.Binding{{configBindings.Up, configBindings.Down, configBindings.Top, configBindings.End}, {configBindings.Edit, configBindings.Refresh}, {appBindings.Back, appBindings.Quit, m.toggleHelpBinding()}},
		}
	case ViewVariablePicker:
		return helpKeyMap{
			short: []key.Binding{variablePickerBindings.Up, variablePickerBindings.Down, variablePickerBindings.Edit, appBindings.Back, m.toggleHelpBinding()},
			full:  [][]key.Binding{{variablePickerBindings.Up, variablePickerBindings.Down, variablePickerBindings.Edit}, {appBindings.Back, appBindings.Quit, m.toggleHelpBinding()}},
		}
	default:
		return nil
	}
}

func (m Model) helpView() string {
	return m.helpViewWidth(max(0, m.width-(contentPadding*2)))
}

func (m Model) helpViewWidth(width int) string {
	keyMap := m.currentHelpKeyMap()
	if keyMap == nil {
		return ""
	}
	helpModel := m.help
	helpModel.Width = width
	return helpModel.View(keyMap)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, appBindings.ToggleHelp) && m.canToggleHelp() {
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	}

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
	case ViewVariablePicker:
		return m.handleVariablePickerKey(msg)
	}
	return m, nil
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	active := m.activePromptInput()
	if active == nil {
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.restorePromptInput()
		m.blurPromptInputs()
		m.inputMode = InputNone
		m.inputContext = ""
		return m, nil

	case "enter":
		return m.submitInput()

	case "tab":
		if m.inputMode == InputSearch || m.inputMode == InputProject || m.inputMode == InputLabel {
			return m.cycleFilterInput()
		}

	}

	prev := active.Value()
	var cmd tea.Cmd
	*active, cmd = active.Update(msg)
	if active.Value() != prev {
		m.syncPromptValue()
	}
	return m, cmd
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
	text := m.activePromptValue()
	mode := m.inputMode
	m.blurPromptInputs()
	m.inputMode = InputNone

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
		m.promptInput.Reset()
		m.inputOriginal = ""
		return m, m.focusPromptInput(InputCreateType)

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
			if err := m.db.UpdateStatus(item.ID, model.StatusBlocked, db.AgentContext{}, false); err != nil {
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
			if err := m.db.UpdateStatus(item.ID, model.StatusCanceled, db.AgentContext{}, false); err != nil {
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

func (m Model) startInput(mode InputMode, label string) (Model, tea.Cmd) {
	m.inputMode = mode
	m.inputLabel = label
	m.inputOriginal = m.promptInitialValue(mode)
	m.setPromptValue(mode, m.inputOriginal)
	return m, m.focusPromptInput(mode)
}

func (m *Model) activePromptInput() *textinput.Model {
	switch m.inputMode {
	case InputSearch:
		return &m.searchInput
	case InputProject:
		return &m.projectInput
	case InputLabel:
		return &m.labelInput
	case InputBlock, InputLog, InputCancel, InputAddDep, InputCreate, InputCreateType, InputBatchStatus, InputBatchPriority:
		return &m.promptInput
	default:
		return nil
	}
}

func (m Model) activePromptValue() string {
	if active := m.activePromptInput(); active != nil {
		return active.Value()
	}
	return ""
}

func (m *Model) setPromptValue(mode InputMode, value string) {
	var input *textinput.Model
	switch mode {
	case InputSearch:
		input = &m.searchInput
	case InputProject:
		input = &m.projectInput
	case InputLabel:
		input = &m.labelInput
	default:
		input = &m.promptInput
	}
	input.SetValue(value)
	input.CursorEnd()
	m.syncPromptValue()
}

func (m *Model) focusPromptInput(_ InputMode) tea.Cmd {
	m.blurPromptInputs()
	if active := m.activePromptInput(); active != nil {
		active.Width = max(20, m.width-(contentPadding*2)-len(m.inputLabel)-4)
		return active.Focus()
	}
	return nil
}

func (m *Model) blurPromptInputs() {
	m.promptInput.Blur()
	m.searchInput.Blur()
	m.projectInput.Blur()
	m.labelInput.Blur()
}

func (m *Model) syncPromptValue() {
	switch m.inputMode {
	case InputSearch:
		m.filterSearch = m.searchInput.Value()
		m.applyFilters()
	case InputProject:
		m.filterProject = m.projectInput.Value()
		m.applyFilters()
	case InputLabel:
		m.filterLabel = m.labelInput.Value()
		m.applyFilters()
	}
}

func (m *Model) restorePromptInput() {
	switch m.inputMode {
	case InputSearch:
		m.searchInput.SetValue(m.inputOriginal)
		m.filterSearch = m.inputOriginal
		m.applyFilters()
	case InputProject:
		m.projectInput.SetValue(m.inputOriginal)
		m.filterProject = m.inputOriginal
		m.applyFilters()
	case InputLabel:
		m.labelInput.SetValue(m.inputOriginal)
		m.filterLabel = m.inputOriginal
		m.applyFilters()
	default:
		m.promptInput.SetValue(m.inputOriginal)
	}
	if active := m.activePromptInput(); active != nil {
		active.CursorEnd()
	}
}

func (m Model) promptInitialValue(mode InputMode) string {
	switch mode {
	case InputSearch:
		return m.filterSearch
	case InputProject:
		return m.filterProject
	case InputLabel:
		return m.filterLabel
	default:
		return ""
	}
}

func (m Model) cycleFilterInput() (tea.Model, tea.Cmd) {
	if m.inputMode != InputSearch && m.inputMode != InputProject && m.inputMode != InputLabel {
		return m, nil
	}
	current := m.activePromptValue()
	original := m.inputOriginal
	mode := InputSearch
	switch m.inputMode {
	case InputSearch:
		mode = InputProject
		m.searchInput.SetValue(current)
	case InputProject:
		mode = InputLabel
		m.projectInput.SetValue(current)
	case InputLabel:
		mode = InputSearch
		m.labelInput.SetValue(current)
	}
	m.inputMode = mode
	m.inputLabel = filterInputLabel(mode)
	m.inputOriginal = original
	return m, m.focusPromptInput(mode)
}

func filterInputLabel(mode InputMode) string {
	switch mode {
	case InputSearch:
		return "Search: "
	case InputProject:
		return "Project: "
	case InputLabel:
		return "Label: "
	default:
		return ""
	}
}

func (m Model) promptOverlayView() string {
	active := m.activePromptInput()
	if active == nil {
		return inputStyle.Render(m.inputLabel)
	}
	input := *active
	input.Width = max(20, m.width-(contentPadding*2)-len(m.inputLabel)-4)
	return inputStyle.Render(m.inputLabel) + input.View()
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
	if width < 20 {
		width = 20
	}
	height := m.height - 10
	if height < 3 {
		height = 3
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
		if item.TemplateID != "" {
			m.err = fmt.Errorf("cannot edit description on template-backed task: descriptions are generated from template variables. Press 'V' to edit variables, or 'R' to re-render from template")
			return m, nil
		}
		return m, func() tea.Msg {
			if err := m.db.SetDescription(item.ID, content); err != nil {
				return actionMsg{err: fmt.Errorf("failed to save description: %w", err)}
			}
			return actionMsg{message: fmt.Sprintf("Updated description for %s", item.ID)}
		}
	}

	// Handle template variable editing (var:<name>)
	if varName, ok := strings.CutPrefix(target, "var:"); ok {
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

	// Check if trying to edit description on template task
	if target == "description" && item.TemplateID != "" {
		m.err = fmt.Errorf("cannot edit description on template-backed task: descriptions are generated from template variables. Press 'V' to edit variables, or 'R' to re-render from template")
		m.inputMode = InputNone
		m.textarea.Blur()
		return m, nil
	}

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
	if item.Status == model.StatusInProgress {
		m.message = fmt.Sprintf("%s is already in progress (use CLI with --resume to take over)", item.ID)
		return m, nil
	}
	if item.Status != model.StatusOpen && item.Status != model.StatusBlocked {
		m.message = "Can only start open or blocked items"
		return m, nil
	}
	return m, func() tea.Msg {
		if err := m.db.UpdateStatus(item.ID, model.StatusInProgress, db.AgentContext{}, false); err != nil {
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
		if err := m.db.UpdateStatus(item.ID, model.StatusDone, db.AgentContext{}, false); err != nil {
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
		if err := m.db.DeleteItem(item.ID, false, false); err != nil {
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
			if err := m.db.UpdateStatus(id, model.StatusDone, db.AgentContext{}, false); err != nil {
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
			if err := m.db.UpdateStatus(id, status, db.AgentContext{}, false); err != nil {
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
	return m, tea.Batch(m.loadItemsPreserving(msg.itemID), m.loadDetail())
}
