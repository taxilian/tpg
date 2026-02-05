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
	"github.com/spf13/pflag"
	"github.com/taxilian/tpg/internal/db"
	"github.com/taxilian/tpg/internal/format"
	"github.com/taxilian/tpg/internal/model"
	"github.com/taxilian/tpg/internal/plugin"
	"github.com/taxilian/tpg/internal/prime"
	"github.com/taxilian/tpg/internal/templates"
	"github.com/taxilian/tpg/internal/tui"
	"github.com/taxilian/tpg/internal/worktree"
	"gopkg.in/yaml.v3"
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
	flagDeleteForce      bool
	flagCancelForce      bool
	flagParent           string
	flagBlocks           string
	flagAfter            string
	flagTemplateID       string
	flagTemplateVars     []string
	flagListParent       string
	flagListType         string
	flagListEpic         string
	flagBlocking         string
	flagBlockedBy        string
	flagHasBlockers      bool
	flagNoBlockers       bool
	flagEditTitle        string
	flagContext          string
	flagOnClose          string
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
	flagMergeConfirm     bool
	flagType             string
	flagPrefix           string

	flagShowWithChildren bool
	flagShowWithDeps     bool
	flagShowWithParent   bool
	flagShowFormat       string
	flagShowVars         bool
	flagDryRun           bool
	flagReadyEpic        string
	flagListAll          bool
	flagIdsOnly          bool
	flagListFlat         bool

	// Edit command flags
	flagEditPriority  int
	flagEditParent    string
	flagEditAddLabels []string
	flagEditRmLabels  []string
	flagEditDesc      string
	flagEditStatus    string
	flagEditParentSet bool // tracks if --parent was explicitly set (to allow empty string)

	flagWorktree       bool
	flagWorktreeBranch string
	flagWorktreeBase   string
	flagWorktreeAllow  bool

	flagDoctorDryRun bool
	flagResume       bool
	flagFromYAML     bool
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

Multi-field input: Use --from-yaml to set multiple flags from stdin YAML.
  tpg epic add "Title" --from-yaml <<EOF
  context: |
    Shared context for tasks
  on_close: |
    Instructions when done
  EOF

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

var epicCmd = &cobra.Command{
	Use:   "epic",
	Short: "Manage epics and their worktrees",
	Long:  `Commands for managing epics, including worktree setup and completion.`,
}

var epicWorktreeCmd = &cobra.Command{
	Use:   "worktree <id>",
	Short: "Set up worktree metadata for an existing epic",
	Long: `Update an existing epic with worktree metadata.

If --branch is not specified, a branch name will be auto-generated.
If --base is not specified, "main" will be used.

Note: This command only stores worktree metadata in tpg. It does NOT create
the actual git worktree. Run 'git worktree add' separately.

Examples:
  tpg epic worktree ep-abc123
  tpg epic worktree ep-abc123 --branch feature/my-custom-branch
  tpg epic worktree ep-abc123 --base develop`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		epicID := args[0]

		// Verify the item exists and is an epic
		item, err := database.GetItem(epicID)
		if err != nil {
			return err
		}
		if item.Type != model.ItemTypeEpic {
			return fmt.Errorf("%s is not an epic", epicID)
		}

		config, _ := db.LoadConfig()

		// Generate branch name if not provided
		branch := flagWorktreeBranch
		if branch == "" {
			branch = generateWorktreeBranch(item.ID, item.Title, worktreePrefix(config))
		}
		if !flagWorktreeAllow && worktreeRequireEpicID(config) && !branchIncludesEpicID(branch, item.ID) {
			suggested := generateWorktreeBranch(item.ID, item.Title, worktreePrefix(config))
			return fmt.Errorf("branch name must include epic id %q (suggested: %s)", item.ID, suggested)
		}

		// Determine base branch
		base := flagWorktreeBase
		if base == "" {
			parentID := ""
			if item.ParentID != nil {
				parentID = *item.ParentID
			}
			base = resolveWorktreeBase(database, parentID)
		}

		// Update the epic with worktree metadata
		if err := database.SetWorktreeMetadata(item.ID, branch, base); err != nil {
			return fmt.Errorf("failed to set worktree metadata: %w", err)
		}

		fmt.Printf("Updated epic %s (worktree expected)\n", item.ID)
		fmt.Printf("  Branch: %s (from %s)\n", branch, base)

		ctx, worktrees := detectWorktreeState()
		repoRoot := ""
		if ctx != nil {
			repoRoot = ctx.RepoRoot
		}
		location := worktreeLocationForEpic(config, repoRoot, item.ID)

		if worktrees != nil {
			if path, ok := worktrees[branch]; ok {
				location = displayWorktreePath(repoRoot, path)
				fmt.Printf("\nWorktree detected for branch %s:\n", branch)
				fmt.Printf("  Location: %s\n", location)
				database.BackupQuiet()
				return nil
			}
		}

		fmt.Printf("\nWorktree not found. Create it with:\n")
		fmt.Printf("  git worktree add -b %s %s %s\n", branch, location, base)
		fmt.Printf("  cd %s\n", location)

		database.BackupQuiet()
		return nil
	},
}

var epicAddCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Create a new epic",
	Long: `Create a new epic for organizing related tasks.

Epics are containers that group related tasks. They auto-complete when all
children are done.

Use --context to provide context shared with all descendant tasks.
Use --on-close for instructions shown when the epic completes.

Use '-' to read a field from stdin:
  tpg epic add "Title" --context - <<EOF
  context here
  EOF

For multiple fields from stdin, use --from-yaml instead of individual flags:
  tpg epic add "Title" --from-yaml <<EOF
  context: |
    Shared context for all tasks
  on_close: |
    Instructions when done
  desc: |
    Optional description
  EOF

Examples:
  # Simple epic
  tpg epic add "Auth system redesign"

  # Epic with context shared to all descendant tasks
  tpg epic add "Payment integration" --context - <<EOF
  Use Stripe API v3. See docs/stripe-guide.md for patterns.
  All payment handlers must include idempotency keys.
  EOF

  # Epic with worktree for isolated development
  tpg epic add "New feature" --worktree

  # With parent epic
  tpg epic add "Sub-feature" --parent ep-abc123 --worktree`,
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

		itemType := model.ItemTypeEpic

		// Generate ID with custom prefix if provided
		var itemID string
		if flagPrefix != "" {
			itemID = model.GenerateIDWithPrefixN(flagPrefix, itemType, model.DefaultIDLength)
		} else {
			itemID, err = database.GenerateItemID(itemType)
			if err != nil {
				return err
			}
		}

		// Initialize from flags
		description := flagDescription
		context := flagContext
		onClose := flagOnClose

		// Handle single-field stdin markers
		stdinUsed := false

		if description == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			description = strings.TrimSpace(string(data))
			stdinUsed = true
		}

		if context == "-" {
			if stdinUsed {
				return fmt.Errorf("cannot use stdin for multiple flags; use --from-yaml instead")
			}
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			context = strings.TrimSpace(string(data))
			stdinUsed = true
		}

		if onClose == "-" {
			if stdinUsed {
				return fmt.Errorf("cannot use stdin for multiple flags; use --from-yaml instead")
			}
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			onClose = strings.TrimSpace(string(data))
		}

		item := &model.Item{
			ID:                  itemID,
			Project:             project,
			Type:                itemType,
			Title:               strings.Join(args, " "),
			Description:         description,
			Status:              model.StatusOpen,
			Priority:            flagPriority,
			SharedContext:       context,
			ClosingInstructions: onClose,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
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

		// Add labels if specified
		for _, labelName := range flagAddLabels {
			if err := database.AddLabelToItem(item.ID, item.Project, labelName); err != nil {
				return err
			}
		}

		// Handle worktree metadata
		if flagWorktree || flagWorktreeBranch != "" {
			config, _ := db.LoadConfig()

			// Generate branch name if not provided
			branch := flagWorktreeBranch
			if branch == "" {
				branch = generateWorktreeBranch(item.ID, item.Title, worktreePrefix(config))
			}
			if !flagWorktreeAllow && worktreeRequireEpicID(config) && !branchIncludesEpicID(branch, item.ID) {
				suggested := generateWorktreeBranch(item.ID, item.Title, worktreePrefix(config))
				return fmt.Errorf("branch name must include epic id %q (suggested: %s)", item.ID, suggested)
			}

			// Determine base branch
			base := flagWorktreeBase
			if base == "" {
				base = resolveWorktreeBase(database, flagParent)
			}

			// Update the epic with worktree metadata
			if err := database.SetWorktreeMetadata(item.ID, branch, base); err != nil {
				return fmt.Errorf("failed to set worktree metadata: %w", err)
			}

			fmt.Println(item.ID)
			ctx, worktrees := detectWorktreeState()
			repoRoot := ""
			if ctx != nil {
				repoRoot = ctx.RepoRoot
			}
			location := worktreeLocationForEpic(config, repoRoot, item.ID)

			foundWorktree := false
			if worktrees != nil {
				if path, ok := worktrees[branch]; ok {
					location = displayWorktreePath(repoRoot, path)
					fmt.Fprintf(os.Stderr, "\nWorktree detected for branch %s:\n", branch)
					fmt.Fprintf(os.Stderr, "  Location: %s\n", location)
					foundWorktree = true
				}
			}
			if !foundWorktree {
				fmt.Fprintf(os.Stderr, "\nWorktree not found. Create it with:\n")
				fmt.Fprintf(os.Stderr, "  git worktree add -b %s %s %s\n", branch, location, base)
				fmt.Fprintf(os.Stderr, "  cd %s\n", location)
			}
		} else {
			fmt.Println(item.ID)
		}

		database.BackupQuiet()
		return nil
	},
}

var epicEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit an epic's settings",
	Long: `Edit an epic's title, context, or on-close instructions.

Use '-' with --context or --on-close to read a single field from stdin.
For multiple fields, use --from-yaml instead of individual flags.

Examples:
  tpg epic edit ep-abc123 --title "New title"

  # Update context shared with all descendants
  tpg epic edit ep-abc123 --context - <<EOF
  Updated guidelines for this epic.
  All tasks should follow the new API patterns.
  EOF

  # Update multiple fields at once via YAML
  tpg epic edit ep-abc123 --from-yaml <<EOF
  title: Updated epic title
  context: |
    New shared context for all tasks.
    Use the updated API patterns.
  on_close: |
    Run the full test suite before closing.
  EOF

  # Clear context
  tpg epic edit ep-abc123 --context ""`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]

		// Verify it's an epic
		item, err := database.GetItem(id)
		if err != nil {
			return err
		}
		if item.Type != model.ItemTypeEpic {
			return fmt.Errorf("%s is not an epic (use 'tpg edit' for tasks)", id)
		}

		updated := false

		if flagEditTitle != "" {
			if err := database.SetTitle(id, flagEditTitle); err != nil {
				return err
			}
			fmt.Printf("Updated title for %s\n", id)
			updated = true
		}

		// Handle single-field stdin markers
		stdinUsed := false

		if cmd.Flags().Changed("context") {
			context := flagContext
			if context == "-" {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read from stdin: %w", err)
				}
				context = strings.TrimSpace(string(data))
				stdinUsed = true
			}
			if err := database.SetSharedContext(id, context); err != nil {
				return err
			}
			fmt.Printf("Updated shared context for %s\n", id)
			updated = true
		}

		if cmd.Flags().Changed("on-close") {
			instructions := flagOnClose
			if instructions == "-" {
				if stdinUsed {
					return fmt.Errorf("cannot use stdin for multiple flags; use --from-yaml instead")
				}
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read from stdin: %w", err)
				}
				instructions = strings.TrimSpace(string(data))
			}
			if err := database.SetClosingInstructions(id, instructions); err != nil {
				return err
			}
			fmt.Printf("Updated closing instructions for %s\n", id)
			updated = true
		}

		if !updated {
			return fmt.Errorf("no changes specified (use --title, --context, or --on-close)")
		}

		database.BackupQuiet()
		return nil
	},
}

var epicListCmd = &cobra.Command{
	Use:   "list [epic-id]",
	Short: "List epics or descendants of an epic",
	Long: `List all epics, or show descendants of a specific epic.

Without arguments, lists all epics (equivalent to 'tpg list --type epic').
With an epic ID, shows all descendants (equivalent to 'tpg list --epic <id>').

Examples:
  tpg epic list              # All epics
  tpg epic list ep-abc123    # Descendants of ep-abc123`,
	Args: cobra.MaximumNArgs(1),
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

		var items []model.Item

		if len(args) == 1 {
			// Show descendants of specific epic
			descendants, err := database.GetDescendants(args[0])
			if err != nil {
				return fmt.Errorf("failed to get descendants: %w", err)
			}
			// Filter out done/canceled by default
			for _, d := range descendants {
				if d.Status != model.StatusDone && d.Status != model.StatusCanceled {
					items = append(items, d)
				}
			}
		} else {
			// Show all epics
			filter := db.ListFilter{
				Project: project,
				Type:    string(model.ItemTypeEpic),
			}
			var err error
			items, err = database.ListItemsFiltered(filter)
			if err != nil {
				return err
			}
			// Filter out done/canceled by default
			filtered := make([]model.Item, 0, len(items))
			for _, item := range items {
				if item.Status != model.StatusDone && item.Status != model.StatusCanceled {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}

		if len(items) == 0 {
			if len(args) == 1 {
				fmt.Println("No active descendants found for this epic")
			} else {
				fmt.Println("No active epics found")
			}
			return nil
		}

		// Populate labels for display
		if err := database.PopulateItemLabels(items); err != nil {
			return err
		}

		printItemsTree(items)
		return nil
	},
}

var epicReplaceCmd = &cobra.Command{
	Use:   "replace <id> <title>",
	Short: "Replace an existing item with an epic",
	Long: `Replace an existing task or epic with a new epic, preserving relationships.

The new epic inherits the old item's:
  - Parent
  - Children
  - Dependencies (both blocking and blocked-by)
  - Logs

The new epic does NOT inherit:
  - Labels (must be re-added if needed)
  - Status (new epic starts as open)

Use '-' with a flag to read a single field from stdin.
For multiple fields, use --from-yaml instead of individual flags.

Examples:
  # Replace a task with an epic
  tpg epic replace ts-abc123 "Now an epic"

  # Replace with context (use heredoc for detailed context)
  tpg epic replace ts-abc123 "Feature epic" --context - <<EOF
  This epic replaces a single task with a multi-step workflow.
  See design doc at docs/feature-design.md
  EOF

  # Replace with multiple fields via YAML
  tpg epic replace ts-abc123 "Feature epic" --from-yaml <<EOF
  context: |
    Context shared with all descendant tasks.
  on_close: |
    Remember to update the changelog.
  priority: 1
  EOF`,
	Args: cobra.MinimumNArgs(2),
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

		oldID := args[0]
		title := strings.Join(args[1:], " ")

		itemType := model.ItemTypeEpic

		// Generate ID with custom prefix if provided
		var newItemID string
		if flagPrefix != "" {
			newItemID = model.GenerateIDWithPrefixN(flagPrefix, itemType, model.DefaultIDLength)
		} else {
			newItemID, err = database.GenerateItemID(itemType)
			if err != nil {
				return err
			}
		}

		// Initialize from flags
		description := flagDescription
		context := flagContext
		onClose := flagOnClose

		// Handle single-field stdin markers
		stdinUsed := false

		if description == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			description = strings.TrimSpace(string(data))
			stdinUsed = true
		}

		if context == "-" {
			if stdinUsed {
				return fmt.Errorf("cannot use stdin for multiple flags; use --from-yaml instead")
			}
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			context = strings.TrimSpace(string(data))
			stdinUsed = true
		}

		if onClose == "-" {
			if stdinUsed {
				return fmt.Errorf("cannot use stdin for multiple flags; use --from-yaml instead")
			}
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			onClose = strings.TrimSpace(string(data))
		}

		newItem := &model.Item{
			ID:                  newItemID,
			Project:             project,
			Type:                itemType,
			Title:               title,
			Description:         description,
			Status:              model.StatusOpen,
			Priority:            flagPriority,
			SharedContext:       context,
			ClosingInstructions: onClose,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}

		// Perform the replacement
		resultID, err := database.ReplaceItem(oldID, newItem)
		if err != nil {
			return err
		}

		// Add labels if specified
		for _, labelName := range flagAddLabels {
			if err := database.AddLabelToItem(resultID, project, labelName); err != nil {
				return err
			}
		}

		fmt.Println(resultID)
		database.BackupQuiet()
		return nil
	},
}

