package db

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DataDir is the per-project data directory name.
	DataDir = ".tpg"
	// DBFile is the database filename inside DataDir.
	DBFile = "tpg.db"
	// ConfigFile is the project config filename inside DataDir.
	ConfigFile = "config.json"
)

// dataDirFromCwd returns the data directory path in the current working directory.
func dataDirFromCwd() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return filepath.Join(wd, DataDir), nil
}

// findDataDir searches upward from the current working directory to locate DataDir.
func findDataDir() (string, error) {
	startDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	dir := startDir
	for {
		candidate := filepath.Join(dir, DataDir)
		info, err := os.Stat(candidate)
		if err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("%s exists but is not a directory", candidate)
			}
			return candidate, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check %s: %w", candidate, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s directory found in %s or any ancestor. Run 'tpg init' first", DataDir, startDir)
		}
		dir = parent
	}
}

// DefaultPath returns the default database path.
// It searches upward from the current directory for DataDir.
// Can be overridden with TPG_DB environment variable.
func DefaultPath() (string, error) {
	if envPath := os.Getenv("TPG_DB"); envPath != "" {
		return envPath, nil
	}
	dataDir, err := findDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, DBFile), nil
}

// InitPath returns the database path for initializing a project in the current directory.
func InitPath() (string, error) {
	dataDir, err := dataDirFromCwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, DBFile), nil
}
