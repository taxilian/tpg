package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/taxilian/tpg/internal/model"
	"github.com/taxilian/tpg/internal/templates"
)

func TestRenderItemTemplate_MissingTemplate(t *testing.T) {
	// Create a pipe to capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	// Create an item with a non-existent template ID
	stepIndex := 0
	item := &model.Item{
		ID:         "test-item-123",
		Title:      "Original Title",
		TemplateID: "non-existent-template",
		StepIndex:  &stepIndex,
	}

	// Create a template cache
	cache := &templateCache{}

	// Call renderItemTemplate
	hashMismatch, err := renderItemTemplate(cache, item)

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured stderr
	var buf bytes.Buffer
	if _, copyErr := io.Copy(&buf, r); copyErr != nil {
		t.Fatalf("failed to read from pipe: %v", copyErr)
	}
	r.Close()
	stderrOutput := buf.String()

	// Verify no error is returned
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify hashMismatch is false
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false, got true")
	}

	// Verify title remains unchanged
	if item.Title != "Original Title" {
		t.Errorf("expected title to remain unchanged, got: %s", item.Title)
	}

	// Verify warning message contains template ID and item ID
	if !strings.Contains(stderrOutput, "non-existent-template") {
		t.Errorf("expected stderr to contain template ID 'non-existent-template', got: %s", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "test-item-123") {
		t.Errorf("expected stderr to contain item ID 'test-item-123', got: %s", stderrOutput)
	}
	if !strings.Contains(stderrOutput, "Warning: template not found") {
		t.Errorf("expected stderr to contain 'Warning: template not found', got: %s", stderrOutput)
	}
}

// mockTemplateCache is a test helper that allows injecting templates without filesystem access
type mockTemplateCache struct {
	templates map[string]*templates.Template
}

func (c *mockTemplateCache) get(id string) (*templates.Template, error) {
	if c.templates == nil {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	if tmpl, ok := c.templates[id]; ok {
		return tmpl, nil
	}
	return nil, fmt.Errorf("template not found: %s", id)
}

// createTestTemplate creates a template with the given steps for testing
func createTestTemplate(id string, steps []templates.Step) *templates.Template {
	return &templates.Template{
		ID:        id,
		Title:     "Test Template",
		Variables: map[string]templates.Variable{},
		Steps:     steps,
		Hash:      "testhash123",
	}
}

func TestRenderItemTemplate_NoTemplateID(t *testing.T) {
	// Item with no TemplateID should return early without error
	item := &model.Item{
		ID:         "test-item-1",
		Title:      "Original Title",
		TemplateID: "",
	}

	cache := &templateCache{}
	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false")
	}
	if item.Title != "Original Title" {
		t.Errorf("expected title to remain unchanged, got: %s", item.Title)
	}
}