var epicFinishCmd = &cobra.Command{
	Use:   "finish <id>",
	Short: "Show cleanup steps for an epic (does not complete it)",
	Long: `Show the closing instructions (if any) and worktree cleanup commands for an epic.

Epics auto-complete when all children are done or canceled - this command does NOT
complete the epic. Use it to see cleanup steps (merge PR, delete worktree, etc.)
that should be done before or after the epic auto-completes.

Examples:
  tpg epic finish ep-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		epicID := args[0]

		// Verify the item exists and is an epic
		item, err := database.GetItem(epicID)
		if err != nil {
			return err
		}
		if item.Type != model.ItemTypeEpic {
			return fmt.Errorf("%s is not an epic", epicID)
		}

		// Show progress stats
		total, open, inProgress, done, err := database.GetChildrenStats(epicID)
		if err != nil {
			return fmt.Errorf("failed to get children stats: %w", err)
		}

		fmt.Printf("Epic: %s\n", item.Title)
		fmt.Printf("Status: %s\n", item.Status)
		if total > 0 {
			fmt.Printf("Progress: %d/%d done", done, total)
			if inProgress > 0 {
				fmt.Printf(", %d in progress", inProgress)
			}
			if open > 0 {
				fmt.Printf(", %d open", open)
			}
			fmt.Println()
		}

		// If not all children are done, show what remains
		if open > 0 || inProgress > 0 {
			fmt.Printf("\nRemaining work:\n")
			descendants, err := database.GetDescendants(epicID)
			if err != nil {
				return fmt.Errorf("failed to get descendants: %w", err)
			}
			for _, d := range descendants {
				if d.Status != model.StatusDone && d.Status != model.StatusCanceled {
					fmt.Printf("  [%s] %s: %s\n", d.Status, d.ID, d.Title)
				}
			}
		}

		// Show closing instructions if any
		if item.ClosingInstructions != "" {
			fmt.Printf("\nClosing instructions:\n%s\n", item.ClosingInstructions)
		}

		// Print worktree cleanup instructions if applicable
		if item.WorktreeBranch != "" {
			config, _ := db.LoadConfig()
			ctx, worktrees := detectWorktreeState()
			repoRoot := ""
			if ctx != nil {
				repoRoot = ctx.RepoRoot
			}
			location := worktreeLocationForEpic(config, repoRoot, epicID)
			if worktrees != nil {
				if path, ok := worktrees[item.WorktreeBranch]; ok {
					location = displayWorktreePath(repoRoot, path)
				}
			}

			fmt.Printf("\nWorktree cleanup:\n")

			// Determine merge target
			mergeTarget := "main"
			if item.ParentID != nil && *item.ParentID != "" {
				// Check if parent has worktree
				parent, err := database.GetItem(*item.ParentID)
				if err == nil && parent.WorktreeBranch != "" {
					mergeTarget = parent.WorktreeBranch
					fmt.Printf("  # Merge to parent epic branch (%s):\n", mergeTarget)
				} else {
					fmt.Printf("  # Merge to main:\n")
				}
			} else {
				fmt.Printf("  # Merge to main:\n")
			}

			fmt.Printf("  git checkout %s\n", mergeTarget)
			fmt.Printf("  git merge %s\n", item.WorktreeBranch)
			fmt.Printf("  git worktree remove %s\n", location)
			fmt.Printf("  git branch -d %s\n", item.WorktreeBranch)
		}

		if item.ClosingInstructions == "" && item.WorktreeBranch == "" {
			fmt.Println("\nNo closing instructions or worktree configured for this epic.")
		}

		return nil
	},
}

var addCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Create a new task",
	Long: `Create a new task.

Returns the generated ID (e.g., ts-XXXXXX). Provide a detailed description since
future context may be unknown when the task is executed.

To create an epic (a container for organizing related tasks that auto-completes
when all children are done), use 'tpg epic add' instead.

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
  EOF

  # Task with metadata
  tpg add "Critical fix" --priority 1 --parent ep-abc123 -l bug

  # Custom type and prefix
  tpg add "Bug fix" --type bug
  tpg add "Story" --type story --prefix st

  # From template (see 'tpg template list')
  tpg add "Feature X" --template tdd --vars-yaml <<EOF
  feature_name: user authentication
  problem: Users need secure login
  requirements: |
    - Validate email format
    - Hash passwords with bcrypt
  EOF

  # Create task with multiple fields via YAML:
  tpg add "Task title" --from-yaml <<EOF
  desc: |
    Detailed description here
  priority: 1
  parent: ep-abc123
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

			// Determine item type for template parent
			parentType := model.ItemTypeTask
			if flagType != "" {
				parentType = model.ItemType(flagType)
			}

			parentID, err := instantiateTemplate(database, project, strings.Join(args, " "), flagTemplateID, varPairs, flagPriority, parentType)
			if err != nil {
				return err
			}

			// Set parent if specified
			if flagParent != "" {
				if err := database.SetParent(parentID, flagParent); err != nil {
					return err
				}
			}

			// Add blocking relationship if specified
			if flagBlocks != "" {
				// This new item blocks the specified item
				// (the blocked item depends on this new one)
				if err := database.AddDep(flagBlocks, parentID); err != nil {
					return err
				}
			}

			// Add dependency relationship if specified
			if flagAfter != "" {
				// This new item depends on the specified item
				// (this new item is blocked by the specified one)
				if err := database.AddDep(parentID, flagAfter); err != nil {
					return err
				}
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
		if flagType != "" {
			itemType = model.ItemType(flagType)
		}

		// Generate ID with custom prefix if provided
		var itemID string
		if flagPrefix != "" {
			itemID = model.GenerateIDWithPrefixN(flagPrefix, itemType, model.DefaultIDLength)
		} else {
			itemID, err = database.GenerateItemID(itemType)
			if err != nil {
				return err
			}
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

		// Handle dry-run mode: preview what would be created
		if flagDryRun {
			fmt.Println("DRY RUN: The following task would be created:")
			fmt.Printf("  ID:          %s\n", item.ID)
			fmt.Printf("  Title:       %s\n", item.Title)
			fmt.Printf("  Type:        %s\n", item.Type)
			fmt.Printf("  Project:     %s\n", item.Project)
			fmt.Printf("  Priority:    %d\n", item.Priority)
			if flagParent != "" {
				fmt.Printf("  Parent:      %s\n", flagParent)
			}
			if flagBlocks != "" {
				fmt.Printf("  Blocks:      %s (this task would block %s)\n", flagBlocks, flagBlocks)
			}
			if flagAfter != "" {
				fmt.Printf("  Depends on:  %s (this task would be blocked by %s)\n", flagAfter, flagAfter)
			}
			if len(flagAddLabels) > 0 {
				fmt.Printf("  Labels:      %s\n", strings.Join(flagAddLabels, ", "))
			}
			if item.Description != "" {
				desc := item.Description
				if len(desc) > 100 {
					desc = desc[:97] + "..."
				}
				fmt.Printf("  Description: %s\n", desc)
			}
			fmt.Println("\nNo task was created (dry-run mode).")
			return nil
		}

		config, _ := db.LoadConfig()

		// Warn if description is very short (including empty) - configurable
		if config != nil && config.ShortDescriptionWarningEnabled() {
			minWords := config.GetMinDescriptionWords()
			if countWords(description) < minWords {
				fmt.Fprintf(os.Stderr, "\nWARNING: This description is very short (%d words, recommend %d+). Does it include\nall context needed for someone not part of the main discussion to understand the task?\nConsider extending with: tpg edit %s --desc\n", countWords(description), minWords, item.ID)
			}
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

var replaceCmd = &cobra.Command{
	Use:   "replace <id> <title>",
	Short: "Replace an existing task/epic with a new one",
	Long: `Replace an existing task or epic with a new one, preserving relationships.

The new item inherits the old item's:
  - Parent
  - Children (if replacing with an epic)
  - Dependencies (both blocking and blocked-by)
  - Logs

The new item does NOT inherit:
  - Labels (must be re-added if needed)
  - Status (new item starts as open)

Constraints:
  - Cannot replace an epic-with-children with a task (tasks can't have children)

Use '-' with a flag to read a single field from stdin.
For multiple fields, use --from-yaml instead of individual flags.

Examples:
  # Replace a task with a new task
  tpg replace ts-abc123 "Better task title"

  # Replace a task with an epic
  tpg replace ts-abc123 "Now an epic" -e

  # Replace with description from stdin
  tpg replace ts-abc123 "New task" --desc - <<EOF
  Updated requirements and context
  EOF

  # Replace with multiple fields via YAML
  tpg replace ts-abc123 "New task" --from-yaml <<EOF
  desc: |
    Updated requirements and context.
    See design doc for details.
  priority: 1
  EOF

  # Replace with different type/priority
  tpg replace ts-abc123 "Bug fix" --type bug --priority 1`,
	Args: cobra.MinimumNArgs(2),
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

		oldID := args[0]
		title := strings.Join(args[1:], " ")

		// Determine item type
		itemType := model.ItemTypeTask
		if flagEpic {
			itemType = model.ItemTypeEpic
		}
		if flagType != "" {
			itemType = model.ItemType(flagType)
		}

		// Generate ID with custom prefix if provided
		var newItemID string
		if flagPrefix != "" {
			newItemID = model.GenerateIDWithPrefixN(flagPrefix, itemType, model.DefaultIDLength)
		} else {
			newItemID, err = database.GenerateItemID(itemType)
			if err != nil {
				return err
			}
		}

		// Initialize from flags
		description := flagDescription
		context := flagContext
		onClose := flagOnClose

		// Handle single-field stdin markers
		stdinUsed := false

		if description == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			description = strings.TrimSpace(string(data))
			stdinUsed = true
		}

		if context == "-" {
			if stdinUsed {
				return fmt.Errorf("cannot use stdin for multiple flags; use --from-yaml instead")
			}
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			context = strings.TrimSpace(string(data))
			stdinUsed = true
		}

		if onClose == "-" {
			if stdinUsed {
				return fmt.Errorf("cannot use stdin for multiple flags; use --from-yaml instead")
			}
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			onClose = strings.TrimSpace(string(data))
		}

		newItem := &model.Item{
			ID:                  newItemID,
			Project:             project,
			Type:                itemType,
			Title:               title,
			Description:         description,
			Status:              model.StatusOpen,
			Priority:            flagPriority,
			SharedContext:       context,
			ClosingInstructions: onClose,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}

		// Perform the replacement
		resultID, err := database.ReplaceItem(oldID, newItem)
		if err != nil {
			return err
		}

		// Add labels if specified
		for _, labelName := range flagAddLabels {
			if err := database.AddLabelToItem(resultID, project, labelName); err != nil {
				return err
			}
		}

		fmt.Println(resultID)
		database.BackupQuiet()
		return nil
	},
}

// applyYAMLFlags reads YAML from stdin and sets corresponding flag values.
// Keys in YAML use underscores (e.g., "some_flag"), which are converted to
// hyphens (e.g., "--some-flag") for flag lookup.
// Supports types: string, int, bool, []string (StringArray).
func applyYAMLFlags(cmd *cobra.Command) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read from stdin: %w", err)
	}
	if len(data) == 0 {
		return nil // No input, nothing to do
	}

	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return fmt.Errorf("failed to parse YAML from stdin: %w", err)
	}

	return applyYAMLFlagsFromData(cmd, yamlData)
}

// applyYAMLFlagsFromData applies YAML data (as a map) to command flags.
// This is the testable core of applyYAMLFlags.
func applyYAMLFlagsFromData(cmd *cobra.Command, yamlData map[string]interface{}) error {
	for key, value := range yamlData {
		// Convert underscores to hyphens for flag name
		flagName := strings.ReplaceAll(key, "_", "-")

		// Look up the flag in the command's flag set (including persistent flags)
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			// Try just the local flags
			flag = cmd.LocalFlags().Lookup(flagName)
		}
		if flag == nil {
			// Try persistent flags from parent
			flag = cmd.InheritedFlags().Lookup(flagName)
		}
		if flag == nil {
			return fmt.Errorf("unknown flag from YAML: %q (converted from %q)", flagName, key)
		}

		// Set the flag value based on its type
		if err := setFlagFromYAML(flag, value); err != nil {
			return fmt.Errorf("failed to set flag %q: %w", flagName, err)
		}
	}

	return nil
}

// setFlagFromYAML sets a flag's value from a YAML-parsed interface{} value.
// Handles type conversion for string, int, bool, and []string (StringArray).
// Also marks the flag as Changed so cmd.Flags().Changed() returns true.
func setFlagFromYAML(flag *pflag.Flag, value interface{}) error {
	if value == nil {
		return nil // Skip nil values
	}

	var err error
	switch flag.Value.Type() {
	case "string":
		strVal, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
		err = flag.Value.Set(strVal)
		if err == nil {
			flag.Changed = true
		}
		return err

	case "int":
		switch v := value.(type) {
		case int:
			err = flag.Value.Set(fmt.Sprintf("%d", v))
		case int64:
			err = flag.Value.Set(fmt.Sprintf("%d", v))
		case float64:
			// YAML often parses integers as float64
			err = flag.Value.Set(fmt.Sprintf("%d", int(v)))
		default:
			return fmt.Errorf("expected int, got %T", value)
		}
		if err == nil {
			flag.Changed = true
		}
		return err

	case "bool":
		boolVal, ok := value.(bool)
		if !ok {
			return fmt.Errorf("expected bool, got %T", value)
		}
		if boolVal {
			err = flag.Value.Set("true")
		} else {
			err = flag.Value.Set("false")
		}
		if err == nil {
			flag.Changed = true
		}
		return err

	case "stringArray":
		// Handle both single string and array of strings
		switch v := value.(type) {
		case string:
			err = flag.Value.Set(v)
			if err == nil {
				flag.Changed = true
			}
			return err
		case []interface{}:
			for _, item := range v {
				strItem, ok := item.(string)
				if !ok {
					return fmt.Errorf("expected string array element, got %T", item)
				}
				if err := flag.Value.Set(strItem); err != nil {
					return err
				}
			}
			flag.Changed = true
			return nil
		case []string:
			for _, strItem := range v {
				if err := flag.Value.Set(strItem); err != nil {
					return err
				}
			}
			flag.Changed = true
			return nil
		default:
			return fmt.Errorf("expected string or string array, got %T", value)
		}

	default:
		return fmt.Errorf("unsupported flag type: %s", flag.Value.Type())
	}
}

// findStdinMarkerFlag checks if any flag value is '-' (stdin marker).
// Returns the flag name if found, empty string otherwise.
// This is used to detect conflicts with --from-yaml which also reads from stdin.
func findStdinMarkerFlag(cmd *cobra.Command) string {
	var found string
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if found != "" {
			return // Already found one
		}
		// Only check string flags that have been explicitly set
		if f.Value.Type() == "string" && f.Changed && f.Value.String() == "-" {
			found = f.Name
		}
	})
	return found
}

// countWords returns the number of words in a string
func countWords(s string) int {
	return len(strings.Fields(s))
}

