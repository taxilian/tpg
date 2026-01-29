package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/model"
	"github.com/taxilian/tpg/internal/plugin"
	"github.com/taxilian/tpg/internal/prime"
	"github.com/taxilian/tpg/internal/templates"
	"github.com/taxilian/tpg/internal/tui"
)

// version is set via ldflags at build time, or read from module info
var version = "dev"

func init() {
	// Try to get version from build info if not set via ldflags
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
	// Update rootCmd.Version since it was initialized before this runs
	rootCmd.Version = version
}

var (
	flagProject          string
	flagInitTaskPrefix   string
	flagInitEpicPrefix   string
	flagStatus           string
	flagEpic             bool
	flagPriority         int
	flagForce            bool
	flagParent           string
	flagBlocks           string
	flagAfter            string
	flagTemplateID       string
	flagTemplateVars     []string
	flagListParent       string
	flagListType         string
	flagBlocking         string
	flagBlockedBy        string
	flagHasBlockers      bool
	flagNoBlockers       bool
	flagEditTitle        string
	flagStatusAll        bool
	flagLearnConcept     []string
	flagLearnFile        []string
	flagLearnEditSummary string
	flagLearnEditDetail  string
	flagLearnStaleReason string
	flagConceptsRecent   bool
	flagConceptsRelated  string
	flagConceptsSummary  string
	flagConceptsRename   string
	flagConceptsStats    bool
	flagContextConcept   []string
	flagContextQuery     string
	flagContextStale     bool
	flagContextSummary   bool
	flagContextID        string
	flagContextJSON      bool
	flagLearnDetail      string
	flagLabelsColor      string
	flagAddLabels        []string
	flagFilterLabels     []string
	flagStaleThreshold   string
	flagDoneOverride     bool

	flagDescription      string
	flagTemplateVarsYAML bool
	flagPrimeCustomize   bool
	flagPrimeRender      string
	flagVerbose          bool
)

func openDB() (*db.DB, error) {
	path, err := db.DefaultPath()
	if err != nil {
		return nil, err
	}
	database, err := db.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w (try running 'tpg init' first)", err)
	}
	// Run any pending migrations
	if err := database.Migrate(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}
	return database, nil
}

func resolveProject() (string, error) {
	if flagProject != "" {
		return flagProject, nil
	}
	return db.DefaultProject()
}

var rootCmd = &cobra.Command{
	Use:     "tpg",
	Short:   "Lightweight task management for agents",
	Version: version,
	Long: `A CLI for managing tasks, epics, and dependencies.
Designed for AI agents to track work across sessions.

Database: .tpg/tpg.db (in project root)

Quick start:
  tpg init
  tpg onboard
  tpg add "Build feature X" -p myproject
  tpg ready -p myproject
  tpg start <id>
  tpg log <id> "progress: made progress on X"
  tpg done <id> "Completed X, results in Y"

Use 'tpg [command] --help' for detailed help on any command.`,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the tpg database",
	Long:  "Creates the .tpg directory in the current directory and initializes the database.",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := db.InitProject(flagInitTaskPrefix, flagInitEpicPrefix)
		if err != nil {
			return err
		}
		database, err := db.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		if err := database.Init(); err != nil {
			return err
		}
		fmt.Printf("Initialized tpg database at %s\n", path)
		fmt.Println("\nNext: run 'tpg onboard' to set up Opencode integration")
		return nil
	},
}

var addCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Create a new task or epic",
	Long: `Create a new task (or epic with -e flag).

Returns the generated ID (ts-XXXXXX for tasks, ep-XXXXXX for epics). It is likely that current context
will be unknown when the task is executed, so provide full explanation of the task.

Examples:
  # Simple task
  tpg add "Fix login bug"

  # Task depending on another (common pattern)
  tpg add "Add tests for auth" --after ts-abc123

  # Task that blocks another
  tpg add "Design API schema" --blocks ts-xyz789

  # With detailed description via stdin
  tpg add "Implement auth" --desc - <<EOF
  Requirements: JWT tokens, refresh support
  Context: Replace auth/legacy.go
  Constraints: Use bcrypt, 1hr expiry
	(other detailed instructions defining the task and providing needed context)
  EOF

  # Epic grouping related tasks
  tpg add "Auth system" -e

  # Task with metadata
  tpg add "Critical fix" --priority 1 --parent ep-abc123 -l bug

  # From template (see 'tpg template list')
  tpg add "Feature X" --template tdd --vars-yaml <<EOF
  feature_name: user authentication
  problem: Users need secure login
  requirements: |
    - Validate email format
    - Hash passwords with bcrypt
  EOF`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			return err
		}

		// Handle template instantiation
		if flagTemplateID != "" {
			if flagParent != "" || flagBlocks != "" || flagAfter != "" || flagEpic {
				return fmt.Errorf("--template cannot be combined with --parent, --blocks, --after, or --epic")
			}

			// Handle template vars from stdin (YAML)
			varPairs := flagTemplateVars
			if flagTemplateVarsYAML {
				if len(flagTemplateVars) > 0 {
					return fmt.Errorf("cannot use both --var and --vars-yaml")
				}
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read from stdin: %w", err)
				}
				varPairs, err = parseTemplateVarsYAML(data)
				if err != nil {
					return fmt.Errorf("failed to parse YAML: %w", err)
				}
			}

			parentID, err := instantiateTemplate(database, project, strings.Join(args, " "), flagTemplateID, varPairs, flagPriority)
			if err != nil {
				return err
			}
			for _, labelName := range flagAddLabels {
				if err := database.AddLabelToItem(parentID, project, labelName); err != nil {
					return err
				}
			}
			fmt.Println(parentID)
			database.BackupQuiet()
			return nil
		}

		itemType := model.ItemTypeTask
		if flagEpic {
			itemType = model.ItemTypeEpic
		}

		itemID, err := db.GenerateItemID(itemType)
		if err != nil {
			return err
		}

		// Handle description from stdin or flag
		description := flagDescription
		if description == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			description = strings.TrimSpace(string(data))
		}

		item := &model.Item{
			ID:          itemID,
			Project:     project,
			Type:        itemType,
			Title:       strings.Join(args, " "),
			Description: description,
			Status:      model.StatusOpen,
			Priority:    flagPriority,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := database.CreateItem(item); err != nil {
			return err
		}

		// Set parent if specified
		if flagParent != "" {
			if err := database.SetParent(item.ID, flagParent); err != nil {
				return err
			}
		}

		// Add blocking relationship if specified
		if flagBlocks != "" {
			// This new item blocks the specified item
			// (the blocked item depends on this new one)
			if err := database.AddDep(flagBlocks, item.ID); err != nil {
				return err
			}
		}

		// Add dependency relationship if specified
		if flagAfter != "" {
			// This new item depends on the specified item
			// (this new item is blocked by the specified one)
			if err := database.AddDep(item.ID, flagAfter); err != nil {
				return err
			}
		}

		// Add labels if specified
		for _, labelName := range flagAddLabels {
			if err := database.AddLabelToItem(item.ID, item.Project, labelName); err != nil {
				return err
			}
		}

		fmt.Println(item.ID)

		// Backup after successful mutation
		database.BackupQuiet()

		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	Long: `List all tasks, optionally filtered by various criteria.

Examples:
  tpg list
  tpg list -p myproject
  tpg list --status open
  tpg list -p myproject --status blocked
  tpg list --parent ep-abc123
  tpg list --type epic
  tpg list --blocking ts-xyz789
  tpg list --blocked-by ts-abc123
  tpg list --has-blockers
  tpg list --no-blockers
  tpg list -l bug -l urgent`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Populate labels for display
		if err := database.PopulateItemLabels(items); err != nil {
			return err
		}
		if err := renderTemplatesForItems(items); err != nil {
			return err
		}

		printItemsTable(items)
		return nil
	},
}

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Show tasks ready for work (unblocked)",
	Long: `Show tasks that are ready to work on.

A task is "ready" when:
  - Status is "open" (not in_progress, blocked, or done)
  - All dependencies are "done"

Results are sorted by priority (1=high first).

Examples:
  tpg ready
  tpg ready -p myproject
  tpg ready -l bug`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			return err
		}

		// Record agent project access
		agentCtx := db.GetAgentContext()
		if agentCtx.IsActive() {
			_ = database.RecordAgentProjectAccess(agentCtx.ID, project)
		}

		items, err := database.ReadyItemsFiltered(project, flagFilterLabels)
		if err != nil {
			return err
		}

		if len(items) == 0 {
			fmt.Println("No ready tasks")
			return nil
		}

		// Populate labels for display
		if err := database.PopulateItemLabels(items); err != nil {
			return err
		}
		if err := renderTemplatesForItems(items); err != nil {
			return err
		}

		printReadyTable(items)
		return nil
	},
}

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show task details",
	Long: `Show full details for a task or epic.

Displays:
  - ID, type, status, priority, project, parent
  - Description (rendered from template if applicable)
  - Latest Update: most recent progress log, blockers, or results
  - Logs: timestamped audit trail
  - Dependencies: tasks that must complete first
  - Suggested concepts: for context retrieval

For templated tasks, the description is rendered from the current template.
A notice appears if the template changed since instantiation.

Example:
  tpg show ts-a1b2c3`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		item, err := database.GetItem(args[0])
		if err != nil {
			return err
		}

		// Record agent project access
		agentCtx := db.GetAgentContext()
		if agentCtx.IsActive() {
			_ = database.RecordAgentProjectAccess(agentCtx.ID, item.Project)
		}

		// Get labels for display
		labels, err := database.GetItemLabels(args[0])
		if err != nil {
			return err
		}
		for _, l := range labels {
			item.Labels = append(item.Labels, l.Name)
		}

		logs, err := database.GetLogs(args[0])
		if err != nil {
			return err
		}

		deps, err := database.GetDeps(args[0])
		if err != nil {
			return err
		}

		depStatuses, err := database.GetDepStatuses(args[0])
		if err != nil {
			return err
		}

		// Get related concepts for context suggestions
		concepts, err := database.GetRelatedConcepts(args[0])
		if err != nil {
			return err
		}

		templateNotice := ""
		cache := &templateCache{}
		if item.TemplateID != "" {
			mismatch, err := renderItemTemplate(cache, item)
			if err != nil {
				return err
			}
			if item.StepIndex == nil {
				tmpl, err := cache.get(item.TemplateID)
				if err != nil {
					return err
				}
				if item.TemplateHash != "" && tmpl.Hash != "" && item.TemplateHash != tmpl.Hash {
					mismatch = true
				}
			}
			if mismatch {
				templateNotice = "Template has changed since instantiation"
			}
		}

		latestProgress := latestProgressLog(logs)
		blockers := filterBlockers(depStatuses)
		printItemDetail(item, logs, deps, blockers, latestProgress, concepts, templateNotice)
		return nil
	},
}

var startCmd = &cobra.Command{
	Use:   "start <id>",
	Short: "Start working on a task",
	Long: `Set a task's status to in_progress.

Use this when you begin working on a task. Updates the timestamp
for stale detection.

Example:
  tpg start ts-a1b2c3`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		// Get item to record project access
		item, err := database.GetItem(args[0])
		if err != nil {
			return err
		}

		agentCtx := db.GetAgentContext()

		// Record agent project access
		if agentCtx.IsActive() {
			_ = database.RecordAgentProjectAccess(agentCtx.ID, item.Project)
		}

		if err := database.UpdateStatus(args[0], model.StatusInProgress, agentCtx); err != nil {
			return err
		}
		fmt.Printf("Started %s\n", args[0])
		return nil
	},
}

