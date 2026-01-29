package prime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taxilian/tpg/internal/db"
)

func TestRenderPrime_DefaultTemplate(t *testing.T) {
	data := PrimeData{
		Open:       5,
		InProgress: 2,
		Blocked:    1,
		Done:       10,
		Ready:      3,
		Project:    "testproject",
		HasDB:      true,
	}

	output, err := RenderPrime(DefaultPrimeTemplate(), data)
	if err != nil {
		t.Fatalf("RenderPrime failed: %v", err)
	}

	// Check key sections are present
	if !strings.Contains(output, "tpg") {
		t.Error("Output should contain 'tpg'")
	}
	if !strings.Contains(output, "3 ready") {
		t.Error("Output should contain ready count")
	}
	if !strings.Contains(output, "10 done, 5 open") {
		t.Error("Output should contain status counts")
	}
}

func TestRenderPrime_WithAgentInProgress(t *testing.T) {
	data := PrimeData{
		Open:       5,
		InProgress: 2,
		Ready:      3,
		HasDB:      true,
		MyInProgItems: []PrimeItem{
			{ID: "ts-123456", Title: "Fix bug", Priority: 1},
			{ID: "ts-789abc", Title: "Add feature", Priority: 2},
		},
		OtherInProgCount: 1,
	}

	output, err := RenderPrime(DefaultPrimeTemplate(), data)
	if err != nil {
		t.Fatalf("RenderPrime failed: %v", err)
	}

	// Should show agent's in-progress items
	if !strings.Contains(output, "ts-123456") {
		t.Error("Output should contain agent's task ID")
	}
	if !strings.Contains(output, "Fix bug") {
		t.Error("Output should contain agent's task title")
	}
	if !strings.Contains(output, "1 in progress (other agents)") {
		t.Error("Output should mention other agents' work")
	}
}

func TestRenderPrime_WithKnowledge(t *testing.T) {
	data := PrimeData{
		HasDB:         true,
		ConceptCount:  5,
		LearningCount: 12,
	}

	output, err := RenderPrime(DefaultPrimeTemplate(), data)
	if err != nil {
		t.Fatalf("RenderPrime failed: %v", err)
	}

	if !strings.Contains(output, "12 learnings in 5 concepts") {
		t.Error("Output should contain knowledge stats")
	}
}

func TestRenderPrime_NoDB(t *testing.T) {
	data := PrimeData{
		HasDB: false,
	}

	output, err := RenderPrime(DefaultPrimeTemplate(), data)
	if err != nil {
		t.Fatalf("RenderPrime failed: %v", err)
	}

	if !strings.Contains(output, "No database - run 'tpg init'") {
		t.Error("Output should indicate no database")
	}
}

func TestRenderPrime_CustomTemplate(t *testing.T) {
	template := `Project: {{.Project}}
Open: {{.Open}}
Done: {{.Done}}
Total: {{add .Open .Done}}`

	data := PrimeData{
		Project: "myproject",
		Open:    5,
		Done:    10,
	}

	output, err := RenderPrime(template, data)
	if err != nil {
		t.Fatalf("RenderPrime failed: %v", err)
	}

	if !strings.Contains(output, "Project: myproject") {
		t.Error("Output should contain project name")
	}
	if !strings.Contains(output, "Total: 15") {
		t.Error("Output should calculate total using add function")
	}
}

func TestRenderPrime_TemplateFunctions(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     PrimeData
		want     string
	}{
		{
			name:     "add function",
			template: "{{add 5 3}}",
			data:     PrimeData{},
			want:     "8",
		},
		{
			name:     "sub function",
			template: "{{sub 10 3}}",
			data:     PrimeData{},
			want:     "7",
		},
		{
			name:     "plural singular",
			template: `{{plural 1 "task" "tasks"}}`,
			data:     PrimeData{},
			want:     "task",
		},
		{
			name:     "plural multiple",
			template: `{{plural 5 "task" "tasks"}}`,
			data:     PrimeData{},
			want:     "tasks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := RenderPrime(tt.template, tt.data)
			if err != nil {
				t.Fatalf("RenderPrime failed: %v", err)
			}
			if strings.TrimSpace(output) != tt.want {
				t.Errorf("got %q, want %q", strings.TrimSpace(output), tt.want)
			}
		})
	}
}

func TestRenderPrime_InvalidTemplate(t *testing.T) {
	template := "{{.InvalidField}}"
	data := PrimeData{}

	_, err := RenderPrime(template, data)
	if err == nil {
		t.Error("Expected error for invalid template field")
	}
}

