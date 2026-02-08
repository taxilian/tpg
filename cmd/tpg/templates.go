package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
	"github.com/taxilian/tpg/internal/templates"
	"gopkg.in/yaml.v3"
)

type templateCache struct {
	templates map[string]*templates.Template
}

func (c *templateCache) get(id string) (*templates.Template, error) {
	if c.templates == nil {
		c.templates = map[string]*templates.Template{}
	}
	if tmpl, ok := c.templates[id]; ok {
		return tmpl, nil
	}
	tmpl, err := templates.LoadTemplate(id)
	if err != nil {
		return nil, err
	}
	c.templates[id] = tmpl
	return tmpl, nil
}

func parseTemplateVars(pairs []string) (map[string]string, error) {
	vars := map[string]string{}
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid variable format: %s (expected name=json-string)", pair)
		}
		name := strings.TrimSpace(parts[0])
		if name == "" {
			return nil, fmt.Errorf("variable name cannot be empty")
		}
		var value string
		if err := json.Unmarshal([]byte(parts[1]), &value); err != nil {
			return nil, fmt.Errorf("invalid JSON string for %s", name)
		}
		vars[name] = value
	}
	return vars, nil
}

// parseTemplateVarsYAML parses YAML from stdin and returns varPairs in name=json-string format
func parseTemplateVarsYAML(data []byte) ([]string, error) {
	var varsMap map[string]interface{}
	if err := yaml.Unmarshal(data, &varsMap); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	var pairs []string
	for name, value := range varsMap {
		// Convert value to string
		var strValue string
		switch v := value.(type) {
		case string:
			strValue = v
		case nil:
			strValue = ""
		default:
			// For other types, marshal to JSON and back to get string representation
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("failed to convert %s value: %w", name, err)
			}
			strValue = string(jsonBytes)
			// If it's a quoted string, unquote it
			if len(strValue) >= 2 && strValue[0] == '"' && strValue[len(strValue)-1] == '"' {
				if err := json.Unmarshal(jsonBytes, &strValue); err == nil {
					// Successfully unquoted
				}
			}
		}

		// Encode as JSON string for varPair
		jsonValue, err := json.Marshal(strValue)
		if err != nil {
			return nil, fmt.Errorf("failed to encode %s: %w", name, err)
		}
		pairs = append(pairs, fmt.Sprintf("%s=%s", name, string(jsonValue)))
	}

	return pairs, nil
}

func randomStepID() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate step id: %w", err)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}

func assignStepIDs(steps []templates.Step) ([]string, error) {
	ids := make([]string, len(steps))
	used := map[string]bool{}
	for i, step := range steps {
		if step.ID == "" {
			continue
		}
		if used[step.ID] {
			return nil, fmt.Errorf("duplicate step id: %s", step.ID)
		}
		ids[i] = step.ID
		used[step.ID] = true
	}
	for i := range steps {
		if ids[i] != "" {
			continue
		}
		for {
			id, err := randomStepID()
			if err != nil {
				return nil, err
			}
			if !used[id] {
				ids[i] = id
				used[id] = true
				break
			}
		}
	}
	return ids, nil
}