var doneCmd = &cobra.Command{
	Use:   "done <id> <results>",
	Short: "Mark a task as done",
	Long: `Mark a task as done with a results message.

The results message is required and should provide context for future work:
  - For implementation: what was built, where to find it, how to use it
  - For investigation: findings, decisions made, next steps
  - For fixes: root cause, solution applied, verification steps

Blocked if the task has unmet dependencies (use --override to force).

Use stdin with '-' for detailed results (recommended):

Examples:
  # Simple result
  tpg done ts-a1b2c3 "Added JWT auth, see auth/jwt.go"

  # Detailed results via stdin (recommended)
  tpg done ts-a1b2c3 - <<EOF
  ## What was built
  - JWT-based authentication system
  - Refresh token mechanism
  - Token expiry handling

  ## Key files
  - auth/jwt.go - Token generation and validation
  - auth/middleware.go - Authentication middleware
  - auth/refresh.go - Refresh token logic

  ## How to use
  See examples in auth/jwt_test.go
  Token format: Bearer <token> in Authorization header

  ## Notes
  - Tokens expire after 1 hour
  - Refresh tokens expire after 30 days
  - Uses RS256 signing algorithm
  EOF

  # Override dependency check
  tpg done ts-a1b2c3 --override "Work superseded by different approach"`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]
		results := strings.TrimSpace(strings.Join(args[1:], " "))

		// Handle stdin
		if results == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			results = strings.TrimSpace(string(data))
		}

		if results == "" {
			return fmt.Errorf("results message is required")
		}

		if !flagDoneOverride {
			hasUnmet, err := database.HasUnmetDeps(id)
			if err != nil {
				return err
			}
			if hasUnmet {
				return fmt.Errorf("cannot mark done with unmet dependencies (use --override to force)")
			}
		}

		agentCtx := db.GetAgentContext()
		if err := database.CompleteItem(id, results, agentCtx); err != nil {
			return err
		}
		fmt.Printf("Completed %s\n", id)

		// Prompt reflection
		fmt.Println(`
Reflect: What would help the next agent? (See instructions for guidance)
  tpg learn "summary" -c concept --detail "explanation"`)

		// Backup after successful mutation
		database.BackupQuiet()

		return nil
	},
}

var cancelCmd = &cobra.Command{
	Use:   "cancel <id> [reason]",
	Short: "Cancel a task without completing it",
	Long: `Cancel a task that is no longer relevant.

Use this instead of delete when you want to preserve the task history
but close it without marking it as successfully completed.

Example:
  tpg cancel ts-a1b2c3
  tpg cancel ts-a1b2c3 "Requirements changed, no longer needed"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]

		agentCtx := db.GetAgentContext()
		if err := database.UpdateStatus(id, model.StatusCanceled, agentCtx); err != nil {
			return err
		}

		if len(args) > 1 {
			reason := strings.Join(args[1:], " ")
			if err := database.AddLog(id, "Canceled: "+reason); err != nil {
				return err
			}
			fmt.Printf("Canceled %s: %s\n", id, reason)
		} else {
			fmt.Printf("Canceled %s\n", id)
		}

		// Backup after successful mutation
		database.BackupQuiet()

		return nil
	},
}

var staleCmd = &cobra.Command{
	Use:   "stale",
	Short: "List stale in-progress tasks",
	Long: `List in-progress tasks with no updates within a threshold.

Example:
  tpg stale
  tpg stale --threshold 30m`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			return err
		}

		threshold, err := time.ParseDuration(flagStaleThreshold)
		if err != nil {
			return fmt.Errorf("invalid threshold: %w", err)
		}
		cutoff := time.Now().Add(-threshold)

		items, err := database.StaleItems(project, cutoff)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Println("No stale tasks")
			return nil
		}

		fmt.Printf("Stale tasks (no updates in %s):\n\n", threshold)
		for _, item := range items {
			age := time.Since(item.UpdatedAt)
			fmt.Printf("%s [%s] %s (%s since update)\n", item.ID, item.Status, item.Title, formatDuration(age))
		}
		return nil
	},
}

var blockCmd = &cobra.Command{
	Use:   "block <id> <reason>",
	Short: "Mark a task as blocked",
	Long: `Mark a task as blocked and log the reason.

Use this when you can't proceed and need to hand off to another agent.

Example:
  tpg block ts-a1b2c3 "Need API spec from product team"`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]
		reason := strings.Join(args[1:], " ")

		agentCtx := db.GetAgentContext()
		if err := database.UpdateStatus(id, model.StatusBlocked, agentCtx); err != nil {
			return err
		}
		if err := database.AddLog(id, "Blocked: "+reason); err != nil {
			return err
		}
		fmt.Printf("Blocked %s: %s\n", id, reason)
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a task or epic",
	Long: `Permanently delete a task or epic and all associated data.

This removes the item, its logs, and any dependencies.
This action cannot be undone.

Example:
  tpg delete ts-a1b2c3`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		if err := database.DeleteItem(args[0]); err != nil {
			return err
		}
		fmt.Printf("Deleted %s\n", args[0])
		return nil
	},
}

var logCmd = &cobra.Command{
	Use:   "log <id> <message>",
	Short: "Add a log entry to a task",
	Long: `Add a timestamped log entry to a task's audit trail.

Updates the task's timestamp (affects stale detection).

Progress logs: Start message with "progress:" to mark major milestones.
Progress logs appear in the "Latest Update" section of tpg show, visible
to agents resuming work. Use them to communicate state to your future self.

For detailed progress updates, use stdin with '-' (recommended):

Examples:
  # Simple log
  tpg log ts-a1b2c3 "Implemented token refresh logic"

  # Progress milestone (visible in Latest Update)
  tpg log ts-a1b2c3 "progress: Auth complete, starting validation"

  # Detailed progress via stdin (recommended for handoffs)
  tpg log ts-a1b2c3 - <<EOF
  progress: JWT implementation complete, moving to refresh tokens
  
  ## What's done
  - Basic JWT generation working
  - Token validation with public key
  - Middleware integrated
  
  ## Next steps
  - Implement refresh token rotation
  - Add token revocation list
  
  ## Issues found
  - Public key loading needs caching (performance)
  - Error messages could be more specific
  EOF`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]
		message := strings.Join(args[1:], " ")

		// Handle stdin
		if message == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			message = strings.TrimSpace(string(data))
		}

		if err := database.AddLog(id, message); err != nil {
			return err
		}
		fmt.Printf("Logged to %s\n", id)
		return nil
	},
}

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Show dependency graph",
	Long: `Show all task dependencies as a graph.

Displays which tasks are blocked by other tasks.

Examples:
  tpg graph
  tpg graph -p myproject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			return err
		}

		edges, err := database.GetAllDeps(project)
		if err != nil {
			return err
		}

		if len(edges) == 0 {
			fmt.Println("No dependencies")
			return nil
		}

		printDepGraph(edges)
		return nil
	},
}

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all projects",
	Long:  `List all projects that have tasks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		projects, err := database.ListProjects()
		if err != nil {
			return err
		}

		if len(projects) == 0 {
			fmt.Println("No projects")
			return nil
		}

		for _, p := range projects {
			fmt.Println(p)
		}
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show project status overview",
	Long: `Show a summary of project status for agent spin-up.

Includes:
  - Count by status (open, in_progress, blocked, done)
  - Recently completed tasks
  - Currently in-progress tasks
  - Blocked tasks with reasons
  - Ready tasks by priority (limited to 10 by default)

Use --all to show all ready tasks.

Examples:
  tpg status
  tpg status -p myproject
  tpg status --all
  tpg status -l bug`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			return err
		}

		agentCtx := db.GetAgentContext()

		// Record agent project access
		if agentCtx.IsActive() {
			_ = database.RecordAgentProjectAccess(agentCtx.ID, project)
		}

		report, err := database.ProjectStatusFiltered(project, flagFilterLabels, agentCtx.ID)
		if err != nil {
			return err
		}

		// Populate labels for all item slices in the report
		_ = database.PopulateItemLabels(report.RecentDone)
		_ = database.PopulateItemLabels(report.InProgItems)
		_ = database.PopulateItemLabels(report.BlockedItems)
		_ = database.PopulateItemLabels(report.ReadyItems)

		cache := &templateCache{}
		if err := renderTemplatesWithCache(cache, report.RecentDone); err != nil {
			return err
		}
		if err := renderTemplatesWithCache(cache, report.InProgItems); err != nil {
			return err
		}
		if err := renderTemplatesWithCache(cache, report.BlockedItems); err != nil {
			return err
		}
		if err := renderTemplatesWithCache(cache, report.ReadyItems); err != nil {
			return err
		}

		printStatusReport(report, flagStatusAll)
		return nil
	},
}

var appendCmd = &cobra.Command{
	Use:   "append <id> <text>",
	Short: "Append text to a task's description",
	Long: `Append text to a task's description.

Use this to add context, decisions, or handoff notes.

Examples:
  tpg append ts-a1b2c3 "Decided to use JWT instead of sessions"
  
  # From stdin
  tpg append ts-a1b2c3 - <<EOF
  ## Decision
  Using JWT for stateless auth
  EOF`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]
		text := strings.Join(args[1:], " ")

		// Handle stdin
		if text == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			text = strings.TrimSpace(string(data))
		}

		if err := database.AppendDescription(id, text); err != nil {
			return err
		}
		fmt.Printf("Appended to %s\n", id)
		return nil
	},
}

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit a task's title or description",
	Long: `Edit a task's title or description.

With --title, updates the title directly without opening an editor.
Without flags, opens the description in your configured editor.

Uses $TPG_EDITOR if set, otherwise defaults to nvim, then nano, then vi.

Examples:
  tpg edit ts-a1b2c3                     # Edit description in editor
  tpg edit ts-a1b2c3 --title "New title" # Update title directly
  TPG_EDITOR=code tpg edit ts-a1b2c3    # Use VS Code as editor`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]

		// If --title flag is set, update title directly
		if flagEditTitle != "" {
			if err := database.SetTitle(id, flagEditTitle); err != nil {
				return err
			}
			fmt.Printf("Updated title for %s\n", id)
			return nil
		}

		// Get current description
		item, err := database.GetItem(id)
		if err != nil {
			return err
		}

		// Get editor (prefer $TPG_EDITOR, then nvim, then nano)
		editor := os.Getenv("TPG_EDITOR")
		if editor == "" {
			if _, err := exec.LookPath("nvim"); err == nil {
				editor = "nvim"
			} else if _, err := exec.LookPath("nano"); err == nil {
				editor = "nano"
			} else {
				editor = "vi"
			}
		}

		// Create temp file
		tmpfile, err := os.CreateTemp("", "tpg-edit-*.md")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath := tmpfile.Name()
		defer func() { _ = os.Remove(tmpPath) }()

		// Write current description
		if _, err := tmpfile.WriteString(item.Description); err != nil {
			_ = tmpfile.Close()
			return fmt.Errorf("failed to write temp file: %w", err)
		}
		if err := tmpfile.Close(); err != nil {
			return fmt.Errorf("failed to close temp file: %w", err)
		}

		// Get original stat for comparison
		origStat, err := os.Stat(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to stat temp file: %w", err)
		}

		// Open editor
		editorCmd := execCommand(editor, tmpPath)
		editorCmd.Stdin = os.Stdin
		editorCmd.Stdout = os.Stdout
		editorCmd.Stderr = os.Stderr
		if err := editorCmd.Run(); err != nil {
			return fmt.Errorf("editor failed: %w", err)
		}

		// Check if file was modified
		newStat, err := os.Stat(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to stat temp file: %w", err)
		}

		if newStat.ModTime().Equal(origStat.ModTime()) {
			fmt.Println("No changes made")
			return nil
		}

		// Read new content
		newContent, err := os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to read temp file: %w", err)
		}

		// Update description
		if err := database.SetDescription(id, string(newContent)); err != nil {
			return err
		}
		fmt.Printf("Updated description for %s\n", id)
		return nil
	},
}

