package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTemplate(t *testing.T) {
	setupTemplatesDir := func(t *testing.T) (string, string, func()) {
		tmpDir := t.TempDir()
		templatesDir := filepath.Join(tmpDir, ".tgz", "templates")
		if err := os.MkdirAll(templatesDir, 0755); err != nil {
			t.Fatalf("failed to create templates dir: %v", err)
		}

		originalWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		cleanup := func() {
			os.Chdir(originalWd)
		}

		return tmpDir, templatesDir, cleanup
	}

	t.Run("loads YAML template", func(t *testing.T) {
		_, templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		yamlContent := `title: Test Template
description: A test template
variables:
  name:
    description: The name to use
steps:
  - id: step1
    title: First step
    description: Do the first thing
  - id: step2
    title: Second step
    description: Do the second thing
    depends:
      - step1
`
		if err := os.WriteFile(filepath.Join(templatesDir, "test.yaml"), []byte(yamlContent), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		tmpl, err := LoadTemplate("test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tmpl.ID != "test" {
			t.Errorf("expected ID 'test', got %q", tmpl.ID)
		}
		if tmpl.Title != "Test Template" {
			t.Errorf("expected Title 'Test Template', got %q", tmpl.Title)
		}
		if tmpl.Description != "A test template" {
			t.Errorf("expected Description 'A test template', got %q", tmpl.Description)
		}
		if len(tmpl.Variables) != 1 {
			t.Errorf("expected 1 variable, got %d", len(tmpl.Variables))
		}
		if _, ok := tmpl.Variables["name"]; !ok {
			t.Error("expected 'name' variable")
		}
		if len(tmpl.Steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(tmpl.Steps))
		}
		if tmpl.Steps[0].ID != "step1" {
			t.Errorf("expected step ID 'step1', got %q", tmpl.Steps[0].ID)
		}
		if len(tmpl.Steps[1].Depends) != 1 || tmpl.Steps[1].Depends[0] != "step1" {
			t.Errorf("expected step2 to depend on step1")
		}
		if tmpl.Hash == "" {
			t.Error("expected Hash to be set")
		}
		if tmpl.SourcePath == "" {
			t.Error("expected SourcePath to be set")
		}
	})

	t.Run("loads YML template", func(t *testing.T) {
		_, templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		ymlContent := `title: YML Template
description: A yml template
steps:
  - id: only-step
    title: Only step
    description: The only step
`
		if err := os.WriteFile(filepath.Join(templatesDir, "yml-test.yml"), []byte(ymlContent), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		tmpl, err := LoadTemplate("yml-test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tmpl.Title != "YML Template" {
			t.Errorf("expected Title 'YML Template', got %q", tmpl.Title)
		}
	})

	t.Run("loads TOML template", func(t *testing.T) {
		_, templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		tomlContent := `title = "TOML Template"
description = "A toml template"

[variables.project]
description = "The project name"

[[steps]]
id = "init"
title = "Initialize"
description = "Initialize the project"

[[steps]]
id = "build"
title = "Build"
description = "Build the project"
depends = ["init"]
`
		if err := os.WriteFile(filepath.Join(templatesDir, "toml-test.toml"), []byte(tomlContent), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		tmpl, err := LoadTemplate("toml-test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tmpl.ID != "toml-test" {
			t.Errorf("expected ID 'toml-test', got %q", tmpl.ID)
		}
		if tmpl.Title != "TOML Template" {
			t.Errorf("expected Title 'TOML Template', got %q", tmpl.Title)
		}
		if len(tmpl.Variables) != 1 {
			t.Errorf("expected 1 variable, got %d", len(tmpl.Variables))
		}
		if _, ok := tmpl.Variables["project"]; !ok {
			t.Error("expected 'project' variable")
		}
		if len(tmpl.Steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(tmpl.Steps))
		}
	})

	t.Run("returns error for empty template ID", func(t *testing.T) {
		_, _, cleanup := setupTemplatesDir(t)
		defer cleanup()

		_, err := LoadTemplate("")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "template id is required") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("returns error for whitespace template ID", func(t *testing.T) {
		_, _, cleanup := setupTemplatesDir(t)
		defer cleanup()

		_, err := LoadTemplate("   ")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "template id is required") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("returns error for template not found", func(t *testing.T) {
		_, _, cleanup := setupTemplatesDir(t)
		defer cleanup()

		_, err := LoadTemplate("nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "template not found") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("returns error for template with no steps", func(t *testing.T) {
		_, templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		yamlContent := `title: Empty Template
description: A template with no steps
steps: []
`
		if err := os.WriteFile(filepath.Join(templatesDir, "empty.yaml"), []byte(yamlContent), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		_, err := LoadTemplate("empty")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "template has no steps") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("returns error for duplicate step ID", func(t *testing.T) {
		_, templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		yamlContent := `title: Duplicate Steps
description: A template with duplicate step IDs
steps:
  - id: same-id
    title: First step
    description: First
  - id: same-id
    title: Second step
    description: Second
`
		if err := os.WriteFile(filepath.Join(templatesDir, "duplicate.yaml"), []byte(yamlContent), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		_, err := LoadTemplate("duplicate")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "duplicate step id") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("allows steps without explicit ID", func(t *testing.T) {
		_, templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		yamlContent := `title: No ID Steps
description: Steps without explicit IDs
steps:
  - title: First step
    description: First
  - title: Second step
    description: Second
`
		if err := os.WriteFile(filepath.Join(templatesDir, "no-ids.yaml"), []byte(yamlContent), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		tmpl, err := LoadTemplate("no-ids")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(tmpl.Steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(tmpl.Steps))
		}
	})

	t.Run("initializes empty variables map", func(t *testing.T) {
		_, templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		yamlContent := `title: No Variables
description: A template without variables
steps:
  - id: step1
    title: Step
    description: A step
`
		if err := os.WriteFile(filepath.Join(templatesDir, "no-vars.yaml"), []byte(yamlContent), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		tmpl, err := LoadTemplate("no-vars")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tmpl.Variables == nil {
			t.Error("expected Variables to be initialized, got nil")
		}
	})

	t.Run("returns error for invalid YAML", func(t *testing.T) {
		_, templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		invalidYAML := `title: Invalid
description: [unclosed bracket
steps:
`
		if err := os.WriteFile(filepath.Join(templatesDir, "invalid.yaml"), []byte(invalidYAML), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		_, err := LoadTemplate("invalid")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse yaml") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("returns error for invalid TOML", func(t *testing.T) {
		_, templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		invalidTOML := `title = "Invalid"
description = unclosed string
`
		if err := os.WriteFile(filepath.Join(templatesDir, "invalid.toml"), []byte(invalidTOML), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		_, err := LoadTemplate("invalid")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse toml") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestRenderText(t *testing.T) {
	t.Run("interpolates single variable", func(t *testing.T) {
		result := RenderText("Hello {{.name}}!", map[string]string{"name": "World"})
		if result != "Hello World!" {
			t.Errorf("expected 'Hello World!', got %q", result)
		}
	})

	t.Run("interpolates multiple variables", func(t *testing.T) {
		vars := map[string]string{
			"greeting":    "Hello",
			"name":        "Alice",
			"punctuation": "!",
		}
		result := RenderText("{{.greeting}} {{.name}}{{.punctuation}}", vars)
		if result != "Hello Alice!" {
			t.Errorf("expected 'Hello Alice!', got %q", result)
		}
	})

	t.Run("interpolates same variable multiple times", func(t *testing.T) {
		result := RenderText("{{.x}} + {{.x}} = 2{{.x}}", map[string]string{"x": "1"})
		if result != "1 + 1 = 21" {
			t.Errorf("expected '1 + 1 = 21', got %q", result)
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		result := RenderText("", map[string]string{"name": "World"})
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("handles empty vars", func(t *testing.T) {
		result := RenderText("Hello {{.name}}!", map[string]string{})
		// With no "name" key, Go template outputs "<no value>"
		if result != "Hello <no value>!" {
			t.Errorf("expected 'Hello <no value>!', got %q", result)
		}
	})

	t.Run("handles nil vars", func(t *testing.T) {
		result := RenderText("Hello {{.name}}!", nil)
		if result != "Hello <no value>!" {
			t.Errorf("expected 'Hello <no value>!', got %q", result)
		}
	})

	t.Run("handles variable with empty value", func(t *testing.T) {
		result := RenderText("Hello {{.name}}!", map[string]string{"name": ""})
		if result != "Hello !" {
			t.Errorf("expected 'Hello !', got %q", result)
		}
	})

	t.Run("handles multiline text", func(t *testing.T) {
		input := `Line 1: {{.var}}
Line 2: {{.var}}
Line 3: end`
		expected := `Line 1: value
Line 2: value
Line 3: end`
		result := RenderText(input, map[string]string{"var": "value"})
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	t.Run("supports conditionals with if", func(t *testing.T) {
		input := `Required: {{.req}}{{if .opt}}
Optional: {{.opt}}{{end}}`

		// With optional value
		result := RenderText(input, map[string]string{"req": "yes", "opt": "also yes"})
		expected := "Required: yes\nOptional: also yes"
		if result != expected {
			t.Errorf("with opt: expected %q, got %q", expected, result)
		}

		// Without optional value
		result = RenderText(input, map[string]string{"req": "yes", "opt": ""})
		expected = "Required: yes"
		if result != expected {
			t.Errorf("without opt: expected %q, got %q", expected, result)
		}
	})

	t.Run("supports hasValue helper", func(t *testing.T) {
		input := `{{if hasValue .opt}}Has value: {{.opt}}{{else}}No value{{end}}`

		result := RenderText(input, map[string]string{"opt": "something"})
		if result != "Has value: something" {
			t.Errorf("with value: expected 'Has value: something', got %q", result)
		}

		result = RenderText(input, map[string]string{"opt": ""})
		if result != "No value" {
			t.Errorf("without value: expected 'No value', got %q", result)
		}

		result = RenderText(input, map[string]string{"opt": "   "})
		if result != "No value" {
			t.Errorf("whitespace only: expected 'No value', got %q", result)
		}
	})

	t.Run("supports default helper", func(t *testing.T) {
		input := `Value: {{default "fallback" .opt}}`

		result := RenderText(input, map[string]string{"opt": "provided"})
		if result != "Value: provided" {
			t.Errorf("with value: expected 'Value: provided', got %q", result)
		}

		result = RenderText(input, map[string]string{"opt": ""})
		if result != "Value: fallback" {
			t.Errorf("without value: expected 'Value: fallback', got %q", result)
		}
	})

	t.Run("supports whitespace trimming", func(t *testing.T) {
		input := `Line 1
{{- if hasValue .opt}}
Optional: {{.opt}}
{{- end}}
Line 2`

		// With value - newlines are trimmed around the conditional
		result := RenderText(input, map[string]string{"opt": "yes"})
		expected := "Line 1\nOptional: yes\nLine 2"
		if result != expected {
			t.Errorf("with value: expected %q, got %q", expected, result)
		}

		// Without value - conditional block is completely removed
		result = RenderText(input, map[string]string{"opt": ""})
		expected = "Line 1\nLine 2"
		if result != expected {
			t.Errorf("without value: expected %q, got %q", expected, result)
		}
	})
}

func TestRenderStep(t *testing.T) {
	t.Run("renders step with variables", func(t *testing.T) {
		step := Step{
			ID:          "step-{{.id}}",
			Title:       "Setup {{.project}}",
			Description: "Initialize the {{.project}} project with {{.framework}}",
			Depends:     []string{"other-step"},
		}
		vars := map[string]string{
			"id":        "1",
			"project":   "myapp",
			"framework": "React",
		}

		result := RenderStep(step, vars)

		// Note: ID is not interpolated based on implementation
		if result.ID != "step-{{.id}}" {
			t.Errorf("expected ID to remain unchanged, got %q", result.ID)
		}
		if result.Title != "Setup myapp" {
			t.Errorf("expected Title 'Setup myapp', got %q", result.Title)
		}
		if result.Description != "Initialize the myapp project with React" {
			t.Errorf("expected Description 'Initialize the myapp project with React', got %q", result.Description)
		}
		if len(result.Depends) != 1 || result.Depends[0] != "other-step" {
			t.Errorf("expected Depends to be preserved")
		}
	})

	t.Run("preserves step with no variables", func(t *testing.T) {
		step := Step{
			ID:          "static-step",
			Title:       "Static Title",
			Description: "Static description",
			Depends:     []string{"dep1", "dep2"},
		}

		result := RenderStep(step, map[string]string{})

		if result.ID != step.ID {
			t.Errorf("expected ID %q, got %q", step.ID, result.ID)
		}
		if result.Title != step.Title {
			t.Errorf("expected Title %q, got %q", step.Title, result.Title)
		}
		if result.Description != step.Description {
			t.Errorf("expected Description %q, got %q", step.Description, result.Description)
		}
		if len(result.Depends) != 2 {
			t.Errorf("expected 2 dependencies, got %d", len(result.Depends))
		}
	})

	t.Run("handles empty step fields", func(t *testing.T) {
		step := Step{
			ID:          "",
			Title:       "",
			Description: "",
			Depends:     nil,
		}

		result := RenderStep(step, map[string]string{"x": "y"})

		if result.ID != "" {
			t.Errorf("expected empty ID, got %q", result.ID)
		}
		if result.Title != "" {
			t.Errorf("expected empty Title, got %q", result.Title)
		}
		if result.Description != "" {
			t.Errorf("expected empty Description, got %q", result.Description)
		}
	})
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple lowercase", "hello", "hello"},
		{"uppercase to lowercase", "Hello World", "hello-world"},
		{"spaces to hyphens", "hello world", "hello-world"},
		{"underscores to hyphens", "hello_world", "hello-world"},
		{"special characters removed", "hello@world!", "helloworld"},
		{"multiple spaces", "hello   world", "hello-world"},
		{"multiple hyphens collapsed", "hello---world", "hello-world"},
		{"leading trailing hyphens trimmed", "---hello---", "hello"},
		{"mixed case and special", "My Feature! (v2)", "my-feature-v2"},
		{"numbers preserved", "version123test", "version123test"},
		{"empty string", "", ""},
		{"only special chars", "!@#$%", ""},
		{"unicode removed", "héllo wörld", "hllo-wrld"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := "Result: {{.name | slugify}}"
			result := RenderText(input, map[string]string{"name": tc.input})
			expected := "Result: " + tc.expected
			if result != expected {
				t.Errorf("slugify(%q): expected %q, got %q", tc.input, expected, result)
			}
		})
	}
}

func TestHashComputation(t *testing.T) {
	setupTemplatesDir := func(t *testing.T) (string, func()) {
		tmpDir := t.TempDir()
		templatesDir := filepath.Join(tmpDir, ".tgz", "templates")
		if err := os.MkdirAll(templatesDir, 0755); err != nil {
			t.Fatalf("failed to create templates dir: %v", err)
		}

		originalWd, err := os.Getwd()
		if err != nil {
			t.Fatalf("failed to get working directory: %v", err)
		}

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		return templatesDir, func() { os.Chdir(originalWd) }
	}

	t.Run("same content produces same hash", func(t *testing.T) {
		templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		content := `title: Hash Test
steps:
  - id: step1
    title: Step
    description: Description
`
		if err := os.WriteFile(filepath.Join(templatesDir, "hash1.yaml"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}
		if err := os.WriteFile(filepath.Join(templatesDir, "hash2.yaml"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		tmpl1, err := LoadTemplate("hash1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tmpl2, err := LoadTemplate("hash2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tmpl1.Hash != tmpl2.Hash {
			t.Errorf("expected same hash for same content, got %q and %q", tmpl1.Hash, tmpl2.Hash)
		}
	})

	t.Run("different content produces different hash", func(t *testing.T) {
		templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		content1 := `title: Hash Test 1
steps:
  - id: step1
    title: Step
    description: Description
`
		content2 := `title: Hash Test 2
steps:
  - id: step1
    title: Step
    description: Different description
`
		if err := os.WriteFile(filepath.Join(templatesDir, "diff1.yaml"), []byte(content1), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}
		if err := os.WriteFile(filepath.Join(templatesDir, "diff2.yaml"), []byte(content2), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		tmpl1, err := LoadTemplate("diff1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tmpl2, err := LoadTemplate("diff2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tmpl1.Hash == tmpl2.Hash {
			t.Errorf("expected different hash for different content")
		}
	})

	t.Run("hash is valid hex string", func(t *testing.T) {
		templatesDir, cleanup := setupTemplatesDir(t)
		defer cleanup()

		content := `title: Hash Format Test
steps:
  - id: step1
    title: Step
    description: Description
`
		if err := os.WriteFile(filepath.Join(templatesDir, "hex.yaml"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}

		tmpl, err := LoadTemplate("hex")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// SHA256 produces 32 bytes = 64 hex characters
		if len(tmpl.Hash) != 64 {
			t.Errorf("expected 64 character hash, got %d characters", len(tmpl.Hash))
		}

		for _, c := range tmpl.Hash {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("invalid hex character in hash: %c", c)
			}
		}
	})
}
