package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/taxilian/tpg/internal/model"
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
