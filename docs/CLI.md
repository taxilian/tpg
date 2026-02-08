# CLI Reference

## Core Commands

| Command | Description |
|---------|-------------|
| `tpg init` | Initialize the database |
| `tpg onboard` | Set up tpg integration for Opencode |
| `tpg add <title>` | Create a work item (returns ID) |
| `tpg epic add <title>` | Create an epic (see Epics section) |
| `tpg list` | List all tasks |
| `tpg list --ids-only` | Output just IDs (useful for scripting) |
| `tpg show <id>` | Show task details, logs, deps, suggested concepts |
| `tpg ready` | Show tasks ready for work (open + deps met), with epic counts |
| `tpg ready --epic <id>` | Show ready tasks filtered by epic |
| `tpg stale` | List in_progress tasks with no recent updates (default: 5 min) |
| `tpg status` | Project overview for agent spin-up |
| `tpg summary` | Show project health overview |
| `tpg prime` | Output context for agent hooks |
| `tpg compact` | Output compaction workflow guidance |
| `tpg tui` | Launch interactive terminal UI (alias: `tpg ui`) |
| `tpg closed` | List recently closed tasks (done/canceled) |
| `tpg history [task-id]` | Show audit history events or run cleanup |

## Work Commands

| Command | Description |
|---------|-------------|
| `tpg start <id> [--resume]` | Set task to in_progress (use `--resume` if already in progress) |
| `tpg done <id> [message]` | Mark task complete |
| `tpg cancel <id> [reason]` | Cancel task (close without completing) |
| `tpg reopen <id> [reason]` | Reopen a closed task, setting it back to open |
| `tpg block <id> <reason>` | Mark blocked (requires `--force`; prefer dependencies instead) |
| `tpg log <id> <message>` | Add timestamped log entry |
| `tpg append <id> <text>` | Append to task description |
| `tpg desc <id> <text>` | Replace task description |
| `tpg edit <id>` | Edit description in $TPG_EDITOR (defaults to nvim, nano, vi) |
| `tpg edit --select-* <filter>` | Bulk edit: --select-status, --select-type, --select-label, --select-parent, --select-epic |
| `tpg merge <source> <target>` | Merge duplicate tasks (requires `--yes-i-am-sure`) |
| `tpg replace <id> <title>` | Replace an existing task/epic with a new one |
| `tpg impact <id>` | Show what tasks would become ready if this task is completed |
| `tpg plan <epic-id>` | Show full epic plan with status and dependencies |

## Organization

| Command | Description |
|---------|-------------|
| `tpg dep <id> blocks <other>` | Add blocking relationship (other blocked until id done) |
| `tpg dep <id> after <other>` | Add dependency (id depends on other) |
| `tpg dep <id> list` | Show all dependencies for a task |
| `tpg dep <id> remove <other>` | Remove dependency between tasks |
| `tpg graph` | Show dependency graph |
| `tpg projects` | List all projects |
| `tpg project <id> <project>` | Set a task's project |

## Epics

Epics are containers that group related tasks. They **auto-complete** when all children are done or canceled—you don't mark them done manually.

| Command | Description |
|---------|-------------|
| `tpg epic add <title>` | Create a new epic |
| `tpg epic edit <id>` | Edit title, context, or on-close instructions |
| `tpg epic list [epic-id]` | List all epics, or descendants of a specific epic |
| `tpg epic replace <id> <title>` | Replace an existing item with an epic |
| `tpg epic finish <id>` | Show closing instructions and cleanup commands |
| `tpg epic worktree <id>` | Set up worktree metadata for existing epic |

### Epic Fields

- **`--context`**: Shared context visible to all descendant tasks. Use for guidelines, API docs, patterns.
- **`--on-close`**: Instructions shown when the epic auto-completes (via `tpg epic finish`).

```bash
# Create epic with shared context
tpg epic add "Payment integration" --context - <<EOF
Use Stripe API v3. See docs/stripe-guide.md for patterns.
All payment handlers must include idempotency keys.
EOF

# Create epic with multiple fields via YAML
tpg epic add "Auth system" --from-yaml <<EOF
context: |
  JWT-based authentication. See RFC 7519.
on_close: |
  Update CHANGELOG.md before closing.
EOF
```

### Epic Behavior