// generateWorktreeBranch generates a branch name from epic ID and title.
// Format: <prefix>/<epic-id>-<slug> where slug is lowercase title with non-alnum→hyphens.
func generateWorktreeBranch(epicID, title, prefix string) string {
	// Convert title to lowercase
	slug := strings.ToLower(title)

	// Replace non-alphanumeric characters with hyphens
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}

	// Collapse multiple hyphens
	slug = result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim hyphens from ends
	slug = strings.Trim(slug, "-")

	// Limit length
	if len(slug) > 50 {
		slug = slug[:50]
	}

	prefix = normalizeWorktreePrefix(prefix)
	if slug == "" {
		if prefix == "" {
			return epicID
		}
		return fmt.Sprintf("%s/%s", prefix, epicID)
	}
	if prefix == "" {
		return fmt.Sprintf("%s-%s", epicID, slug)
	}
	return fmt.Sprintf("%s/%s-%s", prefix, epicID, slug)
}

func normalizeWorktreePrefix(prefix string) string {
	p := strings.TrimSpace(prefix)
	p = strings.TrimSuffix(p, "/")
	return p
}

func worktreeRoot(config *db.Config) string {
	if config == nil || config.Worktree.Root == "" {
		return ".worktrees"
	}
	return config.Worktree.Root
}

func worktreePrefix(config *db.Config) string {
	if config == nil || config.Worktree.BranchPrefix == "" {
		return "feature"
	}
	return config.Worktree.BranchPrefix
}

func worktreeRequireEpicID(config *db.Config) bool {
	if config == nil {
		return true
	}
	return config.Worktree.RequireEpicIDEnabled()
}

func branchIncludesEpicID(branch, epicID string) bool {
	if branch == "" || epicID == "" {
		return false
	}
	re := regexp.MustCompile(`(?i)(^|[^a-z0-9])` + regexp.QuoteMeta(epicID) + `([^a-z0-9]|$)`) //nolint:gomnd
	return re.MatchString(branch)
}

func detectWorktreeState() (*worktree.Context, map[string]string) {
	ctx, err := worktree.DetectContext("")
	if err != nil {
		return nil, nil
	}
	if ctx.RepoRoot == "" {
		return ctx, nil
	}
	worktrees, err := worktree.ListWorktrees(ctx.RepoRoot)
	if err != nil {
		return ctx, nil
	}
	return ctx, worktrees
}

func displayWorktreePath(repoRoot, path string) string {
	if path == "" || repoRoot == "" {
		return path
	}
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}

func worktreeLocationForEpic(config *db.Config, repoRoot, epicID string) string {
	root := worktreeRoot(config)
	if repoRoot == "" {
		return filepath.Join(root, epicID)
	}
	return displayWorktreePath(repoRoot, filepath.Join(repoRoot, root, epicID))
}

func resolveWorktreeBase(database *db.DB, parentID string) string {
	if parentID != "" {
		if parent, err := database.GetItem(parentID); err == nil {
			if parent.WorktreeBranch != "" {
				return parent.WorktreeBranch
			}
		}
	}
	ctx, err := worktree.DetectContext("")
	if err == nil && ctx.CurrentBranch != "" {
		return ctx.CurrentBranch
	}
	return "main"
}

func buildWorktreeInfo(rootEpic *model.Item, epicPath []model.Item, config *db.Config) *WorktreeInfo {
	if rootEpic == nil {
		return nil
	}
	info := &WorktreeInfo{
		EpicID:    rootEpic.ID,
		EpicTitle: rootEpic.Title,
		Branch:    rootEpic.WorktreeBranch,
		Base:      rootEpic.WorktreeBase,
	}
	for _, p := range epicPath {
		info.Path = append(info.Path, p.ID)
	}

	ctx, worktrees := detectWorktreeState()
	repoRoot := ""
	if ctx != nil {
		repoRoot = ctx.RepoRoot
	}
	location := worktreeLocationForEpic(config, repoRoot, rootEpic.ID)

	var actualPath string
	if worktrees != nil {
		if path, ok := worktrees[rootEpic.WorktreeBranch]; ok {
			info.Exists = true
			actualPath = path
			location = displayWorktreePath(repoRoot, path)
		}
	}
	info.Location = location

	if info.Exists && actualPath != "" {
		if cwd, err := os.Getwd(); err == nil {
			if worktree.IsWithinDir(cwd, actualPath) {
				info.InWorktree = true
			}
		}
	}

	return info
}

func worktreeStatusText(info *WorktreeInfo) string {
	if info == nil {
		return ""
	}
	if info.Exists {
		if info.InWorktree {
			return "worktree exists"
		}
		return "not in worktree"
	}
	return "worktree not found"
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	Long: `List tasks, optionally filtered by various criteria.

By default shows hierarchical tree view and excludes done/canceled items.

Examples:
  tpg list                        # Tree view of active items
  tpg list --all                  # Tree view including done/canceled
  tpg list -f                     # Flat list (no hierarchy)
  tpg list --flat                 # Same as -f
  tpg list --epic ep-abc          # Tree view of an epic's descendants
  tpg list -p myproject
  tpg list --status open
  tpg list --status done          # Explicitly show done items
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
		if !flagListAll && !statusExplicitlySet {
			filtered := make([]model.Item, 0, len(items))
			for _, item := range items {
				if item.Status != model.StatusDone && item.Status != model.StatusCanceled {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}

		// Filter to epic descendants if --epic is set
		if flagListEpic != "" {
			descendants, err := database.GetDescendants(flagListEpic)
			if err != nil {
				return fmt.Errorf("failed to get descendants of epic %s: %w", flagListEpic, err)
			}
			// Create a map of descendant IDs for fast lookup
			descendantIDs := make(map[string]bool)
			for _, d := range descendants {
				descendantIDs[d.ID] = true
			}
			// Filter items to only those in the descendant set
			filtered := make([]model.Item, 0, len(items))
			for _, item := range items {
				if descendantIDs[item.ID] {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}

		// Populate labels for display (skip if ids-only)
		if !flagIdsOnly {
			if err := database.PopulateItemLabels(items); err != nil {
				return err
			}
			if err := renderTemplatesForItems(items); err != nil {
				return err
			}
		}

		if flagIdsOnly {
			printItemsIDs(items)
		} else if flagListFlat {
			printItemsTable(items)
		} else {
			printItemsTree(items)
		}
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
  tpg ready -l bug
  tpg ready --epic ep-abc123`,
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

		var items []model.Item

		// Check if filtering by epic
		if flagReadyEpic != "" {
			// Verify the epic exists
			epic, err := database.GetItem(flagReadyEpic)
			if err != nil {
				return err
			}
			if epic.Type != model.ItemTypeEpic {
				return fmt.Errorf("%s is not an epic", flagReadyEpic)
			}

			items, err = database.ReadyItemsForEpic(flagReadyEpic)
			if err != nil {
				return err
			}

			// Filter by project if specified
			if project != "" {
				var filtered []model.Item
				for _, item := range items {
					if item.Project == project {
						filtered = append(filtered, item)
					}
				}
				items = filtered
			}

			// Show epic title in header
			fmt.Printf("Ready tasks for epic %s - %s:\n", epic.ID, epic.Title)
		} else {
			items, err = database.ReadyItemsFiltered(project, flagFilterLabels)
			if err != nil {
				return err
			}
		}

		if len(items) == 0 {
			if flagReadyEpic != "" {
				fmt.Println("No ready tasks for this epic")
			} else {
				fmt.Println("No ready tasks")
			}
		} else {
			// Show count
			if flagReadyEpic != "" {
				fmt.Printf("(%d ready)\n\n", len(items))
			}

			// Populate labels for display
			if err := database.PopulateItemLabels(items); err != nil {
				return err
			}
			if err := renderTemplatesForItems(items); err != nil {
				return err
			}

			printReadyTable(items)
		}

		// Check for in-progress tasks and show a hint
		inProgressStatus := model.StatusInProgress
		inProgressItems, err := database.ListItems(project, &inProgressStatus)
		if err != nil {
			return err
		}
		if len(inProgressItems) > 0 {
			fmt.Printf("\n(%d task(s) currently in-progress — use 'tpg list --status in_progress' to view)\n", len(inProgressItems))
		}

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

		depStatuses, err := database.GetAllDepStatuses(args[0])
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

		// Gather additional data based on flags
		var children []model.Item
		var parentChain []model.Item
		var depChain []db.DepEdge

		if flagShowWithChildren {
			children, err = database.GetDescendants(args[0])
			if err != nil {
				return err
			}
		}

		if flagShowWithParent {
			parentChain, err = database.GetParentChain(args[0])
			if err != nil {
				return err
			}
		}

		if flagShowWithDeps {
			depChain, err = database.GetDependencyChain(args[0])
			if err != nil {
				return err
			}
		}

		// Check if item is under a worktree epic
		rootEpic, epicPath, err := database.GetRootEpic(item.ID)
		if err != nil {
			return err
		}
		config, _ := db.LoadConfig()
		worktreeInfo := buildWorktreeInfo(rootEpic, epicPath, config)

		// Get shared context from ancestors
		sharedContext, err := database.GetAncestorSharedContext(item.ID)
		if err != nil {
			return err
		}

		// Output based on format
		switch flagShowFormat {
		case "json":
			return printItemJSON(item, logs, deps, blockers, latestProgress, concepts, templateNotice, children, parentChain, depChain, worktreeInfo)
		case "yaml":
			return printItemYAML(item, logs, deps, blockers, latestProgress, concepts, templateNotice, children, parentChain, depChain, worktreeInfo)
		case "markdown":
			return printItemMarkdown(item, logs, deps, blockers, latestProgress, concepts, templateNotice, children, parentChain, depChain, worktreeInfo)
		default:
			printItemDetail(item, logs, deps, blockers, latestProgress, concepts, templateNotice, flagShowVars, worktreeInfo, epicPath, sharedContext)
			if flagShowWithParent && len(parentChain) > 0 {
				fmt.Printf("\nParent Chain:\n")
				for _, parent := range parentChain {
					fmt.Printf("  %s [%s] %s\n", parent.ID, parent.Status, parent.Title)
				}
			}
			if flagShowWithChildren && len(children) > 0 {
				fmt.Printf("\nChildren:\n")
				for _, child := range children {
					fmt.Printf("  %s [%s] %s\n", child.ID, child.Status, child.Title)
				}
			}
			if flagShowWithDeps && len(depChain) > 0 {
				fmt.Printf("\nDependency Chain:\n")
				for _, edge := range depChain {
					fmt.Printf("  %s depends on %s [%s]\n", edge.ItemID, edge.DependsOnID, edge.DependsOnStatus)
				}
			}
			return nil
		}
	},
}

var historyCmd = &cobra.Command{
	Use:   "history <id>",
	Short: "Show full history timeline for a task",
	Long: `Show a chronological timeline of everything that happened to a task.

Includes:
  - Creation timestamp with initial metadata
  - All log entries (progress, blocks, cancels, reopens, merges)
  - Completion results (if done)
  - Current status and last update

Example:
  tpg history ts-a1b2c3`,
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

		logs, err := database.GetLogs(args[0])
		if err != nil {
			return err
		}

		labels, err := database.GetItemLabels(args[0])
		if err != nil {
			return err
		}
		var labelNames []string
		for _, l := range labels {
			labelNames = append(labelNames, l.Name)
		}

		deps, err := database.GetDeps(args[0])
		if err != nil {
			return err
		}

		// Header
		fmt.Printf("%s — %s\n", item.ID, item.Title)
		fmt.Printf("Type: %s  Project: %s  Priority: %d\n", item.Type, item.Project, item.Priority)
		if item.ParentID != nil {
			fmt.Printf("Parent: %s\n", *item.ParentID)
		}
		if len(labelNames) > 0 {
			fmt.Printf("Labels: %s\n", strings.Join(labelNames, ", "))
		}
		if len(deps) > 0 {
			fmt.Printf("Dependencies: %s\n", strings.Join(deps, ", "))
		}

		// Timeline
		fmt.Printf("\nTimeline:\n")
		fmt.Printf("  [%s] Created\n", item.CreatedAt.Format("2006-01-02 15:04"))

		for _, log := range logs {
			fmt.Printf("  [%s] %s\n", log.CreatedAt.Format("2006-01-02 15:04"), log.Message)
		}

		if item.Status == model.StatusDone && item.Results != "" {
			fmt.Printf("  [%s] Completed: %s\n", item.UpdatedAt.Format("2006-01-02 15:04"), item.Results)
		}

		// Current state
		fmt.Printf("\nCurrent: %s (updated %s)\n", item.Status, item.UpdatedAt.Format("2006-01-02 15:04"))
		if item.AgentID != nil {
			fmt.Printf("Agent: %s\n", *item.AgentID)
		}

		return nil
	},
}

var startCmd = &cobra.Command{
	Use:   "start <id>",
	Short: "Start working on a task",
	Long: `Set a task's status to in_progress.

Use this when you begin working on a task. Updates the timestamp
for stale detection.

If the task is already in progress, use --resume to continue or take over.

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

		// Check if this is an epic with children - epics are containers, not workable items
		hasChildren, err := database.HasChildren(item.ID)
		if err != nil {
			return err
		}
		if hasChildren {
			return fmt.Errorf(`cannot start %s: epics with children cannot be worked on directly

An epic is a container that organizes related tasks. Work on its children instead:
  tpg ready --epic %s    # Find ready tasks within this epic
  tpg list --epic %s     # See all tasks in this epic`, item.ID, item.ID, item.ID)
		}

		resuming := item.Status == model.StatusInProgress
		if resuming && !flagResume {
			agentInfo := ""
			if item.AgentID != nil && *item.AgentID != "" {
				agentInfo = fmt.Sprintf(" (claimed by %s)", *item.AgentID)
			}
			return fmt.Errorf("task %s is already in progress%s. Use --resume to take over or continue work", item.ID, agentInfo)
		}

		agentCtx := db.GetAgentContext()

		// Record agent project access
		if agentCtx.IsActive() {
			_ = database.RecordAgentProjectAccess(agentCtx.ID, item.Project)
		}

		if err := database.UpdateStatus(args[0], model.StatusInProgress, agentCtx, false); err != nil {
			return err
		}

		// Auto-log the start event for timeline
		logMsg := "Started"
		if resuming {
			logMsg = "Resumed"
		}
		if agentCtx.IsActive() {
			logMsg = fmt.Sprintf("%s (agent: %s)", logMsg, agentCtx.ID)
		}
		_ = database.AddLog(args[0], logMsg)

		if resuming {
			fmt.Printf("Resuming %s (already in progress)\n", args[0])
		} else {
			fmt.Printf("Started %s\n", args[0])
		}

		// Check if task belongs to a worktree epic
		rootEpic, epicPath, err := database.GetRootEpic(item.ID)
		if err != nil {
			return err
		}

		if rootEpic != nil {
			config, _ := db.LoadConfig()
			worktreeInfo := buildWorktreeInfo(rootEpic, epicPath, config)
			if worktreeInfo != nil && !worktreeInfo.InWorktree {
				fmt.Fprintf(os.Stderr, "\nWorktree: %s - %s\n", worktreeInfo.EpicID, worktreeInfo.EpicTitle)
				fmt.Fprintf(os.Stderr, "  Branch: %s\n", worktreeInfo.Branch)
				fmt.Fprintf(os.Stderr, "  Location: %s\n", worktreeInfo.Location)

				base := worktreeInfo.Base
				if base == "" {
					base = "main"
				}
				if worktreeInfo.Exists {
					fmt.Fprintf(os.Stderr, "\n  To work in the correct directory:\n")
					fmt.Fprintf(os.Stderr, "    cd %s\n", worktreeInfo.Location)
				} else {
					fmt.Fprintf(os.Stderr, "\n  Worktree not found. Create it with:\n")
					fmt.Fprintf(os.Stderr, "    git worktree add -b %s %s %s\n", worktreeInfo.Branch, worktreeInfo.Location, base)
					fmt.Fprintf(os.Stderr, "    cd %s\n", worktreeInfo.Location)
				}
			}
		}

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
  tpg done ts-a1b2c3 --override "Work superseded by different approach"

