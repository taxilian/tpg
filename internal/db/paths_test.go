package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindWorktreeRoot_RegularRepo(t *testing.T) {
	// Arrange: Create a regular git repo with .git as a directory
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}

	// Act: Call FindWorktreeRoot
	root, err := FindWorktreeRoot(tempDir)

	// Assert: Should return empty string and no error (not a worktree)
	if err != nil {
		t.Errorf("expected no error for regular repo, got: %v", err)
	}
	if root != "" {
		t.Errorf("expected empty string for regular repo, got: %q", root)
	}
}

func TestFindWorktreeRoot_WorktreeWithValidGitFile(t *testing.T) {
	// Arrange: Create a main repo and a worktree
	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()

	// Create main repo .git directory
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}

	// Create worktree .git file pointing to main repo
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "gitdir: " + mainGitDir + "\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Act: Call FindWorktreeRoot
	root, err := FindWorktreeRoot(worktreeDir)

	// Assert: Should return the main repo path
	if err != nil {
		t.Errorf("expected no error for valid worktree, got: %v", err)
	}
	if root != mainRepo {
		t.Errorf("expected main repo path %q, got: %q", mainRepo, root)
	}
}

func TestFindWorktreeRoot_WorktreeWithMalformedGitFile(t *testing.T) {
	// Arrange: Create a worktree with malformed .git file
	worktreeDir := t.TempDir()

	// Create malformed .git file (missing gitdir: prefix)
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "some random content without gitdir prefix\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Act: Call FindWorktreeRoot
	root, err := FindWorktreeRoot(worktreeDir)

	// Assert: Should return error for malformed file
	if err == nil {
		t.Error("expected error for malformed .git file, got nil")
	}
	if root != "" {
		t.Errorf("expected empty string for malformed file, got: %q", root)
	}
}

func TestFindWorktreeRoot_WorktreeWithEmptyGitFile(t *testing.T) {
	// Arrange: Create a worktree with empty .git file
	worktreeDir := t.TempDir()

	// Create empty .git file
	gitFilePath := filepath.Join(worktreeDir, ".git")
	if err := os.WriteFile(gitFilePath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create empty .git file: %v", err)
	}

	// Act: Call FindWorktreeRoot
	root, err := FindWorktreeRoot(worktreeDir)

	// Assert: Should return error for empty file
	if err == nil {
		t.Error("expected error for empty .git file, got nil")
	}
	if root != "" {
		t.Errorf("expected empty string for empty file, got: %q", root)
	}
}

func TestFindWorktreeRoot_NestedDirectoryWithinWorktree(t *testing.T) {
	// Arrange: Create a main repo and a worktree with nested subdirectory
	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()
	nestedDir := filepath.Join(worktreeDir, "src", "components", "button")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	// Create main repo .git directory
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}

	// Create worktree .git file pointing to main repo
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "gitdir: " + mainGitDir + "\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Act: Call FindWorktreeRoot from nested directory
	root, err := FindWorktreeRoot(nestedDir)

	// Assert: Should still find and return the main repo path
	if err != nil {
		t.Errorf("expected no error for nested directory in worktree, got: %v", err)
	}
	if root != mainRepo {
		t.Errorf("expected main repo path %q, got: %q", mainRepo, root)
	}
}

func TestFindWorktreeRoot_NoGitDirectoryOrFile(t *testing.T) {
	// Arrange: Create a directory with no .git at all
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Act: Call FindWorktreeRoot
	root, err := FindWorktreeRoot(subDir)

	// Assert: Should return empty string and no error (not a git repo)
	if err != nil {
		t.Errorf("expected no error for non-git directory, got: %v", err)
	}
	if root != "" {
		t.Errorf("expected empty string for non-git directory, got: %q", root)
	}
}

func TestFindWorktreeRoot_WorktreeWithRelativePath(t *testing.T) {
	// Arrange: Create a main repo and a worktree with relative gitdir path
	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()

	// Create main repo .git directory
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}

	// Create worktree .git file with relative path (edge case)
	gitFilePath := filepath.Join(worktreeDir, ".git")
	// Use absolute path but test that it handles the format correctly
	gitFileContent := "gitdir: " + mainGitDir + "/worktrees/myworktree\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Act: Call FindWorktreeRoot
	root, err := FindWorktreeRoot(worktreeDir)

	// Assert: Should handle the worktree reference and return main repo
	if err != nil {
		t.Errorf("expected no error for worktree with worktrees path, got: %v", err)
	}
	// The function should extract the main repo path from the worktrees reference
	expectedRoot := mainRepo
	if root != expectedRoot {
		t.Errorf("expected main repo path %q, got: %q", expectedRoot, root)
	}
}

