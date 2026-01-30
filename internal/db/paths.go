package db

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
// If in a git worktree, it will look for local .tpg first, then fall back to the main repo.
// Can be overridden with TPG_DB environment variable.
func DefaultPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return GetDatabasePath(wd)
}

// InitPath returns the database path for initializing a project in the current directory.
func InitPath() (string, error) {
	dataDir, err := dataDirFromCwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, DBFile), nil
}

// FindWorktreeRoot detects if the given directory is in a git worktree and returns
// the main repository root path. If .git is a directory (regular repo) or doesn't exist,
// it returns an empty string. If .git is a file (worktree), it parses the gitdir path
// and returns the main repository root.
func FindWorktreeRoot(startDir string) (string, error) {
	// Search upward from startDir to find .git file or directory
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

// GetDatabasePath returns the database path for the given directory.
// It searches for .tpg directory in the following order:
// 1. Check TPG_DB environment variable (highest priority)
// 2. Search upward from dir for a local .tpg directory
// 3. If in a git worktree, check the main repo for .tpg
// 4. Return error if no database found
func GetDatabasePath(dir string) (string, error) {
	// 1. Check TPG_DB environment variable first
	if envPath := os.Getenv("TPG_DB"); envPath != "" {
		return envPath, nil
	}

	// 2. Search upward from dir for local .tpg directory
	localDataDir, err := findDataDirFrom(dir)
	if err == nil {
		return filepath.Join(localDataDir, DBFile), nil
	}

	// 3. Check if we're in a worktree and look for main repo database
	worktreeRoot, err := FindWorktreeRoot(dir)
	if err != nil {
		return "", err
	}

	if worktreeRoot != "" {
		// Try to find .tpg in the main repo
		mainDataDir, err := findDataDirFrom(worktreeRoot)
		if err == nil {
			return filepath.Join(mainDataDir, DBFile), nil
		}
	}

	// 4. No database found anywhere
	return "", fmt.Errorf("no %s directory found in %s or any ancestor. Run 'tpg init' first", DataDir, dir)
}

// findDataDirFrom searches upward from the given directory to locate DataDir.
func findDataDirFrom(startDir string) (string, error) {
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
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check %s: %w", candidate, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s directory found", DataDir)
		}
		dir = parent
	}
}
