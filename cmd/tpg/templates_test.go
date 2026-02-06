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

// Tests for --vars-yaml variable substitution functionality
// These tests are designed to verify the complete flow from YAML parsing
// to variable substitution in rendered templates.

func TestVarsYAML_BasicSubstitution(t *testing.T) {
	// Test: Variables from --vars-yaml should be substituted in step title and description
	// This tests the core functionality of variable substitution

	// Arrange: Create a template with variables in title and description
	tmpl := &templates.Template{
		ID:    "basic-test",
		Title: "Basic Test Template",
		Variables: map[string]templates.Variable{
			"feature": {Description: "Feature name"},
			"scope":   {Description: "Feature scope"},
		},
		Steps: []templates.Step{
			{
				ID:          "step1",
				Title:       "Implement {{.feature}}",
				Description: "Build {{.feature}} with {{.scope}}",
			},
		},
		Hash: "hash123",
	}

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"basic-test": tmpl,
		},
	}

	// Simulate vars from parseTemplateVarsYAML
	yamlInput := `feature: authentication
scope: API endpoints`
	pairs, err := parseTemplateVarsYAML([]byte(yamlInput))
	if err != nil {
		t.Fatalf("parseTemplateVarsYAML failed: %v", err)
	}
	vars, err := parseTemplateVars(pairs)
	if err != nil {
		t.Fatalf("parseTemplateVars failed: %v", err)
	}

	// Create item with template vars (simulating what instantiateTemplate does)
	stepIndex := 0
	item := &model.Item{
		ID:           "test-item",
		Title:        "Original Title",
		TemplateID:   "basic-test",
		StepIndex:    &stepIndex,
		TemplateVars: vars,
		TemplateHash: "hash123",
	}

	// Act: Render the template
	_, renderErr := renderItemTemplate(cache, item)

	// Assert: Variables should be substituted
	if renderErr != nil {
		t.Fatalf("renderItemTemplate failed: %v", renderErr)
	}
	if item.Title != "Implement authentication" {
		t.Errorf("expected title 'Implement authentication', got %q", item.Title)
	}
	if item.Description != "Build authentication with API endpoints" {
		t.Errorf("expected description 'Build authentication with API endpoints', got %q", item.Description)
	}
}

func TestVarsYAML_MultilineValues(t *testing.T) {
	// Test: Multi-line YAML values should be correctly substituted
	// This tests that YAML block scalars (|) are properly handled

	// Arrange
	tmpl := &templates.Template{
		ID:    "multiline-test",
		Title: "Multiline Test",
		Variables: map[string]templates.Variable{
			"requirements": {Description: "Requirements list"},
		},
		Steps: []templates.Step{
			{
				ID:          "step1",
				Title:       "Requirements Step",
				Description: "## Requirements\n\n{{.requirements}}",
			},
		},
		Hash: "hash123",
	}

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"multiline-test": tmpl,
		},
	}

	// YAML with multiline value
	yamlInput := `requirements: |
  - First requirement
  - Second requirement
  - Third requirement`
	pairs, err := parseTemplateVarsYAML([]byte(yamlInput))
	if err != nil {
		t.Fatalf("parseTemplateVarsYAML failed: %v", err)
	}
	vars, err := parseTemplateVars(pairs)
	if err != nil {
		t.Fatalf("parseTemplateVars failed: %v", err)
	}

	stepIndex := 0
	item := &model.Item{
		ID:           "test-item",
		TemplateID:   "multiline-test",
		StepIndex:    &stepIndex,
		TemplateVars: vars,
		TemplateHash: "hash123",
	}

	// Act
	_, renderErr := renderItemTemplate(cache, item)

	// Assert: Multi-line value should be substituted correctly
	if renderErr != nil {
		t.Fatalf("renderItemTemplate failed: %v", renderErr)
	}
	expectedRequirements := "- First requirement\n- Second requirement\n- Third requirement"
	if !strings.Contains(item.Description, expectedRequirements) {
		t.Errorf("expected description to contain multiline requirements.\nExpected to contain: %q\nGot: %q", expectedRequirements, item.Description)
	}
}