Note: Completing a task with zero log entries will trigger a warning.
Consider logging progress milestones before marking done.`,
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

		// Warn if no logs were recorded during work
		logs, err := database.GetLogs(id)
		if err != nil {
			return err
		}
		if len(logs) == 0 {
			fmt.Fprintf(os.Stderr, `WARNING: Completing %s with zero log entries.

If you discovered anything during this work, log it BEFORE completing:
  tpg log %s "what you found, decided, or changed and why"

Triggers that should always produce a log entry:
  - Discovered a blocker or created a dependency
  - Chose between alternatives (and why)
  - Found existing code that changed your approach
  - Hit something unexpected

`, id, id)
		}

		agentCtx := db.GetAgentContext()
		if err := database.CompleteItem(id, results, agentCtx); err != nil {
			return err
		}

		// Auto-log completion for timeline
		_ = database.AddLog(id, "Completed")

		fmt.Printf("Completed %s\n", id)

		// Check if parent epic should be auto-completed
		if err := autoCompleteParentEpics(database, id); err != nil {
			// Log but don't fail - the main task was completed successfully
			fmt.Fprintf(os.Stderr, "Warning: failed to check parent epic completion: %v\n", err)
		}

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
  tpg cancel ts-a1b2c3 "Requirements changed, no longer needed"
  tpg cancel ts-a1b2c3 --force   # Cancel even if other tasks depend on it

See also: 'tpg delete' to remove a task entirely (no history preserved).`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]

		agentCtx := db.GetAgentContext()
		if err := database.UpdateStatus(id, model.StatusCanceled, agentCtx, flagCancelForce); err != nil {
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

var reopenCmd = &cobra.Command{
	Use:   "reopen <id> [reason]",
	Short: "Reopen a closed task, setting it back to open",
	Long: `Reopen a task that was completed, canceled, or otherwise closed
but needs to be revisited.

This sets the task back to open (pending) status. Use this for
edge cases where a task was closed prematurely.

Example:
  tpg reopen ts-a1b2c3
  tpg reopen ts-a1b2c3 "Fix didn't actually resolve the issue"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]

		agentCtx := db.GetAgentContext()
		if err := database.UpdateStatus(id, model.StatusOpen, agentCtx, false); err != nil {
			return err
		}

		if len(args) > 1 {
			reason := strings.Join(args[1:], " ")
			if err := database.AddLog(id, "Reopened: "+reason); err != nil {
				return err
			}
			fmt.Printf("Reopened %s: %s\n", id, reason)
		} else {
			fmt.Printf("Reopened %s\n", id)
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

var flagBlockForce bool

var blockCmd = &cobra.Command{
	Use:   "block <id> <reason>",
	Short: "Mark a task as blocked (discouraged — use dependencies instead)",
	Long: `Mark a task as blocked and log the reason.

WARNING: Using 'block' is almost never the right approach. Instead:
  - Add a dependency: tpg dep <blocker-task> blocks <this-task>
  - The task will automatically become unblocked when the blocker is done.
  - 'tpg ready' respects dependencies, so agents only see unblocked work.

'block' sets a manual status that requires manual unblocking. Dependencies
are resolved automatically when prerequisite work completes.

If you truly need a manual block (e.g., waiting on an external event with
no corresponding tpg task), use --force:
  tpg block --force ts-a1b2c3 "Waiting on client API credentials"`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !flagBlockForce {
			return fmt.Errorf(`'tpg block' is discouraged — use dependencies instead:

  tpg add "Blocker: <description>" -p 1    # Create a task for what's blocking
  tpg dep <blocker-task> blocks %s          # Link the dependency

The blocked task won't appear in 'tpg ready' until the blocker is done.
Dependencies are resolved automatically; manual blocks require manual unblocking.

If you must use block (e.g., external dependency with no tpg task), re-run with --force`, args[0])
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		id := args[0]
		reason := strings.Join(args[1:], " ")

		agentCtx := db.GetAgentContext()
		if err := database.UpdateStatus(id, model.StatusBlocked, agentCtx, false); err != nil {
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

By default, deletion is blocked if other tasks depend on this item.
Use --force to delete anyway (dependencies will be removed).

Example:
  tpg delete ts-a1b2c3
  tpg delete ts-a1b2c3 --force   # Remove even if other tasks depend on it

See also: 'tpg cancel' to close a task while preserving history.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		if err := database.DeleteItem(args[0], flagDeleteForce); err != nil {
			return err
		}
		fmt.Printf("Deleted %s\n", args[0])
		return nil
	},
}

var (
	flagCleanDone     bool
	flagCleanCanceled bool
	flagCleanLogs     bool
	flagCleanVacuum   bool
	flagCleanAll      bool
	flagCleanDays     int
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up old tasks and compact database",
	Long: `Remove old done/canceled tasks and compact the database.

By default, requires confirmation before deleting. Use --dry-run to preview
what would be deleted, or --force to skip confirmation.

Examples:
  tpg clean --done              # Remove done tasks older than 30 days
  tpg clean --canceled          # Remove canceled tasks older than 30 days
  tpg clean --all               # Remove old done+canceled and vacuum
  tpg clean --all --days 7      # More aggressive: 7 day threshold
  tpg clean --dry-run --all     # Preview what would be deleted
  tpg clean --vacuum            # Just compact the database`,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		// If --all is set, enable all cleanup operations
		if flagCleanAll {
			flagCleanDone = true
			flagCleanCanceled = true
			flagCleanVacuum = true
		}

		// Require at least one operation
		if !flagCleanDone && !flagCleanCanceled && !flagCleanLogs && !flagCleanVacuum {
			return fmt.Errorf("specify at least one of: --done, --canceled, --logs, --vacuum, or --all")
		}

		cutoff := time.Now().AddDate(0, 0, -flagCleanDays)

		// Count what would be deleted
		var doneCount, canceledCount, orphanedLogCount int
		var err2 error

		if flagCleanDone {
			doneCount, err2 = database.CountOldItems(cutoff, model.StatusDone)
			if err2 != nil {
				return err2
			}
		}

		if flagCleanCanceled {
			canceledCount, err2 = database.CountOldItems(cutoff, model.StatusCanceled)
			if err2 != nil {
				return err2
			}
		}

		if flagCleanLogs {
			orphanedLogCount, err2 = database.CountOrphanedLogs()
			if err2 != nil {
				return err2
			}
		}

		// Show what would be deleted
		hasWork := doneCount > 0 || canceledCount > 0 || orphanedLogCount > 0 || flagCleanVacuum
		if !hasWork {
			fmt.Println("Nothing to clean up")
			return nil
		}

		fmt.Println("Found:")
		if flagCleanDone {
			fmt.Printf("  %d done tasks older than %d days\n", doneCount, flagCleanDays)
		}
		if flagCleanCanceled {
			fmt.Printf("  %d canceled tasks older than %d days\n", canceledCount, flagCleanDays)
		}
		if flagCleanLogs && orphanedLogCount > 0 {
			fmt.Printf("  %d orphaned log entries\n", orphanedLogCount)
		}
		if flagCleanVacuum {
			fmt.Println("  Database will be compacted")
		}

		// Dry run - just show what would happen
		if flagDryRun {
			fmt.Println("\nDry run - no changes made")
			return nil
		}

		// Confirm unless --force
		if !flagForce {
			fmt.Print("\nDelete these items? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Aborted")
				return nil
			}
		}

		fmt.Println()

		// Get database size before vacuum
		var sizeBefore int64
		if flagCleanVacuum {
			sizeBefore, _ = database.GetDatabaseSize()
		}

		// Perform deletions
		if flagCleanDone && doneCount > 0 {
			deleted, err := database.DeleteOldItems(cutoff, model.StatusDone)
			if err != nil {
				return err
			}
			fmt.Printf("Deleted %d done tasks\n", deleted)
		}

		if flagCleanCanceled && canceledCount > 0 {
			deleted, err := database.DeleteOldItems(cutoff, model.StatusCanceled)
			if err != nil {
				return err
			}
			fmt.Printf("Deleted %d canceled tasks\n", deleted)
		}

		if flagCleanLogs && orphanedLogCount > 0 {
			deleted, err := database.DeleteOrphanedLogs()
			if err != nil {
				return err
			}
			fmt.Printf("Deleted %d orphaned log entries\n", deleted)
		}

		if flagCleanVacuum {
			fmt.Print("Running VACUUM...")
			if err := database.Vacuum(); err != nil {
				return err
			}
			sizeAfter, _ := database.GetDatabaseSize()
			saved := sizeBefore - sizeAfter
			if saved > 0 {
				fmt.Printf(" done (saved %s)\n", formatSize(saved))
			} else {
				fmt.Println(" done")
			}
		}

		// Backup after successful cleanup
		database.BackupQuiet()

		return nil
	},
}

var logCmd = &cobra.Command{
	Use:   "log <id> <message>",
	Short: "Add a log entry to a task",
	Long: `Add a timestamped log entry to a task's audit trail.

Updates the task's timestamp (affects stale detection).

WHEN TO LOG (do this immediately when it happens):
  • Discovered a blocker or created a dependency
  • Chose between alternatives (log what and why)
  • Found existing code/patterns that change your approach
  • Hit something unexpected (error, missing API, wrong assumption)
  • Finished a key milestone (core logic works, tests pass)

DO NOT LOG routine actions (opened file, read docs, ran command).

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
	Long: `Show all task dependencies as an ASCII tree.

Displays dependency relationships between tasks: which tasks block others
and must be completed first. Each blocked task is shown with its blockers
indented below it.

Output format:
  ts-abc [status] Task that is blocked
    └── ts-xyz [status] Task that blocks ts-abc (must complete first)
    └── ts-def [status] Another blocker for ts-abc

Status values: open, in_progress, done, blocked, canceled

The graph includes ALL tasks with dependencies (including completed ones).
Use 'tpg dep <id> list' to see dependencies for a specific task.
Use 'tpg ready' to see only unblocked tasks available to start.

Examples:
  tpg graph              # Show full dependency graph
  tpg graph -p myproject # Show graph for specific project`,
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

var planCmd = &cobra.Command{
	Use:   "plan <epic-id>",
	Short: "Show full epic plan with status and dependencies",
	Long: `Show comprehensive epic overview with all tasks, status, ready tasks, and blockers.

Displays:
  - Epic details (title, status, description)
  - Progress summary (counts by status, completion %)
  - All child tasks with status in tree format
  - Ready tasks highlighted (unblocked and can be started)
  - Dependency chains and blockers

Examples:
  tpg plan ep-abc123      # Show full plan for epic
  tpg plan ep-abc123 --json  # Output as JSON`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		epicID := args[0]
		epic, err := database.GetItem(epicID)
		if err != nil {
			return err
		}

		if epic.Type != model.ItemTypeEpic {
			return fmt.Errorf("%s is not an epic (type: %s)", epicID, epic.Type)
		}

		// Get all descendants (children at all levels)
		descendants, err := database.GetDescendants(epicID)
		if err != nil {
			return err
		}

		// Populate labels for all items
		allItems := append([]model.Item{*epic}, descendants...)
		if err := database.PopulateItemLabels(allItems); err != nil {
			return err
		}

		// Get dependency info for all descendants
		depInfo := make(map[string][]db.DepStatus)
		blockedBy := make(map[string][]db.DepStatus)
		for _, item := range descendants {
			deps, err := database.GetAllDepStatuses(item.ID)
			if err != nil {
				return err
			}
			depInfo[item.ID] = deps

			blocked, err := database.GetBlockedBy(item.ID)
			if err != nil {
				return err
			}
			blockedBy[item.ID] = blocked
		}

		// Determine which tasks are ready (open + no unmet deps)
		readyTasks := make(map[string]bool)
		for _, item := range descendants {
			if item.Status != model.StatusOpen {
				continue
			}
			hasUnmet := false
			for _, dep := range depInfo[item.ID] {
				if dep.Status != string(model.StatusDone) {
					hasUnmet = true
					break
				}
			}
			if !hasUnmet {
				readyTasks[item.ID] = true
			}
		}

		// Build parent-child relationships for tree display
		childrenMap := make(map[string][]model.Item)
		for _, child := range descendants {
			if child.ParentID != nil {
				childrenMap[*child.ParentID] = append(childrenMap[*child.ParentID], child)
			}
		}

		// Calculate statistics
		stats := calculateEpicStats(descendants)

		if flagContextJSON {
			return printPlanJSON(epic, descendants, childrenMap, depInfo, blockedBy, readyTasks, stats)
		}

		// Print epic header
		fmt.Printf("\n%s [%s] %s\n", epic.ID, epic.Status, epic.Title)
		fmt.Println(strings.Repeat("=", len(epic.ID)+len(epic.Status)+len(epic.Title)+6))
		if epic.Description != "" {
			fmt.Printf("\n%s\n", epic.Description)
		}

		// Print progress summary
		fmt.Printf("\n📊 Progress: %d/%d tasks complete (%.0f%%)\n",
			stats.Done, stats.Total, stats.CompletionPct)
		fmt.Printf("   Open: %d | In Progress: %d | Blocked: %d | Done: %d | Canceled: %d\n",
			stats.Open, stats.InProgress, stats.Blocked, stats.Done, stats.Canceled)

		// Print tree view of all tasks
		fmt.Println("\n📋 Task Tree:")
		if len(descendants) == 0 {
			fmt.Println("   (no tasks)")
		} else {
			printPlanTree(database, epicID, "", childrenMap, depInfo, readyTasks, true)
		}

		// Print ready tasks section
		readyList := []model.Item{}
		for _, item := range descendants {
			if readyTasks[item.ID] {
				readyList = append(readyList, item)
			}
		}
		if len(readyList) > 0 {
			fmt.Println("\n✅ Ready to Start:")
			for _, item := range readyList {
				labels := ""
				if len(item.Labels) > 0 {
					labels = formatLabels(item.Labels) + " "
				}
				fmt.Printf("   %s [pri %d] %s%s\n", item.ID, item.Priority, labels, item.Title)
			}
		}

		// Print dependency chains / blockers
		blockersFound := false
		for _, item := range descendants {
			if item.Status == model.StatusDone || item.Status == model.StatusCanceled {
				continue
			}
			unmetDeps := []db.DepStatus{}
			for _, dep := range depInfo[item.ID] {
				if dep.Status != string(model.StatusDone) {
					unmetDeps = append(unmetDeps, dep)
				}
			}
			if len(unmetDeps) > 0 && !blockersFound {
				fmt.Println("\n⛔ Blocked Tasks:")
				blockersFound = true
			}
			for _, dep := range unmetDeps {
				fmt.Printf("   %s blocked by %s [%s] %s\n", item.ID, dep.ID, dep.Status, dep.Title)
			}
		}

		fmt.Println()
		return nil
	},
}

// epicStats holds statistics for an epic
type epicStats struct {
	Total         int
	Open          int
	InProgress    int
	Blocked       int
	Done          int
	Canceled      int
	CompletionPct float64
}

// calculateEpicStats calculates statistics for an epic's tasks
func calculateEpicStats(tasks []model.Item) epicStats {
	s := epicStats{Total: len(tasks)}
	for _, t := range tasks {
		switch t.Status {
		case model.StatusOpen:
			s.Open++
		case model.StatusInProgress:
			s.InProgress++
		case model.StatusBlocked:
			s.Blocked++
		case model.StatusDone:
			s.Done++
		case model.StatusCanceled:
			s.Canceled++
		}
	}
	if s.Total > 0 {
		s.CompletionPct = float64(s.Done) / float64(s.Total) * 100
	}
	return s
}