func instantiateTemplate(database *db.DB, project, title, templateID string, varPairs []string, priority int, parentType model.ItemType) (string, error) {
	vars, err := parseTemplateVars(varPairs)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(title) == "" {
		return "", fmt.Errorf("title is required for template instantiation")
	}

	tmpl, err := templates.LoadTemplate(templateID)
	if err != nil {
		return "", err
	}

	// Apply defaults and check required variables
	// Variables are required by default unless marked optional or have a default value
	for name, varDef := range tmpl.Variables {
		if _, ok := vars[name]; !ok {
			// If variable has a default value, use it (regardless of Optional flag)
			if varDef.Default != "" {
				vars[name] = varDef.Default
			} else if !varDef.Optional {
				// Only error if no default AND not optional
				return "", fmt.Errorf("missing required template variable: %s", name)
			} else {
				// Optional with no default: use empty string
				vars[name] = ""
			}
		}
	}
	// Check for unknown variables
	for name := range vars {
		if _, ok := tmpl.Variables[name]; !ok {
			return "", fmt.Errorf("unknown template variable: %s", name)
		}
	}

	stepIDs, err := assignStepIDs(tmpl.Steps)
	if err != nil {
		return "", err
	}
	stepIDSet := map[string]bool{}
	for _, id := range stepIDs {
		stepIDSet[id] = true
	}
	for i, step := range tmpl.Steps {
		for _, dep := range step.Depends {
			if !stepIDSet[dep] {
				return "", fmt.Errorf("step %d depends on unknown step id: %s", i, dep)
			}
		}
	}

	// Single-step template: create just a task
	if len(tmpl.Steps) == 1 {
		step := tmpl.Steps[0]
		renderedStep := templates.RenderStep(step, vars)

		// Use step title if provided, otherwise use template title
		itemTitle := renderedStep.Title
		if itemTitle == "" {
			itemTitle = title
		}

		itemID, err := database.GenerateItemID(model.ItemTypeTask)
		if err != nil {
			return "", err
		}

		now := time.Now()
		stepIndex := 0
		item := &model.Item{
			ID:           itemID,
			Project:      project,
			Type:         model.ItemTypeTask,
			Title:        sanitizeTitle(itemTitle),
			Description:  renderedStep.Description,
			Status:       model.StatusOpen,
			Priority:     priority,
			TemplateID:   tmpl.ID,
			StepIndex:    &stepIndex,
			TemplateVars: vars,
			TemplateHash: tmpl.Hash,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := database.CreateItem(item); err != nil {
			return "", err
		}

		printTemplateResult(templateResult{ParentID: itemID, IsEpic: false})
		return itemID, nil
	}

	// Zero-step template: create just a task (no steps to render)
	if len(tmpl.Steps) == 0 {
		itemID, err := database.GenerateItemID(model.ItemTypeTask)
		if err != nil {
			return "", err
		}

		now := time.Now()
		item := &model.Item{
			ID:           itemID,
			Project:      project,
			Type:         model.ItemTypeTask,
			Title:        title,
			Status:       model.StatusOpen,
			Priority:     priority,
			TemplateID:   tmpl.ID,
			TemplateVars: vars,
			TemplateHash: tmpl.Hash,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := database.CreateItem(item); err != nil {
			return "", err
		}

		printTemplateResult(templateResult{ParentID: itemID, IsEpic: false})
		return itemID, nil
	}

	// Multi-step template: create epic with children
	parentType = model.ItemTypeEpic

	// ... rest of multi-step logic continues ...
	parentID, err := database.GenerateItemID(parentType)
	if err != nil {
		return "", err
	}
	createdIDs := []string{}
	cleanup := func() {
		for i := len(createdIDs) - 1; i >= 0; i-- {
			_ = database.DeleteItem(createdIDs[i], true)
		}
	}

	// Check if this template should use worktree
	useWorktree := tmpl.Worktree
	if !useWorktree {
		// Check for variable-based worktree
		if val, ok := vars["use_worktree"]; ok && (val == "true" || val == "yes") {
			useWorktree = true
		}
	}

	// Generate worktree metadata if applicable (only for epics)
	worktreeBranch := ""
	worktreeBase := "main"
	config, _ := db.LoadConfig()
	if useWorktree && parentType == model.ItemTypeEpic {
		worktreeBranch = generateWorktreeBranch(parentID, title, worktreePrefix(config))
		worktreeBase = resolveWorktreeBase(database, "")
		// Check for custom base branch in variables
		if base, ok := vars["base_branch"]; ok && base != "" {
			worktreeBase = base
		}
	}

	now := time.Now()
	parent := &model.Item{
		ID:                  parentID,
		Project:             project,
		Type:                parentType,
		Title:               title,
		Status:              model.StatusOpen,
		Priority:            priority,
		TemplateID:          tmpl.ID,
		TemplateVars:        vars,
		TemplateHash:        tmpl.Hash,
		WorktreeBranch:      worktreeBranch,
		WorktreeBase:        worktreeBase,
		SharedContext:       templates.RenderText(tmpl.Context, vars),
		ClosingInstructions: templates.RenderText(tmpl.OnClose, vars),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := database.CreateItem(parent); err != nil {
		return "", err
	}
	createdIDs = append(createdIDs, parentID)

	childIDs := make([]string, len(tmpl.Steps))
	stepIDToChildID := map[string]string{}
	for i, step := range tmpl.Steps {
		childID, err := database.GenerateItemID(model.ItemTypeTask)
		if err != nil {
			cleanup()
			return "", err
		}
		idx := i

		// Render step title with variable substitution
		renderedStep := templates.RenderStep(step, vars)
		stepTitle := renderedStep.Title
		if stepTitle == "" {
			stepTitle = fmt.Sprintf("%s step %d", tmpl.ID, i+1)
		}

		child := &model.Item{
			ID:           childID,
			Project:      project,
			Type:         model.ItemTypeTask,
			Title:        sanitizeTitle(stepTitle),
			Status:       model.StatusOpen,
			Priority:     priority,
			ParentID:     &parentID,
			TemplateID:   tmpl.ID,
			StepIndex:    &idx,
			TemplateVars: vars,
			TemplateHash: tmpl.Hash,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := database.CreateItem(child); err != nil {
			cleanup()
			return "", err
		}
		createdIDs = append(createdIDs, childID)
		childIDs[i] = childID
		stepIDToChildID[stepIDs[i]] = childID
	}

	for i, step := range tmpl.Steps {
		childID := childIDs[i]
		for _, dep := range step.Depends {
			depID := stepIDToChildID[dep]
			if err := database.AddDep(childID, depID); err != nil {
				cleanup()
				return "", err
			}
		}
	}

	// Print worktree instructions if applicable
	if worktreeBranch != "" {
		fmt.Printf("\n📁 Worktree setup:\n")
		fmt.Printf("  Branch: %s\n", worktreeBranch)
		fmt.Printf("  Base: %s\n", worktreeBase)
		fmt.Printf("\n  Create worktree:\n")
		fmt.Printf("    git worktree add -b %s .worktrees/%s %s\n", worktreeBranch, parentID, worktreeBase)
		fmt.Printf("\n  Navigate to worktree:\n")
		fmt.Printf("    cd .worktrees/%s\n", parentID)
	}

	// Print what was created
	printTemplateResult(templateResult{
		ParentID: parentID,
		ChildIDs: childIDs,
		IsEpic:   parentType == model.ItemTypeEpic,
	})

	return parentID, nil
}

// templateResult holds the result of instantiating a template
type templateResult struct {
	ParentID string   // The main item/epic ID
	ChildIDs []string // Child task IDs (if multi-step)
	IsEpic   bool     // Whether parent is an epic
}

// printTemplateResult outputs what was created
func printTemplateResult(r templateResult) {
	if r.IsEpic && len(r.ChildIDs) > 0 {
		fmt.Printf("Created epic %s with children: %s\n", r.ParentID, strings.Join(r.ChildIDs, ", "))
	} else {
		fmt.Printf("Created task %s\n", r.ParentID)
	}
}

// String returns a formatted string describing what was created
func (r templateResult) String() string {
	if r.IsEpic && len(r.ChildIDs) > 0 {
		return fmt.Sprintf("Created epic %s with children: %s", r.ParentID, strings.Join(r.ChildIDs, ", "))
	}
	return fmt.Sprintf("Created task %s", r.ParentID)
}

func sanitizeTitle(title string) string {
	result := strings.ReplaceAll(title, "\n", " ")
	result = strings.TrimSpace(result)
	result = strings.Join(strings.Fields(result), " ")
	return result
}

func renderItemTemplate(cache *templateCache, item *model.Item) (bool, error) {
	if item.TemplateID == "" {
		return false, nil
	}
	tmpl, err := cache.get(item.TemplateID)
	if err != nil {
		// Log warning but don't fail - allow tpg to work even with missing templates
		fmt.Fprintf(os.Stderr, "Warning: template not found: %s (item: %s)\n", item.TemplateID, item.ID)
		return false, nil
	}

	vars := item.TemplateVars
	if vars == nil {
		vars = map[string]string{}
	}

	// Add system-provided variables that are always available
	// item_id: The ID of the current item being rendered
	vars["item_id"] = item.ID

	// For parent items (no StepIndex), only render if template has exactly one step
	if item.StepIndex == nil {
		if len(tmpl.Steps) == 1 {
			step := templates.RenderStep(tmpl.Steps[0], vars)
			item.Description = step.Description
		}
		hashMismatch := item.TemplateHash != "" && tmpl.Hash != "" && item.TemplateHash != tmpl.Hash
		return hashMismatch, nil
	}

	// For child items with StepIndex, render the specific step
	if *item.StepIndex < 0 || *item.StepIndex >= len(tmpl.Steps) {
		return false, fmt.Errorf("template step index out of range")
	}
	step := templates.RenderStep(tmpl.Steps[*item.StepIndex], vars)
	if step.Title != "" {
		item.Title = sanitizeTitle(step.Title)
	} else {
		item.Title = fmt.Sprintf("%s step %d", tmpl.ID, *item.StepIndex+1)
	}
	item.Description = step.Description
	hashMismatch := item.TemplateHash != "" && tmpl.Hash != "" && item.TemplateHash != tmpl.Hash
	return hashMismatch, nil
}

func renderTemplatesWithCache(cache *templateCache, items []model.Item) error {
	for i := range items {
		if _, err := renderItemTemplate(cache, &items[i]); err != nil {
			return err
		}
	}
	return nil
}

func renderTemplatesForItems(items []model.Item) error {
	return renderTemplatesWithCache(&templateCache{}, items)
}