func TestRenderPrime_TemplateSyntaxError(t *testing.T) {
	template := "{{.Open"
	data := PrimeData{}

	_, err := RenderPrime(template, data)
	if err == nil {
		t.Error("Expected error for template syntax error")
	}
}

func TestGetPrimeLocations(t *testing.T) {
	locations := GetPrimeLocations()

	if len(locations) < 2 {
		t.Error("Should return at least 2 locations (user + global)")
	}

	// Check that user config location is included
	home, _ := os.UserHomeDir()
	userPath := filepath.Join(home, ".config", "tpg", PrimeFileName)
	found := false
	for _, loc := range locations {
		if loc == userPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("User config path %s not in locations", userPath)
	}
}

func TestLoadPrimeTemplate_NoTemplateFound(t *testing.T) {
	// Save current directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)

	// Create temp directory and cd into it
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	templateText, source, err := LoadPrimeTemplate()
	if err != nil {
		t.Fatalf("LoadPrimeTemplate returned error: %v", err)
	}

	if templateText != "" {
		t.Error("Should return empty string when no template found")
	}
	if source != "" {
		t.Error("Should return empty source when no template found")
	}
}

func TestLoadPrimeTemplate_ProjectTemplate(t *testing.T) {
	// Save current directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)

	// Create temp directory structure
	tmpDir := t.TempDir()
	tpgDir := filepath.Join(tmpDir, ".tpg")
	os.MkdirAll(tpgDir, 0755)

	// Write template
	content := "Test template content"
	primePath := filepath.Join(tpgDir, PrimeFileName)
	os.WriteFile(primePath, []byte(content), 0644)

	// Change to temp directory
	os.Chdir(tmpDir)

	templateText, source, err := LoadPrimeTemplate()
	if err != nil {
		t.Fatalf("LoadPrimeTemplate returned error: %v", err)
	}

	if templateText != content {
		t.Errorf("got %q, want %q", templateText, content)
	}
	// Just check that source contains the file name, not exact path (varies by OS)
	if !strings.Contains(source, PrimeFileName) {
		t.Errorf("source should contain %q, got %q", PrimeFileName, source)
	}
}

func TestBuildPrimeData_NilInputs(t *testing.T) {
	data := BuildPrimeData(nil, nil, db.AgentContext{}, nil)

	if data.HasDB {
		t.Error("HasDB should be false when report is nil")
	}
}

func TestBuildPrimeData_WithReport(t *testing.T) {
	report := &db.StatusReport{
		Open:             5,
		InProgress:       2,
		Blocked:          1,
		Done:             10,
		Ready:            3,
		Project:          "testproj",
		OtherInProgCount: 1,
	}

	config := &db.Config{
		Prefixes: struct {
			Task string `json:"task"`
			Epic string `json:"epic"`
		}{
			Task: "ts",
			Epic: "ep",
		},
		DefaultProject: "default",
	}

	agentCtx := db.AgentContext{
		ID:   "agent-123",
		Type: "general",
	}

	data := BuildPrimeData(report, config, agentCtx, nil)

	if !data.HasDB {
		t.Error("HasDB should be true when report is not nil")
	}
	if data.Open != 5 {
		t.Errorf("Open = %d, want 5", data.Open)
	}
	if data.InProgress != 2 {
		t.Errorf("InProgress = %d, want 2", data.InProgress)
	}
	if data.Project != "testproj" {
		t.Errorf("Project = %q, want %q", data.Project, "testproj")
	}
	if data.TaskPrefix != "ts" {
		t.Errorf("TaskPrefix = %q, want %q", data.TaskPrefix, "ts")
	}
	if data.AgentID != "agent-123" {
		t.Errorf("AgentID = %q, want %q", data.AgentID, "agent-123")
	}
	if data.OtherInProgCount != 1 {
		t.Errorf("OtherInProgCount = %d, want 1", data.OtherInProgCount)
	}
}

func TestDefaultPrimeTemplate_NotEmpty(t *testing.T) {
	template := DefaultPrimeTemplate()
	if template == "" {
		t.Error("Default template should not be empty")
	}

	// Should contain key workflow commands
	if !strings.Contains(template, "tpg ready") {
		t.Error("Default template should mention 'tpg ready'")
	}
	if !strings.Contains(template, "tpg start") {
		t.Error("Default template should mention 'tpg start'")
	}
	if !strings.Contains(template, "tpg done") {
		t.Error("Default template should mention 'tpg done'")
	}
}
