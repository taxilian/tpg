package templates

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

const templatesDirName = "templates"

// Template defines a task template.
type Template struct {
	ID          string              `json:"-"`
	Title       string              `yaml:"title" toml:"title"`
	Description string              `yaml:"description" toml:"description"`
	Worktree    bool                `yaml:"worktree" toml:"worktree"` // When true, creates epic with worktree
	Variables   map[string]Variable `yaml:"variables" toml:"variables"`
	Steps       []Step              `yaml:"steps" toml:"steps"`
	SourcePath  string              `json:"-"`
	Hash        string              `json:"-"`
	Source      string              `json:"-"` // "project", "user", or "global"
}

// Variable defines a template variable.
// Variables are required by default. Set optional: true to make them optional.
type Variable struct {
	Description string `yaml:"description" toml:"description"`
	Optional    bool   `yaml:"optional" toml:"optional"`
	Default     string `yaml:"default" toml:"default"`
}

// Step defines a template step.
type Step struct {
	ID          string   `yaml:"id" toml:"id"`
	Title       string   `yaml:"title" toml:"title"`
	Description string   `yaml:"description" toml:"description"`
	Depends     []string `yaml:"depends" toml:"depends"`
}

// TemplateLocation represents a directory that may contain templates.
type TemplateLocation struct {
	Path   string
	Source string // "project", "user", or "global"
}

// GetTemplateLocations returns all template directories in priority order (most local first).
// Priority: worktree-local (.tpg/templates) > worktree-root (.tpg/templates) > user (~/.config/tpg/templates) > global (~/.config/opencode/tpg-templates)
func GetTemplateLocations() []TemplateLocation {
	var locations []TemplateLocation

	// 1. Worktree-local: search upward for .tpg/templates
	if localDir, err := findProjectTemplatesDir(); err == nil {
		locations = append(locations, TemplateLocation{Path: localDir, Source: "project"})

		// 2. Worktree-root: if we're in a worktree, also include the main repo's templates
		// Only add worktree root if it's different from the local dir (i.e., we're actually in a worktree)
		if worktreeRoot, err := findWorktreeRoot(); err == nil && worktreeRoot != "" {
			// Check for .tpg/templates in worktree root
			rootTemplatesDir := filepath.Join(worktreeRoot, ".tpg", templatesDirName)
			if info, err := os.Stat(rootTemplatesDir); err == nil && info.IsDir() {
				// Only add if it's a different directory than the local one
				if rootTemplatesDir != localDir {
					locations = append(locations, TemplateLocation{Path: rootTemplatesDir, Source: "project"})
				}
			}
			// Also check .tgz/templates for backward compatibility
			rootTemplatesDir = filepath.Join(worktreeRoot, ".tgz", templatesDirName)
			if info, err := os.Stat(rootTemplatesDir); err == nil && info.IsDir() {
				if rootTemplatesDir != localDir {
					locations = append(locations, TemplateLocation{Path: rootTemplatesDir, Source: "project"})
				}
			}
		}
	}

	// 3. User config: ~/.config/tpg/templates
	if home, err := os.UserHomeDir(); err == nil {
		userDir := filepath.Join(home, ".config", "tpg", "templates")
		if info, err := os.Stat(userDir); err == nil && info.IsDir() {
			locations = append(locations, TemplateLocation{Path: userDir, Source: "user"})
		}
	}

	// 4. Global/Opencode: ~/.config/opencode/tpg-templates
	if home, err := os.UserHomeDir(); err == nil {
		globalDir := filepath.Join(home, ".config", "opencode", "tpg-templates")
		if info, err := os.Stat(globalDir); err == nil && info.IsDir() {
			locations = append(locations, TemplateLocation{Path: globalDir, Source: "global"})
		}
	}

	return locations
}