- **Auto-complete**: Epics automatically transition to `done` when all children are done/canceled.
- **Cannot start epics with children**: `tpg start` prevents starting an epic that has child tasks—work on the children instead.
- **Replace existing items**: Use `tpg epic replace` to convert a task into an epic, preserving relationships.

### Worktrees

Epics can have associated Git worktrees for isolated development:

```bash
# Create an epic with worktree (auto-generates branch name)
tpg epic add "Implement user authentication" --worktree
# → Creates ep-abc123 with worktree_branch="feature/ep-abc123-implement-user-authentication"

# Create epic with custom branch
tpg epic add "Fix API bugs" --worktree --branch feature/api-fixes --base develop

# Set up worktree for existing epic
tpg epic worktree ep-abc123

# Set up with custom branch
tpg epic worktree ep-abc123 --branch feature/custom-name --base main

# Show closing instructions and cleanup commands
tpg epic finish ep-abc123
# → Shows on-close instructions (if set)
# → Prints merge and cleanup commands
# → For nested epics: merges to parent epic's branch
```

**Branch naming:** Auto-generated branches follow the pattern `feature/<epic-id>-<slug>` where slug is the lowercase title with non-alphanumeric characters replaced by hyphens.

Worktree configuration in `.tpg/config.json`:

```json
{
  "worktree": {
    "branch_prefix": "feature",
    "require_epic_id": true,
    "root": ".worktrees"
  }
}
```

## Labels

| Command | Description |
|---------|-------------|
| `tpg labels` | List all labels for a project |
| `tpg labels add <name>` | Create a new label |
| `tpg labels rm <name>` | Delete a label |
| `tpg labels rename <old> <new>` | Rename a label |
| `tpg label <id> <name>` | Add label to task (creates if needed) |
| `tpg unlabel <id> <name>` | Remove label from task |
| `tpg add "Fix bug" --label bug` | Create task with label (preferred over custom types) |

## Templates

| Command | Description |
|---------|-------------|
| `tpg template list` | List available templates |
| `tpg template show <id>` | Show template details |
| `tpg template usage <id>` | Show template usage and variables |
| `tpg template locations` | Show template search paths |

See [TEMPLATES.md](TEMPLATES.md) for template format and authoring.

## Context Engine

| Command | Description |
|---------|-------------|
| `tpg concepts` | List concepts for a project |
| `tpg concepts --stats` | Show concept statistics |
| `tpg concepts --related <task-id>` | Suggest concepts for a task |
| `tpg context -c <name>` | Retrieve learnings by concept(s) |
| `tpg context -q <query>` | Full-text search on learnings |
| `tpg context --summary` | Show one-liner per learning |
| `tpg context --id <learning-id>` | Load specific learning by ID |
| `tpg learn <summary>` | Log a new learning |
| `tpg learn edit <id>` | Edit a learning's summary or detail |
| `tpg learn stale <id>` | Mark learning as outdated |
| `tpg learn rm <id>` | Delete a learning |

See [CONTEXT.md](CONTEXT.md) for the full context engine guide.

## Data Management

| Command | Description |
|---------|-------------|
| `tpg export` | Export tasks to a single file for LLM consumption |
| `tpg export --json` | Export as JSON |
| `tpg export --jsonl` | Export as JSON Lines |
| `tpg import beads <path>` | Import beads issues into tpg |
| `tpg backup [path]` | Create a backup of the database |
| `tpg backups` | List available backups |
| `tpg restore <path>` | Restore database from a backup |
| `tpg clean --done` | Remove old done tasks |
| `tpg clean --canceled` | Remove old canceled tasks |
| `tpg clean --all` | Remove old done+canceled and vacuum |
| `tpg clean --vacuum` | Just compact the database |
| `tpg doctor` | Check and fix data integrity issues |
| `tpg doctor --dry-run` | Show issues without fixing |

## Configuration

| Command | Description |
|---------|-------------|
| `tpg config` | Show all configuration values |
| `tpg config <key>` | Show specific config value |
| `tpg config <key> <value>` | Set config value |

## Flags

### Global Flags

| Flag | Description |
|------|-------------|
| `--project` | Filter/set project scope |
| `--verbose, -v` | Show agent context and other debug info |
| `--from-yaml` | Read flag values from stdin as YAML (keys use underscores, e.g. `desc: value`) |

### add Command Flags