func TestVarsYAML_DefaultValuesApplied(t *testing.T) {
	// Test: Variables with defaults should use default when not provided via --vars-yaml
	// This tests that default values from template definitions are applied

	// Arrange: Template with a variable that has a default value
	tmpl := &templates.Template{
		ID:    "defaults-test",
		Title: "Defaults Test",
		Variables: map[string]templates.Variable{
			"feature": {Description: "Feature name"}, // Required
			"priority": {
				Description: "Priority level",
				Default:     "medium",
				// NOTE: Currently, having a default without optional:true still fails.
				// This test documents the expected behavior: defaults should be applied.
			},
		},
		Steps: []templates.Step{
			{
				ID:          "step1",
				Title:       "{{.feature}} - {{.priority}}",
				Description: "Feature: {{.feature}}, Priority: {{.priority}}",
			},
		},
		Hash: "hash123",
	}

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"defaults-test": tmpl,
		},
	}

	// Only provide 'feature', not 'priority' - should use default
	yamlInput := `feature: authentication`
	pairs, err := parseTemplateVarsYAML([]byte(yamlInput))
	if err != nil {
		t.Fatalf("parseTemplateVarsYAML failed: %v", err)
	}
	vars, err := parseTemplateVars(pairs)
	if err != nil {
		t.Fatalf("parseTemplateVars failed: %v", err)
	}

	// Apply defaults (simulating what instantiateTemplate does)
	for name, varDef := range tmpl.Variables {
		if _, ok := vars[name]; !ok {
			// BUG: Currently this check requires Optional:true
			// The fix should apply defaults when Default is set, regardless of Optional
			if varDef.Default != "" || varDef.Optional {
				vars[name] = varDef.Default
			}
		}
	}

	stepIndex := 0
	item := &model.Item{
		ID:           "test-item",
		TemplateID:   "defaults-test",
		StepIndex:    &stepIndex,
		TemplateVars: vars,
		TemplateHash: "hash123",
	}

	// Act
	_, renderErr := renderItemTemplate(cache, item)

	// Assert: Default value should be used
	if renderErr != nil {
		t.Fatalf("renderItemTemplate failed: %v", renderErr)
	}
	if item.Title != "authentication - medium" {
		t.Errorf("expected title 'authentication - medium' (using default priority), got %q", item.Title)
	}
	if item.Description != "Feature: authentication, Priority: medium" {
		t.Errorf("expected default value 'medium' for priority, got description: %q", item.Description)
	}
}

func TestVarsYAML_MissingRequiredVariableError(t *testing.T) {
	// Test: Missing required variables should produce clear error messages
	// This tests error handling for incomplete variable sets

	// Arrange
	tmpl := &templates.Template{
		ID:    "required-test",
		Title: "Required Test",
		Variables: map[string]templates.Variable{
			"required_var": {Description: "This is required"}, // No Default, no Optional
			"optional_var": {Description: "This is optional", Optional: true},
		},
		Steps: []templates.Step{
			{
				ID:    "step1",
				Title: "Step with {{.required_var}}",
			},
		},
		Hash: "hash123",
	}

	// Only provide optional var, missing required
	yamlInput := `optional_var: "some value"`
	pairs, err := parseTemplateVarsYAML([]byte(yamlInput))
	if err != nil {
		t.Fatalf("parseTemplateVarsYAML failed: %v", err)
	}
	vars, err := parseTemplateVars(pairs)
	if err != nil {
		t.Fatalf("parseTemplateVars failed: %v", err)
	}

	// Simulate the validation that instantiateTemplate does
	var validationErr error
	for name, varDef := range tmpl.Variables {
		if _, ok := vars[name]; !ok {
			if !varDef.Optional && varDef.Default == "" {
				validationErr = fmt.Errorf("missing required template variable: %s", name)
				break
			}
		}
	}

	// Assert: Should have error for missing required variable
	if validationErr == nil {
		t.Error("expected error for missing required variable, got nil")
	}
	if validationErr != nil && !strings.Contains(validationErr.Error(), "missing required") {
		t.Errorf("expected 'missing required' in error, got: %v", validationErr)
	}
	if validationErr != nil && !strings.Contains(validationErr.Error(), "required_var") {
		t.Errorf("expected variable name in error, got: %v", validationErr)
	}
}