// execCommand wraps exec.Command for testing
var execCommand = func(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

var descCmd = &cobra.Command{
	Use:   "desc <id> <text>",
	Short: "Replace a task's description",
	Long: `Replace a task's entire description with new text.

Use this when you need to rewrite or fix the description content.
For adding to existing content, use 'tpg append' instead.

Examples:
  tpg desc ts-a1b2c3 "New description text here"
  
  # From stdin
  tpg desc ts-a1b2c3 - <<EOF
  # Updated Requirements
  - Requirement 1
  - Requirement 2
  EOF`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]
		text := strings.Join(args[1:], " ")

		// Handle stdin
		if text == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			text = strings.TrimSpace(string(data))
		}

		if err := database.SetDescription(id, text); err != nil {
			return err
		}
		fmt.Printf("Updated description for %s\n", id)
		return nil
	},
}

var parentCmd = &cobra.Command{
	Use:   "parent <id> <epic-id>",
	Short: "Set a task's parent epic",
	Long: `Set the parent epic for a task.

This establishes a hierarchical relationship where tasks belong to epics.
The parent must be an epic (created with -e flag).

Example:
  tpg parent ts-a1b2c3 ep-d4e5f6
  # ts-a1b2c3 is now a child of ep-d4e5f6`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		if err := database.SetParent(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("%s is now under %s\n", args[0], args[1])
		return nil
	},
}

var projectCmd = &cobra.Command{
	Use:   "project <id> <project>",
	Short: "Set a task's project",
	Long: `Set or change the project for a task.

The project will be created if it doesn't exist.

Example:
  tpg project ts-a1b2c3 myproject
  # ts-a1b2c3 is now in myproject`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		if err := database.SetProject(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("%s is now in project %s\n", args[0], args[1])
		return nil
	},
}

var blocksCmd = &cobra.Command{
	Use:   "blocks <id> <other-id>",
	Short: "Mark a task as blocking another",
	Long: `Mark a task as blocking another task.

The second task cannot be started until the first is done.

Example:
  tpg blocks ts-a1b2c3 ts-d4e5f6
  # ts-d4e5f6 cannot start until ts-a1b2c3 is done`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		// blocks A B means B depends on A (A blocks B)
		if err := database.AddDep(args[1], args[0]); err != nil {
			return err
		}
		fmt.Printf("%s now blocks %s\n", args[0], args[1])
		return nil
	},
}

var labelCmd = &cobra.Command{
	Use:   "label <item-id> <label-name>",
	Short: "Add a label to a task",
	Long: `Add a label to a task or epic.

Creates the label if it doesn't exist (like concepts).

Example:
  tpg label ts-a1b2c3 bug
  tpg label ts-a1b2c3 urgent`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		// Get item to find its project
		item, err := database.GetItem(args[0])
		if err != nil {
			return err
		}

		if err := database.AddLabelToItem(args[0], item.Project, args[1]); err != nil {
			return err
		}
		fmt.Printf("Added label %q to %s\n", args[1], args[0])
		return nil
	},
}

var unlabelCmd = &cobra.Command{
	Use:   "unlabel <item-id> <label-name>",
	Short: "Remove a label from a task",
	Long: `Remove a label from a task or epic.

Example:
  tpg unlabel ts-a1b2c3 bug`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		// Get item to find its project
		item, err := database.GetItem(args[0])
		if err != nil {
			return err
		}

		if err := database.RemoveLabelFromItem(args[0], item.Project, args[1]); err != nil {
			return err
		}
		fmt.Printf("Removed label %q from %s\n", args[1], args[0])
		return nil
	},
}

var learnCmd = &cobra.Command{
	Use:   "learn <summary>",
	Short: "Log a learning for future context retrieval",
	Long: `Log a learning discovered during work.

Learnings are tagged with concepts for organized retrieval.
Concepts are created automatically if they don't exist.

If a task is in progress for the project, the learning is linked to it.

Examples:
  tpg learn "Token refresh has race condition" -p myproject -c auth -c concurrency
  tpg learn "Config loaded from env first" -p myproject -c config -f config.go
  tpg learn "Token refresh issue" -c auth -p myproject --detail "The mutex only protects..."
  echo "multi-line detail" | tpg learn "summary" -c auth -p myproject --detail -`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate required flags
		if len(flagLearnConcept) == 0 {
			return fmt.Errorf("at least one concept is required (-c)")
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

		// Get current in-progress task for this project
		taskID, _ := database.GetCurrentTaskID(project)

		// Handle detail from stdin
		detail := flagLearnDetail
		if detail == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			detail = strings.TrimSpace(string(data))
		}

		now := time.Now()
		learning := &model.Learning{
			ID:        model.GenerateLearningID(),
			Project:   project,
			CreatedAt: now,
			UpdatedAt: now,
			TaskID:    taskID,
			Summary:   strings.Join(args, " "),
			Detail:    detail,
			Status:    model.LearningStatusActive,
			Concepts:  flagLearnConcept,
			Files:     flagLearnFile,
		}

		if err := database.CreateLearning(learning); err != nil {
			return err
		}

		// Build output
		output := learning.ID
		if taskID != nil {
			output += fmt.Sprintf(" (linked to %s)", *taskID)
		}
		fmt.Println(output)

		// Backup after successful mutation
		database.BackupQuiet()

		return nil
	},
}

var learnEditCmd = &cobra.Command{
	Use:   "edit <learning-id>",
	Short: "Edit a learning's summary or detail",
	Long: `Edit an existing learning's summary or detail.

Examples:
  tpg learn edit lrn-abc123 --summary "Updated summary"
  tpg learn edit lrn-abc123 --detail "Full context explanation"
  echo "multi-line" | tpg learn edit lrn-abc123 --detail -`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagLearnEditSummary == "" && flagLearnEditDetail == "" {
			return fmt.Errorf("--summary or --detail is required")
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		if flagLearnEditSummary != "" {
			if err := database.UpdateLearningSummary(args[0], flagLearnEditSummary); err != nil {
				return err
			}
		}

		if flagLearnEditDetail != "" {
			detail := flagLearnEditDetail
			if detail == "-" {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read from stdin: %w", err)
				}
				detail = strings.TrimSpace(string(data))
			}
			if err := database.UpdateLearningDetail(args[0], detail); err != nil {
				return err
			}
		}

		fmt.Printf("Updated %s\n", args[0])
		return nil
	},
}

var learnStaleCmd = &cobra.Command{
	Use:   "stale <learning-id> [learning-id...]",
	Short: "Mark learnings as stale (outdated)",
	Long: `Mark one or more learnings as stale when they're outdated but still useful for reference.

Examples:
  tpg learn stale lrn-abc123 --reason "Refactored in v2"
  tpg learn stale lrn-a lrn-b lrn-c --reason "Compacted into lrn-xyz"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		for _, id := range args {
			if err := database.UpdateLearningStatus(id, model.LearningStatusStale); err != nil {
				return err
			}
		}

		// Output
		if len(args) == 1 {
			if flagLearnStaleReason != "" {
				fmt.Printf("Marked %s as stale: %s\n", args[0], flagLearnStaleReason)
			} else {
				fmt.Printf("Marked %s as stale\n", args[0])
			}
		} else {
			if flagLearnStaleReason != "" {
				fmt.Printf("Marked %d learnings as stale: %s\n", len(args), flagLearnStaleReason)
			} else {
				fmt.Printf("Marked %d learnings as stale\n", len(args))
			}
		}
		return nil
	},
}

var learnRmCmd = &cobra.Command{
	Use:   "rm <learning-id>",
	Short: "Delete a learning",
	Long: `Permanently delete a learning and its concept associations.

Example:
  tpg learn rm lrn-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		if err := database.DeleteLearning(args[0]); err != nil {
			return err
		}
		fmt.Printf("Deleted %s\n", args[0])
		return nil
	},
}

var conceptsCmd = &cobra.Command{
	Use:   "concepts [name]",
	Short: "List or edit concepts for a project",
	Long: `List all concepts for a project, or edit a concept.

Concepts are knowledge categories that group related learnings.
Default sort is by learning count (most used first).

Examples:
  tpg concepts -p myproject                        # list concepts
  tpg concepts -p myproject --recent               # sort by last updated
  tpg concepts -p myproject --stats                # show count and oldest age
  tpg concepts --related ts-abc123                 # suggest concepts for a task
  tpg concepts fts -p myproject --summary "..."    # set concept summary
  tpg concepts fts -p myproject --rename "search"  # rename concept`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		// Edit mode: concept name provided with --summary or --rename
		if len(args) > 0 && (flagConceptsSummary != "" || flagConceptsRename != "") {
			project, err := resolveProject()
			if err != nil {
				return err
			}
			if flagConceptsSummary != "" {
				if err := database.SetConceptSummary(args[0], project, flagConceptsSummary); err != nil {
					return err
				}
				fmt.Printf("Updated %s\n", args[0])
			}
			if flagConceptsRename != "" {
				if err := database.RenameConcept(args[0], flagConceptsRename, project); err != nil {
					return err
				}
				fmt.Printf("Renamed %s -> %s\n", args[0], flagConceptsRename)
			}
			return nil
		}

		// Stats mode
		if flagConceptsStats {
			project, err := resolveProject()
			if err != nil {
				return err
			}
			stats, err := database.ListConceptsWithStats(project)
			if err != nil {
				return err
			}
			if len(stats) == 0 {
				fmt.Println("No concepts")
				return nil
			}
			printConceptsStats(stats)
			return nil
		}

		// List mode
		var concepts []model.Concept

		if flagConceptsRelated != "" {
			// Get concepts related to a task
			concepts, err = database.GetRelatedConcepts(flagConceptsRelated)
			if err != nil {
				return err
			}
		} else {
			// List all concepts for project
			project, err := resolveProject()
			if err != nil {
				return err
			}
			concepts, err = database.ListConcepts(project, flagConceptsRecent)
			if err != nil {
				return err
			}
		}

		if len(concepts) == 0 {
			fmt.Println("No concepts")
			return nil
		}

		printConceptsTable(concepts)
		return nil
	},
}

var labelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "List or manage labels for a project",
	Long: `List all labels for a project.

Labels are tags for categorizing tasks (bug, feature, refactor, etc).
Labels are project-scoped and identified by name.

Examples:
  tpg labels -p myproject           # list all labels
  tpg labels add bug -p myproject   # create a label
  tpg labels rm bug -p myproject    # delete a label
  tpg labels rename bug critical -p myproject`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			return err
		}
		labels, err := database.ListLabels(project)
		if err != nil {
			return err
		}

		if len(labels) == 0 {
			fmt.Println("No labels")
			return nil
		}

		printLabelsTable(labels)
		return nil
	},
}

var labelsAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a new label",
	Long: `Create a new label in a project.

Labels are created on first use when attached to items, but you can
also create them explicitly with this command.

Examples:
  tpg labels add bug -p myproject
  tpg labels add urgent -p myproject --color "#ff0000"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			return err
		}

		now := time.Now()
		label := &model.Label{
			ID:        model.GenerateLabelID(),
			Name:      args[0],
			Project:   project,
			Color:     flagLabelsColor,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := database.CreateLabel(label); err != nil {
			return err
		}
		fmt.Printf("Created label: %s\n", args[0])
		return nil
	},
}

var labelsRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Delete a label",
	Long: `Delete a label and remove it from all items.

Example:
  tpg labels rm bug -p myproject`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			return err
		}
		if err := database.DeleteLabel(project, args[0]); err != nil {
			return err
		}
		fmt.Printf("Deleted label: %s\n", args[0])
		return nil
	},
}

var labelsRenameCmd = &cobra.Command{
	Use:   "rename <old-name> <new-name>",
	Short: "Rename a label",
	Long: `Rename a label. All items with this label will be updated.

Example:
  tpg labels rename bug critical -p myproject`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			return err
		}
		if err := database.RenameLabel(project, args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Renamed label: %s -> %s\n", args[0], args[1])
		return nil
	},
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Retrieve learnings for context",
	Long: `Retrieve learnings by concept, full-text search, or specific ID.