func TestFindWorktreeRoot_GitFileWithExtraWhitespace(t *testing.T) {
	// Arrange: Create a worktree with .git file containing extra whitespace
	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()

	// Create main repo .git directory
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}

	// Create worktree .git file with extra whitespace
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "  gitdir:   " + mainGitDir + "   \n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Act: Call FindWorktreeRoot
	root, err := FindWorktreeRoot(worktreeDir)

	// Assert: Should handle whitespace and return main repo path
	if err != nil {
		t.Errorf("expected no error for .git file with whitespace, got: %v", err)
	}
	if root != mainRepo {
		t.Errorf("expected main repo path %q, got: %q", mainRepo, root)
	}
}

func TestFindWorktreeRoot_NonExistentGitdirPath(t *testing.T) {
	// Arrange: Create a worktree with .git file pointing to non-existent path
	worktreeDir := t.TempDir()

	// Create .git file pointing to non-existent directory
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "gitdir: /nonexistent/path/to/gitdir\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Act: Call FindWorktreeRoot
	root, err := FindWorktreeRoot(worktreeDir)

	// Assert: Should handle gracefully - could return error or empty
	// The behavior depends on implementation, but it shouldn't panic
	_ = root
	_ = err
}

// GetDatabasePath tests for worktree-aware database path resolution

func TestGetDatabasePath_RegularRepo(t *testing.T) {
	// Arrange: Create a regular git repo with .tpg directory
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	dataDir := filepath.Join(tempDir, ".tpg")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg directory: %v", err)
	}

	// Act: Call GetDatabasePath
	path, err := GetDatabasePath(tempDir)

	// Assert: Should return local .tpg/tpg.db path
	if err != nil {
		t.Errorf("expected no error for regular repo, got: %v", err)
	}
	expectedPath := filepath.Join(dataDir, DBFile)
	if path != expectedPath {
		t.Errorf("expected path %q, got: %q", expectedPath, path)
	}
}

func TestGetDatabasePath_WorktreeWithLocalDatabase(t *testing.T) {
	// Arrange: Create a main repo and a worktree with local .tpg
	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()

	// Create main repo .git directory
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}

	// Create worktree .git file pointing to main repo
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "gitdir: " + mainGitDir + "\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Create local .tpg directory in worktree
	worktreeDataDir := filepath.Join(worktreeDir, ".tpg")
	if err := os.MkdirAll(worktreeDataDir, 0755); err != nil {
		t.Fatalf("failed to create worktree .tpg directory: %v", err)
	}

	// Act: Call GetDatabasePath
	path, err := GetDatabasePath(worktreeDir)

	// Assert: Should return worktree-local database path
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedPath := filepath.Join(worktreeDataDir, DBFile)
	if path != expectedPath {
		t.Errorf("expected local path %q, got: %q", expectedPath, path)
	}
}

func TestGetDatabasePath_WorktreeWithoutLocalDatabase(t *testing.T) {
	// Arrange: Create a main repo with .tpg and a worktree without local .tpg
	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()

	// Create main repo .git directory
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}

	// Create main repo .tpg directory
	mainDataDir := filepath.Join(mainRepo, ".tpg")
	if err := os.MkdirAll(mainDataDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .tpg directory: %v", err)
	}

	// Create worktree .git file pointing to main repo
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "gitdir: " + mainGitDir + "\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Note: No local .tpg in worktreeDir

	// Act: Call GetDatabasePath
	path, err := GetDatabasePath(worktreeDir)

	// Assert: Should fall back to main repo database
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedPath := filepath.Join(mainDataDir, DBFile)
	if path != expectedPath {
		t.Errorf("expected main repo path %q, got: %q", expectedPath, path)
	}
}