// findProjectTemplatesDir searches upward from CWD for .tpg/templates (note: .tpg not .tgz)
func findProjectTemplatesDir() (string, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := startDir
	for {
		// Check .tpg/templates first (new location)
		candidate := filepath.Join(dir, ".tpg", templatesDirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		// Also check .tgz/templates for backward compatibility
		candidate = filepath.Join(dir, ".tgz", templatesDirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no templates directory found")
		}
		dir = parent
	}
}

// findWorktreeRoot detects if the current directory is in a git worktree and returns
// the main repository root path. If .git is a directory (regular repo) or doesn't exist,
// it returns an empty string.
func findWorktreeRoot() (string, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Search upward to find .git file or directory
	dir := startDir
	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if info.IsDir() {
				// Regular repo - not a worktree
				return "", nil
			}
			// It's a file - this is a worktree
			return parseGitFile(gitPath)
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check %s: %w", gitPath, err)
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .git
			return "", nil
		}
		dir = parent
	}
}

// parseGitFile parses a .git file (used by worktrees) and extracts the main repo path.
// The file format is: "gitdir: <path>" where path points to the main repo's .git directory.
func parseGitFile(gitFilePath string) (string, error) {
	file, err := os.Open(gitFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open .git file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("failed to read .git file: %w", err)
		}
		return "", fmt.Errorf(".git file is empty")
	}

	line := strings.TrimSpace(scanner.Text())

	// Parse "gitdir: <path>" format
	const prefix = "gitdir:"
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("malformed .git file: missing gitdir prefix")
	}

	gitDir := strings.TrimSpace(line[len(prefix):])
	if gitDir == "" {
		return "", fmt.Errorf("malformed .git file: empty gitdir path")
	}

	// Extract the main repo path from the gitdir path
	// gitdir points to something like /path/to/repo/.git or /path/to/repo/.git/worktrees/myworktree
	// We need to find the main repo root (parent of .git)

	// If it's a worktrees path, go up to find the main .git directory
	gitDir = filepath.Clean(gitDir)

	// Walk up through "worktrees" directories to find the main .git
	for strings.Contains(gitDir, "worktrees") {
		parent := filepath.Dir(gitDir)
		if parent == gitDir {
			break
		}
		gitDir = parent
	}

	// Now gitDir should be the main .git directory
	// The repo root is the parent of .git
	repoRoot := filepath.Dir(gitDir)

	return repoRoot, nil
}

// ListTemplates returns all available templates from all locations.
// Templates from more local locations override those from more global locations.
// Searches recursively through subdirectories within each template location.
func ListTemplates() ([]*Template, error) {
	locations := GetTemplateLocations()
	if len(locations) == 0 {
		return nil, nil // No templates directories found, return empty list
	}

	// Map of template ID to template (later entries don't override earlier ones)
	seen := make(map[string]*Template)

	for _, loc := range locations {
		// Walk the directory tree recursively
		_ = filepath.Walk(loc.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors, continue walking
			}
			if info.IsDir() {
				return nil // Continue into directories
			}

			ext := strings.ToLower(filepath.Ext(info.Name()))
			if ext != ".yaml" && ext != ".yml" && ext != ".toml" {
				return nil
			}

			id := strings.TrimSuffix(info.Name(), ext)
			if _, exists := seen[id]; exists {
				// Already have this template from a higher-priority location
				return nil
			}

			tmpl, err := loadTemplateFromPath(path, id, loc.Source)
			if err != nil {
				// Skip invalid templates in listing
				return nil
			}
			seen[id] = tmpl
			return nil
		})
	}

	// Convert to sorted slice
	var result []*Template
	for _, tmpl := range seen {
		result = append(result, tmpl)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

// LoadTemplate loads a template by ID, searching all template locations.
func LoadTemplate(id string) (*Template, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("template id is required")
	}

	locations := GetTemplateLocations()
	if len(locations) == 0 {
		startDir, _ := os.Getwd()
		return nil, fmt.Errorf("no templates directory found in %s or any ancestor, ~/.config/tpg/templates, or ~/.config/opencode/tpg-templates", startDir)
	}

	// Search in priority order
	for _, loc := range locations {
		path, err := findTemplatePathInDir(loc.Path, id)
		if err != nil {
			continue
		}
		return loadTemplateFromPath(path, id, loc.Source)
	}

	return nil, fmt.Errorf("template not found: %s", id)
}