Use this to load relevant context before starting work on a task.

Examples:
  tpg context -p myproject --summary                # all learnings, grouped by concept
  tpg context -c auth -c concurrency -p myproject   # by concepts
  tpg context -q "rate limit" -p myproject          # full-text search
  tpg context -c auth --summary -p myproject        # one-liner per learning
  tpg context --id lrn-abc123                       # specific learning by ID
  tpg context -c auth --include-stale -p myproject  # include stale learnings
  tpg context -c auth --json -p myproject           # JSON output for agents`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		// Mode 1: Specific learning by ID
		if flagContextID != "" {
			learning, err := database.GetLearning(flagContextID)
			if err != nil {
				return err
			}
			if flagContextJSON {
				return printLearningsJSON([]model.Learning{*learning})
			}
			printLearnings([]model.Learning{*learning})
			return nil
		}

		project, err := resolveProject()
		if err != nil {
			return err
		}

		// Mode 2: All learnings with --summary (no concepts/query required)
		if flagContextSummary && len(flagContextConcept) == 0 && flagContextQuery == "" {
			learnings, err := database.GetAllLearnings(project, flagContextStale)
			if err != nil {
				return err
			}

			if len(learnings) == 0 {
				if flagContextJSON {
					fmt.Println("[]")
					return nil
				}
				fmt.Println("No learnings found")
				return nil
			}

			if flagContextJSON {
				return printLearningsJSON(learnings)
			}

			// Get concept summaries for grouped output
			concepts, _ := database.ListConcepts(project, false)
			conceptMap := make(map[string]string)
			for _, c := range concepts {
				conceptMap[c.Name] = c.Summary
			}
			printAllLearningSummaries(learnings, conceptMap)
			return nil
		}

		// Modes 3 & 4 require concepts or query
		if len(flagContextConcept) == 0 && flagContextQuery == "" {
			return fmt.Errorf("specify concepts (-c), query (-q), or use --summary for all")
		}

		var learnings []model.Learning

		if len(flagContextConcept) > 0 {
			learnings, err = database.GetLearningsByConcepts(project, flagContextConcept, flagContextStale)
		} else {
			learnings, err = database.SearchLearnings(project, flagContextQuery, flagContextStale)
		}
		if err != nil {
			return err
		}

		if len(learnings) == 0 {
			if flagContextJSON {
				fmt.Println("[]")
				return nil
			}
			fmt.Println("No learnings found")
			return nil
		}

		// JSON mode
		if flagContextJSON {
			return printLearningsJSON(learnings)
		}

		// Mode 3: Summary mode (one-liners) for specific concepts
		if flagContextSummary {
			// Get concept summaries for header
			concepts, _ := database.ListConcepts(project, false)
			conceptMap := make(map[string]string)
			for _, c := range concepts {
				conceptMap[c.Name] = c.Summary
			}
			printLearningSummaries(learnings, flagContextConcept, conceptMap)
			return nil
		}

		// Mode 4: Full output
		printLearnings(learnings)
		return nil
	},
}

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Set up tpg integration for AI agents",
	Long: `Set up tpg integration for AI agents.

This command:
1. Writes a tpg workflow snippet to AGENTS.md in the current directory
2. Installs the OpenCode plugin for automatic context injection

Creates files if they don't exist. Skips if already configured (use --force to update).

Example:
  cd ~/code/myproject
  tpg onboard
  tpg onboard --force      # Update existing configuration`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runOnboardOpencode(flagForce)
	},
}

func findAgentsMD() string {
	// Check for existing file with exact case match
	// (os.Stat is case-insensitive on macOS, so we use ReadDir)
	entries, err := os.ReadDir(".")
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.EqualFold(name, "agents.md") {
				return name // Return actual casing
			}
		}
	}
	// Default to uppercase if none exists
	return "AGENTS.md"
}

// runOnboardOpencode sets up tpg integration for Opencode (writes to AGENTS.md)
func runOnboardOpencode(force bool) error {
	agentsPath := findAgentsMD()
	snippet := `## Task Tracking

This project uses **tpg** for cross-session task management.
Run ` + "`tpg prime`" + ` for workflow context, or configure hooks for auto-injection.

**Quick reference:**
` + "```" + `
tpg ready              # Find unblocked work
tpg add "Title" -p X   # Create task
tpg start <id>         # Claim work
tpg log <id> "msg"     # Log progress
tpg done <id>          # Complete work
` + "```" + `

For full workflow: ` + "`tpg prime`" + `
`

	// Check if file exists
	content, err := os.ReadFile(agentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new file
			if err := os.WriteFile(agentsPath, []byte(snippet), 0644); err != nil {
				return fmt.Errorf("failed to create %s: %w", agentsPath, err)
			}
			fmt.Printf("Created %s with tpg integration\n", agentsPath)
		} else {
			return fmt.Errorf("failed to read %s: %w", agentsPath, err)
		}
	} else {
		// Check if already onboarded
		if strings.Contains(string(content), "## Task Tracking") {
			if force {
				// Replace existing section
				newContent := replaceTaskTrackingSection(string(content), snippet)
				if err := os.WriteFile(agentsPath, []byte(newContent), 0644); err != nil {
					return fmt.Errorf("failed to update %s: %w", agentsPath, err)
				}
				fmt.Printf("Updated Task Tracking section in %s\n", agentsPath)
			} else {
				fmt.Printf("%s already has Task Tracking section\n", agentsPath)
			}
		} else {
			// Append to existing file
			newContent := string(content)
			if !strings.HasSuffix(newContent, "\n") {
				newContent += "\n"
			}
			newContent += "\n" + snippet

			if err := os.WriteFile(agentsPath, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("failed to update %s: %w", agentsPath, err)
			}
			fmt.Printf("Added tpg integration to %s\n", agentsPath)
		}
	}

	// Install OpenCode plugin if opencode or shuvcode is available
	if installed, upToDate, symlink, err := installOpencodePlugin(force); err != nil {
		fmt.Printf("\nWarning: failed to install OpenCode plugin: %v\n", err)
	} else if installed {
		fmt.Println("\nInstalled OpenCode plugin to .opencode/plugins/tpg.ts")
		fmt.Println("  Plugin injects tpg prime on new sessions and compaction,")
		fmt.Println("  and sets AGENT_ID/AGENT_TYPE on tpg commands.")
	} else if upToDate {
		fmt.Println("\nOpenCode plugin already up to date (.opencode/plugins/tpg.ts)")
	} else if symlink {
		fmt.Println("\nOpenCode plugin is symlinked (.opencode/plugins/tpg.ts)")
	} else if detectOpencode() != "" {
		fmt.Println("\nOpenCode plugin was modified (.opencode/plugins/tpg.ts)")
		fmt.Println("  Skipping automatic update to preserve your changes.")
		fmt.Println("  Use --force to overwrite with latest version.")
	} else {
		fmt.Println()
		fmt.Println("For hooks, add 'tpg prime' to your agent's session start hook if available.")
		fmt.Println("Otherwise, run 'tpg prime' and paste output into agent context.")
	}

	// Add .tpg/tpg.db to .gitignore if not already present
	if err := ensureGitignore(); err != nil {
		fmt.Printf("\nWarning: failed to update .gitignore: %v\n", err)
	}

	return nil
}

// ensureGitignore adds .tpg/tpg.db to .gitignore if not already present
func ensureGitignore() error {
	gitignorePath := ".gitignore"
	entry := ".tpg/tpg.db"

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new .gitignore
			if err := os.WriteFile(gitignorePath, []byte(entry+"\n"), 0644); err != nil {
				return err
			}
			fmt.Println("\nCreated .gitignore with .tpg/tpg.db")
			return nil
		}
		return err
	}

	// Check if already present
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == entry {
			return nil // Already present
		}
	}

	// Append to existing .gitignore
	newContent := string(content)
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += entry + "\n"

	if err := os.WriteFile(gitignorePath, []byte(newContent), 0644); err != nil {
		return err
	}
	fmt.Println("\nAdded .tpg/tpg.db to .gitignore")
	return nil
}

// detectOpencode returns the name of the installed opencode binary ("opencode" or "shuvcode"),
// or empty string if neither is found.
func detectOpencode() string {
	if _, err := exec.LookPath("opencode"); err == nil {
		return "opencode"
	}
	if _, err := exec.LookPath("shuvcode"); err == nil {
		return "shuvcode"
	}
	return ""
}

// pluginVersionPattern matches the version header line in the plugin file
var pluginVersionPattern = regexp.MustCompile(`^// tpg-plugin version:(\S+) hash:(\S+)`)

// calculatePluginHash computes SHA256 hash of plugin content
func calculatePluginHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:8]) // Use first 8 chars for brevity
}

// readPluginVersion reads the version and hash from the first line of plugin file
// Returns (version, hash, restOfFile, error)
func readPluginVersion(path string) (string, string, string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", "", err
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return "", "", "", fmt.Errorf("empty file")
	}

	firstLine := lines[0]
	matches := pluginVersionPattern.FindStringSubmatch(firstLine)
	if matches == nil {
		// No header - file was modified or is from older version
		return "", "", string(content), nil
	}

	version := matches[1]
	hash := matches[2]
	rest := strings.Join(lines[1:], "\n")
	return version, hash, rest, nil
}

// installOpencodePlugin installs the tpg plugin into .opencode/plugins/tpg.ts.
// Returns (installed, upToDate, isSymlink, error):
//   - installed: true if a new/updated version was written
//   - upToDate: true if already at current version (not modified)
//   - isSymlink: true if plugin is a symlink (user customization)
//   - error: any error that occurred
func installOpencodePlugin(force bool) (bool, bool, bool, error) {
	if detectOpencode() == "" {
		return false, false, false, nil
	}

	pluginDir := filepath.Join(".opencode", "plugins")
	pluginPath := filepath.Join(pluginDir, "tpg.ts")

	// Check if it's a symlink - never overwrite symlinks (user intentionally linked it)
	if info, err := os.Lstat(pluginPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return false, false, true, nil // Symlink exists, leave it alone
		}
	}

	// Use embedded source (--force always uses latest embedded version)
	source := plugin.OpencodeSource

	// Check if plugin already exists and decide whether to update
	if _, err := os.Stat(pluginPath); err == nil && !force {
		// File exists and we're not forcing - check if we should update
		oldVersion, oldHash, oldContent, err := readPluginVersion(pluginPath)
		if err != nil {
			// Can't read it, skip
			return false, false, false, nil
		}

		if oldVersion == "" {
			// No version header - file was modified (old version or custom)
			return false, false, false, nil
		}

		// Calculate hash of current content (without header)
		currentHash := calculatePluginHash(oldContent)

		if currentHash != oldHash {
			// Content was modified
			return false, false, false, nil
		}

		// File is unmodified, safe to update
		if oldVersion == version {
			// Already up to date
			return false, true, false, nil
		}

		// Need to update - fall through to write
	} else if err != nil {
		// File doesn't exist, will install
	}

	// Write plugin to project
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return false, false, false, fmt.Errorf("failed to create %s: %w", pluginDir, err)
	}

	// Calculate hash of new source and add header
	sourceHash := calculatePluginHash(source)
	header := fmt.Sprintf("// tpg-plugin version:%s hash:%s (auto-generated, do not modify)\n", version, sourceHash)
	contentWithHeader := header + source

	// Write the file
	if err := os.WriteFile(pluginPath, []byte(contentWithHeader), 0644); err != nil {
		return false, false, false, fmt.Errorf("failed to write plugin: %w", err)
	}

	return true, false, false, nil
}

