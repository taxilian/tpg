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

// UpdatePrefixes updates task/epic prefixes in config.
func UpdatePrefixes(taskPrefix, epicPrefix string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}
	if taskPrefix != "" {
		config.Prefixes.Task = normalizePrefix(taskPrefix)
	}
	if epicPrefix != "" {
		config.Prefixes.Epic = normalizePrefix(epicPrefix)
	}
	return SaveConfig(config)
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

// mergeConfigs merges multiple configs, with later configs overriding earlier ones.
// It applies defaults after merging.
func mergeConfigs(dataDir string, configs []*Config) *Config {
	merged := &Config{}

	// Merge in order: earlier configs are base, later configs override
	for _, cfg := range configs {
		if cfg.Prefixes.Task != "" {
			merged.Prefixes.Task = cfg.Prefixes.Task
		}
		if cfg.Prefixes.Epic != "" {
			merged.Prefixes.Epic = cfg.Prefixes.Epic
		}
		if cfg.DefaultProject != "" {
			merged.DefaultProject = cfg.DefaultProject
		}
		if cfg.IDLength != 0 {
			merged.IDLength = cfg.IDLength
		}
		// Merge custom prefixes - later configs override individual entries
		if cfg.CustomPrefixes != nil {
			if merged.CustomPrefixes == nil {
				merged.CustomPrefixes = make(map[string]string)
			}
			for k, v := range cfg.CustomPrefixes {
				merged.CustomPrefixes[k] = v
			}
		}
	}

	applyDefaults(merged, dataDir)
	return merged
}

// loadConfigFromPath loads a config from a specific file path.
// Returns nil config if file doesn't exist.
func loadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config from %s: %w", path, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config from %s: %w", path, err)
	}
	return &config, nil
}

// LoadMergedConfigWithPaths loads and merges configs from the specified paths.
// Later configs override earlier ones. Missing files are skipped gracefully.
// Returns error if any config file has invalid JSON.
func LoadMergedConfigWithPaths(paths ...string) (*Config, error) {
	var configs []*Config

	for _, path := range paths {
		cfg, err := loadConfigFromPath(path)
		if err != nil {
			return nil, err
		}
		if cfg != nil {
			configs = append(configs, cfg)
		}
	}

	// Get data directory for defaults - use the last path's directory or current
	dataDir := "."
	if len(paths) > 0 {
		dataDir = filepath.Dir(filepath.Dir(paths[len(paths)-1]))
	}

	return mergeConfigs(dataDir, configs), nil
}

// LoadMergedConfig loads and merges configs from standard locations:
// - System config (e.g., /etc/tpg/config.json)
// - User config (e.g., ~/.config/tpg/config.json)
// - Worktree config (found by searching upward from current directory)
// Later configs override earlier ones.
func LoadMergedConfig() (*Config, error) {
	// Find worktree data directory
	dataDir, err := findDataDir()
	if err != nil {
		return nil, err
	}

	// Build list of paths to check
	var paths []string

	// System config
	if sysPath := os.Getenv("TPG_SYSTEM_CONFIG"); sysPath != "" {
		paths = append(paths, sysPath)
	}

	// User config
	if home, err := os.UserHomeDir(); err == nil {
		userConfigPath := filepath.Join(home, ".config", "tpg", ConfigFile)
		paths = append(paths, userConfigPath)
	}

	// Worktree config (highest priority)
	paths = append(paths, filepath.Join(dataDir, ConfigFile))

	// Load and merge
	merged, err := LoadMergedConfigWithPaths(paths...)
	if err != nil {
		return nil, err
	}

	// Re-apply defaults with correct dataDir
	applyDefaults(merged, dataDir)
	return merged, nil
}
