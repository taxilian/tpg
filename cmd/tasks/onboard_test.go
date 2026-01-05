package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnboard_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	claudePath := filepath.Join(dir, "CLAUDE.md")

	// Change to temp dir for the test
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	output := captureOutput(func() {
		if err := runOnboard(); err != nil {
			t.Fatalf("runOnboard failed: %v", err)
		}
	})

	// Check output message
	if !strings.Contains(output, "Created CLAUDE.md") {
		t.Errorf("expected 'Created CLAUDE.md' message, got: %s", output)
	}

	// Check file was created with correct content
	content, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}

	if !strings.Contains(string(content), "## Task Tracking") {
		t.Error("missing '## Task Tracking' header")
	}
	if !strings.Contains(string(content), "tasks prime") {
		t.Error("missing 'tasks prime' reference")
	}
	if !strings.Contains(string(content), "tasks ready") {
		t.Error("missing 'tasks ready' command")
	}
}

func TestOnboard_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	claudePath := filepath.Join(dir, "CLAUDE.md")

	// Create existing file
	existingContent := "# My Project\n\nSome existing content.\n"
	if err := os.WriteFile(claudePath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to write existing CLAUDE.md: %v", err)
	}

	// Change to temp dir for the test
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	output := captureOutput(func() {
		if err := runOnboard(); err != nil {
			t.Fatalf("runOnboard failed: %v", err)
		}
	})

	// Check output message includes filename
	if !strings.Contains(output, "Added tasks integration to CLAUDE.md") {
		t.Errorf("expected 'Added tasks integration to CLAUDE.md' message, got: %s", output)
	}

	// Check file has both old and new content
	content, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}

	if !strings.Contains(string(content), "# My Project") {
		t.Error("missing original content")
	}
	if !strings.Contains(string(content), "## Task Tracking") {
		t.Error("missing appended Task Tracking section")
	}
}

func TestOnboard_Idempotent(t *testing.T) {
	dir := t.TempDir()
	claudePath := filepath.Join(dir, "CLAUDE.md")

	// Create file that already has Task Tracking section
	existingContent := "# My Project\n\n## Task Tracking\n\nAlready configured.\n"
	if err := os.WriteFile(claudePath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to write existing CLAUDE.md: %v", err)
	}

	// Change to temp dir for the test
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	output := captureOutput(func() {
		if err := runOnboard(); err != nil {
			t.Fatalf("runOnboard failed: %v", err)
		}
	})

	// Check output message
	if !strings.Contains(output, "Already onboarded") {
		t.Errorf("expected 'Already onboarded' message, got: %s", output)
	}

	// Check file wasn't modified
	content, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}

	if string(content) != existingContent {
		t.Error("file was modified when it should have been left alone")
	}
}

func TestOnboard_LowercaseFile(t *testing.T) {
	dir := t.TempDir()
	claudePath := filepath.Join(dir, "claude.md") // lowercase

	// Create existing lowercase file
	existingContent := "# My Project\n"
	if err := os.WriteFile(claudePath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("failed to write existing claude.md: %v", err)
	}

	// Change to temp dir for the test
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	output := captureOutput(func() {
		if err := runOnboard(); err != nil {
			t.Fatalf("runOnboard failed: %v", err)
		}
	})

	// Should show the actual lowercase filename
	if !strings.Contains(output, "claude.md") {
		t.Errorf("expected output to mention 'claude.md', got: %s", output)
	}

	// Should have appended to lowercase file, not created CLAUDE.md
	content, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("failed to read claude.md: %v", err)
	}

	if !strings.Contains(string(content), "## Task Tracking") {
		t.Error("should have appended to existing claude.md")
	}

	// Verify no separate CLAUDE.md was created (check actual filenames)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() == "CLAUDE.md" {
			t.Error("should not have created separate CLAUDE.md")
		}
	}
}

func TestOnboard_SnippetContent(t *testing.T) {
	dir := t.TempDir()
	claudePath := filepath.Join(dir, "CLAUDE.md")

	// Change to temp dir for the test
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	captureOutput(func() {
		if err := runOnboard(); err != nil {
			t.Fatalf("runOnboard failed: %v", err)
		}
	})

	content, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}

	// Check all key commands are present
	commands := []string{
		"tasks ready",
		"tasks add",
		"tasks start",
		"tasks log",
		"tasks done",
		"tasks prime",
	}

	for _, cmd := range commands {
		if !strings.Contains(string(content), cmd) {
			t.Errorf("missing command reference: %s", cmd)
		}
	}
}
