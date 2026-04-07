package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
)

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
		currentItemID := item.ID
		for {
			epicInfo, err := m.db.CheckParentEpicCompletion(currentItemID)
			if err != nil || epicInfo == nil {
				break
			}
			if err := m.db.AutoCompleteEpic(epicInfo.Epic.ID); err != nil {
				return actionMsg{err: err}
			}
			currentItemID = epicInfo.Epic.ID
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
			currentItemID := id
			for {
				epicInfo, err := m.db.CheckParentEpicCompletion(currentItemID)
				if err != nil || epicInfo == nil {
					break
				}
				if err := m.db.AutoCompleteEpic(epicInfo.Epic.ID); err != nil {
					return actionMsg{err: err}
				}
				currentItemID = epicInfo.Epic.ID
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
