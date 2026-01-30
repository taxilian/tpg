package prime

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/templates"
)

const PrimeFileName = "PRIME.md"

// PrimeData contains all data available to prime templates
type PrimeData struct {
	// Status counts
	Open       int
	InProgress int
	Blocked    int
	Done       int
	Canceled   int
	Ready      int

	// Current work samples (limited to avoid overwhelming output)
	MyInProgItems    []PrimeItem
	OtherInProgCount int
	BlockedCount     int

	// Stale items (in-progress with no updates > 5 min)
	StaleItems []PrimeItem
	StaleCount int

	// Config
	Project        string
	TaskPrefix     string
	EpicPrefix     string
	DefaultProject string
	HasDB          bool

	// Agent context
	AgentID    string
	AgentType  string
	IsSubagent bool

	// Subagent-specific: tasks assigned to this subagent session
	SubagentTasks     []PrimeItem // in-progress tasks assigned to this subagent
	SubagentTaskCount int

	// Knowledge base stats
	ConceptCount  int
	LearningCount int

	// Available templates
	Templates     []*templates.Template
	TemplateCount int
}

// PrimeItem is a simplified view of model.Item for templates
type PrimeItem struct {
	ID       string
	Title    string
	Priority int
}

// GetPrimeLocations returns paths to check for PRIME.md (most local first)
func GetPrimeLocations() []string {
	var locations []string

	// 1. Project: .tpg/PRIME.md (search upward)
	if projectPath := findProjectPrime(); projectPath != "" {
		locations = append(locations, projectPath)
	}

	// 2. User: ~/.config/tpg/PRIME.md
	if home, err := os.UserHomeDir(); err == nil {
		locations = append(locations, filepath.Join(home, ".config", "tpg", PrimeFileName))
	}

	// 3. Global: ~/.config/opencode/tpg-prime.md
	if home, err := os.UserHomeDir(); err == nil {
		locations = append(locations, filepath.Join(home, ".config", "opencode", "tpg-prime.md"))
	}

	return locations
}