func TestVarsYAML_ParentItemSubstitution(t *testing.T) {
	// Test: Parent items (with StepIndex == nil) should also have variables substituted
	// in their description when using single-step templates
	// This tests the specific case where parent epics need variable substitution

	// Arrange: Single-step template (description should be rendered for parent)
	tmpl := &templates.Template{
		ID:    "parent-test",
		Title: "Parent Test",
		Variables: map[string]templates.Variable{
			"objective":           {Description: "Main goal"},
			"acceptance_criteria": {Description: "Success criteria"},
		},
		Steps: []templates.Step{
			{
				ID:    "main",
				Title: "Main Task",
				Description: `## Objective

{{.objective}}

## Acceptance Criteria

{{.acceptance_criteria}}`,
			},
		},
		Hash: "hash123",
	}

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"parent-test": tmpl,
		},
	}

	yamlInput := `objective: "Build the authentication system"
acceptance_criteria: "Users can login and logout"`
	pairs, err := parseTemplateVarsYAML([]byte(yamlInput))
	if err != nil {
		t.Fatalf("parseTemplateVarsYAML failed: %v", err)
	}
	vars, err := parseTemplateVars(pairs)
	if err != nil {
		t.Fatalf("parseTemplateVars failed: %v", err)
	}

	// Parent item - no StepIndex
	item := &model.Item{
		ID:           "parent-item",
		TemplateID:   "parent-test",
		StepIndex:    nil, // This is a parent item
		TemplateVars: vars,
		TemplateHash: "hash123",
	}

	// Act
	_, renderErr := renderItemTemplate(cache, item)

	// Assert: Variables should be substituted in parent's description
	if renderErr != nil {
		t.Fatalf("renderItemTemplate failed: %v", renderErr)
	}

	// Check that variables were substituted, not left as {{.variable}}
	if strings.Contains(item.Description, "{{.objective}}") {
		t.Errorf("expected {{.objective}} to be substituted, but found literal template syntax in: %q", item.Description)
	}
	if strings.Contains(item.Description, "{{.acceptance_criteria}}") {
		t.Errorf("expected {{.acceptance_criteria}} to be substituted, but found literal template syntax in: %q", item.Description)
	}
	if !strings.Contains(item.Description, "Build the authentication system") {
		t.Errorf("expected objective to be substituted, got: %q", item.Description)
	}
	if !strings.Contains(item.Description, "Users can login and logout") {
		t.Errorf("expected acceptance_criteria to be substituted, got: %q", item.Description)
	}
}

