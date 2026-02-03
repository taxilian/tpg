package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/taxilian/tpg/internal/model"
)

const (
	DefaultTaskPrefix = "ts"
	DefaultEpicPrefix = "ep"
)

// Config holds per-project settings stored in .tpg/config.json.
type Config struct {
	Prefixes       PrefixConfig      `json:"prefixes"`
	CustomPrefixes map[string]string `json:"custom_prefixes"`
	DefaultProject string            `json:"default_project"`
	IDLength       int               `json:"id_length,omitempty"`
	Warnings       WarningsConfig    `json:"warnings,omitempty"`
	Worktree       WorktreeConfig    `json:"worktree,omitempty"`
}

// WarningsConfig controls which warnings are shown.
type WarningsConfig struct {
	// ShortDescription warns when description has fewer than MinDescriptionWords words.
	// Set to false to disable. Default is true.
	ShortDescription *bool `json:"short_description,omitempty"`
	// MinDescriptionWords is the minimum word count before warning. Default is 15.
	MinDescriptionWords int `json:"min_description_words,omitempty"`
}

// WorktreeConfig holds settings for Git worktree integration.
type WorktreeConfig struct {
	BranchPrefix  string `json:"branch_prefix,omitempty"`   // Default "feature"
	RequireEpicID *bool  `json:"require_epic_id,omitempty"` // Default true
	Root          string `json:"root,omitempty"`            // Default ".worktrees"
}

// DefaultMinDescriptionWords is the default threshold for short description warnings.
const DefaultMinDescriptionWords = 15

// ShortDescriptionWarningEnabled returns whether short description warnings are enabled.
func (c *Config) ShortDescriptionWarningEnabled() bool {
	if c.Warnings.ShortDescription == nil {
		return true // default to enabled
	}
	return *c.Warnings.ShortDescription
}

// GetMinDescriptionWords returns the minimum word count for descriptions.
func (c *Config) GetMinDescriptionWords() int {
	if c.Warnings.MinDescriptionWords <= 0 {
		return DefaultMinDescriptionWords
	}
	return c.Warnings.MinDescriptionWords
}

// PrefixConfig holds ID prefixes for items.
type PrefixConfig struct {
	Task string `json:"task"`
	Epic string `json:"epic"`
}

func defaultProjectName(dataDir string) string {
	root := filepath.Dir(dataDir)
	name := filepath.Base(root)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "default"
	}
	return name
}

func normalizePrefix(prefix string) string {
	p := strings.TrimSpace(prefix)
	p = strings.TrimSuffix(p, "-")
	return p
}

func applyDefaults(config *Config, dataDir string) {
	if config.Prefixes.Task == "" {
		config.Prefixes.Task = DefaultTaskPrefix
	}
	if config.Prefixes.Epic == "" {
		config.Prefixes.Epic = DefaultEpicPrefix
	}
	if config.DefaultProject == "" {
		config.DefaultProject = defaultProjectName(dataDir)
	}
	if config.IDLength == 0 {
		config.IDLength = model.DefaultIDLength
	}
	// Worktree defaults
	if config.Worktree.BranchPrefix == "" {
		config.Worktree.BranchPrefix = "feature"
	}
	if config.Worktree.Root == "" {
		config.Worktree.Root = ".worktrees"
	}
	if config.Worktree.RequireEpicID == nil {
		defaultRequire := true
		config.Worktree.RequireEpicID = &defaultRequire
	}
}

// LoadConfig reads the project config from .tpg/config.json.
// If no config exists, defaults are returned.
func LoadConfig() (*Config, error) {
	dataDir, err := findDataDir()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(dataDir, ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			config := &Config{}
			applyDefaults(config, dataDir)
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	applyDefaults(&config, dataDir)
	return &config, nil
}

func saveConfigAt(dataDir string, config *Config) error {
	applyDefaults(config, dataDir)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}
	configPath := filepath.Join(dataDir, ConfigFile)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// SaveConfig writes the project config to .tpg/config.json.
func SaveConfig(config *Config) error {
	dataDir, err := findDataDir()
	if err != nil {
		return err
	}
	return saveConfigAt(dataDir, config)
}

// InitProject creates the .tpg directory in the current directory and writes config.
func InitProject(taskPrefix, epicPrefix string) (string, error) {
	dataDir, err := dataDirFromCwd()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create %s directory: %w", DataDir, err)
	}

	config := &Config{}
	if taskPrefix != "" {
		config.Prefixes.Task = normalizePrefix(taskPrefix)
	}
	if epicPrefix != "" {
		config.Prefixes.Epic = normalizePrefix(epicPrefix)
	}
	applyDefaults(config, dataDir)

	if err := saveConfigAt(dataDir, config); err != nil {
		return "", err
	}

	return filepath.Join(dataDir, DBFile), nil
}

// DefaultProject returns the default project name.
func DefaultProject() (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return config.DefaultProject, nil
}

// GetPrefixForType returns the prefix for a given item type.
// Checks custom_prefixes first, then falls back to default prefixes.
func (c *Config) GetPrefixForType(itemType string) string {
	// Check custom prefixes first
	if c.CustomPrefixes != nil {
		if prefix, ok := c.CustomPrefixes[itemType]; ok && prefix != "" {
			return normalizePrefix(prefix)
		}
	}
	// Fall back to default prefixes
	switch itemType {
	case "task":
		return c.Prefixes.Task
	case "epic":
		return c.Prefixes.Epic
	default:
		// For unknown types, return a generic prefix
		return "it"
	}
}

// RequireEpicIDEnabled returns whether explicit branch names must include the epic ID.
func (c WorktreeConfig) RequireEpicIDEnabled() bool {
	if c.RequireEpicID == nil {
		return true
	}
	return *c.RequireEpicID
}