func TestGetDatabasePath_WorktreeFallbackPriority(t *testing.T) {
	// Arrange: Create main repo with .tpg and worktree with local .tpg
	// Local should take priority over root
	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()

	// Create main repo .git directory
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}

	// Create main repo .tpg directory
	mainDataDir := filepath.Join(mainRepo, ".tpg")
	if err := os.MkdirAll(mainDataDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .tpg directory: %v", err)
	}

	// Create worktree .git file pointing to main repo
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "gitdir: " + mainGitDir + "\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Create local .tpg directory in worktree
	worktreeDataDir := filepath.Join(worktreeDir, ".tpg")
	if err := os.MkdirAll(worktreeDataDir, 0755); err != nil {
		t.Fatalf("failed to create worktree .tpg directory: %v", err)
	}

	// Act: Call GetDatabasePath
	path, err := GetDatabasePath(worktreeDir)

	// Assert: Should prefer local over root (fallback logic: local first, then root)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedPath := filepath.Join(worktreeDataDir, DBFile)
	if path != expectedPath {
		t.Errorf("expected local path %q (local should take priority), got: %q", expectedPath, path)
	}
}

func TestGetDatabasePath_NestedInWorktree(t *testing.T) {
	// Arrange: Create main repo and worktree with nested subdirectory
	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()
	nestedDir := filepath.Join(worktreeDir, "src", "components")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	// Create main repo .git directory
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}

	// Create main repo .tpg directory
	mainDataDir := filepath.Join(mainRepo, ".tpg")
	if err := os.MkdirAll(mainDataDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .tpg directory: %v", err)
	}

	// Create worktree .git file pointing to main repo
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "gitdir: " + mainGitDir + "\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// No local .tpg in worktree

	// Act: Call GetDatabasePath from nested directory
	path, err := GetDatabasePath(nestedDir)

	// Assert: Should find and use main repo database
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedPath := filepath.Join(mainDataDir, DBFile)
	if path != expectedPath {
		t.Errorf("expected main repo path %q, got: %q", expectedPath, path)
	}
}

func TestGetDatabasePath_NoDatabaseAnywhere(t *testing.T) {
	// Arrange: Create a directory with no .tpg anywhere
	tempDir := t.TempDir()

	// Act: Call GetDatabasePath
	path, err := GetDatabasePath(tempDir)

	// Assert: Should return error indicating no database found
	if err == nil {
		t.Error("expected error when no database exists, got nil")
	}
	if path != "" {
		t.Errorf("expected empty path when no database found, got: %q", path)
	}
}

func TestGetDatabasePath_RespectsTPGDBEnvVar(t *testing.T) {
	// Arrange: Create directories and set TPG_DB env var
	tempDir := t.TempDir()
	customDbPath := filepath.Join(tempDir, "custom", "database.db")

	// Set environment variable
	oldEnv := os.Getenv("TPG_DB")
	os.Setenv("TPG_DB", customDbPath)
	defer os.Setenv("TPG_DB", oldEnv)

	// Act: Call GetDatabasePath
	path, err := GetDatabasePath(tempDir)

	// Assert: Should return the env var path, ignoring worktree logic
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if path != customDbPath {
		t.Errorf("expected env var path %q, got: %q", customDbPath, path)
	}
}

func TestGetDatabasePath_WorktreeWithLocalInParent(t *testing.T) {
	// Arrange: Create main repo and worktree where .tpg exists in parent of worktree root
	// This tests that we search upward from current dir for local .tpg first
	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()
	subDir := filepath.Join(worktreeDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Create main repo .git directory
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}

	// Create main repo .tpg directory
	mainDataDir := filepath.Join(mainRepo, ".tpg")
	if err := os.MkdirAll(mainDataDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .tpg directory: %v", err)
	}

	// Create worktree .git file pointing to main repo
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "gitdir: " + mainGitDir + "\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Create .tpg in worktree root (parent of subDir)
	worktreeDataDir := filepath.Join(worktreeDir, ".tpg")
	if err := os.MkdirAll(worktreeDataDir, 0755); err != nil {
		t.Fatalf("failed to create worktree .tpg directory: %v", err)
	}

	// Act: Call GetDatabasePath from subDir
	path, err := GetDatabasePath(subDir)

	// Assert: Should find .tpg in worktree root (searching upward)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedPath := filepath.Join(worktreeDataDir, DBFile)
	if path != expectedPath {
		t.Errorf("expected worktree root path %q, got: %q", expectedPath, path)
	}
}

// Tests for dataDirFromCwd function

func TestDataDirFromCwd_Success(t *testing.T) {
	// Arrange: Save current directory and change to temp dir
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Get the actual working directory after chdir (handles /private prefix on macOS)
	actualWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get actual working directory: %v", err)
	}

	// Act: Call dataDirFromCwd
	path, err := dataDirFromCwd()

	// Assert: Should return path to .tpg in current directory
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedPath := filepath.Join(actualWd, DataDir)
	if path != expectedPath {
		t.Errorf("expected path %q, got: %q", expectedPath, path)
	}
}