// printPlanTree prints the epic tree with dependency indicators
func printPlanTree(database *db.DB, parentID string, prefix string, childrenMap map[string][]model.Item, depInfo map[string][]db.DepStatus, readyTasks map[string]bool, isRoot bool) error {
	children := childrenMap[parentID]
	if len(children) == 0 {
		return nil
	}

	for i, child := range children {
		isLast := i == len(children)-1
		branch := "├──"
		if isLast {
			branch = "└──"
		}

		// Build status indicator
		statusIndicator := ""
		if readyTasks[child.ID] {
			statusIndicator = "✅ "
		} else if child.Status == model.StatusDone {
			statusIndicator = "✓ "
		} else if child.Status == model.StatusBlocked {
			statusIndicator = "⛔ "
		} else if child.Status == model.StatusInProgress {
			statusIndicator = "▶ "
		}

		// Check if this task has unmet dependencies
		hasDeps := len(depInfo[child.ID]) > 0
		depIndicator := ""
		if hasDeps {
			unmetCount := 0
			for _, dep := range depInfo[child.ID] {
				if dep.Status != string(model.StatusDone) {
					unmetCount++
				}
			}
			if unmetCount > 0 {
				depIndicator = fmt.Sprintf(" (%d deps)", unmetCount)
			}
		}

		fmt.Printf("%s%s %s%s [%s] %s%s\n", prefix, branch, statusIndicator, child.ID, child.Status, child.Title, depIndicator)

		// Recurse into children
		childPrefix := prefix
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}

		if err := printPlanTree(database, child.ID, childPrefix, childrenMap, depInfo, readyTasks, false); err != nil {
			return err
		}
	}

	return nil
}

// PlanJSON is the JSON output format for the plan command
type PlanJSON struct {
	Epic          EpicSummaryJSON    `json:"epic"`
	Stats         epicStats          `json:"stats"`
	Tasks         []PlanTaskJSON     `json:"tasks"`
	ReadyTasks    []string           `json:"ready_tasks"`
	BlockedChains []BlockedChainJSON `json:"blocked_chains,omitempty"`
}

// EpicSummaryJSON is a minimal epic representation
type EpicSummaryJSON struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
}

// PlanTaskJSON represents a task in the plan output
type PlanTaskJSON struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	Status       string           `json:"status"`
	Priority     int              `json:"priority"`
	ParentID     *string          `json:"parent_id,omitempty"`
	Labels       []string         `json:"labels,omitempty"`
	IsReady      bool             `json:"is_ready"`
	Dependencies []DepBlockerJSON `json:"dependencies,omitempty"`
}

// DepBlockerJSON represents a dependency that blocks a task
type DepBlockerJSON struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// BlockedChainJSON represents a chain of blocked tasks
type BlockedChainJSON struct {
	TaskID    string           `json:"task_id"`
	TaskTitle string           `json:"task_title"`
	BlockedBy []DepBlockerJSON `json:"blocked_by"`
}

// printPlanJSON outputs the plan as JSON
func printPlanJSON(epic *model.Item, descendants []model.Item, childrenMap map[string][]model.Item, depInfo map[string][]db.DepStatus, blockedBy map[string][]db.DepStatus, readyTasks map[string]bool, stats epicStats) error {
	output := PlanJSON{
		Epic: EpicSummaryJSON{
			ID:          epic.ID,
			Title:       epic.Title,
			Status:      string(epic.Status),
			Description: epic.Description,
		},
		Stats:      stats,
		ReadyTasks: []string{},
	}

	// Build task list
	for _, item := range descendants {
		isReady := readyTasks[item.ID]
		if isReady {
			output.ReadyTasks = append(output.ReadyTasks, item.ID)
		}

		task := PlanTaskJSON{
			ID:       item.ID,
			Title:    item.Title,
			Status:   string(item.Status),
			Priority: item.Priority,
			ParentID: item.ParentID,
			Labels:   item.Labels,
			IsReady:  isReady,
		}

		// Add dependencies
		for _, dep := range depInfo[item.ID] {
			task.Dependencies = append(task.Dependencies, DepBlockerJSON{
				ID:     dep.ID,
				Title:  dep.Title,
				Status: dep.Status,
			})
		}

		output.Tasks = append(output.Tasks, task)

		// Add to blocked chains if has unmet deps and not done/canceled
		if item.Status != model.StatusDone && item.Status != model.StatusCanceled {
			unmetDeps := []DepBlockerJSON{}
			for _, dep := range depInfo[item.ID] {
				if dep.Status != string(model.StatusDone) {
					unmetDeps = append(unmetDeps, DepBlockerJSON{
						ID:     dep.ID,
						Title:  dep.Title,
						Status: dep.Status,
					})
				}
			}
			if len(unmetDeps) > 0 {
				output.BlockedChains = append(output.BlockedChains, BlockedChainJSON{
					TaskID:    item.ID,
					TaskTitle: item.Title,
					BlockedBy: unmetDeps,
				})
			}
		}
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
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

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Show project health overview",
	Long: `Show a high-level health overview of the project.

Displays:
  - Total task count
  - Tasks by status (open, in_progress, blocked, done, canceled)
  - Ready count (tasks available to work on)
  - Epics in progress count
  - Stale tasks count (in-progress with no updates >5min)

Examples:
  tpg summary
  tpg summary -p myproject`,
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

		stats, err := database.GetSummaryStats(project)
		if err != nil {
			return err
		}

		printSummaryStats(stats)
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
	Use:   "edit [id...] [flags]",
	Short: "Edit task fields",
	Long: `Edit one or more tasks' fields.

SELECTION:
  Provide item IDs as arguments, or use --select-* flags to select items.
  Cannot mix explicit IDs with select flags.

FIELD CHANGES:
  --title, --desc        Single item only (opens editor if no field flags)
  --priority, --parent   Can apply to multiple items
  --add-label, --remove-label   Can apply to multiple items
  --status               Requires --force (prefer start/done/block/cancel commands)

For epic-specific fields (--context, --on-close), use 'tpg epic edit'.

Examples:
  tpg edit ts-abc                            # Open description in editor
  tpg edit ts-abc --title "New title"        # Change title
  tpg edit ts-abc --priority 1               # Set high priority
  tpg edit ts-abc ts-def --priority 2        # Set priority on multiple
  tpg edit ts-abc --parent ep-xyz            # Move under epic
  tpg edit ts-abc --parent ""                # Remove from parent
  tpg edit ts-abc --add-label bug            # Add label
  tpg edit --select-label bug --priority 1   # All items with 'bug' label
  tpg edit --select-epic ep-xyz --add-label done   # All descendants of epic
  tpg edit ts-abc --status open --force      # Force status change
  tpg edit ts-abc --dry-run --priority 1     # Preview changes`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		// Check if --parent was explicitly set (to distinguish "" from unset)
		flagEditParentSet = cmd.Flags().Changed("parent")

		// Determine if any select flags are set
		hasFilters := flagStatus != "" || flagListParent != "" || flagListType != "" ||
			flagListEpic != "" || len(flagFilterLabels) > 0

		// Validate: can't mix explicit IDs with select flags
		if len(args) > 0 && hasFilters {
			return fmt.Errorf("cannot use select flags with explicit item IDs")
		}

		// Collect items to edit
		var items []model.Item
		if hasFilters {
			// Use filters to find items
			filter := db.ListFilter{
				Parent: flagListParent,
				Type:   flagListType,
				Labels: flagFilterLabels,
			}
			if flagStatus != "" {
				s := model.Status(flagStatus)
				filter.Status = &s
			}
			items, err = database.ListItemsFiltered(filter)
			if err != nil {
				return fmt.Errorf("failed to query items: %w", err)
			}

			// Further filter by epic descendants if --select-epic is set
			if flagListEpic != "" {
				descendants, err := database.GetDescendants(flagListEpic)
				if err != nil {
					return fmt.Errorf("failed to get descendants of %s: %w", flagListEpic, err)
				}
				descendantIDs := make(map[string]bool)
				for _, d := range descendants {
					descendantIDs[d.ID] = true
				}
				filtered := make([]model.Item, 0)
				for _, item := range items {
					if descendantIDs[item.ID] {
						filtered = append(filtered, item)
					}
				}
				items = filtered
			}

			if len(items) == 0 {
				return fmt.Errorf("no items match the filter criteria")
			}
		} else if len(args) > 0 {
			// Use explicit IDs
			for _, id := range args {
				item, err := database.GetItem(id)
				if err != nil {
					return fmt.Errorf("item not found: %s", id)
				}
				items = append(items, *item)
			}
		} else {
			return fmt.Errorf("provide item IDs or use --select-* flags to select items")
		}

		// Check for single-item-only flags with multiple items
		if len(items) > 1 {
			if flagEditTitle != "" {
				return fmt.Errorf("--title can only be used with a single item (got %d)", len(items))
			}
			if flagEditDesc != "" {
				return fmt.Errorf("--desc can only be used with a single item (got %d)", len(items))
			}
		}

		// Check if --status requires --force
		if flagEditStatus != "" && !flagForce {
			return fmt.Errorf("--status requires --force (prefer: tpg start/done/block/cancel)")
		}

		// Check if any field flags are set
		hasFieldFlags := flagEditTitle != "" || flagEditPriority != 0 || flagEditParentSet ||
			len(flagEditAddLabels) > 0 || len(flagEditRmLabels) > 0 || flagEditDesc != "" ||
			flagEditStatus != ""

		// If no field flags and single item, open editor for description
		if !hasFieldFlags && len(items) == 1 {
			return editInEditor(database, items[0].ID)
		}

		// If no field flags and multiple items, error
		if !hasFieldFlags {
			return fmt.Errorf("no field flags specified for %d items (use --title, --priority, --parent, --add-label, --remove-label, --desc, or --status)", len(items))
		}

		// Read description from stdin if needed
		descValue := flagEditDesc
		if descValue == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			descValue = strings.TrimSpace(string(data))
		}

		// Dry run preview
		if flagDryRun {
			fmt.Printf("Would edit %d item(s):\n", len(items))
			for _, item := range items {
				fmt.Printf("  %s: %s\n", item.ID, item.Title)
			}
			fmt.Println("\nChanges:")
			if flagEditTitle != "" {
				fmt.Printf("  title: %q\n", flagEditTitle)
			}
			if flagEditPriority != 0 {
				fmt.Printf("  priority: %d\n", flagEditPriority)
			}
			if flagEditParentSet {
				if flagEditParent == "" {
					fmt.Println("  parent: (remove)")
				} else {
					fmt.Printf("  parent: %s\n", flagEditParent)
				}
			}
			for _, label := range flagEditAddLabels {
				fmt.Printf("  add label: %s\n", label)
			}
			for _, label := range flagEditRmLabels {
				fmt.Printf("  remove label: %s\n", label)
			}
			if descValue != "" {
				fmt.Printf("  description: (%d chars)\n", len(descValue))
			}
			if flagEditStatus != "" {
				fmt.Printf("  status: %s (forced)\n", flagEditStatus)
			}
			return nil
		}

		// Apply changes to all items
		for _, item := range items {
			if flagEditTitle != "" {
				if err := database.SetTitle(item.ID, flagEditTitle); err != nil {
					return fmt.Errorf("failed to set title for %s: %w", item.ID, err)
				}
			}
			if flagEditPriority != 0 {
				if err := database.UpdatePriority(item.ID, flagEditPriority); err != nil {
					return fmt.Errorf("failed to set priority for %s: %w", item.ID, err)
				}
			}
			if flagEditParentSet {
				if flagEditParent == "" {
					// Remove parent
					if err := database.ClearParent(item.ID); err != nil {
						return fmt.Errorf("failed to clear parent for %s: %w", item.ID, err)
					}
				} else {
					if err := database.SetParent(item.ID, flagEditParent); err != nil {
						return fmt.Errorf("failed to set parent for %s: %w", item.ID, err)
					}
				}
			}
			for _, label := range flagEditAddLabels {
				if err := database.AddLabelToItem(item.ID, item.Project, label); err != nil {
					return fmt.Errorf("failed to add label %q to %s: %w", label, item.ID, err)
				}
			}
			for _, label := range flagEditRmLabels {
				if err := database.RemoveLabelFromItem(item.ID, item.Project, label); err != nil {
					// Label might not exist, skip silently
					continue
				}
			}
			if descValue != "" {
				if err := database.SetDescription(item.ID, descValue); err != nil {
					return fmt.Errorf("failed to set description for %s: %w", item.ID, err)
				}
			}
			if flagEditStatus != "" {
				if err := database.UpdateStatus(item.ID, model.Status(flagEditStatus), db.AgentContext{}, false); err != nil {
					return fmt.Errorf("failed to set status for %s: %w", item.ID, err)
				}
			}
		}

		if len(items) == 1 {
			fmt.Printf("Updated %s\n", items[0].ID)
		} else {
			fmt.Printf("Updated %d items\n", len(items))
		}
		return nil
	},
}

// editInEditor opens the item's description in an external editor
func editInEditor(database *db.DB, id string) error {
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

// dep command - parent for dependency management subcommands
var depCmd = &cobra.Command{
	Use:   "dep <id> <action> [other-id]",
	Short: "Manage task dependencies",
	Long: `Manage dependencies between tasks.

Actions:
  blocks <other-id>     Mark this task as blocking another (other cannot start until this is done)
  after <other-id>      Mark this task as depending on another (this cannot start until other is done)
  list                  Show all dependencies for this task
  remove <other-id>     Remove a dependency relationship
  unblock <other-id>    Alias for remove (symmetric with blocks)

Understanding blocks vs after:

  tpg dep ts-a blocks ts-b    # ts-a must finish before ts-b can start
  tpg dep ts-b after ts-a     # Same thing, different perspective

  ts-a  ───►  ts-b
  (blocker)   (blocked)

The arrow shows execution order: the blocker must complete first.

Examples:
  tpg dep ts-a1b2c3 blocks ts-d4e5f6     # ts-d4e5f6 waits for ts-a1b2c3
  tpg dep ts-d4e5f6 after ts-a1b2c3      # same thing, other direction
  tpg dep ts-a1b2c3 list                  # show all deps for ts-a1b2c3
  tpg dep ts-a1b2c3 remove ts-d4e5f6     # remove dependency between them
  tpg dep ts-a1b2c3 unblock ts-d4e5f6    # same as remove`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		action := args[1]

		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		switch action {
		case "blocks":
			if len(args) < 3 {
				return fmt.Errorf("usage: tpg dep <id> blocks <other-id>")
			}
			otherID := args[2]
			// "A blocks B" means B depends on A
			if err := database.AddDep(otherID, id); err != nil {
				return err
			}
			fmt.Printf("%s now blocks %s\n", id, otherID)

		case "after":
			if len(args) < 3 {
				return fmt.Errorf("usage: tpg dep <id> after <other-id>")
			}
			otherID := args[2]
			// "A after B" means A depends on B
			if err := database.AddDep(id, otherID); err != nil {
				return err
			}
			fmt.Printf("%s now depends on %s\n", id, otherID)

		case "remove", "unblock":
			if len(args) < 3 {
				return fmt.Errorf("usage: tpg dep <id> remove <other-id>")
			}
			otherID := args[2]
			// Try both directions
			err1 := database.RemoveDep(id, otherID)
			err2 := database.RemoveDep(otherID, id)
			if err1 != nil && err2 != nil {
				return fmt.Errorf("no dependency found between %s and %s", id, otherID)
			}
			fmt.Printf("Removed dependency between %s and %s\n", id, otherID)

		case "list":
			// Show what this task depends on (including inherited deps)
			waitingOn, err := database.GetAllDepStatuses(id)
			if err != nil {
				return err
			}
			// Show what this task blocks
			blocking, err := database.GetBlockedBy(id)
			if err != nil {
				return err
			}

			if len(waitingOn) == 0 && len(blocking) == 0 {
				fmt.Printf("%s has no dependencies\n", id)
				return nil
			}

			if len(waitingOn) > 0 {
				fmt.Printf("Waiting on:\n")
				for _, dep := range waitingOn {
					fmt.Printf("  %s [%s] %s\n", dep.ID, dep.Status, dep.Title)
				}
			}
			if len(blocking) > 0 {
				fmt.Printf("Blocks:\n")
				for _, dep := range blocking {
					fmt.Printf("  %s [%s] %s\n", dep.ID, dep.Status, dep.Title)
				}
			}

		default:
			return fmt.Errorf("unknown action %q (use: blocks, after, list, remove)", action)
		}

		return nil
	},
}