| Flag | Description |
|------|-------------|
| `-p, --priority` | Priority: 1=high, 2=medium (default), 3=low |
| `--parent <id>` | Set parent item at creation |
| `--blocks <id>` | Set task this will block at creation |
| `--after <id>` | Set task this depends on at creation |
| `-l, --label` | Attach label at creation (repeatable) |
| `--template <id>` | Template ID to instantiate |
| `--var 'name="value"'` | Template variable value |
| `--vars-yaml` | Read template variables from stdin as YAML |
| `--desc <text>` | Description (use `-` for stdin) |
| `--type <type>` | Item type: "task" (default) or "epic" |
| `--prefix <prefix>` | Custom ID prefix |
| `--dry-run` | Preview what would be created |

### list Command Flags

| Flag | Description |
|------|-------------|
| `-a, --all` | Show all items including done and canceled |
| `--status <status>` | Filter by status (open, in_progress, blocked, done, canceled) |
| `--parent <id>` | Filter by parent epic ID |
| `--type <type>` | Filter by item type (task, epic) |
| `--epic <id>` | Filter to descendants of this epic |
| `--blocking <id>` | Show items that block the given ID |
| `--blocked-by <id>` | Show items blocked by the given ID |
| `--has-blockers` | Show only items with unresolved blockers |
| `--no-blockers` | Show only items with no blockers |
| `--ids-only` | Output only IDs, one per line |
| `-f, --flat` | Show flat list instead of tree view |
| `-l, --label` | Filter by label (repeatable, AND logic) |

### epic add Command Flags

| Flag | Description |
|------|-------------|
| `-p, --priority` | Priority: 1=high, 2=medium, 3=low |
| `--parent <id>` | Parent epic ID |
| `-l, --label` | Label to attach (repeatable) |
| `--desc <text>` | Description (use `-` for stdin) |
| `--prefix <prefix>` | Custom ID prefix |
| `--context <text>` | Shared context for all descendants (use `-` for stdin) |
| `--on-close <text>` | Instructions shown when epic auto-completes (use `-` for stdin) |
| `--worktree` | Create epic with worktree metadata |
| `--branch <name>` | Custom branch name for worktree |
| `--base <branch>` | Base branch for worktree (default: main) |
| `--allow-any-branch` | Allow branch names that do not include the epic ID |

### show Command Flags

| Flag | Description |
|------|-------------|
| `--with-children` | Show task and all descendants |
| `--with-deps` | Show full dependency chain (transitive) |
| `--with-parent` | Show parent chain up to root |
| `--format <format>` | Output format (json, yaml, markdown) |
| `--vars` | Show raw template variables instead of rendered description |

### edit Command Flags

| Flag | Description |
|------|-------------|
| `--title <text>` | New title (single item only) |
| `--priority <n>` | New priority (1=high, 2=medium, 3=low) |
| `--parent <id>` | New parent epic ID (use `""` to remove) |
| `--add-label <name>` | Label to add (repeatable) |
| `--remove-label <name>` | Label to remove (repeatable) |
| `--desc <text>` | New description (single item only, use `-` for stdin) |
| `--status <status>` | Force status change (requires `--force`) |
| `--select-status <status>` | Select items by status |
| `--select-type <type>` | Select items by type |
| `--select-label <name>` | Select items by label (repeatable) |
| `--select-parent <id>` | Select items by parent |
| `--select-epic <id>` | Select descendants of epic |
| `--dry-run` | Preview changes without applying |
| `--force` | Required for --status changes |

### ready Command Flags

| Flag | Description |
|------|-------------|
| `-l, --label` | Filter by label (repeatable, AND logic) |
| `--epic <id>` | Show ready tasks for a specific epic |

### status Command Flags

| Flag | Description |
|------|-------------|
| `--all` | Show all ready tasks (default: limit to 10) |
| `-l, --label` | Filter by label (repeatable, AND logic) |

### learn Command Flags

| Flag | Description |
|------|-------------|
| `-c, --concept` | Concept to tag this learning with (repeatable) |
| `-f, --file` | Related file (repeatable) |
| `--detail <text>` | Full context/explanation (use `-` for stdin) |

### context Command Flags

| Flag | Description |
|------|-------------|
| `-c, --concept` | Concept to retrieve learnings for (repeatable) |
| `-q, --query` | Full-text search query |
| `--include-stale` | Include stale learnings in results |
| `--summary` | Show one-liner per learning (no detail) |
| `--id <learning-id>` | Load specific learning by ID |
| `--json` | Output as JSON |