// replaceTaskTrackingSection finds the "## Task Tracking" section and replaces it
// with the new snippet. The section ends at the next heading (# or ##) or EOF.
func replaceTaskTrackingSection(content, snippet string) string {
	// Find where "## Task Tracking" starts
	startIdx := strings.Index(content, "## Task Tracking")
	if startIdx == -1 {
		return content
	}

	// Find where the section ends (next heading or EOF)
	rest := content[startIdx+len("## Task Tracking"):]
	endIdx := -1

	// Look for next heading (line starting with #)
	lines := strings.Split(rest, "\n")
	charCount := 0
	for i, line := range lines {
		if i > 0 && len(line) > 0 && line[0] == '#' {
			endIdx = startIdx + len("## Task Tracking") + charCount
			break
		}
		charCount += len(line) + 1 // +1 for newline
	}

	// Build new content
	before := content[:startIdx]
	var after string
	if endIdx == -1 {
		// Section goes to EOF
		after = ""
	} else {
		after = content[endIdx:]
	}

	// Ensure proper spacing
	result := strings.TrimRight(before, "\n")
	if result != "" {
		result += "\n\n"
	}
	result += strings.TrimRight(snippet, "\n")
	if after != "" {
		result += "\n\n" + strings.TrimLeft(after, "\n")
	} else {
		result += "\n"
	}

	return result
}

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Output context for AI agent hooks",
	Long: `Output essential workflow context for AI agents.

Designed to run on session start hooks to ensure agents maintain
context about the tpg workflow.

Customize the output template with --customize. Use --render to test
a specific template file.

For Opencode: The plugin installed by 'tpg onboard' handles this automatically.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		config, _ := db.LoadConfig()
		agentCtx := db.GetAgentContext()

		// Handle --customize flag
		if flagPrimeCustomize {
			return handlePrimeCustomize()
		}

		// Handle --render flag
		if flagPrimeRender != "" {
			return handlePrimeRender(flagPrimeRender, database, config, agentCtx)
		}

		// Normal prime operation
		if err != nil {
			// Still output prime content even if DB fails
			renderPrime(nil, config, agentCtx, nil)
			return nil
		}
		defer func() { _ = database.Close() }()

		project, _ := resolveProject()

		// Record agent project access
		if agentCtx.IsActive() {
			_ = database.RecordAgentProjectAccess(agentCtx.ID, project)
		}

		report, _ := database.ProjectStatusFiltered(project, nil, agentCtx.ID)

		renderPrime(report, config, agentCtx, database)
		return nil
	},
}

func init() {
	primeCmd.Flags().BoolVar(&flagPrimeCustomize, "customize", false, "Create/edit custom prime template")
	primeCmd.Flags().StringVar(&flagPrimeRender, "render", "", "Render specific template file (for testing)")
}

var compactCmd = &cobra.Command{
	Use:   "compact",
	Short: "Output compaction workflow guidance for agents",
	Long: `Output guidance for grooming learnings and concepts.

Use this when learnings have accumulated and need review.
Covers identifying redundancy, staleness, and quality issues.

The workflow uses two phases:
1. Discovery: Scan summaries to identify grooming candidates
2. Selection: Load detail for candidates, groom, repeat

Example:
  tpg compact              # Output compaction guidance
  tpg compact -p myproject # Include project-specific stats`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			printCompactContent(nil)
			return nil
		}
		defer func() { _ = database.Close() }()

		project, err := resolveProject()
		if err != nil {
			printCompactContent(nil)
			return nil
		}
		stats, _ := database.ListConceptsWithStats(project)

		printCompactContent(stats)
		return nil
	},
}

// Template commands

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage templates",
	Long: `Commands for managing templates.

Templates define standardized ways to solve problems. A "tdd" template encodes
the standard approach for test-driven development. A "discovery" template
defines how to investigate unknowns.

Template locations (searched in priority order):
  1. Project: .tpg/templates/ (searched upward from current directory)
  2. User: ~/.config/tpg/templates/
  3. Global: ~/.config/opencode/tpg-templates/`,
}

var flagTemplateListDetail bool

var templateListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List available templates",
	Long: `List all available templates from all template locations.

Default output is a compact one-liner per template.
Use --detail for full variable and step information.

Templates from more local locations (project) override global ones with the same ID.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tmpls, err := templates.ListTemplates()
		if err != nil {
			return err
		}

		if len(tmpls) == 0 {
			fmt.Println("No templates found")
			fmt.Println("\nTemplate locations searched:")
			locs := templates.GetTemplateLocations()
			if len(locs) == 0 {
				fmt.Println("  (none found)")
				fmt.Println("\nCreate templates in one of these locations:")
				fmt.Println("  .tpg/templates/           (project)")
				fmt.Println("  ~/.config/tpg/templates/  (user)")
				fmt.Println("  ~/.config/opencode/tpg-templates/ (global)")
			} else {
				for _, loc := range locs {
					fmt.Printf("  %s (%s)\n", loc.Path, loc.Source)
				}
			}
			return nil
		}

		if flagTemplateListDetail {
			// Detailed output (original behavior)
			for i, tmpl := range tmpls {
				if i > 0 {
					fmt.Println()
				}
				fmt.Printf("%s", tmpl.ID)
				if tmpl.Source != "" {
					fmt.Printf(" (%s)", tmpl.Source)
				}
				fmt.Println()
				if tmpl.Description != "" {
					fmt.Printf("  %s\n", tmpl.Description)
				}
				if len(tmpl.Variables) > 0 {
					fmt.Println("  Variables:")
					for name, v := range tmpl.Variables {
						req := "required"
						if v.Optional {
							req = "optional"
						}
						if v.Default != "" {
							req = fmt.Sprintf("default: %s", v.Default)
						}
						desc := v.Description
						if desc == "" {
							desc = "(no description)"
						}
						fmt.Printf("    %s (%s): %s\n", name, req, desc)
					}
				}
				fmt.Printf("  Steps: %d\n", len(tmpl.Steps))
			}
		} else {
			// Compact output (new default)
			for _, tmpl := range tmpls {
				desc := tmpl.Description
				if desc == "" {
					desc = "(no description)"
				}
				fmt.Printf("%s (%d variables, %d steps): %s\n", tmpl.ID, len(tmpl.Variables), len(tmpl.Steps), desc)
			}
			fmt.Println("\nUse 'tpg template usage <id>' for usage details")
		}
		return nil
	},
}

var templateShowCmd = &cobra.Command{
	Use:   "show <template-id>",
	Short: "Show template details",
	Long: `Show detailed information about a template.

Displays:
  - Template ID, source file, title, description
  - Variables: name, required/optional, default value, description
  - Steps: ID, title, description, dependencies

Use this to understand what a template does before instantiating it.

Example:
  tpg template show tdd`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tmpl, err := templates.LoadTemplate(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Template: %s\n", tmpl.ID)
		if tmpl.Source != "" {
			fmt.Printf("Source: %s (%s)\n", tmpl.SourcePath, tmpl.Source)
		} else {
			fmt.Printf("Source: %s\n", tmpl.SourcePath)
		}
		if tmpl.Title != "" {
			fmt.Printf("Title: %s\n", tmpl.Title)
		}
		if tmpl.Description != "" {
			fmt.Printf("Description: %s\n", tmpl.Description)
		}

		if len(tmpl.Variables) > 0 {
			fmt.Println("\nVariables:")
			for name, v := range tmpl.Variables {
				req := "required"
				if v.Optional {
					req = "optional"
				}
				if v.Default != "" {
					req = fmt.Sprintf("default: %q", v.Default)
				}
				fmt.Printf("  %s (%s)\n", name, req)
				if v.Description != "" {
					fmt.Printf("    %s\n", v.Description)
				}
			}
		}

		fmt.Println("\nSteps:")
		for i, step := range tmpl.Steps {
			stepID := step.ID
			if stepID == "" {
				stepID = fmt.Sprintf("step-%d", i+1)
			}
			fmt.Printf("  %d. [%s] %s\n", i+1, stepID, step.Title)
			if step.Description != "" {
				// Indent description
				lines := strings.Split(step.Description, "\n")
				for _, line := range lines {
					fmt.Printf("       %s\n", line)
				}
			}
			if len(step.Depends) > 0 {
				fmt.Printf("       Depends: %s\n", strings.Join(step.Depends, ", "))
			}
		}

		return nil
	},
}

var templateUsageCmd = &cobra.Command{
	Use:   "usage <template-id>",
	Short: "Show template usage and variables",
	Long: `Show how to use a template, including all variables and their descriptions.

Unlike 'show', this does not display the full step descriptions - just enough
to understand what variables to provide.

Example:
  tpg template usage tdd-task`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tmpl, err := templates.LoadTemplate(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("Template: %s\n", tmpl.ID)
		if tmpl.Description != "" {
			fmt.Printf("  %s\n", tmpl.Description)
		}
		fmt.Printf("  Steps: %d\n", len(tmpl.Steps))

		if len(tmpl.Variables) > 0 {
			// Separate required and optional
			var required, optional []string
			for name := range tmpl.Variables {
				v := tmpl.Variables[name]
				if v.Optional || v.Default != "" {
					optional = append(optional, name)
				} else {
					required = append(required, name)
				}
			}

			if len(required) > 0 {
				fmt.Println("\nRequired variables:")
				for _, name := range required {
					v := tmpl.Variables[name]
					fmt.Printf("  %s\n", name)
					if v.Description != "" {
						fmt.Printf("    %s\n", v.Description)
					}
				}
			}

			if len(optional) > 0 {
				fmt.Println("\nOptional variables:")
				for _, name := range optional {
					v := tmpl.Variables[name]
					if v.Default != "" {
						fmt.Printf("  %s (default: %s)\n", name, v.Default)
					} else {
						fmt.Printf("  %s\n", name)
					}
					if v.Description != "" {
						fmt.Printf("    %s\n", v.Description)
					}
				}
			}
		}

		// Show example usage
		fmt.Println("\nExample:")
		fmt.Printf("  tpg add \"Title\" --template %s --vars-yaml <<EOF\n", tmpl.ID)
		for name := range tmpl.Variables {
			v := tmpl.Variables[name]
			if !v.Optional && v.Default == "" {
				fmt.Printf("  %s: \"value\"\n", name)
			}
		}
		fmt.Println("  EOF")

		return nil
	},
}

var templateLocationsCmd = &cobra.Command{
	Use:   "locations",
	Short: "Show template search locations",
	Long:  `Show all directories that are searched for templates, in priority order.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		locs := templates.GetTemplateLocations()

		fmt.Println("Template search locations (highest priority first):")
		fmt.Println()

		if len(locs) == 0 {
			fmt.Println("No template directories found.")
			fmt.Println()
			fmt.Println("Templates can be placed in:")
			fmt.Println("  .tpg/templates/                    (project - searched upward)")
			fmt.Println("  ~/.config/tpg/templates/           (user)")
			fmt.Println("  ~/.config/opencode/tpg-templates/  (global)")
		} else {
			for _, loc := range locs {
				fmt.Printf("  [%s] %s\n", loc.Source, loc.Path)
			}
		}

		return nil
	},
}

var flagBackupQuiet bool

