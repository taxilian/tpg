package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
)

var (
	flagExportOutput string
	flagExportJSON   bool
	flagExportJSONL  bool
	flagExportAll    bool
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export tasks to a single file for LLM consumption",
	Long: `Export tasks with full details to stdout or a file.

By default, outputs markdown format optimized for LLM consumption.
Use --json for JSON output or --jsonl for JSON Lines format (one object per line).

Supports the same filters as 'tpg list':
  --status, --label, --parent, --type, --all, --project,
  --has-blockers, --no-blockers, --blocking, --blocked-by

By default, excludes done and canceled items. Use --all to include everything.

Examples:
  tpg export                          # Export active tasks to stdout
  tpg export -o tasks.md              # Export to file
  tpg export --json                   # Export as JSON
  tpg export --jsonl                  # Export as JSON Lines (one object per line)
  tpg export --all                    # Include done/canceled
  tpg export --status open            # Only open tasks
  tpg export -l bug                   # Only tasks with 'bug' label
  tpg export --parent ep-abc123       # Only children of epic`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate mutually exclusive flags
		if flagExportJSON && flagExportJSONL {
			return fmt.Errorf("--json and --jsonl are mutually exclusive")
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			return err
		}

		var status *model.Status
		statusExplicitlySet := flagStatus != ""
		if flagStatus != "" {
			s := model.Status(flagStatus)
			if !s.IsValid() {
				return fmt.Errorf("invalid status: %s (valid: open, in_progress, blocked, done, canceled)", flagStatus)
			}
			status = &s
		}

		filter := db.ListFilter{
			Project:     project,
			Status:      status,
			Parent:      flagListParent,
			Type:        flagListType,
			Blocking:    flagBlocking,
			BlockedBy:   flagBlockedBy,
			HasBlockers: flagHasBlockers,
			NoBlockers:  flagNoBlockers,
			Labels:      flagFilterLabels,
		}

		items, err := database.ListItemsFiltered(filter)
		if err != nil {
			return err
		}

		// Filter out done/canceled items by default (unless --all or --status is set)
		if !flagExportAll && !statusExplicitlySet {
			filtered := make([]model.Item, 0, len(items))
			for _, item := range items {
				if item.Status != model.StatusDone && item.Status != model.StatusCanceled {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}

		// Gather full details for each item
		exportData := make([]ExportData, 0, len(items))
		for i := range items {
			item := &items[i]

			// Get labels
			labels, err := database.GetItemLabels(item.ID)
			if err != nil {
				return fmt.Errorf("failed to get labels for %s: %w", item.ID, err)
			}

			// Get logs
			logs, err := database.GetLogs(item.ID)
			if err != nil {
				return fmt.Errorf("failed to get logs for %s: %w", item.ID, err)
			}

			// Get dependencies
			deps, err := database.GetDeps(item.ID)
			if err != nil {
				return fmt.Errorf("failed to get deps for %s: %w", item.ID, err)
			}

			// Get dependency statuses for richer output
			depStatuses, err := database.GetDepStatuses(item.ID)
			if err != nil {
				return fmt.Errorf("failed to get dep statuses for %s: %w", item.ID, err)
			}

			exportData = append(exportData, ExportData{
				Item:         item,
				Labels:       labels,
				Logs:         logs,
				Dependencies: deps,
				DepStatuses:  depStatuses,
			})
		}

		// Determine output destination
		var output io.Writer = os.Stdout
		if flagExportOutput != "" {
			f, err := os.Create(flagExportOutput)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer f.Close()
			output = f
		}

		// Generate output
		if flagExportJSON {
			return exportJSON(output, exportData)
		}
		if flagExportJSONL {
			return exportJSONL(output, exportData)
		}
		return exportMarkdown(output, exportData)
	},
}

// ExportData represents the data structure for a single exported task.
type ExportData struct {
	Item         *model.Item
	Labels       []model.Label
	Logs         []model.Log
	Dependencies []string
	DepStatuses  []db.DepStatus
}

// ExportDataJSON is the JSON representation of export data
type ExportDataJSON struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Project      string            `json:"project"`
	Title        string            `json:"title"`
	Status       string            `json:"status"`
	Priority     int               `json:"priority"`
	ParentID     *string           `json:"parent_id,omitempty"`
	Description  string            `json:"description,omitempty"`
	Results      string            `json:"results,omitempty"`
	Labels       []string          `json:"labels,omitempty"`
	TemplateID   string            `json:"template_id,omitempty"`
	StepIndex    *int              `json:"step_index,omitempty"`
	TemplateVars map[string]string `json:"template_vars,omitempty"`
	Dependencies []DepStatusJSON   `json:"dependencies,omitempty"`
	Logs         []ExportLogJSON   `json:"logs,omitempty"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

// DepStatusJSON is the JSON representation of a dependency status
type DepStatusJSON struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// ExportLogJSON is the JSON representation of a log entry for export
type ExportLogJSON struct {
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

func exportJSON(w io.Writer, data []ExportData) error {
	jsonData := make([]ExportDataJSON, 0, len(data))
	for _, d := range data {
		jsonData = append(jsonData, convertToJSONItem(d))
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonData)
}

// convertToJSONItem converts ExportData to ExportDataJSON
func convertToJSONItem(d ExportData) ExportDataJSON {
	item := d.Item

	// Convert labels to string slice
	labelNames := make([]string, 0, len(d.Labels))
	for _, l := range d.Labels {
		labelNames = append(labelNames, l.Name)
	}

	// Convert dependencies
	deps := make([]DepStatusJSON, 0, len(d.DepStatuses))
	for _, dep := range d.DepStatuses {
		deps = append(deps, DepStatusJSON{
			ID:     dep.ID,
			Title:  dep.Title,
			Status: dep.Status,
		})
	}

	// Convert logs
	logs := make([]ExportLogJSON, 0, len(d.Logs))
	for _, log := range d.Logs {
		logs = append(logs, ExportLogJSON{
			Message:   log.Message,
			CreatedAt: log.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return ExportDataJSON{
		ID:           item.ID,
		Type:         string(item.Type),
		Project:      item.Project,
		Title:        item.Title,
		Status:       string(item.Status),
		Priority:     item.Priority,
		ParentID:     item.ParentID,
		Description:  item.Description,
		Results:      item.Results,
		Labels:       labelNames,
		TemplateID:   item.TemplateID,
		StepIndex:    item.StepIndex,
		TemplateVars: item.TemplateVars,
		Dependencies: deps,
		Logs:         logs,
		CreatedAt:    item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:    item.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func exportJSONL(w io.Writer, data []ExportData) error {
	encoder := json.NewEncoder(w)
	for _, d := range data {
		jsonItem := convertToJSONItem(d)
		if err := encoder.Encode(jsonItem); err != nil {
			return err
		}
	}
	return nil
}

func exportMarkdown(w io.Writer, data []ExportData) error {
	if len(data) == 0 {
		fmt.Fprintln(w, "# Task Export")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "No tasks found matching the filter criteria.")
		return nil
	}

	fmt.Fprintln(w, "# Task Export")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Total: %d tasks\n", len(data))
	fmt.Fprintln(w)

	for i, d := range data {
		item := d.Item

		// Task header
		fmt.Fprintf(w, "## %s: %s\n\n", item.ID, item.Title)

		// Metadata line
		fmt.Fprintf(w, "**Status:** %s | **Priority:** %d | **Project:** %s", item.Status, item.Priority, item.Project)
		if item.ParentID != nil {
			fmt.Fprintf(w, " | **Parent:** %s", *item.ParentID)
		}
		fmt.Fprintln(w)

		// Labels
		if len(d.Labels) > 0 {
			labelNames := make([]string, 0, len(d.Labels))
			for _, l := range d.Labels {
				labelNames = append(labelNames, l.Name)
			}
			fmt.Fprintf(w, "**Labels:** %s\n", strings.Join(labelNames, ", "))
		}

		// Template info
		if item.TemplateID != "" {
			fmt.Fprintf(w, "**Template:** %s", item.TemplateID)
			if item.StepIndex != nil {
				fmt.Fprintf(w, " (step %d)", *item.StepIndex+1)
			}
			fmt.Fprintln(w)
		}

		fmt.Fprintln(w)

		// Description
		if item.Description != "" {
			fmt.Fprintln(w, "### Description")
			fmt.Fprintln(w)
			fmt.Fprintln(w, item.Description)
			fmt.Fprintln(w)
		}

		// Results (for completed tasks)
		if item.Results != "" {
			fmt.Fprintln(w, "### Results")
			fmt.Fprintln(w)
			fmt.Fprintln(w, item.Results)
			fmt.Fprintln(w)
		}

		// Dependencies
		if len(d.DepStatuses) > 0 {
			fmt.Fprintln(w, "### Dependencies")
			fmt.Fprintln(w)
			for _, dep := range d.DepStatuses {
				fmt.Fprintf(w, "- %s [%s] %s\n", dep.ID, dep.Status, dep.Title)
			}
			fmt.Fprintln(w)
		}

		// Logs
		if len(d.Logs) > 0 {
			fmt.Fprintln(w, "### Logs")
			fmt.Fprintln(w)
			for _, log := range d.Logs {
				fmt.Fprintf(w, "- **%s** %s\n", log.CreatedAt.Format("2006-01-02 15:04"), log.Message)
			}
			fmt.Fprintln(w)
		}

		// Separator between tasks (except for last one)
		if i < len(data)-1 {
			fmt.Fprintln(w, "---")
			fmt.Fprintln(w)
		}
	}

	return nil
}

func init() {
	// Export flags - reuse list command flags where applicable
	exportCmd.Flags().StringVarP(&flagExportOutput, "output", "o", "", "Output file path (default: stdout)")
	exportCmd.Flags().BoolVar(&flagExportJSON, "json", false, "Output as JSON instead of markdown")
	exportCmd.Flags().BoolVar(&flagExportJSONL, "jsonl", false, "Output as JSON Lines (one object per line)")
	exportCmd.Flags().BoolVarP(&flagExportAll, "all", "a", false, "Include done and canceled tasks")
	exportCmd.Flags().StringVar(&flagStatus, "status", "", "Filter by status (open, in_progress, blocked, done, canceled)")
	exportCmd.Flags().StringVar(&flagListParent, "parent", "", "Filter by parent epic ID")
	exportCmd.Flags().StringVar(&flagListType, "type", "", "Filter by item type (task, epic)")
	exportCmd.Flags().StringVar(&flagBlocking, "blocking", "", "Show items that block the given ID")
	exportCmd.Flags().StringVar(&flagBlockedBy, "blocked-by", "", "Show items blocked by the given ID")
	exportCmd.Flags().BoolVar(&flagHasBlockers, "has-blockers", false, "Show only items with unresolved blockers")
	exportCmd.Flags().BoolVar(&flagNoBlockers, "no-blockers", false, "Show only items with no blockers")
	exportCmd.Flags().StringArrayVarP(&flagFilterLabels, "label", "l", nil, "Filter by label (can be repeated, AND logic)")

	rootCmd.AddCommand(exportCmd)
}