### export Command Flags

| Flag | Description |
|------|-------------|
| `-o, --output <path>` | Output file path (default: stdout) |
| `--json` | Output as JSON instead of markdown |
| `--jsonl` | Output as JSON Lines (one object per line) |
| `-a, --all` | Include done and canceled tasks |
| `--status <status>` | Filter by status |
| `--parent <id>` | Filter by parent epic ID |
| `--type <type>` | Filter by item type |
| `--blocking <id>` | Show items that block the given ID |
| `--blocked-by <id>` | Show items blocked by the given ID |
| `--has-blockers` | Show only items with unresolved blockers |
| `--no-blockers` | Show only items with no blockers |
| `-l, --label` | Filter by label (repeatable, AND logic) |

### clean Command Flags

| Flag | Description |
|------|-------------|
| `--done` | Remove done tasks older than N days |
| `--canceled` | Remove canceled tasks older than N days |
| `--logs` | Remove orphaned logs |
| `--vacuum` | Run SQLite VACUUM to compact database |
| `--all` | Do all cleanup (done + canceled + vacuum) |
| `--days <n>` | Age threshold in days (default: 30) |
| `--dry-run` | Show what would be deleted |
| `--force` | Skip confirmation prompt |

### history Command Flags

| Flag | Description |
|------|-------------|
| `-n, --limit <n>` | Max number of results (default 50) |
| `-a, --agent <id>` | Filter by agent ID |
| `-s, --since <duration>` | Filter by time (e.g., '24h', '7d') |
| `--event-type <type>` | Filter by event type |
| `--cleanup` | Run history cleanup |
| `--dry-run` | With --cleanup, show what would be deleted |
| `--json` | Output as JSON |

### closed Command Flags

| Flag | Description |
|------|-------------|
| `-n, --limit <n>` | Maximum number of tasks to show (default: 20) |
| `-s, --since <duration>` | Show tasks closed since duration (e.g., 24h, 7d). Default: 7d |
| `--status <status>` | Filter by status (done, canceled) |

### Other Command Flags