var backupCmd = &cobra.Command{
	Use:   "backup [path]",
	Short: "Create a backup of the database",
	Long: `Create a backup of the tpg database.

Backups are stored in ~/.tpg/backups/ by default with timestamped names.
The last 10 backups are kept; older ones are automatically pruned.

Optionally specify a custom path for the backup file.

Examples:
  tpg backup                    # Create backup in ~/.tpg/backups/
  tpg backup ~/my-backup.db     # Create backup at custom path
  tpg backup --quiet            # Silent backup (for hooks)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		var backupPath string
		if len(args) > 0 {
			// Custom path - use VACUUM INTO directly
			backupPath = args[0]
			_, err = database.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
			if err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}
		} else {
			backupPath, err = database.Backup()
			if err != nil {
				return err
			}
		}

		if !flagBackupQuiet {
			fmt.Printf("Backup created: %s\n", backupPath)
		}
		return nil
	},
}

var backupsCmd = &cobra.Command{
	Use:   "backups",
	Short: "List available backups",
	Long: `List all available database backups.

Shows backups in ~/.tpg/backups/, newest first.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		backups, err := db.ListBackups()
		if err != nil {
			return err
		}

		if len(backups) == 0 {
			fmt.Println("No backups found")
			return nil
		}

		fmt.Printf("%-30s  %10s  %s\n", "BACKUP", "SIZE", "CREATED")
		for _, b := range backups {
			size := formatSize(b.Size)
			age := formatTimeAgo(b.ModTime)
			fmt.Printf("%-30s  %10s  %s\n", b.Name, size, age)
		}
		return nil
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore <path>",
	Short: "Restore database from a backup",
	Long: `Restore the tpg database from a backup file.

This replaces the current database with the backup.
A backup of the current database is created first.

Examples:
  tpg restore ~/.tpg/backups/tpg-2024-01-09T12-00-00.db
  tpg restore ~/my-backup.db`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		backupPath := args[0]

		// First, create a backup of current state
		database, err := openDB()
		if err != nil {
			// If we can't open the DB, that's fine - just restore
			fmt.Println("Note: Could not backup current database (may not exist)")
		} else {
			preRestorePath, err := database.Backup()
			_ = database.Close()
			if err != nil {
				fmt.Printf("Warning: Could not backup current database: %v\n", err)
			} else {
				fmt.Printf("Current database backed up to: %s\n", preRestorePath)
			}
		}

		// Restore from backup
		if err := db.Restore(backupPath); err != nil {
			return err
		}

		fmt.Printf("Restored from: %s\n", backupPath)
		return nil
	},
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