func findTemplatePathInDir(dir, id string) (string, error) {
	// First check the root directory (fast path)
	candidates := []string{
		filepath.Join(dir, id+".yaml"),
		filepath.Join(dir, id+".yml"),
		filepath.Join(dir, id+".toml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Search recursively through subdirectories
	var foundPath string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if ext != ".yaml" && ext != ".yml" && ext != ".toml" {
			return nil
		}

		fileID := strings.TrimSuffix(info.Name(), ext)
		if fileID == id {
			foundPath = path
			return filepath.SkipAll // Stop walking, we found it
		}
		return nil
	})

	if foundPath != "" {
		return foundPath, nil
	}
	return "", fmt.Errorf("template not found: %s", id)
}

func loadTemplateFromPath(path, id, source string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}

	hash := sha256.Sum256(data)

	var tmpl Template
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			return nil, fmt.Errorf("failed to parse yaml template: %w", err)
		}
	case ".toml":
		if err := toml.Unmarshal(data, &tmpl); err != nil {
			return nil, fmt.Errorf("failed to parse toml template: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported template extension: %s", filepath.Ext(path))
	}

	if len(tmpl.Steps) == 0 {
		return nil, fmt.Errorf("template has no steps")
	}

	// Check for duplicate explicit step IDs
	seen := map[string]bool{}
	for _, step := range tmpl.Steps {
		if step.ID == "" {
			continue
		}
		if seen[step.ID] {
			return nil, fmt.Errorf("duplicate step id: %s", step.ID)
		}
		seen[step.ID] = true
	}

	tmpl.ID = id
	tmpl.SourcePath = path
	tmpl.Source = source
	tmpl.Hash = hex.EncodeToString(hash[:])
	if tmpl.Variables == nil {
		tmpl.Variables = map[string]Variable{}
	}
	return &tmpl, nil
}

// templateFuncs provides helper functions for templates.
var templateFuncs = template.FuncMap{
	// hasValue returns true if the value is non-empty
	"hasValue": func(s string) bool {
		return strings.TrimSpace(s) != ""
	},
	// default returns the value if non-empty, otherwise the default
	"default": func(defaultVal, val string) string {
		if strings.TrimSpace(val) != "" {
			return val
		}
		return defaultVal
	},
	// slugify converts a string to a URL/branch-friendly slug
	"slugify": func(s string) string {
		// Convert to lowercase
		s = strings.ToLower(s)
		// Replace spaces and underscores with hyphens
		s = strings.ReplaceAll(s, " ", "-")
		s = strings.ReplaceAll(s, "_", "-")
		// Remove any character that isn't alphanumeric or hyphen
		var result strings.Builder
		for _, r := range s {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				result.WriteRune(r)
			}
		}
		// Collapse multiple hyphens into one
		s = result.String()
		for strings.Contains(s, "--") {
			s = strings.ReplaceAll(s, "--", "-")
		}
		// Trim leading/trailing hyphens
		s = strings.Trim(s, "-")
		return s
	},
}

// RenderText interpolates variables using Go's text/template.
// Supports conditionals: {{if .var}}...{{end}}
// Supports defaults: {{default "fallback" .var}}
// Supports hasValue: {{if hasValue .var}}...{{end}}
func RenderText(input string, vars map[string]string) string {
	if vars == nil {
		vars = map[string]string{}
	}

	tmpl, err := template.New("").Funcs(templateFuncs).Parse(input)
	if err != nil {
		// Return input unchanged if template parsing fails
		return input
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		// Return input unchanged on execution error
		return input
	}

	return buf.String()
}

// RenderStep interpolates variables into a step.
func RenderStep(step Step, vars map[string]string) Step {
	return Step{
		ID:          step.ID,
		Title:       RenderText(step.Title, vars),
		Description: RenderText(step.Description, vars),
		Depends:     step.Depends,
	}
}