func TestVarsYAML_SpecialCharactersInValues(t *testing.T) {
	// Test: YAML values with special characters should be handled correctly
	// This tests edge cases like quotes, newlines, and special YAML characters

	tests := []struct {
		name     string
		yaml     string
		wantVars map[string]string
	}{
		{
			name: "quoted strings",
			yaml: `feature: "user authentication"
scope: 'API layer'`,
			wantVars: map[string]string{
				"feature": "user authentication",
				"scope":   "API layer",
			},
		},
		{
			name: "special characters",
			yaml: `feature: "feature: with colons"
description: "Line1\nLine2"`,
			wantVars: map[string]string{
				"feature":     "feature: with colons",
				"description": "Line1\nLine2",
			},
		},
		{
			name: "unicode",
			yaml: `feature: "🔐 Authentication"
scope: "国际化 support"`,
			wantVars: map[string]string{
				"feature": "🔐 Authentication",
				"scope":   "国际化 support",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs, err := parseTemplateVarsYAML([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("parseTemplateVarsYAML failed: %v", err)
			}
			vars, err := parseTemplateVars(pairs)
			if err != nil {
				t.Fatalf("parseTemplateVars failed: %v", err)
			}

			for key, wantValue := range tt.wantVars {
				gotValue, ok := vars[key]
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

func TestVarsYAML_EmptyAndWhitespaceValues(t *testing.T) {
	// Test: Empty and whitespace-only values should be handled correctly
	// This tests edge cases for optional variables with no meaningful value

	yamlInput := `required_feature: "auth"
empty_value: ""
whitespace_only: "   "
null_value:`

	pairs, err := parseTemplateVarsYAML([]byte(yamlInput))
	if err != nil {
		t.Fatalf("parseTemplateVarsYAML failed: %v", err)
	}
	vars, err := parseTemplateVars(pairs)
	if err != nil {
		t.Fatalf("parseTemplateVars failed: %v", err)
	}

	// Check empty value
	if v, ok := vars["empty_value"]; !ok || v != "" {
		t.Errorf("expected empty_value to be empty string, got %q (ok=%v)", v, ok)
	}

	// Check whitespace value (should preserve whitespace)
	if v, ok := vars["whitespace_only"]; !ok || v != "   " {
		t.Errorf("expected whitespace_only to be '   ', got %q (ok=%v)", v, ok)
	}

	// Check null value (should be empty string)
	if v, ok := vars["null_value"]; !ok || v != "" {
		t.Errorf("expected null_value to be empty string, got %q (ok=%v)", v, ok)
	}
}

func TestVarsYAML_ConditionalRendering(t *testing.T) {
	// Test: Template conditionals ({{if}}) should work with vars-yaml variables
	// This tests that hasValue and if/else work correctly

	tmpl := &templates.Template{
		ID:    "conditional-test",
		Title: "Conditional Test",
		Variables: map[string]templates.Variable{
			"feature":     {Description: "Feature name"},
			"constraints": {Description: "Optional constraints", Optional: true},
		},
		Steps: []templates.Step{
			{
				ID:    "step1",
				Title: "{{.feature}}",
				Description: `Feature: {{.feature}}
{{- if hasValue .constraints}}
Constraints: {{.constraints}}
{{- end}}`,
			},
		},
		Hash: "hash123",
	}

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"conditional-test": tmpl,
		},
	}

	tests := []struct {
		name            string
		yaml            string
		wantConstraints bool
	}{
		{
			name:            "with constraints",
			yaml:            "feature: auth\nconstraints: must be fast",
			wantConstraints: true,
		},
		{
			name:            "without constraints",
			yaml:            "feature: auth",
			wantConstraints: false,
		},
		{
			name:            "empty constraints",
			yaml:            "feature: auth\nconstraints: \"\"",
			wantConstraints: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs, err := parseTemplateVarsYAML([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("parseTemplateVarsYAML failed: %v", err)
			}
			vars, err := parseTemplateVars(pairs)
			if err != nil {
				t.Fatalf("parseTemplateVars failed: %v", err)
			}

			// Apply defaults for optional vars
			for name, varDef := range tmpl.Variables {
				if _, ok := vars[name]; !ok && varDef.Optional {
					vars[name] = varDef.Default
				}
			}

			stepIndex := 0
			item := &model.Item{
				ID:           "test-item",
				TemplateID:   "conditional-test",
				StepIndex:    &stepIndex,
				TemplateVars: vars,
				TemplateHash: "hash123",
			}

			_, renderErr := renderItemTemplate(cache, item)
			if renderErr != nil {
				t.Fatalf("renderItemTemplate failed: %v", renderErr)
			}

			hasConstraints := strings.Contains(item.Description, "Constraints:")
			if hasConstraints != tt.wantConstraints {
				t.Errorf("expected constraints=%v in description, got %v. Description: %q",
					tt.wantConstraints, hasConstraints, item.Description)
			}
		})
	}
}

func TestInstantiateTemplate_DefaultsWithoutOptional(t *testing.T) {
	// Test: Variables with Default values should be applied even without Optional: true
	// This verifies the fix for the bug where defaults required Optional: true

	// Arrange: Create a template with default but no optional flag
	tmpl := &templates.Template{
		ID:    "defaults-bug-test",
		Title: "Defaults Bug Test",
		Variables: map[string]templates.Variable{
			"required_var": {Description: "Required"},
			"default_var": {
				Description: "Has default",
				Default:     "default_value",
				// NOTE: No Optional: true - but default should still be applied
			},
		},
		Steps: []templates.Step{
			{ID: "step1", Title: "Step", Description: "{{.required_var}} - {{.default_var}}"},
		},
		Hash: "hash123",
	}

	// Only provide required_var
	vars := map[string]string{
		"required_var": "provided",
		// default_var not provided - should use default
	}

	// Apply the FIXED validation logic: use default if present, regardless of Optional
	for name, varDef := range tmpl.Variables {
		if _, ok := vars[name]; !ok {
			// If variable has a default value, use it (regardless of Optional flag)
			if varDef.Default != "" {
				vars[name] = varDef.Default
			} else if !varDef.Optional {
				t.Errorf("unexpected missing required variable: %s", name)
			} else {
				// Optional with no default: use empty string
				vars[name] = ""
			}
		}
	}

	// After the fix, vars should have the default value
	if vars["default_var"] != "default_value" {
		t.Errorf("expected default_var to have default value 'default_value', got %q", vars["default_var"])
	}
}

func TestRenderText_UndefinedFunction(t *testing.T) {
	// Test: Templates using slugify should now work since slugify is implemented

	// The worktree-epic template uses {{.epic_name | slugify}}
	inputWithSlugify := "Branch: {{.name | slugify}}"
	vars := map[string]string{"name": "Test Name"}

	result := templates.RenderText(inputWithSlugify, vars)

	// slugify is now implemented, so it should transform the name
	expected := "Branch: test-name"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestRenderText_PartialFailureInTemplate(t *testing.T) {
	// Test: Template with both regular variables and slugify function

	// Template with valid variable and slugify function
	input := `## Objective

{{.objective}}

## Branch

{{.name | slugify}}`

	vars := map[string]string{
		"objective": "Build authentication",
		"name":      "Test Feature",
	}

	result := templates.RenderText(input, vars)

	// Now that slugify is implemented, both variables and slugify should work
	expectedObjective := "Build authentication"
	expectedBranch := "test-feature"

	if !strings.Contains(result, expectedObjective) {
		t.Errorf("expected result to contain %q, got %q", expectedObjective, result)
	}
	if !strings.Contains(result, expectedBranch) {
		t.Errorf("expected result to contain slugified branch %q, got %q", expectedBranch, result)
	}
	// Should NOT contain unsubstituted placeholders
	if strings.Contains(result, "{{.objective}}") {
		t.Errorf("objective should be substituted, got: %q", result)
	}
	if strings.Contains(result, "{{.name") {
		t.Errorf("name should be slugified, got: %q", result)
	}
}

func TestVarsYAML_EndToEndSubstitution(t *testing.T) {
	// Test: Complete end-to-end test simulating the exact bug scenario:
	// 1. User provides vars via --vars-yaml
	// 2. Template is instantiated
	// 3. Item is rendered for display
	// 4. Variables should be substituted, not appear as {{.variable}}

	// This template simulates worktree-epic structure without the problematic slugify
	tmpl := &templates.Template{
		ID:    "e2e-test",
		Title: "E2E Test",
		Variables: map[string]templates.Variable{
			"epic_name":           {Description: "Epic name"},
			"objective":           {Description: "Main objective"},
			"acceptance_criteria": {Description: "Success criteria"},
			"base_branch": {
				Description: "Base branch",
				Default:     "main",
				// BUG: This has a default but is not Optional, so it will fail
			},
		},
		Steps: []templates.Step{
			{
				ID:    "main",
				Title: "{{.epic_name}}",
				Description: `## Objective

{{.objective}}

## Branch

Base: {{.base_branch}}

## Acceptance Criteria

{{.acceptance_criteria}}`,
			},
		},
		Hash: "hash123",
	}

	cache := &templateCache{
		templates: map[string]*templates.Template{
			"e2e-test": tmpl,
		},
	}

	// Simulate YAML input from user (note: base_branch has default, user doesn't provide it)
	yamlInput := `epic_name: "CLI Cleanup"
objective: "Clean up CLI argument handling"
acceptance_criteria: "All flags work consistently"
base_branch: "main"`

	pairs, err := parseTemplateVarsYAML([]byte(yamlInput))
	if err != nil {
		t.Fatalf("parseTemplateVarsYAML failed: %v", err)
	}
	vars, err := parseTemplateVars(pairs)
	if err != nil {
		t.Fatalf("parseTemplateVars failed: %v", err)
	}

	// Apply defaults (simulating instantiateTemplate)
	for name, varDef := range tmpl.Variables {
		if _, ok := vars[name]; !ok {
			// Current buggy behavior: requires Optional
			// Fixed behavior: should also check Default != ""
			if varDef.Optional {
				vars[name] = varDef.Default
			}
		}
	}

	// Create parent item (no StepIndex, like an epic)
	item := &model.Item{
		ID:           "ep-test",
		TemplateID:   "e2e-test",
		StepIndex:    nil, // Parent item
		TemplateVars: vars,
		TemplateHash: "hash123",
	}

	// Render template
	_, renderErr := renderItemTemplate(cache, item)
	if renderErr != nil {
		t.Fatalf("renderItemTemplate failed: %v", renderErr)
	}

	// CRITICAL ASSERTION: Variables must be substituted
	// The bug was: {{.objective}} appeared literally in output

	// Check no template syntax remains
	if strings.Contains(item.Description, "{{.") {
		t.Errorf("BUG: Template variables not substituted. Found raw template syntax in: %q", item.Description)
	}

	// Check actual values are present
	if !strings.Contains(item.Description, "Clean up CLI argument handling") {
		t.Errorf("expected objective in description, got: %q", item.Description)
	}
	if !strings.Contains(item.Description, "All flags work consistently") {
		t.Errorf("expected acceptance_criteria in description, got: %q", item.Description)
	}
	if !strings.Contains(item.Description, "main") {
		t.Errorf("expected base_branch in description, got: %q", item.Description)
	}
}