var tuiCmd = &cobra.Command{
	Use:     "tui",
	Aliases: []string{"ui"},
	Short:   "Launch interactive terminal UI",
	Long: `Launch an interactive terminal UI for managing tasks.

Navigation:
  j/k or arrows    Move cursor up/down
  g/G or home/end  Jump to first/last item
  enter or l       View task details
  esc or h         Go back to list

Actions:
  s   Start task (open/blocked -> in_progress)
  d   Mark done (in_progress -> done)
  b   Block task (prompts for reason)
  L   Log progress (prompts for message)
  c   Cancel task (prompts for optional reason)
  n   Create new task (inherits project from selected item)
  D   Delete task
  a   Add dependency (prompts for blocker ID)
  r   Refresh task list

Filtering:
  /       Search by title/ID/description
  p       Filter by project
  1-5     Toggle status: 1=open 2=in_progress 3=blocked 4=done 5=canceled
  0       Show all statuses
  esc     Clear filters, or quit if none set

Press q to quit.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		return tui.Run(database)
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&flagProject, "project", "p", "", "Project scope")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show agent context and other debug info")

	// Show agent context when verbose
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if flagVerbose {
			agentID := os.Getenv("AGENT_ID")
			agentType := os.Getenv("AGENT_TYPE")
			if agentID != "" || agentType != "" {
				fmt.Fprintf(os.Stderr, "[agent] ID=%s TYPE=%s\n", agentID, agentType)
			}
		}
	}

	// add flags
	addCmd.Flags().BoolVarP(&flagEpic, "epic", "e", false, "Create an epic instead of a task")
	addCmd.Flags().IntVar(&flagPriority, "priority", 2, "Priority (1=high, 2=medium, 3=low)")
	addCmd.Flags().StringVar(&flagParent, "parent", "", "Parent epic ID")
	addCmd.Flags().StringVar(&flagBlocks, "blocks", "", "ID of task this will block (it depends on this)")
	addCmd.Flags().StringVar(&flagAfter, "after", "", "ID of task this depends on (must complete first)")
	addCmd.Flags().StringArrayVarP(&flagAddLabels, "label", "l", nil, "Label to attach (can be repeated)")
	addCmd.Flags().StringVar(&flagTemplateID, "template", "", "Template ID to instantiate")
	addCmd.Flags().StringArrayVar(&flagTemplateVars, "var", nil, "Template variable value (name=json-string)")
	addCmd.Flags().BoolVar(&flagTemplateVarsYAML, "vars-yaml", false, "Read template variables from stdin as YAML")
	addCmd.Flags().StringVar(&flagDescription, "desc", "", "Description (use '-' for stdin)")

	// init flags
	initCmd.Flags().StringVar(&flagInitTaskPrefix, "prefix", "", "Task ID prefix (default: ts)")
	initCmd.Flags().StringVar(&flagInitTaskPrefix, "task-prefix", "", "Task ID prefix (default: ts)")
	initCmd.Flags().StringVar(&flagInitEpicPrefix, "epic-prefix", "", "Epic ID prefix (default: ep)")

	// list flags
	listCmd.Flags().StringVar(&flagStatus, "status", "", "Filter by status (open, in_progress, blocked, done, canceled)")
	listCmd.Flags().StringVar(&flagListParent, "parent", "", "Filter by parent epic ID")
	listCmd.Flags().StringVar(&flagListType, "type", "", "Filter by item type (task, epic)")
	listCmd.Flags().StringVar(&flagBlocking, "blocking", "", "Show items that block the given ID")
	listCmd.Flags().StringVar(&flagBlockedBy, "blocked-by", "", "Show items blocked by the given ID")
	listCmd.Flags().BoolVar(&flagHasBlockers, "has-blockers", false, "Show only items with unresolved blockers")
	listCmd.Flags().BoolVar(&flagNoBlockers, "no-blockers", false, "Show only items with no blockers")
	listCmd.Flags().StringArrayVarP(&flagFilterLabels, "label", "l", nil, "Filter by label (can be repeated, AND logic)")

	// stale flags
	staleCmd.Flags().StringVar(&flagStaleThreshold, "threshold", "5m", "Threshold for stale in-progress tasks")

	// done flags
	doneCmd.Flags().BoolVar(&flagDoneOverride, "override", false, "Allow completion with unmet dependencies")

	// onboard flags
	onboardCmd.Flags().BoolVar(&flagForce, "force", false, "Replace existing Task Tracking section")

	// edit flags
	editCmd.Flags().StringVar(&flagEditTitle, "title", "", "New title for the task")

	// ready flags
	readyCmd.Flags().StringArrayVarP(&flagFilterLabels, "label", "l", nil, "Filter by label (can be repeated, AND logic)")

	// status flags
	statusCmd.Flags().BoolVar(&flagStatusAll, "all", false, "Show all ready tasks (default: limit to 10)")
	statusCmd.Flags().StringArrayVarP(&flagFilterLabels, "label", "l", nil, "Filter by label (can be repeated, AND logic)")

	// learn flags
	learnCmd.Flags().StringArrayVarP(&flagLearnConcept, "concept", "c", nil, "Concept to tag this learning with (can be repeated)")
	learnCmd.Flags().StringArrayVarP(&flagLearnFile, "file", "f", nil, "Related file (can be repeated)")
	learnCmd.Flags().StringVar(&flagLearnDetail, "detail", "", "Full context/explanation (use '-' for stdin)")

	// learn subcommands
	learnCmd.AddCommand(learnEditCmd)
	learnCmd.AddCommand(learnStaleCmd)
	learnCmd.AddCommand(learnRmCmd)

	// learn edit flags
	learnEditCmd.Flags().StringVar(&flagLearnEditSummary, "summary", "", "New summary for the learning")
	learnEditCmd.Flags().StringVar(&flagLearnEditDetail, "detail", "", "New detail for the learning (use '-' for stdin)")

	// learn stale flags
	learnStaleCmd.Flags().StringVar(&flagLearnStaleReason, "reason", "", "Reason for marking as stale")

	// concepts flags
	conceptsCmd.Flags().BoolVar(&flagConceptsRecent, "recent", false, "Sort by last updated instead of learning count")
	conceptsCmd.Flags().StringVar(&flagConceptsRelated, "related", "", "Suggest concepts related to a task")
	conceptsCmd.Flags().StringVar(&flagConceptsSummary, "summary", "", "Set concept summary (requires concept name as argument)")
	conceptsCmd.Flags().StringVar(&flagConceptsRename, "rename", "", "Rename concept (requires concept name as argument)")
	conceptsCmd.Flags().BoolVar(&flagConceptsStats, "stats", false, "Show statistics (count and oldest learning age)")

	// labels flags
	labelsAddCmd.Flags().StringVar(&flagLabelsColor, "color", "", "Label color (hex, e.g. #ff0000)")

	// labels subcommands
	labelsCmd.AddCommand(labelsAddCmd)
	labelsCmd.AddCommand(labelsRmCmd)
	labelsCmd.AddCommand(labelsRenameCmd)

	// context flags
	contextCmd.Flags().StringArrayVarP(&flagContextConcept, "concept", "c", nil, "Concept to retrieve learnings for (can be repeated)")
	contextCmd.Flags().StringVarP(&flagContextQuery, "query", "q", "", "Full-text search query")
	contextCmd.Flags().BoolVar(&flagContextStale, "include-stale", false, "Include stale learnings in results")
	contextCmd.Flags().BoolVar(&flagContextSummary, "summary", false, "Show one-liner per learning (no detail)")
	contextCmd.Flags().StringVar(&flagContextID, "id", "", "Load specific learning by ID")
	contextCmd.Flags().BoolVar(&flagContextJSON, "json", false, "Output as JSON for machine processing")

	// backup flags
	backupCmd.Flags().BoolVarP(&flagBackupQuiet, "quiet", "q", false, "Silent backup (no output)")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(readyCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(doneCmd)
	rootCmd.AddCommand(cancelCmd)
	rootCmd.AddCommand(blockCmd)
	rootCmd.AddCommand(staleCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(projectsCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(appendCmd)
	rootCmd.AddCommand(descCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(parentCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(blocksCmd)
	rootCmd.AddCommand(labelCmd)
	rootCmd.AddCommand(unlabelCmd)
	rootCmd.AddCommand(learnCmd)
	rootCmd.AddCommand(conceptsCmd)
	rootCmd.AddCommand(labelsCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(primeCmd)
	rootCmd.AddCommand(compactCmd)
	rootCmd.AddCommand(onboardCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(backupsCmd)
	rootCmd.AddCommand(restoreCmd)

	// Template subcommands and flags
	templateListCmd.Flags().BoolVar(&flagTemplateListDetail, "detail", false, "Show full variable details")
	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateShowCmd)
	templateCmd.AddCommand(templateUsageCmd)
	templateCmd.AddCommand(templateLocationsCmd)
	rootCmd.AddCommand(templateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Output formatting

func printItemsTable(items []model.Item) {
	if len(items) == 0 {
		fmt.Println("No items")
		return
	}

	fmt.Printf("%-12s %-12s %-4s %s\n", "ID", "STATUS", "PRI", "TITLE")
	for _, item := range items {
		title := item.Title
		if len(item.Labels) > 0 {
			title = formatLabels(item.Labels) + " " + title
		}
		fmt.Printf("%-12s %-12s %-4d %s\n", item.ID, item.Status, item.Priority, title)
	}
}

func printReadyTable(items []model.Item) {
	if len(items) == 0 {
		fmt.Println("No items")
		return
	}

	fmt.Printf("%-12s %-4s %s\n", "ID", "PRI", "TITLE")
	for _, item := range items {
		title := item.Title
		if len(item.Labels) > 0 {
			title = formatLabels(item.Labels) + " " + title
		}
		fmt.Printf("%-12s %-4d %s\n", item.ID, item.Priority, title)
	}
}

// formatLabels returns labels in [label1] [label2] format.
func formatLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	var parts []string
	for _, l := range labels {
		parts = append(parts, "["+l+"]")
	}
	return strings.Join(parts, " ")
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func isProgressMessage(message string) bool {
	trimmed := strings.TrimSpace(message)
	trimmed = strings.ToLower(trimmed)
	return strings.HasPrefix(trimmed, "progress:")
}

func latestProgressLog(logs []model.Log) *model.Log {
	for i := len(logs) - 1; i >= 0; i-- {
		if isProgressMessage(logs[i].Message) {
			return &logs[i]
		}
	}
	return nil
}

func filterBlockers(deps []db.DepStatus) []db.DepStatus {
	var blockers []db.DepStatus
	for _, dep := range deps {
		if dep.Status != string(model.StatusDone) {
			blockers = append(blockers, dep)
		}
	}
	return blockers
}

func indentLines(text, indent string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

func printItemDetail(item *model.Item, logs []model.Log, deps []string, blockers []db.DepStatus, latestProgress *model.Log, concepts []model.Concept, templateNotice string) {
	fmt.Printf("ID:          %s\n", item.ID)
	fmt.Printf("Type:        %s\n", item.Type)
	fmt.Printf("Project:     %s\n", item.Project)
	fmt.Printf("Title:       %s\n", item.Title)
	fmt.Printf("Status:      %s\n", item.Status)
	fmt.Printf("Priority:    %d\n", item.Priority)
	if item.ParentID != nil {
		fmt.Printf("Parent:      %s\n", *item.ParentID)
	}
	if len(item.Labels) > 0 {
		fmt.Printf("Labels:      %s\n", strings.Join(item.Labels, ", "))
	}
	if item.TemplateID != "" {
		fmt.Printf("Template:    %s\n", item.TemplateID)
		if item.StepIndex != nil {
			fmt.Printf("Step:        %d\n", *item.StepIndex+1)
		}
	}

	fmt.Printf("\nLatest Update:\n")
	if latestProgress != nil {
		fmt.Printf("  Progress: [%s] %s\n", latestProgress.CreatedAt.Format("2006-01-02 15:04"), latestProgress.Message)
	} else {
		fmt.Printf("  Progress: (none)\n")
	}
	if len(blockers) > 0 {
		fmt.Printf("  Blockers:\n")
		for _, dep := range blockers {
			fmt.Printf("    - %s [%s] %s\n", dep.ID, dep.Status, dep.Title)
		}
	} else {
		fmt.Printf("  Blockers: none\n")
	}
	if item.Results != "" {
		fmt.Printf("  Results:\n%s\n", indentLines(item.Results, "    "))
	}
	if templateNotice != "" {
		fmt.Printf("  Template: %s\n", templateNotice)
	}

	if item.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", item.Description)
	}

	if item.TemplateID != "" && item.StepIndex == nil && len(item.TemplateVars) > 0 {
		fmt.Printf("\nTemplate Context:\n")
		for key, value := range item.TemplateVars {
			fmt.Printf("  %s:\n%s\n", key, indentLines(value, "    "))
		}
	}

	if len(deps) > 0 {
		fmt.Printf("\nDependencies:\n")
		for _, dep := range deps {
			fmt.Printf("  - %s\n", dep)
		}
	}

	if len(logs) > 0 {
		logLimit := 50
		displayLogs := logs
		truncated := 0
		if len(logs) > logLimit {
			displayLogs = logs[len(logs)-logLimit:]
			truncated = len(logs) - logLimit
		}
		fmt.Printf("\nLogs:\n")
		for _, log := range displayLogs {
			fmt.Printf("  [%s] %s\n", log.CreatedAt.Format("2006-01-02 15:04"), log.Message)
		}
		if truncated > 0 {
			fmt.Printf("  ... (%d earlier logs truncated)\n", truncated)
		}
	}

	if len(concepts) > 0 {
		fmt.Printf("\nSuggested context:\n")
		var conceptFlags []string
		for _, c := range concepts {
			summary := c.Summary
			if summary == "" {
				summary = "(no summary)"
			}
			fmt.Printf("  %s (%d) - %s\n", c.Name, c.LearningCount, summary)
			conceptFlags = append(conceptFlags, "-c "+c.Name)
		}
		fmt.Printf("\nLoad with: tpg context %s -p %s --summary\n", strings.Join(conceptFlags, " "), item.Project)
	}
}

func printStatusReport(report *db.StatusReport, showAll bool) {
	project := report.Project
	if project == "" {
		project = "(all)"
	}
	fmt.Printf("Project: %s\n\n", project)

	fmt.Printf("Summary: %d open, %d in progress, %d blocked, %d done, %d canceled (%d ready)\n\n",
		report.Open, report.InProgress, report.Blocked, report.Done, report.Canceled, report.Ready)

	// Show project in output when viewing all projects
	showProject := report.Project == ""

	// Show stale items first (important warning)
	if len(report.StaleItems) > 0 {
		fmt.Printf("⚠️  Stale (%d task(s) with no updates >5min):\n", len(report.StaleItems))
		if len(report.StaleItems) <= 20 {
			for _, item := range report.StaleItems {
				fmt.Printf("  %s\n", formatStatusItem(item, showProject, false))
			}
		} else {
			// Too many to list - show IDs only
			ids := make([]string, len(report.StaleItems))
			for i, item := range report.StaleItems {
				ids[i] = item.ID
			}
			fmt.Printf("  IDs: %s\n", strings.Join(ids, ", "))
		}
		fmt.Println()
	}

	if len(report.RecentDone) > 0 {
		fmt.Println("Recently completed:")
		for _, item := range report.RecentDone {
			fmt.Printf("  %s\n", formatStatusItem(item, showProject, false))
		}
		fmt.Println()
	}

	// Show agent-aware in-progress sections if agent context is active
	if report.AgentID != "" {
		if len(report.MyInProgItems) > 0 {
			fmt.Println("My work in progress:")
			for _, item := range report.MyInProgItems {
				fmt.Printf("  %s\n", formatStatusItem(item, showProject, false))
			}
			fmt.Println()
		}
		if report.OtherInProgCount > 0 {
			fmt.Printf("Other agents: %d task(s) in progress\n\n", report.OtherInProgCount)
		}
	} else {
		// No agent context - show all in-progress items together
		if len(report.InProgItems) > 0 {
			fmt.Println("In progress:")
			for _, item := range report.InProgItems {
				fmt.Printf("  %s\n", formatStatusItem(item, showProject, false))
			}
			fmt.Println()
		}
	}

	if len(report.BlockedItems) > 0 {
		fmt.Println("Blocked:")
		for _, item := range report.BlockedItems {
			fmt.Printf("  %s\n", formatStatusItem(item, showProject, false))
		}
		fmt.Println()
	}

	if len(report.ReadyItems) > 0 {
		fmt.Println("Ready for work:")
		readyLimit := 10
		displayItems := report.ReadyItems
		remaining := 0
		if !showAll && len(report.ReadyItems) > readyLimit {
			displayItems = report.ReadyItems[:readyLimit]
			remaining = len(report.ReadyItems) - readyLimit
		}
		for _, item := range displayItems {
			fmt.Printf("  %s\n", formatStatusItem(item, showProject, true))
		}
		if remaining > 0 {
			fmt.Printf("  (+%d more, use --all to see all)\n", remaining)
		}
	}
}

func formatStatusItem(item model.Item, showProject, showPriority bool) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("[%s]", item.ID))
	if showProject && item.Project != "" {
		parts = append(parts, fmt.Sprintf("(%s)", item.Project))
	}
	if len(item.Labels) > 0 {
		parts = append(parts, formatLabels(item.Labels))
	}
	parts = append(parts, item.Title)
	if showPriority {
		parts = append(parts, fmt.Sprintf("(pri %d)", item.Priority))
	}
	return strings.Join(parts, " ")
}

func printConceptsTable(concepts []model.Concept) {
	fmt.Printf("%-20s %10s  %-12s  %s\n", "NAME", "LEARNINGS", "LAST UPDATED", "SUMMARY")
	for _, c := range concepts {
		ago := formatTimeAgo(c.LastUpdated)
		summary := c.Summary
		if len(summary) > 40 {
			summary = summary[:37] + "..."
		}
		fmt.Printf("%-20s %10d  %-12s  %s\n", c.Name, c.LearningCount, ago, summary)
	}
}

func printConceptsStats(stats []db.ConceptStats) {
	fmt.Printf("%-20s %6s  %s\n", "CONCEPT", "COUNT", "OLDEST")
	for _, s := range stats {
		oldest := "-"
		if s.OldestAge != nil {
			oldest = formatDurationShort(*s.OldestAge)
		}
		fmt.Printf("%-20s %6d  %s\n", s.Name, s.LearningCount, oldest)
	}
}

func formatDurationShort(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	hours := int(d.Hours())
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return "<1h"
}

func printLabelsTable(labels []model.Label) {
	fmt.Printf("%-20s  %-12s  %s\n", "NAME", "CREATED", "COLOR")
	for _, l := range labels {
		ago := formatTimeAgo(l.CreatedAt)
		color := l.Color
		if color == "" {
			color = "-"
		}
		fmt.Printf("%-20s  %-12s  %s\n", l.Name, ago, color)
	}
}

func printLearnings(learnings []model.Learning) {
	for i, l := range learnings {
		if i > 0 {
			fmt.Println()
		}

		// Header with ID, status, and age
		status := ""
		if l.Status == model.LearningStatusStale {
			status = " [stale]"
		}
		fmt.Printf("## %s%s (%s)\n", l.ID, status, formatTimeAgo(l.CreatedAt))

		// Summary
		fmt.Println(l.Summary)

		// Detail if present
		if l.Detail != "" {
			fmt.Printf("\n%s\n", l.Detail)
		}

		// Metadata
		if len(l.Concepts) > 0 {
			fmt.Printf("\nConcepts: %s\n", strings.Join(l.Concepts, ", "))
		}
		if len(l.Files) > 0 {
			fmt.Printf("Files: %s\n", strings.Join(l.Files, ", "))
		}
		if l.TaskID != nil {
			fmt.Printf("Task: %s\n", *l.TaskID)
		}
	}
}

// LearningJSON is the JSON serialization format for learnings.
type LearningJSON struct {
	ID        string   `json:"id"`
	Summary   string   `json:"summary"`
	Detail    string   `json:"detail,omitempty"`
	Concepts  []string `json:"concepts"`
	Files     []string `json:"files,omitempty"`
	CreatedAt string   `json:"created_at"`
	Status    string   `json:"status"`
}

func printLearningsJSON(learnings []model.Learning) error {
	output := make([]LearningJSON, 0, len(learnings))
	for _, l := range learnings {
		lj := LearningJSON{
			ID:        l.ID,
			Summary:   l.Summary,
			Detail:    l.Detail,
			Concepts:  l.Concepts,
			Files:     l.Files,
			CreatedAt: l.CreatedAt.Format(time.RFC3339),
			Status:    string(l.Status),
		}
		if lj.Concepts == nil {
			lj.Concepts = []string{}
		}
		output = append(output, lj)
	}
	b, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(b))
	return nil
}

func printLearningSummaries(learnings []model.Learning, requestedConcepts []string, conceptSummaries map[string]string) {
	// Print concept headers with summaries
	for _, conceptName := range requestedConcepts {
		summary := conceptSummaries[conceptName]
		if summary == "" {
			summary = "(no summary)"
		}
		fmt.Printf("%s: %s\n", conceptName, summary)
	}
	if len(requestedConcepts) > 0 {
		fmt.Println()
	}

	// Print one-liner per learning
	for _, l := range learnings {
		status := ""
		if l.Status == model.LearningStatusStale {
			status = " [stale]"
		}
		fmt.Printf("  %s: %s%s\n", l.ID, l.Summary, status)
	}
}

func printAllLearningSummaries(learnings []model.Learning, conceptSummaries map[string]string) {
	// Group learnings by concept
	type conceptGroup struct {
		summary   string
		learnings []model.Learning
	}
	groups := make(map[string]*conceptGroup)
	conceptOrder := []string{}

	for _, l := range learnings {
		for _, conceptName := range l.Concepts {
			if groups[conceptName] == nil {
				groups[conceptName] = &conceptGroup{
					summary: conceptSummaries[conceptName],
				}
				conceptOrder = append(conceptOrder, conceptName)
			}
			groups[conceptName].learnings = append(groups[conceptName].learnings, l)
		}
	}

	// Print grouped by concept
	for i, conceptName := range conceptOrder {
		if i > 0 {
			fmt.Println()
		}
		group := groups[conceptName]
		summary := group.summary
		if summary == "" {
			summary = "(no summary)"
		}
		fmt.Printf("%s: %s\n", conceptName, summary)

		for _, l := range group.learnings {
			status := ""
			if l.Status == model.LearningStatusStale {
				status = " [stale]"
			}
			fmt.Printf("  %s: %s%s\n", l.ID, l.Summary, status)
		}
	}
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

func printDepGraph(edges []db.DepEdge) {
	// Group by item
	type depInfo struct {
		title  string
		status string
		deps   []db.DepEdge
	}
	items := make(map[string]*depInfo)
	order := []string{}

	for _, e := range edges {
		if _, ok := items[e.ItemID]; !ok {
			items[e.ItemID] = &depInfo{title: e.ItemTitle, status: e.ItemStatus}
			order = append(order, e.ItemID)
		}
		items[e.ItemID].deps = append(items[e.ItemID].deps, e)
	}

	for _, id := range order {
		info := items[id]
		fmt.Printf("%s [%s] %s\n", id, info.status, info.title)
		for i, dep := range info.deps {
			prefix := "├──"
			if i == len(info.deps)-1 {
				prefix = "└──"
			}
			fmt.Printf("  %s %s [%s] %s\n", prefix, dep.DependsOnID, dep.DependsOnStatus, dep.DependsOnTitle)
		}
	}
}

func printPrimeContent(report *db.StatusReport) {
	fmt.Println(`# Tpg CLI Context

This project uses 'tpg' for cross-session task management.
Run 'tpg status' to see current state.

## Starting Work

When picking up a task:
1. tpg show <task>                 # See task + suggested concepts
2. tpg context -c X -c Y           # Load relevant concepts
   tpg context -c X --summary      # Or scan first if many learnings

Load context that's relevant to your task. Don't skip it, don't load everything.

## SESSION CLOSE PROTOCOL

Before ending ANY session, you MUST complete ALL of these steps:

1. Log progress on active tasks:
   tpg log <id> "What you accomplished"

2. Verify artifacts are updated:
   - If you changed behavior: is help text / CLI output updated?
   - If you added features: is documentation current?
   - If you fixed bugs: do error messages reflect the fix?
   - Do new tests need to be written? Do existing tests need updating?
   Run the relevant commands to confirm outputs match the code.

3. Update task status:
   - tpg done <id>     # if complete (will prompt for reflection)
   - tpg block <id> "reason"  # if blocked

4. Add handoff context for next agent:
   tpg append <id> "Next steps: ..."

5. Update parent epic (if task is part of one):
   tpg append <epic-id> "Completed X, next: Y"

## Logging Learnings

When tpg done prompts for reflection, ask: What would help the next agent?
  - What pattern or technique proved effective?
  - What gotcha would trap someone unfamiliar?
  - What's not obvious from reading the code?

Validate insights with the user before logging.

To log (ALWAYS include both summary AND detail):
  tpg concepts                              # Check existing concepts first
  tpg learn "summary" -c concept --detail "full explanation"

Why both? Two-phase retrieval:
  - Summary: one-liner for scanning/discovery
  - Detail: full context when selected
  Without detail, future agents get only the one-liner.

Good learnings are specific and actionable:
  ✓ tpg learn "Schema migrations need built binary" -c database \
      --detail "go run doesn't embed assets; must use go build first"

Not learnings (use tpg log instead):
  ✗ "Fixed the auth bug"
  ✗ "This file handles authentication"

NEVER end a session without updating task state.
Work is NOT complete until tpg reflects reality.

## Core Rules

- Use 'tpg' for strategic work tracking (persists across sessions)
- Use TodoWrite for tactical within-session checklists
- Always claim work before starting: tpg start <id>
- Log progress frequently, not just at the end

## Essential Commands

# Finding work
tpg status              # Overview
tpg ready               # Tasks ready to work on
tpg show <id>           # Full details + suggested concepts

# Working
tpg start <id>          # Claim a task
tpg log <id> "message"  # Log progress
tpg done <id>           # Mark complete
tpg block <id> "why"    # Mark blocked

# Creating
tpg add "title" -p project    # New task
tpg add "title" -l bug        # With label
tpg add "title" -e            # New epic

# Editing
tpg append <id> "text"        # Add to description
tpg label <id> <name>         # Add label to task

# Context retrieval
tpg context -c concept        # Load learnings for a concept
tpg context -c X --summary    # Scan one-liners first
tpg concepts                  # List available concepts
tpg learn "summary" -c X --detail "explanation"  # Log with both parts

# Filtering
tpg list -p myproject         # Filter by project
tpg list --status open        # Filter by status
tpg list -l bug -l urgent     # Filter by labels (AND)
tpg ready -p myproject        # Ready in project`)

	fmt.Println("\n## Current State")

	if report != nil {
		if len(report.InProgItems) > 0 {
			fmt.Println("\nIn progress:")
			for _, item := range report.InProgItems {
				fmt.Printf("  [%s] %s\n", item.ID, item.Title)
			}
		}

		if len(report.BlockedItems) > 0 {
			fmt.Println("\nBlocked:")
			for _, item := range report.BlockedItems {
				fmt.Printf("  [%s] %s\n", item.ID, item.Title)
			}
		}

		fmt.Println("\nRun 'tpg ready [-p project]' to find unblocked work.")
	} else {
		fmt.Println("\n(No database connection - run 'tpg init' if needed)")
	}
}