// Tests for findDataDir function

func TestFindDataDir_FindsLocalDirectory(t *testing.T) {
	// Arrange: Save current directory and change to temp dir with .tpg
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, DataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg directory: %v", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Get the actual working directory after chdir (handles /private prefix on macOS)
	actualWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get actual working directory: %v", err)
	}

	// Act: Call findDataDir
	foundDir, err := findDataDir()

	// Assert: Should find the local .tpg directory
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedDir := filepath.Join(actualWd, DataDir)
	if foundDir != expectedDir {
		t.Errorf("expected directory %q, got: %q", expectedDir, foundDir)
	}
}

func TestFindDataDir_SearchesUpward(t *testing.T) {
	// Arrange: Save current directory and change to nested dir
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, DataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg directory: %v", err)
	}

	nestedDir := filepath.Join(tempDir, "src", "components")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	if err := os.Chdir(nestedDir); err != nil {
		t.Fatalf("failed to change to nested directory: %v", err)
	}

	// Get the actual working directory after chdir (handles /private prefix on macOS)
	actualNestedWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get actual working directory: %v", err)
	}

	// Act: Call findDataDir
	foundDir, err := findDataDir()

	// Assert: Should find .tpg in parent directory
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	// The foundDir should be in an ancestor of the actual working directory
	// Just verify it ends with the correct suffix
	expectedSuffix := filepath.Join(DataDir)
	if !strings.HasSuffix(foundDir, expectedSuffix) {
		t.Errorf("expected directory to end with %q, got: %q", expectedSuffix, foundDir)
	}
	// And verify the foundDir exists
	if _, err := os.Stat(foundDir); os.IsNotExist(err) {
		t.Errorf("found directory does not exist: %s", foundDir)
	}
	// Verify we're in a subdirectory of where .tpg was found
	if !strings.HasPrefix(actualNestedWd, filepath.Dir(foundDir)) {
		t.Errorf("working directory %q should be under %q", actualNestedWd, filepath.Dir(foundDir))
	}
}

func TestFindDataDir_NotADirectory(t *testing.T) {
	// Arrange: Save current directory and change to temp dir with .tpg as file
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, DataDir)
	// Create .tpg as a file, not a directory
	if err := os.WriteFile(dataDir, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("failed to create .tpg file: %v", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Act: Call findDataDir
	_, err = findDataDir()

	// Assert: Should return error because .tpg is not a directory
	if err == nil {
		t.Error("expected error when .tpg is not a directory, got nil")
	}
}

func TestFindDataDir_NotFound(t *testing.T) {
	// Arrange: Save current directory and change to temp dir without .tpg
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Act: Call findDataDir
	_, err = findDataDir()

	// Assert: Should return error when no .tpg found
	if err == nil {
		t.Error("expected error when .tpg not found, got nil")
	}
}

// Tests for DefaultPath function

func TestDefaultPath_UsesEnvVar(t *testing.T) {
	// Arrange: Set TPG_DB environment variable
	customPath := "/custom/path/to/db.db"
	oldEnv := os.Getenv("TPG_DB")
	os.Setenv("TPG_DB", customPath)
	defer os.Setenv("TPG_DB", oldEnv)

	// Act: Call DefaultPath
	path, err := DefaultPath()

	// Assert: Should return env var path
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if path != customPath {
		t.Errorf("expected path %q, got: %q", customPath, path)
	}
}

func TestDefaultPath_FindsLocalDatabase(t *testing.T) {
	// Arrange: Save current directory and change to temp dir with .tpg
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Clear any TPG_DB env var
	oldEnv := os.Getenv("TPG_DB")
	os.Unsetenv("TPG_DB")
	defer os.Setenv("TPG_DB", oldEnv)

	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, DataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg directory: %v", err)
	}

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Get the actual working directory after chdir (handles /private prefix on macOS)
	actualWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get actual working directory: %v", err)
	}

	// Act: Call DefaultPath
	path, err := DefaultPath()

	// Assert: Should return local database path
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedPath := filepath.Join(actualWd, DataDir, DBFile)
	if path != expectedPath {
		t.Errorf("expected path %q, got: %q", expectedPath, path)
	}
}

// Tests for InitPath function