// blocksCmd kept for backward compatibility
var blocksCmd = &cobra.Command{
	Use:        "blocks <id> <other-id>",
	Short:      "Mark a task as blocking another (deprecated: use 'tpg dep <id> blocks <other-id>')",
	Deprecated: "use 'tpg dep <id> blocks <other-id>' instead",
	Args:       cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

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
tpg ready                        # Find unblocked work
tpg start <id>                   # Claim work
tpg done <id>                    # Complete work
tpg dep <id> blocks <other-id>   # Set dependency
tpg dep <id> list                # Show dependencies

# Creating tasks — always use heredoc for full context:
tpg add "Title" -p 1 --desc - <<EOF
What to do, why it matters, constraints, acceptance criteria.
Future agents won't have your current context—be thorough.
EOF

# Logging progress — always use heredoc for detail:
tpg log <id> - <<EOF
Decisions made, alternatives considered, blockers found,
milestones reached. Skip routine actions (opened file, ran cmd).
EOF
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

	// Create .tpg/.gitignore to exclude db and backups
	if err := ensureGitignore(); err != nil {
		fmt.Printf("\nWarning: failed to update .gitignore: %v\n", err)
	}

	return nil
}

// ensureGitignore creates .tpg/.gitignore to exclude db and backups
func ensureGitignore() error {
	gitignorePath := filepath.Join(".tpg", ".gitignore")
	desired := "tpg.db\nbackups/\n"

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.WriteFile(gitignorePath, []byte(desired), 0644); err != nil {
				return err
			}
			fmt.Println("\nCreated .tpg/.gitignore")
			return nil
		}
		return err
	}

	// Check if both entries are present
	text := string(content)
	needsDB := !strings.Contains(text, "tpg.db")
	needsBackups := !strings.Contains(text, "backups/")

	if !needsDB && !needsBackups {
		return nil // Already has both
	}

	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if needsDB {
		text += "tpg.db\n"
	}
	if needsBackups {
		text += "backups/\n"
	}

	if err := os.WriteFile(gitignorePath, []byte(text), 0644); err != nil {
		return err
	}
	fmt.Println("\nUpdated .tpg/.gitignore")
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

		// File is unmodified, check if content actually changed
		newSourceHash := calculatePluginHash(source)
		if newSourceHash == oldHash {
			// Source content is identical, no update needed
			return false, true, false, nil
		}

		// Content changed - this is an upgrade
		fmt.Printf("\nUpgrading OpenCode plugin from %s to %s\n", oldVersion, version)
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

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check and fix data integrity issues",
	Long: `Scans for common data integrity issues like circular dependencies and offers to fix them.

Checks performed:
  1. Parent-child circular dependencies (epic depends on its child task)
  2. General circular dependencies (A depends on B depends on C depends on A)

Examples:
  tpg doctor              # Check and optionally fix issues
  tpg doctor --dry-run    # Show issues without fixing`,
	RunE: runDoctor,
}

func runDoctor(cmd *cobra.Command, args []string) error {
	database, err := openDB()
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()

	fmt.Println("🔍 Checking for data integrity issues...")
	fmt.Println()

	// Check 1: Parent-child circular dependencies
	fmt.Println("1. Checking for parent-child circular dependencies...")
	parentChildDeps, err := database.FindParentChildCircularDeps()
	if err != nil {
		return fmt.Errorf("failed to check parent-child deps: %w", err)
	}

	if len(parentChildDeps) == 0 {
		fmt.Println("   ✓ No parent-child circular dependencies found")
	} else {
		fmt.Printf("   ⚠️  Found %d parent-child circular dependencies:\n", len(parentChildDeps))
		for _, dep := range parentChildDeps {
			fmt.Printf("      - %s depends on %s (parent-child relationship)\n", dep.ParentID, dep.ChildID)
		}

		if !flagDoctorDryRun {
			fmt.Print("\n   Fix these dependencies? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) == "y" {
				fixed, err := database.FixAllParentChildCircularDeps()
				if err != nil {
					return fmt.Errorf("failed to fix deps: %w", err)
				}
				fmt.Printf("   ✓ Fixed %d dependencies\n", fixed)
			}
		} else {
			fmt.Println("\n   (dry-run mode - no changes made)")
		}
	}

	// Check 2: General circular dependencies
	fmt.Println("\n2. Checking for other circular dependencies...")
	circularDeps, err := database.FindCircularDeps()
	if err != nil {
		return fmt.Errorf("failed to check circular deps: %w", err)
	}

	if len(circularDeps) == 0 {
		fmt.Println("   ✓ No circular dependencies found")
	} else {
		fmt.Printf("   ⚠️  Found %d circular dependencies:\n", len(circularDeps))
		for _, dep := range circularDeps {
			fmt.Printf("      - Cycle: %s\n", strings.Join(dep.CyclePath, " -> "))
		}
		fmt.Println("\n   These must be fixed manually. Use 'tpg dep <id> remove <other-id>' to break the cycle.")
	}

	fmt.Println("\n✅ Doctor check complete!")
	return nil
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

		project, err := resolveProject()
		if err != nil {
			return err
		}

		return tui.Run(database, project)
	},
}

var impactCmd = &cobra.Command{
	Use:   "impact <id>",
	Short: "Show what tasks would become ready if this task is completed",
	Long: `Show the impact of completing a task — what tasks would become ready.

This command shows both direct and transitive effects. When task X is completed,
any task that depends only on X (and other tasks that would also become ready)
will be listed.

The output shows tasks organized by depth (distance from the original task):
  Depth 1: Tasks directly blocked by this task
  Depth 2+: Tasks blocked by those tasks, and so on

Examples:
  tpg impact ts-a1b2c3          # Show what completing ts-a1b2c3 would unblock
  tpg impact ts-a1b2c3 --json   # Output as JSON`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		itemID := args[0]
		impact, err := database.GetImpact(itemID)
		if err != nil {
			return err
		}

		if len(impact) == 0 {
			fmt.Println("No tasks would become ready")
			return nil
		}

		if flagContextJSON {
			return printImpactJSON(impact)
		}

		printImpact(impact, itemID)
		return nil
	},
}

var mergeCmd = &cobra.Command{
	Use:   "merge <source-id> <target-id>",
	Short: "Merge duplicate tasks (source into target)",
	Long: `Merge source task into target, combining all metadata.

This is a destructive operation — the source item is deleted after merging.
Requires --yes-i-am-sure to confirm.

What gets merged:
  - Dependencies (both directions) are transferred to target
  - Logs are moved to target with a merge note
  - Labels are copied to target
  - Source description is appended to target

Fails if merging would create a circular dependency.

Examples:
  tpg merge ts-abc ts-xyz --yes-i-am-sure   # merge ts-abc into ts-xyz`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !flagMergeConfirm {
			return fmt.Errorf("this permanently deletes the source item — pass --yes-i-am-sure to confirm")
		}

		sourceID := args[0]
		targetID := args[1]

		database, err := openDB()
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		// Show what will happen before merging
		src, err := database.GetItem(sourceID)
		if err != nil {
			return fmt.Errorf("source: %w", err)
		}
		tgt, err := database.GetItem(targetID)
		if err != nil {
			return fmt.Errorf("target: %w", err)
		}

		fmt.Printf("Merging %s (%s) → %s (%s)\n", sourceID, src.Title, targetID, tgt.Title)

		if err := database.MergeItems(sourceID, targetID); err != nil {
			return err
		}

		fmt.Printf("Merged. %s has been deleted.\n", sourceID)
		database.BackupQuiet()
		return nil
	},
}