| Command | Flag | Description |
|---------|------|-------------|
| `start` | `--resume` | Resume an already in-progress task |
| `done` | `--override` | Allow completion with unmet dependencies |
| `cancel` | `--force` | Cancel even if tasks depend on this item |
| `delete` | `--force` | Delete even if tasks depend on this item |
| `block` | `--force` | Force manual block (prefer dependencies instead) |
| `stale` | `--threshold <duration>` | Threshold for stale in-progress tasks (default: 5m) |
| `merge` | `--yes-i-am-sure` | Confirm destructive merge operation |
| `backup` | `-q, --quiet` | Silent backup (no output) |
| `impact` | `--json` | Output as JSON |
| `plan` | `--json` | Output as JSON |
| `prime` | `--customize` | Create/edit custom prime template |
| `prime` | `--render <path>` | Render specific template file (for testing) |
| `onboard` | `--force` | Replace existing Task Tracking section |
| `doctor` | `--dry-run` | Show issues without fixing |
| `concepts` | `--recent` | Sort by last updated |
| `concepts` | `--stats` | Show count and oldest learning age |
| `concepts` | `--related <task-id>` | Suggest concepts for a task |
| `concepts <name>` | `--summary <text>` | Set concept summary |
| `concepts <name>` | `--rename <new-name>` | Rename concept |
| `labels add` | `--color <hex>` | Label color (e.g. #ff0000) |
| `learn stale` | `--reason <text>` | Reason for marking as stale |
| `learn edit` | `--summary <text>` | New summary for the learning |
| `learn edit` | `--detail <text>` | New detail for the learning (use `-` for stdin) |
| `epic worktree` | `--branch <name>` | Custom branch name |
| `epic worktree` | `--base <branch>` | Base branch |
| `epic worktree` | `--allow-any-branch` | Allow branch names without epic ID |

## ID Format

IDs are auto-generated with type prefixes:

**Prefixes:**
- `ts-XXXXXX` — tasks (e.g., `ts-a1b2c3`)
- `ep-XXXXXX` — epics (e.g., `ep-f0a20b`)

Configure custom prefixes in `.tpg/config.json`:

```json
{
  "prefixes": {
    "task": "ts",
    "epic": "ep"
  }
}
```

ID length is configurable via `id_length` in `.tpg/config.json` (default: 3 characters, base-36 alphabet `[0-9a-z]`).

**Note:** The type system only supports "task" and "epic". Use labels to categorize work (e.g., `--label bug`, `--label story`, `--label feature`). Migration v6 automatically converts old arbitrary types to labels.

## Removed Commands

The following commands have been removed. Use these alternatives:

| Removed | Replacement |
|---------|-------------|
| `tpg add -e <title>` | `tpg epic add <title>` |
| `tpg parent <id> <epic-id>` | `tpg edit <id> --parent <epic-id>` |
| `tpg set-status <id> <status>` | `tpg done <id>`, `tpg cancel <id>`, `tpg block <id>` |

## Worktree Workflow Example

```bash
# 1. Create an epic with worktree
tpg epic add "Implement OAuth2 authentication" --worktree
# → Creates ep-abc123 with worktree_branch="feature/ep-abc123-implement-oauth2-authentication"
# → Prints worktree setup instructions:
#    git worktree add -b feature/ep-abc123-implement-oauth2-authentication .worktrees/ep-abc123 main
#    cd .worktrees/ep-abc123

# 2. Add tasks to the epic
tpg add "Set up OAuth2 provider config" --parent ep-abc123
tpg add "Implement token refresh" --parent ep-abc123
tpg add "Add logout endpoint" --parent ep-abc123

# 3. View ready tasks for this epic
tpg ready --epic ep-abc123
# Ready tasks for epic ep-abc123 - Implement OAuth2 authentication:
# (3 ready)

# 4. Start working on a task (worktree guidance shown)
tpg start ts-def456
# Started ts-def456
#
# 📁 Worktree: ep-abc123 - Implement OAuth2 authentication
#    Branch: feature/ep-abc123-implement-oauth2-authentication
#    Location: .worktrees/ep-abc123
#
#    To work in the correct directory:
#    cd .worktrees/ep-abc123

# 5. Show task details with worktree context
tpg show ts-def456
# ...
# Worktree:
#   Epic:     ep-abc123 - Implement OAuth2 authentication
#   Branch:   feature/ep-abc123-implement-oauth2-authentication
#   Location: .worktrees/ep-abc123
#   Status:   (check with: git worktree list)
#   Path:     ep-abc123 -> ts-def456
#
#   To create worktree:
#     git worktree add -b feature/ep-abc123-implement-oauth2-authentication .worktrees/ep-abc123 main
#     cd .worktrees/ep-abc123

# 6. When all tasks are done, epic auto-completes. Show cleanup instructions:
tpg epic finish ep-abc123
# → Shows closing instructions (if set via --on-close)
#
# Cleanup instructions:
#   # Merge to main:
#   git checkout main
#   git merge feature/ep-abc123-implement-oauth2-authentication
#   git worktree remove .worktrees/ep-abc123
#   git branch -d feature/ep-abc123-implement-oauth2-authentication
```

## Ready Command Output

`tpg ready` shows tasks ready for work, grouped by epic with counts:

```bash
tpg ready
# Ready tasks:
# (8 ready)
#
# ep-abc123 - Implement OAuth2 authentication (3 / 5 tasks ready)
#   ts-def456  Set up OAuth2 provider config
#   ts-ghi789  Implement token refresh
#   ts-jkl012  Add logout endpoint
#
# ep-xyz789 - Payment integration (2 / 8 tasks ready)
#   ts-mno345  Configure Stripe keys
#   ts-pqr678  Implement checkout flow
#
# (no epic)
#   ts-stu901  Update README
#   ts-vwx234  Fix typo in config
#   ts-yza567  Add license file
```

The format `(X / Y tasks ready)` shows X ready tasks out of Y total tasks in the epic.

## Stale Status Display

In-progress tasks older than 5 minutes display with a "stale" indicator:

```bash
tpg list --status in_progress
# ID         STATUS       TITLE
# ts-abc123  in_progress  Implement auth      ⚠ stale (23m)
# ts-def456  in_progress  Add tests           
```

The stale indicator helps identify abandoned work. Use `tpg stale` to list only stale tasks:

```bash
tpg stale                  # Default: 5 minute threshold
tpg stale --threshold 10m  # Custom threshold
```

## Shell Completion

tpg provides intelligent shell autocompletion for commands, flags, and IDs.

### Setup

**Important:** Completion must be installed via a completion file, not sourced directly.

**Bash:**
```bash
# Save to bash completions directory
mkdir -p ~/.local/share/bash-completion/completions
tpg completion bash > ~/.local/share/bash-completion/completions/tpg

# Or system-wide (may require sudo)
tpg completion bash > /etc/bash_completion.d/tpg

# Then reload your shell or source the file
source ~/.local/share/bash-completion/completions/tpg
```

**Zsh:**
```bash
# Method 1: User-local completions (recommended)
mkdir -p ~/.zsh/completions
tpg completion zsh > ~/.zsh/completions/_tpg

# Add to ~/.zshrc (must be before any compinit call):
fpath=(~/.zsh/completions $fpath)

# Then reload:
source ~/.zshrc

# Method 2: Oh-my-zsh
mkdir -p ~/.oh-my-zsh/completions
tpg completion zsh > ~/.oh-my-zsh/completions/_tpg

# Method 3: System-wide (macOS with Homebrew)
tpg completion zsh > $(brew --prefix)/share/zsh/site-functions/_tpg
```

**Fish:**
```bash
# User-local
mkdir -p ~/.config/fish/completions
tpg completion fish > ~/.config/fish/completions/tpg.fish

# Or load temporarily for current session
tpg completion fish | source
```

### Troubleshooting

**Completion not working?**

1. Verify the completion file exists:
   ```bash
   ls ~/.zsh/completions/_tpg  # zsh
   ls ~/.local/share/bash-completion/completions/tpg  # bash
   ```

2. Check that fpath includes your completions directory (zsh):
   ```bash
   echo $fpath | tr ' ' '\n' | grep completions
   ```

3. Ensure compinit is called after updating fpath in ~/.zshrc:
   ```bash
   fpath=(~/.zsh/completions $fpath)
   autoload -Uz compinit && compinit
   ```

4. Test completion directly:
   ```bash
   tpg __complete show ts-
   # Should output task IDs, not files
   ```

### What Gets Completed

- **Item IDs**: When typing commands like `tpg show`, `tpg done`, `tpg start`, the shell suggests matching task/epic IDs with their titles
- **Epic IDs**: Commands that expect epics (`--epic`, `--parent`) filter to show only epic IDs
- **Labels**: Flag completions for `--label` show available labels
- **Projects**: Flag completions for `--project` show available project names
- **Status values**: `--status` suggests valid statuses (open, in_progress, done, etc.)
- **Template IDs**: `--template` suggests available template IDs

### Examples

```bash
# Type partial ID and press TAB
tpg show ts-a<TAB>
# ts-a0c335  TUI: Add log viewing to task detail screen
# ts-abc123  Implement authentication

# Complete epic IDs for --epic flag
tpg ready --epic ep-<TAB>
# ep-oop  Worktree support for shared database
# ep-b4m  Template system improvements

# Complete status values
tpg list --status <TAB>
# open
# in_progress
# done
# blocked
# canceled
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `TPG_DB` | Override default database location |
| `TPG_EDITOR` | Editor for `tpg edit` command (defaults to nvim, nano, vi) |
| `AGENT_ID` | Current agent ID (set by OpenCode plugin) |
| `AGENT_TYPE` | Agent type (set by OpenCode plugin) |

## Data Model

- **Items**: Work items with title, description, status, priority. Types are "task" or "epic".
- **Type**: Either "task" or "epic". Use labels for categorization (bug, feature, refactor, etc.).
- **Status**: `open` -> `in_progress` -> `done` (or `blocked`, `canceled`). In-progress tasks older than 5 minutes display as "stale" with ⚠ badge.
- **Dependencies**: Item A can depend on Item B (A is blocked until B is done)
- **Parent**: Any item can be a parent of other items, creating hierarchies
- **Labels**: Tags for categorization (bug, feature, refactor, etc), project-scoped
- **Logs**: Timestamped audit trail per item
- **Projects**: String tag to scope work (e.g., "gaia", "myapp")
- **Concepts**: Knowledge categories within a project (e.g., "auth", "database")
- **Learnings**: Specific insights tagged with concepts, with summary and detail
- **History**: Audit trail of all changes to items

Database location: `.tpg/tpg.db` (in current directory, or override with `TPG_DB`).