func TestInitPath_ReturnsCorrectPath(t *testing.T) {
	// Arrange: Save current directory and change to temp dir
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Get the actual working directory after chdir (handles /private prefix on macOS)
	actualWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get actual working directory: %v", err)
	}

	// Act: Call InitPath
	path, err := InitPath()

	// Assert: Should return path to tpg.db in .tpg directory
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedPath := filepath.Join(actualWd, DataDir, DBFile)
	if path != expectedPath {
		t.Errorf("expected path %q, got: %q", expectedPath, path)
	}
}

// Tests for parseGitFile function (edge cases not covered by FindWorktreeRoot tests)

func TestParseGitFile_EmptyGitdirPath(t *testing.T) {
	// Arrange: Create a .git file with empty gitdir path
	tempDir := t.TempDir()
	gitFilePath := filepath.Join(tempDir, ".git")
	gitFileContent := "gitdir: \n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Act: Call parseGitFile
	_, err := parseGitFile(gitFilePath)

	// Assert: Should return error for empty gitdir path
	if err == nil {
		t.Error("expected error for empty gitdir path, got nil")
	}
}

func TestParseGitFile_MultipleLines(t *testing.T) {
	// Arrange: Create a .git file with multiple lines (only first should matter)
	mainRepo := t.TempDir()
	tempDir := t.TempDir()
	gitFilePath := filepath.Join(tempDir, ".git")
	mainGitDir := filepath.Join(mainRepo, ".git")
	gitFileContent := "gitdir: " + mainGitDir + "\nsecond line\nthird line\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Act: Call parseGitFile
	root, err := parseGitFile(gitFilePath)

	// Assert: Should parse first line correctly
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedRoot := mainRepo
	if root != expectedRoot {
		t.Errorf("expected root %q, got: %q", expectedRoot, root)
	}
}

func TestParseGitFile_DeeplyNestedWorktrees(t *testing.T) {
	// Arrange: Create a .git file with deeply nested worktrees path
	mainRepo := t.TempDir()
	tempDir := t.TempDir()
	gitFilePath := filepath.Join(tempDir, ".git")
	// Simulate a deeply nested worktrees path
	mainGitDir := filepath.Join(mainRepo, ".git", "worktrees", "wt1", "worktrees", "wt2")
	gitFileContent := "gitdir: " + mainGitDir + "\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Act: Call parseGitFile
	root, err := parseGitFile(gitFilePath)

	// Assert: Should extract main repo root
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedRoot := mainRepo
	if root != expectedRoot {
		t.Errorf("expected root %q, got: %q", expectedRoot, root)
	}
}

func TestParseGitFile_GitdirWithTrailingSlash(t *testing.T) {
	// Arrange: Create a .git file with gitdir path having trailing slash
	mainRepo := t.TempDir()
	tempDir := t.TempDir()
	gitFilePath := filepath.Join(tempDir, ".git")
	mainGitDir := filepath.Join(mainRepo, ".git")
	gitFileContent := "gitdir: " + mainGitDir + "/\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Act: Call parseGitFile
	root, err := parseGitFile(gitFilePath)

	// Assert: Should handle trailing slash correctly
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	expectedRoot := mainRepo
	if root != expectedRoot {
		t.Errorf("expected root %q, got: %q", expectedRoot, root)
	}
}

func TestParseGitFile_NonExistentFile(t *testing.T) {
	// Arrange: Use a non-existent file path
	nonExistentPath := "/nonexistent/path/to/.git"

	// Act: Call parseGitFile
	_, err := parseGitFile(nonExistentPath)

	// Assert: Should return error for non-existent file
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

// Tests for findDataDirFrom function (edge cases)

func TestFindDataDirFrom_FindsInStartDir(t *testing.T) {
	// Arrange: Create a directory with .tpg
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, DataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg directory: %v", err)
	}

	// Act: Call findDataDirFrom
	foundDir, err := findDataDirFrom(tempDir)

	// Assert: Should find .tpg in start directory
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if foundDir != dataDir {
		t.Errorf("expected directory %q, got: %q", dataDir, foundDir)
	}
}

func TestFindDataDirFrom_FindsInAncestor(t *testing.T) {
	// Arrange: Create nested directory structure
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, DataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create .tpg directory: %v", err)
	}

	nestedDir := filepath.Join(tempDir, "a", "b", "c")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	// Act: Call findDataDirFrom from deeply nested dir
	foundDir, err := findDataDirFrom(nestedDir)

	// Assert: Should find .tpg in ancestor
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if foundDir != dataDir {
		t.Errorf("expected directory %q, got: %q", dataDir, foundDir)
	}
}

