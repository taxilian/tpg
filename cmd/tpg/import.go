package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/taxilian/tpg/internal/model"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import tasks from external sources",
	Long:  `Import tasks from external sources like beads (legacy issue tracker).`,
}

var importBeadsCmd = &cobra.Command{
	Use:   "beads <path-to-issues.jsonl>",
	Short: "Import beads issues into tpg",
	Long: `Import beads issues from a JSONL file into tpg.

Mapping:
  - Original bd-XXX IDs are preserved (no collision risk)
  - beads status → tpg status (open→open, closed→done)
  - close_reason imported as results
  - Dependencies preserved
  - beads issue_type mapped to tpg type

Example:
  tpg import beads /path/to/.beads/issues.jsonl`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runImportBeads(args[0])
	},
}

type beadsDependency struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

type beadsIssue struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Status       string            `json:"status"`
	Priority     int               `json:"priority"`
	IssueType    string            `json:"issue_type"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	ClosedAt     *time.Time        `json:"closed_at,omitempty"`
	CloseReason  string            `json:"close_reason,omitempty"`
	CreatedBy    string            `json:"created_by"`
	Project      string            `json:"project,omitempty"`
	Dependencies []beadsDependency `json:"dependencies,omitempty"`
}

func runImportBeads(path string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open beads file: %w", err)
	}
	defer func() { _ = file.Close() }()

	var issues []beadsIssue
	scanner := bufio.NewScanner(file)

	// Increase scanner buffer size to handle long lines (up to 1MB)
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, 64*1024)    // Initial 64KB buffer
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var issue beadsIssue
		if err := json.Unmarshal([]byte(line), &issue); err != nil {
			return fmt.Errorf("failed to parse line %d: %w", lineNum, err)
		}
		issues = append(issues, issue)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if len(issues) == 0 {
		return fmt.Errorf("no issues found in file")
	}

	project := issues[0].Project
	if project == "" {
		project = "imported"
	}

	createdCount := 0
	skippedCount := 0
	for _, issue := range issues {
		var status model.Status
		switch issue.Status {
		case "open":
			status = model.StatusOpen
		case "closed":
			status = model.StatusDone
		default:
			status = model.StatusOpen
		}

		itemType := model.ItemType(issue.IssueType)
		if !itemType.IsValid() {
			itemType = model.ItemTypeTask
		}

		updatedAt := issue.UpdatedAt
		if issue.ClosedAt != nil {
			updatedAt = *issue.ClosedAt
		}

		item := &model.Item{
			ID:          issue.ID,
			Project:     project,
			Type:        itemType,
			Title:       issue.Title,
			Description: issue.Description,
			Status:      status,
			Priority:    issue.Priority,
			Results:     issue.CloseReason,
			CreatedAt:   issue.CreatedAt,
			UpdatedAt:   updatedAt,
		}

		if err := database.CreateItem(item); err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				fmt.Printf("Skipping %s (already exists)\n", issue.ID)
				skippedCount++
				continue
			}
			return fmt.Errorf("failed to create item %s: %w", issue.ID, err)
		}
		createdCount++
	}

	depCount := 0
	for _, issue := range issues {
		for _, dep := range issue.Dependencies {
			if dep.Type == "blocks" || dep.Type == "" {
				if err := database.AddDep(dep.IssueID, dep.DependsOnID); err != nil {
					fmt.Printf("Warning: failed to add dependency %s -> %s: %v\n", dep.DependsOnID, dep.IssueID, err)
					continue
				}
				depCount++
			}
		}
	}

	fmt.Printf("Imported %d issues into project '%s'\n", createdCount, project)
	if skippedCount > 0 {
		fmt.Printf("Skipped %d existing issues\n", skippedCount)
	}
	if depCount > 0 {
		fmt.Printf("Created %d dependencies\n", depCount)
	}

	database.BackupQuiet()
	return nil
}
