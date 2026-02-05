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
| `tpg ready` | Show tasks ready for work (open + deps met) |
| `tpg ready --epic <id>` | Show ready tasks filtered by epic |
| `tpg status` | Project overview for agent spin-up |
| `tpg prime` | Output context for agent hooks |
| `tpg compact` | Output compaction workflow guidance |
| `tpg tui` | Launch interactive terminal UI (alias: `tpg ui`) |

## Work Commands

| Command | Description |
|---------|-------------|
| `tpg start <id> [--resume]` | Set task to in_progress (use `--resume` if already in progress) |
| `tpg done <id> [message]` | Mark task complete |
| `tpg cancel <id> [reason]` | Cancel task (close without completing) |
| `tpg block <id> <reason>` | Mark blocked (requires `--force`; prefer dependencies instead) |
| `tpg log <id> <message>` | Add timestamped log entry |
| `tpg append <id> <text>` | Append to task description |
| `tpg desc <id> <text>` | Replace task description |
| `tpg edit <id>` | Edit description in $TPG_EDITOR (defaults to nvim, nano, vi) |
| `tpg edit --select <filter>` | Bulk edit: --type, --status, --label, --parent, --project |
| `tpg merge <source> <target>` | Merge duplicate tasks (requires `--yes-i-am-sure`) |

## Organization

| Command | Description |
|---------|-------------|
| `tpg dep <id> blocks <other>` | Add blocking relationship (other blocked until id done) |
| `tpg dep <id> after <other>` | Add dependency (id depends on other) |
| `tpg dep <id> list` | Show all dependencies for a task |
| `tpg dep <id> remove <other>` | Remove dependency between tasks |
| `tpg graph` | Show dependency graph |
| `tpg projects` | List all projects |
| `tpg add "<title>" --type <type>` | Create item with custom type (task, epic, bug, story, etc.) |

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

## Templates

| Command | Description |
|---------|-------------|
| `tpg template list` | List available templates |
| `tpg template show <id>` | Show template details |
| `tpg template locations` | Show template search paths |

See [TEMPLATES.md](TEMPLATES.md) for template format and authoring.

## Context Engine

| Command | Description |
|---------|-------------|
| `tpg concepts` | List concepts for a project |
| `tpg context -c <name>` | Retrieve learnings by concept(s) |
| `tpg context -q <query>` | Full-text search on learnings |
| `tpg learn <summary>` | Log a new learning |
| `tpg learn edit <id>` | Edit a learning's summary or detail |
| `tpg learn stale <id>` | Mark learning as outdated |
| `tpg learn rm <id>` | Delete a learning |

See [CONTEXT.md](CONTEXT.md) for the full context engine guide.

## Flags

| Flag | Commands | Description |
|------|----------|-------------|
| `--from-yaml` | all | Read flag values from stdin as YAML (keys use underscores, e.g. `desc: value`) |
| `--project` | all | Filter/set project scope |
| `-p, --priority` | add | Priority: 1=high, 2=medium (default), 3=low |
| `--type <type>` | add | Create item with custom type (task, epic, bug, story, etc.) |
| `-l, --label` | add, list, ready, status | Attach label at creation / filter by label (repeatable, AND logic) |
| `--parent <id>` | add, list | Set parent item at creation / filter by parent (any type can have children) |
| `--blocks` | add | Set task this will block at creation |
| `--worktree` | epic add | Create epic with worktree metadata |
| `--branch` | epic add, epic worktree | Custom branch name for worktree (default: auto-generated) |
| `--base` | epic add, epic worktree | Base branch for worktree (default: main) |
| `--context` | epic add, epic edit, epic replace | Shared context for all descendants (use `-` for stdin) |
| `--on-close` | epic add, epic edit, epic replace | Instructions shown when epic auto-completes (use `-` for stdin) |
| `--status` | list | Filter by status |
| `--epic <id>` | list, ready | Filter by parent epic |
| `--ids-only` | list | Output just IDs, one per line |
| `--type` | list | Filter by item type (task, epic, bug, etc.) |
| `--blocking` | list | Show items that block the given ID |
| `--blocked-by` | list | Show items blocked by the given ID |
| `--has-blockers` | list | Show only items with unresolved blockers |
| `--no-blockers` | list | Show only items with no blockers |
| `--all` | status | Show all ready tasks (default: limit to 10) |
| `--resume` | start | Resume an already in-progress task |

## ID Format

IDs are auto-generated with configurable type prefixes:

**Default prefixes:**
- `ts-XXXXXX` — tasks (e.g., `ts-a1b2c3`)
- `ep-XXXXXX` — epics (e.g., `ep-f0a20b`)

**Arbitrary types:** Any type can be used (bug, story, feature, etc.). Configure custom prefixes in `.tpg/config.json`:

```json
{
  "prefixes": {
    "task": "ts",
    "epic": "ep"
  },
  "custom_prefixes": {
    "bug": "bg",
    "story": "st"
  }
}
```

ID length is configurable via `id_length` in `.tpg/config.json` (default: 3 characters, base-36 alphabet `[0-9a-z]`).

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

## Environment Variables

| Variable | Description |
|----------|-------------|
| `TPG_DB` | Override default database location |
| `TPG_EDITOR` | Editor for `tpg edit` command (defaults to nvim, nano, vi) |

## Data Model

- **Items**: Work items of arbitrary types (task, epic, bug, story, etc.) with title, description, status, priority
- **Type**: Arbitrary string identifying the item type. Any type can have child items via the parent relationship.
- **Status**: `open` -> `in_progress` -> `done` (or `blocked`, `canceled`)
- **Dependencies**: Item A can depend on Item B (A is blocked until B is done)
- **Parent**: Any item can be a parent of other items, creating hierarchies
- **Labels**: Tags for categorization (bug, feature, refactor, etc), project-scoped
- **Logs**: Timestamped audit trail per item
- **Projects**: String tag to scope work (e.g., "gaia", "myapp")
- **Concepts**: Knowledge categories within a project (e.g., "auth", "database")
- **Learnings**: Specific insights tagged with concepts, with summary and detail

Database location: `.tpg/tpg.db` (in current directory, or override with `TPG_DB`).