func TestFindDataDirFrom_NotFoundInAnyAncestor(t *testing.T) {
	// Arrange: Create directory without .tpg
	tempDir := t.TempDir()
	nestedDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	// Act: Call findDataDirFrom
	_, err := findDataDirFrom(nestedDir)

	// Assert: Should return error
	if err == nil {
		t.Error("expected error when .tpg not found, got nil")
	}
}

func TestFindDataDirFrom_FileInsteadOfDir(t *testing.T) {
	// Arrange: Create .tpg as a file
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, DataDir)
	if err := os.WriteFile(dataDir, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("failed to create .tpg file: %v", err)
	}

	// Act: Call findDataDirFrom
	_, err := findDataDirFrom(tempDir)

	// Assert: Should return error
	if err == nil {
		t.Error("expected error when .tpg is a file, got nil")
	}
}

// Integration tests for worktree detection scenarios

func TestWorktreeDetection_FullWorkflow(t *testing.T) {
	// Arrange: Create a complete worktree scenario
	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()

	// Set up main repo with .git and .tpg
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}
	mainDataDir := filepath.Join(mainRepo, ".tpg")
	if err := os.MkdirAll(mainDataDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .tpg directory: %v", err)
	}

	// Set up worktree with .git file pointing to main repo
	gitFilePath := filepath.Join(worktreeDir, ".git")
	gitFileContent := "gitdir: " + mainGitDir + "\n"
	if err := os.WriteFile(gitFilePath, []byte(gitFileContent), 0644); err != nil {
		t.Fatalf("failed to create .git file: %v", err)
	}

	// Create nested subdirectory in worktree
	nestedDir := filepath.Join(worktreeDir, "packages", "frontend", "src")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	// Act & Assert: Test from worktree root
	path, err := GetDatabasePath(worktreeDir)
	if err != nil {
		t.Errorf("expected no error from worktree root, got: %v", err)
	}
	expectedPath := filepath.Join(mainDataDir, DBFile)
	if path != expectedPath {
		t.Errorf("from worktree root: expected path %q, got: %q", expectedPath, path)
	}

	// Act & Assert: Test from nested directory
	path, err = GetDatabasePath(nestedDir)
	if err != nil {
		t.Errorf("expected no error from nested dir, got: %v", err)
	}
	if path != expectedPath {
		t.Errorf("from nested dir: expected path %q, got: %q", expectedPath, path)
	}
}

func TestWorktreeDetection_MultipleWorktrees(t *testing.T) {
	// Arrange: Create main repo with multiple worktrees
	mainRepo := t.TempDir()
	worktree1 := t.TempDir()
	worktree2 := t.TempDir()

	// Set up main repo
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(mainGitDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .git directory: %v", err)
	}
	mainDataDir := filepath.Join(mainRepo, ".tpg")
	if err := os.MkdirAll(mainDataDir, 0755); err != nil {
		t.Fatalf("failed to create main repo .tpg directory: %v", err)
	}

	// Set up worktree 1
	gitFile1 := filepath.Join(worktree1, ".git")
	if err := os.WriteFile(gitFile1, []byte("gitdir: "+mainGitDir+"/worktrees/wt1\n"), 0644); err != nil {
		t.Fatalf("failed to create worktree1 .git file: %v", err)
	}

	// Set up worktree 2
	gitFile2 := filepath.Join(worktree2, ".git")
	if err := os.WriteFile(gitFile2, []byte("gitdir: "+mainGitDir+"/worktrees/wt2\n"), 0644); err != nil {
		t.Fatalf("failed to create worktree2 .git file: %v", err)
	}

	// Act & Assert: Both worktrees should resolve to main repo
	path1, err := GetDatabasePath(worktree1)
	if err != nil {
		t.Errorf("expected no error for worktree1, got: %v", err)
	}
	path2, err := GetDatabasePath(worktree2)
	if err != nil {
		t.Errorf("expected no error for worktree2, got: %v", err)
	}

	expectedPath := filepath.Join(mainDataDir, DBFile)
	if path1 != expectedPath {
		t.Errorf("worktree1: expected path %q, got: %q", expectedPath, path1)
	}
	if path2 != expectedPath {
		t.Errorf("worktree2: expected path %q, got: %q", expectedPath, path2)
	}
}
