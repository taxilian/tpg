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
	Prefixes       PrefixConfig `json:"prefixes"`
	DefaultProject string       `json:"default_project"`
	IDLength       int          `json:"id_length,omitempty"`
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