func TestRenderItemTemplate_NoStepIndex_MultiStep(t *testing.T) {
	// Parent item with multi-step template should not have description rendered
	tmpl := createTestTemplate("multi-step-template", []templates.Step{
		{ID: "step1", Title: "Step 1", Description: "First step description"},
		{ID: "step2", Title: "Step 2", Description: "Second step description"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"multi-step-template": tmpl,
		},
	}

	item := &model.Item{
		ID:           "test-item-2",
		Title:        "Parent Task Title",
		TemplateID:   "multi-step-template",
		StepIndex:    nil, // No step index - this is a parent item
		TemplateHash: "testhash123",
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false")
	}
	// Title should remain unchanged for parent items
	if item.Title != "Parent Task Title" {
		t.Errorf("expected title to remain unchanged, got: %s", item.Title)
	}
	// Description should NOT be set for multi-step templates
	if item.Description != "" {
		t.Errorf("expected description to remain empty for multi-step parent, got: %s", item.Description)
	}
}

func TestRenderItemTemplate_NoStepIndex_SingleStep(t *testing.T) {
	// Parent item with single-step template should have description rendered
	tmpl := createTestTemplate("single-step-template", []templates.Step{
		{ID: "step1", Title: "Step 1", Description: "Single step description with {{.feature}}"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"single-step-template": tmpl,
		},
	}

	item := &model.Item{
		ID:           "test-item-2b",
		Title:        "Parent Task Title",
		TemplateID:   "single-step-template",
		StepIndex:    nil, // No step index - this is a parent item
		TemplateVars: map[string]string{"feature": "auth"},
		TemplateHash: "testhash123",
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false")
	}
	// Title should remain unchanged for parent items
	if item.Title != "Parent Task Title" {
		t.Errorf("expected title to remain unchanged, got: %s", item.Title)
	}
	// Description SHOULD be set for single-step templates
	expectedDesc := "Single step description with auth"
	if item.Description != expectedDesc {
		t.Errorf("expected description '%s', got: %s", expectedDesc, item.Description)
	}
}

func TestRenderItemTemplate_NoStepIndex_SingleStep_HashMismatch(t *testing.T) {
	// Parent item with single-step template should detect hash mismatch
	tmpl := createTestTemplate("single-step-template", []templates.Step{
		{ID: "step1", Title: "Step 1", Description: "Single step description"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"single-step-template": tmpl,
		},
	}

	item := &model.Item{
		ID:           "test-item-hash-mismatch",
		Title:        "Parent Task Title",
		TemplateID:   "single-step-template",
		StepIndex:    nil,                // No step index - this is a parent item
		TemplateHash: "differenthash456", // Different from template hash "testhash123"
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if !hashMismatch {
		t.Errorf("expected hashMismatch to be true when hashes differ")
	}
	// Title should remain unchanged for parent items
	if item.Title != "Parent Task Title" {
		t.Errorf("expected title to remain unchanged, got: %s", item.Title)
	}
	// Description SHOULD still be set even with hash mismatch
	if item.Description != "Single step description" {
		t.Errorf("expected description 'Single step description', got: %s", item.Description)
	}
}

func TestRenderItemTemplate_NoStepIndex_SingleStep_EmptyDescription(t *testing.T) {
	// Parent item with single-step template that has empty description
	tmpl := createTestTemplate("single-step-template", []templates.Step{
		{ID: "step1", Title: "Step 1", Description: ""},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"single-step-template": tmpl,
		},
	}

	item := &model.Item{
		ID:           "test-item-empty-desc",
		Title:        "Parent Task Title",
		Description:  "Original description",
		TemplateID:   "single-step-template",
		StepIndex:    nil, // No step index - this is a parent item
		TemplateHash: "testhash123",
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false")
	}
	// Description should be set to empty (overwriting original)
	if item.Description != "" {
		t.Errorf("expected description to be empty, got: %s", item.Description)
	}
}

func TestRenderItemTemplate_NoStepIndex_ZeroSteps(t *testing.T) {
	// Parent item with template that has zero steps
	tmpl := createTestTemplate("zero-step-template", []templates.Step{})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"zero-step-template": tmpl,
		},
	}

	item := &model.Item{
		ID:           "test-item-zero-steps",
		Title:        "Parent Task Title",
		Description:  "Original description",
		TemplateID:   "zero-step-template",
		StepIndex:    nil, // No step index - this is a parent item
		TemplateHash: "testhash123",
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false")
	}
	// Description should remain unchanged (zero steps != single step)
	if item.Description != "Original description" {
		t.Errorf("expected description to remain unchanged, got: %s", item.Description)
	}
}

func TestRenderItemTemplate_NoStepIndex_MultiStep_HashMismatch(t *testing.T) {
	// Parent item with multi-step template should detect hash mismatch
	tmpl := createTestTemplate("multi-step-template", []templates.Step{
		{ID: "step1", Title: "Step 1", Description: "First step description"},
		{ID: "step2", Title: "Step 2", Description: "Second step description"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"multi-step-template": tmpl,
		},
	}

	item := &model.Item{
		ID:           "test-item-multi-hash",
		Title:        "Parent Task Title",
		TemplateID:   "multi-step-template",
		StepIndex:    nil,                // No step index - this is a parent item
		TemplateHash: "differenthash456", // Different from template hash "testhash123"
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if !hashMismatch {
		t.Errorf("expected hashMismatch to be true when hashes differ")
	}
	// Title should remain unchanged for parent items
	if item.Title != "Parent Task Title" {
		t.Errorf("expected title to remain unchanged, got: %s", item.Title)
	}
	// Description should NOT be set for multi-step templates
	if item.Description != "" {
		t.Errorf("expected description to remain empty for multi-step parent, got: %s", item.Description)
	}
}

func TestRenderItemTemplate_NoStepIndex_ThreeSteps(t *testing.T) {
	// Parent item with three-step template should not have description rendered
	tmpl := createTestTemplate("three-step-template", []templates.Step{
		{ID: "step1", Title: "Step 1", Description: "First step description"},
		{ID: "step2", Title: "Step 2", Description: "Second step description"},
		{ID: "step3", Title: "Step 3", Description: "Third step description"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"three-step-template": tmpl,
		},
	}

	item := &model.Item{
		ID:           "test-item-three-steps",
		Title:        "Parent Task Title",
		TemplateID:   "three-step-template",
		StepIndex:    nil, // No step index - this is a parent item
		TemplateHash: "testhash123",
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false")
	}
	// Title should remain unchanged for parent items
	if item.Title != "Parent Task Title" {
		t.Errorf("expected title to remain unchanged, got: %s", item.Title)
	}
	// Description should NOT be set for multi-step templates (3 steps > 1)
	if item.Description != "" {
		t.Errorf("expected description to remain empty for three-step parent, got: %s", item.Description)
	}
}

func TestRenderItemTemplate_StepIndexOutOfRange(t *testing.T) {
	// Create a template with only 2 steps
	tmpl := createTestTemplate("test-template", []templates.Step{
		{ID: "step1", Title: "Step 1", Description: "First step"},
		{ID: "step2", Title: "Step 2", Description: "Second step"},
	})

	// Create a mock cache with the template
	cache := &templateCache{
		templates: map[string]*templates.Template{
			"test-template": tmpl,
		},
	}

	// Test with step index that's too high
	stepIndex := 5 // Out of range (only 2 steps: 0 and 1)
	item := &model.Item{
		ID:         "test-item-3",
		Title:      "Original Title",
		TemplateID: "test-template",
		StepIndex:  &stepIndex,
	}

	_, err := renderItemTemplate(cache, item)

	if err == nil {
		t.Errorf("expected error for out of range step index, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected 'out of range' error, got: %v", err)
	}
}

func TestRenderItemTemplate_NegativeStepIndex(t *testing.T) {
	// Create a template
	tmpl := createTestTemplate("test-template", []templates.Step{
		{ID: "step1", Title: "Step 1", Description: "First step"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"test-template": tmpl,
		},
	}

	// Test with negative step index
	stepIndex := -1
	item := &model.Item{
		ID:         "test-item-4",
		Title:      "Original Title",
		TemplateID: "test-template",
		StepIndex:  &stepIndex,
	}

	_, err := renderItemTemplate(cache, item)

	if err == nil {
		t.Errorf("expected error for negative step index, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected 'out of range' error, got: %v", err)
	}
}

func TestRenderItemTemplate_SuccessfulRendering(t *testing.T) {
	// Create a template with variable substitution
	tmpl := createTestTemplate("test-template", []templates.Step{
		{ID: "step1", Title: "Implement {{.feature}}", Description: "Build the {{.feature}} feature"},
		{ID: "step2", Title: "Test {{.feature}}", Description: "Test the {{.feature}} feature"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"test-template": tmpl,
		},
	}

	stepIndex := 0
	item := &model.Item{
		ID:           "test-item-5",
		Title:        "Original Title",
		TemplateID:   "test-template",
		StepIndex:    &stepIndex,
		TemplateVars: map[string]string{"feature": "authentication"},
		TemplateHash: "testhash123", // Same as template hash
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false when hashes match")
	}
	if item.Title != "Implement authentication" {
		t.Errorf("expected title 'Implement authentication', got: %s", item.Title)
	}
	if item.Description != "Build the authentication feature" {
		t.Errorf("expected description 'Build the authentication feature', got: %s", item.Description)
	}
}

func TestRenderItemTemplate_SecondStep(t *testing.T) {
	// Test rendering the second step (index 1)
	tmpl := createTestTemplate("test-template", []templates.Step{
		{ID: "step1", Title: "First Step", Description: "First description"},
		{ID: "step2", Title: "Second Step", Description: "Second description"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"test-template": tmpl,
		},
	}

	stepIndex := 1
	item := &model.Item{
		ID:           "test-item-6",
		Title:        "Original Title",
		TemplateID:   "test-template",
		StepIndex:    &stepIndex,
		TemplateHash: "testhash123",
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false")
	}
	if item.Title != "Second Step" {
		t.Errorf("expected title 'Second Step', got: %s", item.Title)
	}
	if item.Description != "Second description" {
		t.Errorf("expected description 'Second description', got: %s", item.Description)
	}
}

func TestRenderItemTemplate_HashMismatch(t *testing.T) {
	// Test that hash mismatch is detected
	tmpl := createTestTemplate("test-template", []templates.Step{
		{ID: "step1", Title: "Step Title", Description: "Step description"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"test-template": tmpl,
		},
	}

	stepIndex := 0
	item := &model.Item{
		ID:           "test-item-7",
		Title:        "Original Title",
		TemplateID:   "test-template",
		StepIndex:    &stepIndex,
		TemplateHash: "differenthash456", // Different from template hash
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if !hashMismatch {
		t.Errorf("expected hashMismatch to be true when hashes differ")
	}
	// Title should still be rendered
	if item.Title != "Step Title" {
		t.Errorf("expected title 'Step Title', got: %s", item.Title)
	}
}

func TestRenderItemTemplate_EmptyStepTitle(t *testing.T) {
	// Test that empty step title gets a default
	tmpl := createTestTemplate("my-template", []templates.Step{
		{ID: "step1", Title: "", Description: "Step with no title"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"my-template": tmpl,
		},
	}

	stepIndex := 0
	item := &model.Item{
		ID:           "test-item-8",
		Title:        "Original Title",
		TemplateID:   "my-template",
		StepIndex:    &stepIndex,
		TemplateHash: "testhash123",
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false")
	}
	// Should get default title format: "template-id step N"
	expectedTitle := "my-template step 1"
	if item.Title != expectedTitle {
		t.Errorf("expected title '%s', got: %s", expectedTitle, item.Title)
	}
}

func TestRenderItemTemplate_NilTemplateVars(t *testing.T) {
	// Test that nil TemplateVars doesn't cause panic
	tmpl := createTestTemplate("test-template", []templates.Step{
		{ID: "step1", Title: "Static Title", Description: "Static description"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"test-template": tmpl,
		},
	}

	stepIndex := 0
	item := &model.Item{
		ID:           "test-item-9",
		Title:        "Original Title",
		TemplateID:   "test-template",
		StepIndex:    &stepIndex,
		TemplateVars: nil, // Explicitly nil
		TemplateHash: "testhash123",
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false")
	}
	if item.Title != "Static Title" {
		t.Errorf("expected title 'Static Title', got: %s", item.Title)
	}
}

func TestRenderItemTemplate_MultilineTitle(t *testing.T) {
	// Test that multiline titles are sanitized
	tmpl := createTestTemplate("test-template", []templates.Step{
		{ID: "step1", Title: "Title with\nnewline", Description: "Description"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"test-template": tmpl,
		},
	}

	stepIndex := 0
	item := &model.Item{
		ID:           "test-item-10",
		Title:        "Original Title",
		TemplateID:   "test-template",
		StepIndex:    &stepIndex,
		TemplateHash: "testhash123",
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false")
	}
	// Newlines should be replaced with spaces
	if strings.Contains(item.Title, "\n") {
		t.Errorf("expected title to have newlines removed, got: %s", item.Title)
	}
	if item.Title != "Title with newline" {
		t.Errorf("expected title 'Title with newline', got: %s", item.Title)
	}
}

func TestRenderItemTemplate_EmptyItemHash(t *testing.T) {
	// Test that empty item hash doesn't trigger mismatch
	tmpl := createTestTemplate("test-template", []templates.Step{
		{ID: "step1", Title: "Step Title", Description: "Description"},
	})

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"test-template": tmpl,
		},
	}

	stepIndex := 0
	item := &model.Item{
		ID:           "test-item-11",
		Title:        "Original Title",
		TemplateID:   "test-template",
		StepIndex:    &stepIndex,
		TemplateHash: "", // Empty hash
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false when item hash is empty")
	}
}

func TestRenderItemTemplate_EmptyTemplateHash(t *testing.T) {
	// Test that empty template hash doesn't trigger mismatch
	tmpl := &templates.Template{
		ID:        "test-template",
		Title:     "Test Template",
		Variables: map[string]templates.Variable{},
		Steps: []templates.Step{
			{ID: "step1", Title: "Step Title", Description: "Description"},
		},
		Hash: "", // Empty template hash
	}

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"test-template": tmpl,
		},
	}

	stepIndex := 0
	item := &model.Item{
		ID:           "test-item-12",
		Title:        "Original Title",
		TemplateID:   "test-template",
		StepIndex:    &stepIndex,
		TemplateHash: "somehash",
	}

	hashMismatch, err := renderItemTemplate(cache, item)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if hashMismatch {
		t.Errorf("expected hashMismatch to be false when template hash is empty")
	}
}

func TestParseTemplateVarsYAML(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "simple strings",
			yaml: `feature_name: authentication
scope: API endpoint`,
			want: map[string]string{
				"feature_name": "authentication",
				"scope":        "API endpoint",
			},
		},
		{
			name: "multiline string",
			yaml: `feature_name: auth
requirements: |
  Line 1
  Line 2`,
			want: map[string]string{
				"feature_name": "auth",
				"requirements": "Line 1\nLine 2",
			},
		},
		{
			name: "empty value",
			yaml: `feature_name: auth
constraints:`,
			want: map[string]string{
				"feature_name": "auth",
				"constraints":  "",
			},
		},
		{
			name:    "invalid yaml",
			yaml:    `feature_name: [unclosed`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs, err := parseTemplateVarsYAML([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTemplateVarsYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Parse back to map for comparison
			got, err := parseTemplateVars(pairs)
			if err != nil {
				t.Fatalf("parseTemplateVars() failed: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Errorf("got %d vars, want %d", len(got), len(tt.want))
			}

			for key, wantValue := range tt.want {
				gotValue, ok := got[key]
				if !ok {
					t.Errorf("missing key %q", key)
					continue
				}
				if gotValue != wantValue {
					t.Errorf("key %q: got %q, want %q", key, gotValue, wantValue)
				}
			}
		})
	}
}