var configCmd = &cobra.Command{
	Use:   "config [key] [value]",
	Short: "View or modify configuration",
	Long: `View or modify tpg configuration.

Without arguments: show all config values
With one argument: show specific config value
With two arguments: set config value

Examples:
  tpg config                              # Show all config
  tpg config prefixes.task                # Show task prefix
  tpg config prefixes.task ts             # Set task prefix to "ts"
  tpg config warnings.short_description false  # Disable warning
  tpg config warnings.min_description_words 20 # Set threshold`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := db.LoadConfig()
		if err != nil {
			return err
		}

		switch len(args) {
		case 0:
			// Show all config
			fields := db.GetConfigFields(config)
			for _, f := range fields {
				fmt.Printf("%s = %s\n", f.Path, db.FormatConfigValue(f.Value))
			}
		case 1:
			// Show specific value
			val, err := db.GetConfigField(config, args[0])
			if err != nil {
				return err
			}
			fmt.Println(db.FormatConfigValue(val))
		case 2:
			// Set value
			if err := db.SetConfigField(config, args[0], args[1]); err != nil {
				return err
			}
			if err := db.SaveConfig(config); err != nil {
				return err
			}
			fmt.Printf("Set %s = %s\n", args[0], args[1])
		default:
			return fmt.Errorf("too many arguments")
		}
		return nil
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&flagProject, "project", "", "Project scope")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Show agent context and other debug info")
	rootCmd.PersistentFlags().BoolVar(&flagFromYAML, "from-yaml", false, "Read flag values from stdin as YAML (keys use underscores, e.g. desc: value)")

	// Handle --from-yaml and show agent context when verbose
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Handle --from-yaml: read YAML from stdin and set flag values
		if flagFromYAML {
			// Check for conflicting '-' stdin markers on any flag
			if flagName := findStdinMarkerFlag(cmd); flagName != "" {
				return fmt.Errorf("cannot use --from-yaml with '-' stdin marker on --%s\n"+
					"Use YAML key '%s: |' in stdin instead of --%s -",
					flagName, strings.ReplaceAll(flagName, "-", "_"), flagName)
			}
			if err := applyYAMLFlags(cmd); err != nil {
				return err
			}
		}

		// Show agent context when verbose
		if flagVerbose {
			agentID := os.Getenv("AGENT_ID")
			agentType := os.Getenv("AGENT_TYPE")
			if agentID != "" || agentType != "" {
				fmt.Fprintf(os.Stderr, "[agent] ID=%s TYPE=%s\n", agentID, agentType)
			}
		}
		return nil
	}

	// add flags
	addCmd.Flags().IntVarP(&flagPriority, "priority", "p", 2, "Priority (1=high, 2=medium, 3=low)")
	addCmd.Flags().StringVar(&flagParent, "parent", "", "Parent epic ID")
	addCmd.Flags().StringVar(&flagBlocks, "blocks", "", "ID of task this will block (it depends on this)")
	addCmd.Flags().StringVar(&flagAfter, "after", "", "ID of task this depends on (must complete first)")
	addCmd.Flags().StringArrayVarP(&flagAddLabels, "label", "l", nil, "Label to attach (can be repeated)")
	addCmd.Flags().StringVar(&flagTemplateID, "template", "", "Template ID to instantiate")
	addCmd.Flags().StringArrayVar(&flagTemplateVars, "var", nil, "Template variable value (name=json-string)")
	addCmd.Flags().BoolVar(&flagTemplateVarsYAML, "vars-yaml", false, "Read template variables from stdin as YAML")
	addCmd.Flags().StringVar(&flagDescription, "desc", "", "Description (use '-' for stdin)")
	addCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Preview what would be created without actually creating")
	addCmd.Flags().StringVar(&flagType, "type", "", "Item type (default: task)")
	addCmd.Flags().StringVar(&flagPrefix, "prefix", "", "Custom ID prefix (overrides auto-generated prefix)")

	// init flags
	initCmd.Flags().StringVar(&flagInitTaskPrefix, "prefix", "", "Task ID prefix (default: ts)")
	initCmd.Flags().StringVar(&flagInitTaskPrefix, "task-prefix", "", "Task ID prefix (default: ts)")
	initCmd.Flags().StringVar(&flagInitEpicPrefix, "epic-prefix", "", "Epic ID prefix (default: ep)")

	// list flags
	listCmd.Flags().BoolVarP(&flagListAll, "all", "a", false, "Show all items including done and canceled (default: hide done/canceled)")
	listCmd.Flags().StringVar(&flagStatus, "status", "", "Filter by status (open, in_progress, blocked, done, canceled)")
	listCmd.Flags().StringVar(&flagListParent, "parent", "", "Filter by parent epic ID")
	listCmd.Flags().StringVar(&flagListType, "type", "", "Filter by item type (task, epic)")
	listCmd.Flags().StringVar(&flagListEpic, "epic", "", "Filter to descendants of this epic ID")
	listCmd.Flags().StringVar(&flagBlocking, "blocking", "", "Show items that block the given ID")
	listCmd.Flags().StringVar(&flagBlockedBy, "blocked-by", "", "Show items blocked by the given ID")
	listCmd.Flags().BoolVar(&flagHasBlockers, "has-blockers", false, "Show only items with unresolved blockers")
	listCmd.Flags().BoolVar(&flagNoBlockers, "no-blockers", false, "Show only items with no blockers")
	listCmd.Flags().BoolVar(&flagIdsOnly, "ids-only", false, "Output only IDs, one per line (pipe-friendly)")
	listCmd.Flags().BoolVarP(&flagListFlat, "flat", "f", false, "Show flat list instead of tree view")
	listCmd.Flags().StringArrayVarP(&flagFilterLabels, "label", "l", nil, "Filter by label (can be repeated, AND logic)")

	// merge flags
	mergeCmd.Flags().BoolVar(&flagMergeConfirm, "yes-i-am-sure", false, "Confirm destructive merge operation")

	// stale flags
	staleCmd.Flags().StringVar(&flagStaleThreshold, "threshold", "5m", "Threshold for stale in-progress tasks")

	// done flags
	doneCmd.Flags().BoolVar(&flagDoneOverride, "override", false, "Allow completion with unmet dependencies")

	// start flags
	startCmd.Flags().BoolVar(&flagResume, "resume", false, "Resume an already in-progress task")

	// onboard flags
	onboardCmd.Flags().BoolVar(&flagForce, "force", false, "Replace existing Task Tracking section")

	// edit flags - field setters
	editCmd.Flags().StringVar(&flagEditTitle, "title", "", "New title (single item only)")
	editCmd.Flags().IntVar(&flagEditPriority, "priority", 0, "New priority (1=high, 2=medium, 3=low)")
	editCmd.Flags().StringVar(&flagEditParent, "parent", "", "New parent epic ID (use \"\" to remove)")
	editCmd.Flags().StringArrayVar(&flagEditAddLabels, "add-label", nil, "Label to add (repeatable)")
	editCmd.Flags().StringArrayVar(&flagEditRmLabels, "remove-label", nil, "Label to remove (repeatable)")
	editCmd.Flags().StringVar(&flagEditDesc, "desc", "", "New description (single item only, use '-' for stdin)")
	editCmd.Flags().StringVar(&flagEditStatus, "status", "", "Force status change (requires --force)")

	// edit flags - selection filters (reuse list flag variables)
	editCmd.Flags().StringVar(&flagStatus, "select-status", "", "Select items by status")
	editCmd.Flags().StringVar(&flagListParent, "select-parent", "", "Select items by parent epic ID")
	editCmd.Flags().StringVar(&flagListType, "select-type", "", "Select items by item type")
	editCmd.Flags().StringVar(&flagListEpic, "select-epic", "", "Select descendants of epic")
	editCmd.Flags().StringArrayVarP(&flagFilterLabels, "select-label", "l", nil, "Select items by label (repeatable, AND logic)")

	// edit flags - control
	editCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Preview changes without applying")
	editCmd.Flags().BoolVar(&flagForce, "force", false, "Required for --status changes")

	// ready flags
	readyCmd.Flags().StringArrayVarP(&flagFilterLabels, "label", "l", nil, "Filter by label (can be repeated, AND logic)")
	readyCmd.Flags().StringVar(&flagReadyEpic, "epic", "", "Show ready tasks for a specific epic")

	// status flags
	statusCmd.Flags().BoolVar(&flagStatusAll, "all", false, "Show all ready tasks (default: limit to 10)")
	statusCmd.Flags().StringArrayVarP(&flagFilterLabels, "label", "l", nil, "Filter by label (can be repeated, AND logic)")

	// show flags
	showCmd.Flags().BoolVar(&flagShowWithChildren, "with-children", false, "Show task and all descendants")
	showCmd.Flags().BoolVar(&flagShowWithDeps, "with-deps", false, "Show full dependency chain (transitive)")
	showCmd.Flags().BoolVar(&flagShowWithParent, "with-parent", false, "Show parent chain up to root")
	showCmd.Flags().StringVar(&flagShowFormat, "format", "", "Output format (json, yaml, markdown)")
	showCmd.Flags().BoolVar(&flagShowVars, "vars", false, "Show raw template variables instead of rendered description")

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

	// impact flags
	impactCmd.Flags().BoolVar(&flagContextJSON, "json", false, "Output as JSON")

	// plan flags
	planCmd.Flags().BoolVar(&flagContextJSON, "json", false, "Output as JSON")

	// clean flags
	cleanCmd.Flags().BoolVar(&flagCleanDone, "done", false, "Remove done tasks older than N days")
	cleanCmd.Flags().BoolVar(&flagCleanCanceled, "canceled", false, "Remove canceled tasks older than N days")
	cleanCmd.Flags().BoolVar(&flagCleanLogs, "logs", false, "Remove orphaned logs")
	cleanCmd.Flags().BoolVar(&flagCleanVacuum, "vacuum", false, "Run SQLite VACUUM to compact database")
	cleanCmd.Flags().BoolVar(&flagCleanAll, "all", false, "Do all cleanup (done + canceled + vacuum)")
	cleanCmd.Flags().IntVar(&flagCleanDays, "days", 30, "Age threshold in days")
	cleanCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Show what would be deleted without actually deleting")
	cleanCmd.Flags().BoolVar(&flagForce, "force", false, "Skip confirmation prompt")

	// delete flags
	deleteCmd.Flags().BoolVar(&flagDeleteForce, "force", false, "Delete even if tasks depend on this item")
	// cancel flags
	cancelCmd.Flags().BoolVar(&flagCancelForce, "force", false, "Cancel even if tasks depend on this item")

	rootCmd.AddCommand(initCmd)

	// epic subcommands and flags
	epicWorktreeCmd.Flags().StringVar(&flagWorktreeBranch, "branch", "", "Custom branch name (default: auto-generated)")
	epicWorktreeCmd.Flags().StringVar(&flagWorktreeBase, "base", "", "Base branch (default: parent worktree branch or current branch)")
	epicWorktreeCmd.Flags().BoolVar(&flagWorktreeAllow, "allow-any-branch", false, "Allow branch names that do not include the epic ID")

	// epicAddCmd flags
	epicAddCmd.Flags().IntVarP(&flagPriority, "priority", "p", 2, "Priority (1=high, 2=medium, 3=low)")
	epicAddCmd.Flags().StringVar(&flagParent, "parent", "", "Parent epic ID")
	epicAddCmd.Flags().StringArrayVarP(&flagAddLabels, "label", "l", nil, "Label to attach (can be repeated)")
	epicAddCmd.Flags().StringVar(&flagDescription, "desc", "", "Description (use '-' for stdin)")
	epicAddCmd.Flags().StringVar(&flagPrefix, "prefix", "", "Custom ID prefix (overrides auto-generated prefix)")
	epicAddCmd.Flags().StringVar(&flagContext, "context", "", "Context shared with all descendants (use '-' for stdin)")
	epicAddCmd.Flags().StringVar(&flagOnClose, "on-close", "", "Instructions shown when epic auto-completes (use '-' for stdin)")
	epicAddCmd.Flags().BoolVar(&flagWorktree, "worktree", false, "Create epic with worktree metadata (generates branch name)")
	epicAddCmd.Flags().StringVar(&flagWorktreeBranch, "branch", "", "Custom branch name for worktree (default: auto-generated)")
	epicAddCmd.Flags().StringVar(&flagWorktreeBase, "base", "", "Base branch for worktree (default: parent worktree branch or current branch)")
	epicAddCmd.Flags().BoolVar(&flagWorktreeAllow, "allow-any-branch", false, "Allow branch names that do not include the epic ID")

	// epicEditCmd flags
	epicEditCmd.Flags().StringVar(&flagEditTitle, "title", "", "New title for the epic")
	epicEditCmd.Flags().StringVar(&flagContext, "context", "", "Context shared with all descendants (use '-' for stdin)")
	epicEditCmd.Flags().StringVar(&flagOnClose, "on-close", "", "Instructions shown when epic auto-completes (use '-' for stdin)")

	// epicReplaceCmd flags
	epicReplaceCmd.Flags().IntVarP(&flagPriority, "priority", "p", 2, "Priority (1=high, 2=medium, 3=low)")
	epicReplaceCmd.Flags().StringArrayVarP(&flagAddLabels, "label", "l", nil, "Label to attach (can be repeated)")
	epicReplaceCmd.Flags().StringVar(&flagDescription, "desc", "", "Description (use '-' for stdin)")
	epicReplaceCmd.Flags().StringVar(&flagPrefix, "prefix", "", "Custom ID prefix (overrides auto-generated prefix)")
	epicReplaceCmd.Flags().StringVar(&flagContext, "context", "", "Context shared with all descendants (use '-' for stdin)")
	epicReplaceCmd.Flags().StringVar(&flagOnClose, "on-close", "", "Instructions shown when epic auto-completes (use '-' for stdin)")

	epicCmd.AddCommand(epicAddCmd)
	epicCmd.AddCommand(epicEditCmd)
	epicCmd.AddCommand(epicListCmd)
	epicCmd.AddCommand(epicReplaceCmd)
	epicCmd.AddCommand(epicWorktreeCmd)
	epicCmd.AddCommand(epicFinishCmd)
	rootCmd.AddCommand(epicCmd)

	rootCmd.AddCommand(addCmd)

	// replace flags (subset of add flags that make sense for replacement)
	replaceCmd.Flags().BoolVarP(&flagEpic, "epic", "e", false, "Replace with an epic instead of a task")
	replaceCmd.Flags().IntVarP(&flagPriority, "priority", "p", 2, "Priority (1=high, 2=medium, 3=low)")
	replaceCmd.Flags().StringArrayVarP(&flagAddLabels, "label", "l", nil, "Label to attach (can be repeated)")
	replaceCmd.Flags().StringVar(&flagDescription, "desc", "", "Description (use '-' for stdin)")
	replaceCmd.Flags().StringVar(&flagType, "type", "", "Item type (default: task, or epic if -e flag used)")
	replaceCmd.Flags().StringVar(&flagPrefix, "prefix", "", "Custom ID prefix (overrides auto-generated prefix)")
	replaceCmd.Flags().StringVar(&flagContext, "context", "", "Context shared with all descendants (use '-' for stdin, epics only)")
	replaceCmd.Flags().StringVar(&flagOnClose, "on-close", "", "Instructions shown when epic auto-completes (use '-' for stdin)")
	rootCmd.AddCommand(replaceCmd)

	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(readyCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(doneCmd)
	rootCmd.AddCommand(cancelCmd)
	rootCmd.AddCommand(reopenCmd)
	blockCmd.Flags().BoolVar(&flagBlockForce, "force", false, "Force manual block (prefer dependencies instead)")
	rootCmd.AddCommand(blockCmd)
	rootCmd.AddCommand(staleCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(summaryCmd)
	rootCmd.AddCommand(projectsCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(appendCmd)
	rootCmd.AddCommand(descCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(depCmd)
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
	rootCmd.AddCommand(mergeCmd)
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(backupsCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(impactCmd)
	rootCmd.AddCommand(configCmd)

	// doctor flags
	doctorCmd.Flags().BoolVar(&flagDoctorDryRun, "dry-run", false, "Show issues without fixing")
	rootCmd.AddCommand(doctorCmd)

	// Import subcommands
	importCmd.AddCommand(importBeadsCmd)
	rootCmd.AddCommand(importCmd)

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

func printItemsIDs(items []model.Item) {
	for _, item := range items {
		fmt.Println(item.ID)
	}
}

func printItemsTable(items []model.Item) {
	if len(items) == 0 {
		fmt.Println("No items")
		return
	}

	now := time.Now()
	fmt.Printf("%-12s %-12s %-4s %s\n", "ID", "STATUS", "PRI", "TITLE")
	for _, item := range items {
		title := item.Title
		if len(item.Labels) > 0 {
			title = formatLabels(item.Labels) + " " + title
		}
		status := format.StatusDisplay(item, now)
		// Add ⚠ prefix for stale items
		if format.IsStale(item, now) {
			title = "⚠ " + title
		}
		fmt.Printf("%-12s %-12s %-4d %s\n", item.ID, status, item.Priority, title)
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

// treeNode represents an item in the hierarchical tree view.
type treeNode struct {
	Item        model.Item
	Level       int
	HasChildren bool
	IsLastChild bool
	// ParentLasts tracks whether each ancestor level was a last child.
	// Used to determine whether to draw │ or space for indentation.
	ParentLasts []bool
}

// buildTreeNodes constructs a hierarchical tree from items.
// Returns flattened list with level information, all nodes expanded.
func buildTreeNodes(items []model.Item) []treeNode {
	// Create a map of all items for quick lookup
	itemMap := make(map[string]model.Item)
	for _, item := range items {
		itemMap[item.ID] = item
	}

	// Create a map of parent -> children relationships
	childrenMap := make(map[string][]model.Item)
	for _, item := range items {
		if item.ParentID != nil {
			childrenMap[*item.ParentID] = append(childrenMap[*item.ParentID], item)
		}
	}

	var nodes []treeNode

	// Find root items (no parent or parent not in filtered list)
	for _, item := range items {
		isRoot := item.ParentID == nil
		if item.ParentID != nil {
			if _, hasParent := itemMap[*item.ParentID]; !hasParent {
				isRoot = true // Parent not in filtered list, treat as root
			}
		}

		if isRoot {
			nodes = append(nodes, treeNode{
				Item:        item,
				Level:       0,
				HasChildren: len(childrenMap[item.ID]) > 0,
			})
			// Recursively add all children (always expanded in CLI)
			nodes = append(nodes, getChildNodes(item.ID, 1, childrenMap, nil)...)
		}
	}

	return nodes
}

// getChildNodes recursively gets child nodes for a parent.
func getChildNodes(parentID string, level int, childrenMap map[string][]model.Item, parentLasts []bool) []treeNode {
	var nodes []treeNode
	children := childrenMap[parentID]

	for i, child := range children {
		isLast := i == len(children)-1
		nodes = append(nodes, treeNode{
			Item:        child,
			Level:       level,
			HasChildren: len(childrenMap[child.ID]) > 0,
			IsLastChild: isLast,
			ParentLasts: parentLasts,
		})
		// Recursively add grandchildren, passing down current node's last-child status
		childParentLasts := append(append([]bool{}, parentLasts...), isLast)
		nodes = append(nodes, getChildNodes(child.ID, level+1, childrenMap, childParentLasts)...)
	}

	return nodes
}

// buildTreePrefix creates the indentation and branch indicators for a tree node.
func buildTreePrefix(node treeNode) string {
	if node.Level == 0 {
		// Root level - indicator only
		if node.HasChildren {
			return "▼ "
		}
		return "○ "
	}

	// Build indentation based on level, using ParentLasts to determine │ vs space
	prefix := ""
	for i := 0; i < node.Level-1; i++ {
		if i < len(node.ParentLasts) && node.ParentLasts[i] {
			prefix += "   " // Parent was last child, no line continuation
		} else {
			prefix += "│  "
		}
	}

	// Add branch connector
	if node.IsLastChild {
		prefix += "└─ "
	} else {
		prefix += "├─ "
	}

	// Add indicator for nodes with children
	if node.HasChildren {
		prefix += "▼ "
	} else {
		prefix += "○ "
	}

	return prefix
}

func printItemsTree(items []model.Item) {
	if len(items) == 0 {
		fmt.Println("No items")
		return
	}

	nodes := buildTreeNodes(items)
	now := time.Now()

	fmt.Printf("%-12s %-12s %-4s %s\n", "ID", "STATUS", "PRI", "TITLE")
	for _, node := range nodes {
		title := node.Item.Title
		if len(node.Item.Labels) > 0 {
			title = formatLabels(node.Item.Labels) + " " + title
		}
		prefix := buildTreePrefix(node)
		status := format.StatusDisplay(node.Item, now)
		// Add ⚠ prefix for stale items
		if format.IsStale(node.Item, now) {
			title = "⚠ " + title
		}
		fmt.Printf("%-12s %-12s %-4d %s%s\n", node.Item.ID, status, node.Item.Priority, prefix, title)
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

// autoCompleteParentEpics recursively checks and completes parent epics when all children are done.
func autoCompleteParentEpics(database *db.DB, itemID string) error {
	for {
		info, err := database.CheckParentEpicCompletion(itemID)
		if err != nil {
			return err
		}
		if info == nil {
			return nil // No more parents to complete
		}

		epic := info.Epic

		// Show closing instructions if any
		hasInstructions := info.ClosingInstructions != "" || info.WorktreeBranch != ""

		if hasInstructions {
			fmt.Printf("\n─── Epic %s: %s ───\n", epic.ID, epic.Title)
			fmt.Println("All child tasks completed. Before closing this epic:")
		}

		if info.ClosingInstructions != "" {
			fmt.Printf("\n%s\n", info.ClosingInstructions)
		}

		if info.WorktreeBranch != "" {
			base := info.WorktreeBase
			if base == "" {
				base = "main"
			}
			fmt.Printf(`
Worktree cleanup:
  1. Review and commit any remaining changes in the worktree
  2. Push the branch: git push -u origin %s
  3. Create a pull request to merge into %s
  4. After merge, remove the worktree: git worktree remove <path>
  5. Delete the branch if no longer needed: git branch -d %s

Note: If AGENTS.md has specific merge instructions for this project, follow those instead.
`, info.WorktreeBranch, base, info.WorktreeBranch)
		}

		// Auto-complete the epic
		if err := database.AutoCompleteEpic(epic.ID); err != nil {
			return fmt.Errorf("failed to auto-complete epic %s: %w", epic.ID, err)
		}
		_ = database.AddLog(epic.ID, "Auto-completed (all children done)")

		fmt.Printf("\nAuto-completed epic %s: %s\n", epic.ID, epic.Title)

		// Continue up the chain to check grandparent epics
		itemID = epic.ID
	}
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

// ItemJSON is the JSON serialization format for items.
type ItemJSON struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	Project        string            `json:"project"`
	Title          string            `json:"title"`
	Description    string            `json:"description,omitempty"`
	Status         string            `json:"status"`
	Priority       int               `json:"priority"`
	ParentID       *string           `json:"parent_id,omitempty"`
	Labels         []string          `json:"labels,omitempty"`
	TemplateID     string            `json:"template_id,omitempty"`
	StepIndex      *int              `json:"step_index,omitempty"`
	TemplateVars   map[string]string `json:"template_vars,omitempty"`
	Results        string            `json:"results,omitempty"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
	AgentID        *string           `json:"agent_id,omitempty"`
	Logs           []LogJSON         `json:"logs,omitempty"`
	Dependencies   []string          `json:"dependencies,omitempty"`
	Blockers       []BlockerJSON     `json:"blockers,omitempty"`
	LatestProgress *LogJSON          `json:"latest_progress,omitempty"`
	Concepts       []string          `json:"suggested_concepts,omitempty"`
	Children       []ItemSummaryJSON `json:"children,omitempty"`
	ParentChain    []ItemSummaryJSON `json:"parent_chain,omitempty"`
	DepChain       []DepEdgeJSON     `json:"dependency_chain,omitempty"`
}

// LogJSON represents a log entry in JSON format.
type LogJSON struct {
	ID        string `json:"id"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

// BlockerJSON represents a blocker in JSON format.
type BlockerJSON struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// ItemSummaryJSON is a minimal item representation for chains.
type ItemSummaryJSON struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// DepEdgeJSON represents a dependency edge in JSON format.
type DepEdgeJSON struct {
	ItemID          string `json:"item_id"`
	ItemStatus      string `json:"item_status"`
	DependsOnID     string `json:"depends_on_id"`
	DependsOnStatus string `json:"depends_on_status"`
}

func printItemDetail(item *model.Item, logs []model.Log, deps []string, blockers []db.DepStatus, latestProgress *model.Log, concepts []model.Concept, templateNotice string, showVars bool, worktreeInfo *WorktreeInfo, epicPath []model.Item, sharedContext []db.SharedContextEntry) {
	fmt.Printf("ID:          %s\n", item.ID)
	fmt.Printf("Type:        %s\n", item.Type)
	fmt.Printf("Project:     %s\n", item.Project)
	fmt.Printf("Title:       %s\n", item.Title)
	now := time.Now()
	status := format.StatusDisplay(*item, now)
	if format.IsStale(*item, now) {
		fmt.Printf("Status:      %s [STALE]\n", status)
	} else {
		fmt.Printf("Status:      %s\n", status)
	}
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

	// Display worktree information if applicable
	if worktreeInfo != nil {
		fmt.Printf("\nWorktree:\n")
		fmt.Printf("  Epic:     %s - %s\n", worktreeInfo.EpicID, worktreeInfo.EpicTitle)
		fmt.Printf("  Branch:   %s\n", worktreeInfo.Branch)
		if worktreeInfo.Base != "" {
			fmt.Printf("  Base:     %s\n", worktreeInfo.Base)
		}
		fmt.Printf("  Location: %s\n", worktreeInfo.Location)
		fmt.Printf("  Status:   %s\n", worktreeStatusText(worktreeInfo))

		// Show path from epic to this item
		if len(epicPath) > 1 {
			fmt.Printf("  Path:     ")
			for i, pathItem := range epicPath {
				if i > 0 {
					fmt.Print(" -> ")
				}
				fmt.Printf("%s \"%s\"", pathItem.ID, pathItem.Title)
			}
			fmt.Println()
		}

		base := worktreeInfo.Base
		if base == "" {
			base = "main"
		}
		if !worktreeInfo.Exists {
			fmt.Printf("\n  To create worktree:\n")
			fmt.Printf("    git worktree add -b %s %s %s\n", worktreeInfo.Branch, worktreeInfo.Location, base)
			fmt.Printf("    cd %s\n", worktreeInfo.Location)
		} else if !worktreeInfo.InWorktree {
			fmt.Printf("\n  To work in the correct directory:\n")
			fmt.Printf("    cd %s\n", worktreeInfo.Location)
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

	// Show shared context from ancestor epics
	if len(sharedContext) > 0 {
		fmt.Printf("\nShared Context (from parent epics):\n")
		for _, entry := range sharedContext {
			fmt.Printf("── %s: %s ──\n", entry.EpicID, entry.EpicTitle)
			fmt.Printf("%s\n", entry.SharedContext)
		}
	}

	// Show template variables only with --vars flag, otherwise show description
	if showVars && item.TemplateID != "" && len(item.TemplateVars) > 0 {
		fmt.Printf("\nTemplate Variables:\n")
		for key, value := range item.TemplateVars {
			fmt.Printf("  %s:\n%s\n", key, indentLines(value, "    "))
		}
	} else if item.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", item.Description)
	}

	if len(deps) > 0 {
		fmt.Printf("\nDependencies:\n")
		for _, dep := range deps {
			fmt.Printf("  - %s\n", dep)
		}
	}

	if len(logs) == 0 {
		fmt.Printf("\nLogs: 0 entries\n")
	} else {
		logLimit := 50
		displayLogs := logs
		truncated := 0
		if len(logs) > logLimit {
			displayLogs = logs[len(logs)-logLimit:]
			truncated = len(logs) - logLimit
		}
		if len(logs) == 1 {
			fmt.Printf("\nLogs: 1 entry\n")
		} else {
			fmt.Printf("\nLogs: %d entries\n", len(logs))
		}
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

// ShowData represents all data for structured output formats
type ShowData struct {
	Item           *model.Item     `json:"item" yaml:"item"`
	Logs           []model.Log     `json:"logs" yaml:"logs"`
	Dependencies   []string        `json:"dependencies" yaml:"dependencies"`
	Blockers       []db.DepStatus  `json:"blockers" yaml:"blockers"`
	LatestProgress *model.Log      `json:"latest_progress,omitempty" yaml:"latest_progress,omitempty"`
	Concepts       []model.Concept `json:"concepts,omitempty" yaml:"concepts,omitempty"`
	TemplateNotice string          `json:"template_notice,omitempty" yaml:"template_notice,omitempty"`
	Children       []model.Item    `json:"children,omitempty" yaml:"children,omitempty"`
	ParentChain    []model.Item    `json:"parent_chain,omitempty" yaml:"parent_chain,omitempty"`
	DepChain       []db.DepEdge    `json:"dependency_chain,omitempty" yaml:"dependency_chain,omitempty"`
	Worktree       *WorktreeInfo   `json:"worktree,omitempty" yaml:"worktree,omitempty"`
}

// WorktreeInfo represents worktree context for an item
type WorktreeInfo struct {
	EpicID     string   `json:"epic_id" yaml:"epic_id"`
	EpicTitle  string   `json:"epic_title" yaml:"epic_title"`
	Branch     string   `json:"branch" yaml:"branch"`
	Base       string   `json:"base" yaml:"base"`
	Location   string   `json:"location" yaml:"location"`
	Exists     bool     `json:"exists" yaml:"exists"`
	InWorktree bool     `json:"in_worktree" yaml:"in_worktree"`
	Path       []string `json:"path,omitempty" yaml:"path,omitempty"`
}

func printItemJSON(item *model.Item, logs []model.Log, deps []string, blockers []db.DepStatus, latestProgress *model.Log, concepts []model.Concept, templateNotice string, children []model.Item, parentChain []model.Item, depChain []db.DepEdge, worktreeInfo *WorktreeInfo) error {
	data := ShowData{
		Item:           item,
		Logs:           logs,
		Dependencies:   deps,
		Blockers:       blockers,
		LatestProgress: latestProgress,
		Concepts:       concepts,
		TemplateNotice: templateNotice,
		Children:       children,
		ParentChain:    parentChain,
		DepChain:       depChain,
	}

	if worktreeInfo != nil {
		data.Worktree = worktreeInfo
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func printItemYAML(item *model.Item, logs []model.Log, deps []string, blockers []db.DepStatus, latestProgress *model.Log, concepts []model.Concept, templateNotice string, children []model.Item, parentChain []model.Item, depChain []db.DepEdge, worktreeInfo *WorktreeInfo) error {
	data := ShowData{
		Item:           item,
		Logs:           logs,
		Dependencies:   deps,
		Blockers:       blockers,
		LatestProgress: latestProgress,
		Concepts:       concepts,
		TemplateNotice: templateNotice,
		Children:       children,
		ParentChain:    parentChain,
		DepChain:       depChain,
	}

	if worktreeInfo != nil {
		data.Worktree = worktreeInfo
	}

	// Simple YAML output - in production would use a YAML library
	fmt.Printf("item:\n")
	fmt.Printf("  id: %s\n", item.ID)
	fmt.Printf("  type: %s\n", item.Type)
	fmt.Printf("  project: %s\n", item.Project)
	fmt.Printf("  title: %q\n", item.Title)
	fmt.Printf("  status: %s\n", item.Status)
	fmt.Printf("  priority: %d\n", item.Priority)
	if item.ParentID != nil {
		fmt.Printf("  parent_id: %s\n", *item.ParentID)
	}
	if len(item.Labels) > 0 {
		fmt.Printf("  labels: [%s]\n", strings.Join(item.Labels, ", "))
	}
	if item.Description != "" {
		fmt.Printf("  description: |\n%s\n", indentLines(item.Description, "    "))
	}
	if len(deps) > 0 {
		fmt.Printf("dependencies:\n")
		for _, d := range deps {
			fmt.Printf("  - %s\n", d)
		}
	}
	if len(logs) > 0 {
		fmt.Printf("logs:\n")
		for _, log := range logs {
			fmt.Printf("  - time: %s\n", log.CreatedAt.Format(time.RFC3339))
			fmt.Printf("    message: %q\n", log.Message)
		}
	}
	if len(children) > 0 {
		fmt.Printf("children:\n")
		for _, child := range children {
			fmt.Printf("  - id: %s\n    title: %q\n    status: %s\n", child.ID, child.Title, child.Status)
		}
	}
	if len(parentChain) > 0 {
		fmt.Printf("parent_chain:\n")
		for _, parent := range parentChain {
			fmt.Printf("  - id: %s\n    title: %q\n    status: %s\n", parent.ID, parent.Title, parent.Status)
		}
	}
	if len(depChain) > 0 {
		fmt.Printf("dependency_chain:\n")
		for _, edge := range depChain {
			fmt.Printf("  - item: %s\n    depends_on: %s\n    status: %s\n", edge.ItemID, edge.DependsOnID, edge.DependsOnStatus)
		}
	}
	if data.Worktree != nil {
		fmt.Printf("worktree:\n")
		fmt.Printf("  epic_id: %s\n", data.Worktree.EpicID)
		fmt.Printf("  epic_title: %q\n", data.Worktree.EpicTitle)
		fmt.Printf("  branch: %s\n", data.Worktree.Branch)
		fmt.Printf("  base: %s\n", data.Worktree.Base)
		fmt.Printf("  location: %s\n", data.Worktree.Location)
		fmt.Printf("  exists: %v\n", data.Worktree.Exists)
		fmt.Printf("  in_worktree: %v\n", data.Worktree.InWorktree)
		if len(data.Worktree.Path) > 0 {
			fmt.Printf("  path: [%s]\n", strings.Join(data.Worktree.Path, ", "))
		}
	}
	return nil
}

func printItemMarkdown(item *model.Item, logs []model.Log, deps []string, blockers []db.DepStatus, latestProgress *model.Log, concepts []model.Concept, templateNotice string, children []model.Item, parentChain []model.Item, depChain []db.DepEdge, worktreeInfo *WorktreeInfo) error {
	fmt.Printf("# %s\n\n", item.Title)
	fmt.Printf("**ID:** %s  \n", item.ID)
	fmt.Printf("**Type:** %s  \n", item.Type)
	fmt.Printf("**Project:** %s  \n", item.Project)
	fmt.Printf("**Status:** %s  \n", item.Status)
	fmt.Printf("**Priority:** %d  \n", item.Priority)
	if item.ParentID != nil {
		fmt.Printf("**Parent:** %s  \n", *item.ParentID)
	}
	if len(item.Labels) > 0 {
		fmt.Printf("**Labels:** %s  \n", strings.Join(item.Labels, ", "))
	}
	fmt.Println()

	if item.Description != "" {
		fmt.Printf("## Description\n\n%s\n\n", item.Description)
	}

	if latestProgress != nil {
		fmt.Printf("## Latest Progress\n\n")
		fmt.Printf("**[%s]** %s\n\n", latestProgress.CreatedAt.Format("2006-01-02 15:04"), latestProgress.Message)
	}

	if len(blockers) > 0 {
		fmt.Printf("## Blockers\n\n")
		for _, b := range blockers {
			fmt.Printf("- **%s** [%s] %s\n", b.ID, b.Status, b.Title)
		}
		fmt.Println()
	}

	if len(deps) > 0 {
		fmt.Printf("## Dependencies\n\n")
		for _, d := range deps {
			fmt.Printf("- %s\n", d)
		}
		fmt.Println()
	}

	if len(parentChain) > 0 {
		fmt.Printf("## Parent Chain\n\n")
		for _, parent := range parentChain {
			fmt.Printf("- **%s** [%s] %s\n", parent.ID, parent.Status, parent.Title)
		}
		fmt.Println()
	}

	if len(children) > 0 {
		fmt.Printf("## Children\n\n")
		for _, child := range children {
			fmt.Printf("- **%s** [%s] %s\n", child.ID, child.Status, child.Title)
		}
		fmt.Println()
	}

	if len(depChain) > 0 {
		fmt.Printf("## Dependency Chain\n\n")
		for _, edge := range depChain {
			fmt.Printf("- %s → %s [%s]\n", edge.ItemID, edge.DependsOnID, edge.DependsOnStatus)
		}
		fmt.Println()
	}

	if worktreeInfo != nil {
		fmt.Printf("## Worktree\n\n")
		fmt.Printf("**Epic:** %s - %s  \n", worktreeInfo.EpicID, worktreeInfo.EpicTitle)
		fmt.Printf("**Branch:** %s  \n", worktreeInfo.Branch)
		if worktreeInfo.Base != "" {
			fmt.Printf("**Base:** %s  \n", worktreeInfo.Base)
		}
		fmt.Printf("**Location:** %s  \n", worktreeInfo.Location)
		fmt.Printf("**Status:** %s  \n", worktreeStatusText(worktreeInfo))
		if len(worktreeInfo.Path) > 1 {
			fmt.Printf("**Path:** %s\n", strings.Join(worktreeInfo.Path, " → "))
		}
		fmt.Println()
	}

	if len(logs) > 0 {
		fmt.Printf("## Logs\n\n")
		for _, log := range logs {
			fmt.Printf("- **%s** %s\n", log.CreatedAt.Format("2006-01-02 15:04"), log.Message)
		}
		fmt.Println()
	}

	if len(concepts) > 0 {
		fmt.Printf("## Suggested Context\n\n")
		for _, c := range concepts {
			summary := c.Summary
			if summary == "" {
				summary = "(no summary)"
			}
			fmt.Printf("- **%s** (%d learnings) - %s\n", c.Name, c.LearningCount, summary)
		}
		fmt.Println()
	}

	if templateNotice != "" {
		fmt.Printf("> **Note:** %s\n\n", templateNotice)
	}

	return nil
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

func printSummaryStats(stats *db.SummaryStats) {
	project := stats.Project
	if project == "" {
		project = "(all)"
	}
	fmt.Printf("Project: %s\n\n", project)

	fmt.Printf("Total tasks: %d\n\n", stats.Total)

	fmt.Println("By status:")
	fmt.Printf("  Open:        %d\n", stats.Open)
	fmt.Printf("  In Progress: %d\n", stats.InProgress)
	fmt.Printf("  Blocked:     %d\n", stats.Blocked)
	fmt.Printf("  Done:        %d\n", stats.Done)
	fmt.Printf("  Canceled:    %d\n", stats.Canceled)
	fmt.Println()

	fmt.Printf("Ready to work: %d\n", stats.Ready)
	fmt.Printf("Epics in progress: %d\n", stats.EpicsInProgress)
	if stats.Stale > 0 {
		fmt.Printf("⚠️  Stale tasks: %d\n", stats.Stale)
	} else {
		fmt.Printf("Stale tasks: %d\n", stats.Stale)
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

// printImpact displays the impact analysis in a human-readable format.
func printImpact(items []db.ImpactItem, sourceID string) {
	fmt.Printf("Completing %s would make %d task(s) ready:\n\n", sourceID, len(items))

	currentDepth := 0
	for _, item := range items {
		if item.Depth != currentDepth {
			currentDepth = item.Depth
			fmt.Printf("\nDepth %d (via chain of %d completed task(s)):\n", currentDepth, currentDepth)
		}
		fmt.Printf("  %s [pri %d] %s\n", item.ID, item.Priority, item.Title)
	}
}

// ImpactJSON is the JSON serialization format for impact items.
type ImpactJSON struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Priority int    `json:"priority"`
	Depth    int    `json:"depth"`
}

func printImpactJSON(items []db.ImpactItem) error {
	output := make([]ImpactJSON, 0, len(items))
	for _, item := range items {
		output = append(output, ImpactJSON{
			ID:       item.ID,
			Title:    item.Title,
			Priority: item.Priority,
			Depth:    item.Depth,
		})
	}
	b, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(b))
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