func findProjectPrime() string {
	dir, _ := os.Getwd()
	for {
		candidate := filepath.Join(dir, ".tpg", PrimeFileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// LoadPrimeTemplate loads custom template from search locations
// Returns template text, source path, and error
// Returns empty string for template if no custom template found (not an error)
func LoadPrimeTemplate() (string, string, error) {
	for _, path := range GetPrimeLocations() {
		if data, err := os.ReadFile(path); err == nil {
			return string(data), path, nil
		}
	}
	return "", "", nil // No custom template found
}

// RenderPrime renders the prime template with given data
func RenderPrime(templateText string, data PrimeData) (string, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"plural": func(count int, singular, plural string) string {
			if count == 1 {
				return singular
			}
			return plural
		},
	}

	tmpl, err := template.New("prime").Funcs(funcMap).Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

// DefaultPrimeTemplate returns the condensed default template
func DefaultPrimeTemplate() string {
	return `# TPG Context

This project uses **tpg** for cross-session task management.
{{if .Project}}Project: {{.Project}}{{else if .DefaultProject}}Default: {{.DefaultProject}}{{end}}

## Status
{{if not .HasDB -}}
No database - run 'tpg init'
{{else -}}
{{if gt .StaleCount 0 -}}
**⚠️ STALE ({{.StaleCount}} task{{if ne .StaleCount 1}}s{{end}} with no updates >5min):**
{{if gt (len .StaleItems) 0 -}}
{{range .StaleItems}}  • [{{.ID}}] {{.Title}}
{{end}}
{{else -}}
  (too many to list - run 'tpg stale')
{{end}}
{{end -}}
{{if gt (len .MyInProgItems) 0 -}}
**Your work:**
{{range .MyInProgItems}}  • [{{.ID}}] {{.Title}}{{if eq .Priority 1}} ⚡{{end}}
{{end}}
{{end -}}
- {{.Ready}} ready (use 'tpg ready')
{{if gt .OtherInProgCount 0}}- {{.OtherInProgCount}} in progress (other agents){{end}}
{{if gt .Blocked 0}}- {{.Blocked}} blocked{{end}}
- {{.Done}} done, {{.Open}} open
{{if gt .ConceptCount 0}}- {{.LearningCount}} learnings in {{.ConceptCount}} concepts{{end}}
{{end}}
{{if .IsSubagent -}}
{{if gt .SubagentTaskCount 0 -}}

## Your Assigned Task{{if gt .SubagentTaskCount 1}}s{{end}}
{{if eq .SubagentTaskCount 1 -}}
You have 1 task assigned to this session:
{{range .SubagentTasks}}  • [{{.ID}}] {{.Title}}{{if eq .Priority 1}} ⚡{{end}}
{{end}}
{{else -}}
You have {{.SubagentTaskCount}} tasks assigned to this session:
{{range .SubagentTasks}}  • [{{.ID}}]{{end}}

**Note:** You have multiple tasks assigned. Unless you're intentionally working on them together, consider reviewing to finish or close some:
  tpg done <id> "completed"
  tpg cancel <id>
{{end -}}
{{end -}}
{{end -}}

## Workflow

**Start:** 'tpg ready' → 'tpg show <id>' → 'tpg start <id>'
**During:** 'tpg log <id> "msg"' — log significant milestones as you work
**Finish:** 'tpg done <id>' | blocked? → 'tpg dep <blocker> blocks <id>'
**Context:** 'tpg concepts' → 'tpg context -c <name>'

**Logging:** You MUST call 'tpg log <id> "msg"' when any of these happen:
- You discover a blocker or create a dependency
- You choose between alternatives (log what and why)
- You find existing code/patterns that change your approach
- You answer a key unknown that required searching or testing to resolve
- You hit something unexpected (error, missing API, wrong assumption)
- You finish a key milestone (core logic works, tests pass)
Do NOT log routine actions (opened file, read docs, ran a command).
If you complete a task with zero logs, 'tpg done' will warn you.

## Templates

{{if gt .TemplateCount 0 -}}
Available templates ({{.TemplateCount}}):
{{range .Templates}}  {{.ID}} ({{len .Variables}} vars): {{.Description}}
{{end}}
Use:
  tpg add "Title" --template <id> --vars-yaml <<EOF
  problem: "What we're solving"
  requirements: |
    - Specific requirement 1
    - Specific requirement 2
  context: "Any helpful background or constraints"
  EOF
{{else -}}
No templates found. Create templates in .tpg/templates/ to standardize workflows.
{{end -}}

## Key Commands

  tpg ready                        # Available work
  tpg show <id>                    # Task details
  tpg start <id>                   # Claim task
  tpg log <id> "msg"               # Log progress
  tpg done <id>                    # Complete
  tpg dep <id> blocks <other-id>   # Set dependency
  tpg dep <id> list                # Show dependencies
  tpg status                       # Overview
  tpg context -c X                 # Load learnings
`
}

// BuildPrimeData constructs PrimeData from database queries and config
func BuildPrimeData(report *db.StatusReport, config *db.Config, agentCtx db.AgentContext, database *db.DB) PrimeData {
	data := PrimeData{
		HasDB:      report != nil,
		AgentID:    agentCtx.ID,
		AgentType:  agentCtx.Type,
		IsSubagent: agentCtx.IsSubagent(),
	}

	if config != nil {
		data.TaskPrefix = config.Prefixes.Task
		data.EpicPrefix = config.Prefixes.Epic
		data.DefaultProject = config.DefaultProject
	}

	if report != nil {
		data.Open = report.Open
		data.InProgress = report.InProgress
		data.Blocked = report.Blocked
		data.Done = report.Done
		data.Canceled = report.Canceled
		data.Ready = report.Ready
		data.Project = report.Project
		data.OtherInProgCount = report.OtherInProgCount
		data.BlockedCount = len(report.BlockedItems)

		// Convert agent's items to PrimeItem
		for _, item := range report.MyInProgItems {
			data.MyInProgItems = append(data.MyInProgItems, PrimeItem{
				ID:       item.ID,
				Title:    item.Title,
				Priority: item.Priority,
			})
		}

		// Convert stale items (limit to 20 for display)
		data.StaleCount = len(report.StaleItems)
		staleLimit := 20
		if data.StaleCount <= staleLimit {
			for _, item := range report.StaleItems {
				data.StaleItems = append(data.StaleItems, PrimeItem{
					ID:       item.ID,
					Title:    item.Title,
					Priority: item.Priority,
				})
			}
		}
	}

	// If this is a subagent, get tasks assigned to this specific session
	if data.IsSubagent && agentCtx.ID != "" && database != nil {
		if tasks, err := database.InProgressItemsByAgent(agentCtx.ID); err == nil {
			data.SubagentTaskCount = len(tasks)
			for _, item := range tasks {
				data.SubagentTasks = append(data.SubagentTasks, PrimeItem{
					ID:       item.ID,
					Title:    item.Title,
					Priority: item.Priority,
				})
			}
		}
	}

	// Get knowledge base stats
	if database != nil && report != nil {
		if count, err := database.GetConceptCount(report.Project); err == nil {
			data.ConceptCount = count
		}
		if count, err := database.GetLearningCount(report.Project); err == nil {
			data.LearningCount = count
		}
	}

	// Get available templates
	if tmplList, err := templates.ListTemplates(); err == nil {
		data.Templates = tmplList
		data.TemplateCount = len(tmplList)
	}

	return data
}