// renderPrime renders the prime output using template system with fallback
func renderPrime(report *db.StatusReport, config *db.Config, agentCtx db.AgentContext, database *db.DB) {
	// Try to load custom template
	templateText, source, err := prime.LoadPrimeTemplate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading prime template: %v\n", err)
		printPrimeContent(report) // Fallback to old implementation
		return
	}

	// Use default if no custom template found
	if templateText == "" {
		templateText = prime.DefaultPrimeTemplate()
		source = "(default)"
	}

	// Build template data
	data := prime.BuildPrimeData(report, config, agentCtx, database)

	// Render
	output, err := prime.RenderPrime(templateText, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering prime template from %s: %v\n", source, err)
		fmt.Fprintf(os.Stderr, "Falling back to default output.\n\n")
		printPrimeContent(report)
		return
	}

	fmt.Print(output)
}

// handlePrimeCustomize creates or edits the custom prime template
func handlePrimeCustomize() error {
	// Search upward for existing .tpg directory
	var tpgDir string
	startDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not determine current directory: %w", err)
	}

	dir := startDir
	for {
		candidate := filepath.Join(dir, ".tpg")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			tpgDir = candidate
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, create in current directory
			tpgDir = filepath.Join(startDir, ".tpg")
			if err := os.MkdirAll(tpgDir, 0755); err != nil {
				return fmt.Errorf("could not create .tpg directory: %w", err)
			}
			break
		}
		dir = parent
	}

	primePath := filepath.Join(tpgDir, prime.PrimeFileName)

	// Create template with default content if it doesn't exist
	if _, err := os.Stat(primePath); os.IsNotExist(err) {
		defaultContent := prime.DefaultPrimeTemplate()
		if err := os.WriteFile(primePath, []byte(defaultContent), 0644); err != nil {
			return fmt.Errorf("could not create template: %w", err)
		}
		fmt.Printf("Created template at: %s\n", primePath)
	}

	// Open in editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	fmt.Printf("Opening %s in %s...\n", primePath, editor)
	cmd := exec.Command(editor, primePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	// Validate template syntax
	content, err := os.ReadFile(primePath)
	if err != nil {
		return fmt.Errorf("could not read template: %w", err)
	}

	// Try to render with empty data to check syntax
	emptyData := prime.PrimeData{}
	if _, err := prime.RenderPrime(string(content), emptyData); err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: Template has syntax errors: %v\n", err)
		fmt.Fprintf(os.Stderr, "Fix errors and run 'tpg prime' to test.\n")
		return nil
	}

	fmt.Println("\nTemplate saved and validated successfully!")
	fmt.Println("Run 'tpg prime' to test the output.")
	return nil
}

// handlePrimeRender renders a specific template file (for testing)
func handlePrimeRender(templatePath string, database *db.DB, config *db.Config, agentCtx db.AgentContext) error {
	// Read the template file
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("could not read template file: %w", err)
	}

	// Get project status if database available
	var report *db.StatusReport
	if database != nil {
		project, _ := resolveProject()
		report, _ = database.ProjectStatusFiltered(project, nil, agentCtx.ID)
	}

	// Build template data
	data := prime.BuildPrimeData(report, config, agentCtx, database)

	// Render
	output, err := prime.RenderPrime(string(content), data)
	if err != nil {
		return fmt.Errorf("template rendering failed: %w", err)
	}

	fmt.Print(output)
	return nil
}

func printCompactContent(stats []db.ConceptStats) {
	fmt.Println(`# Compact Learnings

Groom learnings and concepts using two phases: **discovery** then **selection**.

## Why Compact?

Over time, learnings accumulate. Without grooming:
- Redundant entries waste context tokens
- Stale learnings mislead agents
- Unclear summaries reduce retrieval value
- Fragmented insights are harder to find

Compaction keeps the knowledge base high-signal and navigable.

## Phase 1: Discovery

Scan all learning summaries grouped by concept:

` + "```" + `bash
tpg context -p <project> --summary   # All learnings, grouped by concept
` + "```" + `

Flag candidates:
- **Redundant**: Similar summaries (potential duplicates)
- **Stale**: References old code/patterns, or old learnings (7+ days) that may be outdated
- **Low quality**: Vague summaries, missing detail, not actionable
- **Fragmented**: Related small learnings that should be one

Present candidates to user. Confirm which to address.

## Phase 2: Selection & Grooming

Load full detail only for flagged candidates:

` + "```" + `bash
tpg context --id lrn-abc123          # Specific learning
` + "```" + `

For each candidate, determine action:
- **Archive**: Redundant or superseded → ` + "`tpg learn stale <id> --reason \"...\"`" + `
- **Update**: Valid but unclear → ` + "`tpg learn edit <id> --summary \"...\"`" + `
- **Consolidate**: Merge related → archive originals, create new combined learning
- **Keep**: No changes needed

Present changes to user. Execute after approval.

## Repeat

After each batch:
- Re-run discovery if significant changes
- Continue until no candidates remain or user is satisfied

## Quality Guidelines

**Good summaries**: Specific, actionable, self-contained
**Archive when**: Superseded, redundant, or references removed code
**Consolidate when**: Multiple learnings express the same insight
**Update when**: Insight valid but poorly expressed`)

	// Show current stats if available
	if len(stats) > 0 {
		fmt.Println("\n## Current Stats")
		fmt.Printf("\n%-20s %6s  %s\n", "CONCEPT", "COUNT", "OLDEST")
		for _, s := range stats {
			oldest := "-"
			if s.OldestAge != nil {
				oldest = formatDurationShort(*s.OldestAge)
			}
			fmt.Printf("%-20s %6d  %s\n", s.Name, s.LearningCount, oldest)
		}

		// Highlight compaction candidates
		var candidates []string
		for _, s := range stats {
			if s.LearningCount >= 5 {
				candidates = append(candidates, s.Name)
			}
		}
		if len(candidates) > 0 {
			fmt.Printf("\nCompaction candidates (5+ learnings): %s\n", strings.Join(candidates, ", "))
		}
	}
}
