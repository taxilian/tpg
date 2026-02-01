# Worktree Support Design

## Overview

This document describes the worktree support feature for tpg, which allows epics to have dedicated git worktrees with feature branches. This enables isolated development environments for large features while maintaining tpg's simple workflow.

tpg is **read-only** with respect to git. It stores metadata and prints instructions, but never creates branches or worktrees itself. This preserves tpg's philosophy of being a tool for thinking beings — it provides context and information, but the human decides and executes.

## Goals

1. **Zero-knowledge agents**: Agents use the same commands; tpg shows worktree context but never changes behavior based on location
2. **Simple setup**: One command to register epic + worktree metadata and print instructions
3. **Automatic scoping**: `tpg ready` shows epic-scoped tasks when in a worktree
4. **Clear context**: `tpg show` always displays epic ancestry and worktree info
5. **Safe completion**: Validation before merge, manual merge step for safety
6. **No git side effects**: tpg only stores metadata and prints instructions

## Design Philosophy

**"Invisible until needed"** — The worktree feature should feel automatic. Agents shouldn't need to learn new commands or flags. They run `tpg ready`, `tpg show`, `tpg start` as always, and tpg adapts to the context.

**"Human in the loop"** — tpg never performs git operations. It detects, suggests, and informs, but the human executes. This prevents surprises and respects the developer's judgment.

## Data Model

### Schema Changes (Migration v4)

Add to `items` table:

| Column | Type | Description |
|--------|------|-------------|
| `worktree_branch` | TEXT | Git branch name (e.g., `feature/ep-abc123-worktree-support`). NULL = no worktree |
| `worktree_base` | TEXT | Intended base branch for manual worktree creation (e.g., `main`) |

Only epics use these fields. Tasks inherit worktree context by walking up the parent chain to find the root epic.

### Model Changes

```go
type Item struct {
    // ... existing fields ...
    WorktreeBranch string  // From worktree_branch column
    WorktreeBase   string  // From worktree_base column
}
```

## Branch-Based Worktree Detection

Instead of storing worktree paths (which can move), we store the **feature branch name** and dynamically detect if a worktree exists for that branch.

### Detection Algorithm

```go
// 1. Find the main git repository root
//    - Walk up looking for .git directory (main repo) or .git file (worktree)
//    - If .git is a file, parse it to find the main repo's .git directory
//    - The main repo root is the parent of the .git directory
repoRoot := findGitRepoRoot()  // handles both main repo and worktrees

// 2. List all worktrees and their branches by reading .git/worktrees/
worktrees := listWorktrees(repoRoot)  // map[branch]worktreePath

// 3. Check if current directory is a worktree (via .git file)
if isWorktreeDir(cwd) {
    // Get branch from worktree's .git file
    branch := getWorktreeBranch(cwd)
    epic := db.FindEpicByBranch(branch)
    return epic, worktrees[branch]
}

// 4. Not in a worktree - check if on a worktree branch (for info only)
currentBranch := getCurrentBranch(repoRoot)
epic := db.FindEpicByBranch(currentBranch)
if epic != nil {
    if path, exists := worktrees[currentBranch]; exists {
        return epic, path  // worktree exists but we're not in it
    }
    return epic, ""  // branch configured but no worktree
}

return nil, ""
```

**Key principle:** Detection is informational only. It shows worktree context but never changes command behavior. Users must explicitly use flags like `--epic` to filter results.

### Getting Worktree Info Without Git

**Finding repo root:** Walk up directory tree looking for `.git`:
- If `.git` is a directory → this is the main repo root
- If `.git` is a file → parse the `gitdir:` line to find the main repo's `.git` directory
- The repo root is the parent of the `.git` directory

**Getting current branch:** Read `.git/HEAD`:
- `ref: refs/heads/main` → branch is `main`
- `e5d3b57bcd2aa852d3fc8bf9f8f2035ffddd3e45` → detached HEAD

**Listing worktrees:** Parse `.git/worktrees/*/HEAD` and `.git/worktrees/*/gitdir`:
- Each subdirectory in `.git/worktrees/` is a worktree
- `HEAD` contains the branch name
- `gitdir` points to the worktree's `.git` file location

**Checking if in worktree:** Check if `.git` is a file (not directory). If file, parse `gitdir:` line to find the worktree entry in `.git/worktrees/`.

This requires only file operations — no git subprocess or library needed.

Detection is informational only. It is used to show worktree context and to scope `tpg ready` when the user is inside an existing worktree. It never triggers git operations.

### Worktree Setup (Manual)

tpg does **not** create branches or worktrees. It only stores metadata and prints setup instructions. The actual git operations are performed manually by the user.

Suggested command pattern:

```bash
git worktree add -b <branch> <path> <base>
```

**Default worktree path hint:** `.worktrees/<epic-id>/` (relative to repo root)

Benefits of a consistent default:
- Predictable pattern
- Easy to `.gitignore`
- Can be recreated if deleted (same path every time)

### Branch Naming Policy

By default, tpg generates branches as `feature/<epic-id>-<slug>`, where the slug is derived from the epic title.
The prefix (`feature/`) and whether the epic id is required are configurable (see Configuration).

If `require_epic_id` is true, any explicit `--branch` must include the epic id (case-insensitive, as word boundary).

## Command Changes

### `tpg add -e --worktree` (Modified)

Create an epic with worktree metadata.

**New flags (when `-e` is used):**

| Flag | Default | Description |
|------|---------|-------------|
| `--worktree` | (none) | Enable worktree metadata for this epic |
| `--branch <name>` | `feature/<epic-id>-<slug>` | Custom branch name (prefix and enforcement are configurable) |
| `--base <branch>` | parent's worktree branch, or current branch | Base branch hint for printed instructions. Defaults to parent's worktree branch if parent has one (encourages cascade pattern)

**Behavior:**

1. Create epic in database (normal flow)
2. Store `worktree_branch` and `worktree_base` on epic
3. If repo root is detectable, scan `.git/worktrees` to see if the branch has a worktree
4. Print either the detected worktree location or manual setup instructions

**Examples:**

```bash
# Create new epic and register worktree metadata
$ tpg add "Implement worktree support" -e --worktree
Created epic ep-abc123 (worktree expected)
  Branch: feature/ep-abc123-worktree-support (from main)

Worktree not found. Create it with:
  git worktree add -b feature/ep-abc123-worktree-support .worktrees/ep-abc123 main

Then:
  cd .worktrees/ep-abc123
  tpg ready

# Branch already exists with worktree - show it
$ tpg add "Another epic" -e --worktree --branch feature/ep-abc123-worktree-support
Worktree detected for branch feature/ep-abc123-worktree-support:
  Location: .worktrees/ep-abc123/
  
Epic ep-def456 created and linked to detected worktree.
```

**Error cases:**

```bash
# Branch doesn't include epic id (when require_epic_id is true)
$ tpg add "Big Feature" -e --worktree --branch feature/my-feature
Error: Branch name must include epic id "ep-abc123" (or use --allow-any-branch)

Suggested: feature/ep-abc123-my-feature
```

### `tpg ready` (Modified)

**New flag:**

| Flag | Description |
|------|-------------|
| `--epic <id>` | Filter to show only tasks under this epic (descendants) |

**Examples:**

```bash
# See all ready tasks (default behavior)
$ tpg ready
[shows all ready tasks across all epics]

# Filter to specific epic
$ tpg ready --epic ep-abc123
[shows only ready tasks in epic ep-abc123]
```

**Note:** There is no auto-scoping based on worktree location. Users must explicitly use `--epic` to filter.

### `tpg show` (Modified)

Always computes and displays epic ancestry when applicable.

**New output fields:**

```
ID:          ts-def456
Type:        task
Project:     tpg
Title:       Implement auto-detection
Status:      open
Priority:    1
Parent:      ts-ghi789

Epic:        ep-abc123 "Implement worktree support"
Epic path:   → ts-ghi789 "Git helpers" → ep-abc123 "Implement worktree support"
Worktree:    .worktrees/ep-abc123/ (branch: feature/ep-abc123-worktree-support)
Status:      ✓ worktree exists
```

**Worktree status indicators:**
- `✓ worktree exists` — worktree detected at expected location
- `✗ worktree not found` — worktree path doesn't exist (show recreate command)
- `⚠ not in worktree` — on worktree branch but not in worktree directory

**Structured output** (JSON/YAML/markdown) includes:

```json
{
  "item": { ... },
  "epic": {
    "id": "ep-abc123",
    "title": "Implement worktree support",
    "path": ["ts-ghi789", "ep-abc123"]
  },
  "worktree": {
    "branch": "feature/ep-abc123-worktree-support",
    "base": "main",
    "path": ".worktrees/ep-abc123/",
    "exists": true,
    "in_worktree": false
  }
}
```

### `tpg start` (Modified)

Shows worktree context when task belongs to worktree epic:

```bash
# In main repo, task has worktree
$ tpg start ts-def456
Note: This task belongs to epic ep-abc123 (branch: feature/ep-abc123-worktree-support)
      Worktree: .worktrees/ep-abc123/ (not currently in worktree)
      
      To work in the correct environment:
        cd .worktrees/ep-abc123
        
Started ts-def456

# Already in worktree
$ cd .worktrees/ep-abc123
$ tpg start ts-def456
Started ts-def456
```

### `tpg plan` (Modified)

If the epic has a worktree, show it in the header:

```
ep-abc123 [in_progress] Implement worktree support
============================================
Worktree: .worktrees/ep-abc123/ (branch: feature/ep-abc123-worktree-support, base: main)
Status: ✓ worktree exists

Progress: 3/10 (30%)
...
```

### `tpg list` (Modified)

**New flags:**

| Flag | Description |
|------|-------------|
| `--epic <id>` | Filter to descendants of an epic |
| `--ids-only` | Output only IDs (one per line, pipe-friendly) |

**Note:** `list` is an explicit querying tool; filtering requires explicit flags.

## New Commands

### `tpg epic worktree <epic-id>`

Add or update worktree metadata for an existing epic.

**Flags:**

| Flag | Description |
|------|-------------|
| `--branch <name>` | Branch name (default: `feature/<epic-id>-<slug>`) |
| `--base <branch>` | Base branch hint for printed instructions (default: current) |
| `--allow-any-branch` | Skip epic id validation in branch name |

**Examples:**

```bash
# Register worktree metadata for an existing epic
$ tpg epic worktree ep-abc123
Updated epic ep-abc123 (worktree expected)
  Branch: feature/ep-abc123-worktree-support (from main)

Worktree not found. Create it with:
  git worktree add -b feature/ep-abc123-worktree-support .worktrees/ep-abc123 main

# Link epic to an existing branch
$ tpg epic worktree ep-abc123 --branch feature/ep-abc123-auth
Updated epic ep-abc123 (worktree expected)
  Branch: feature/ep-abc123-auth (from main)

Worktree detected:
  .worktrees/ep-abc123
```

### `tpg epic finish <epic-id>`

Complete a worktree epic.

**Behavior:**

1. Validate all descendants are done or canceled
2. Mark epic as done
3. Output merge/cleanup instructions:

```bash
$ tpg epic finish ep-abc123
Epic ep-abc123 completed (12 tasks done, 1 canceled).

Worktree: .worktrees/ep-abc123/
Branch: feature/ep-abc123-worktree-support
Base: main

To merge and clean up:
  git checkout main
  git merge feature/ep-abc123-worktree-support
  git worktree remove .worktrees/ep-abc123/
  git branch -d feature/ep-abc123-worktree-support

```

**Safety:** This command only outputs instructions. tpg never runs git operations.

## Bulk Operations (Refactored `tpg edit`)

**Breaking changes:** This refactor removes `tpg set-status` and `tpg parent`, consolidating all modification into `tpg edit`.

### New `tpg edit` Interface

```
Usage:
  tpg edit <id> [flags]                    # Edit single item
  tpg edit <id1> <id2>... [flags]          # Edit multiple items
  tpg edit --select <filter>... [flags]    # Edit filtered items

Fields (safe for bulk):
  --priority <1|2|3>         Set priority
  --parent <id|''>           Set parent (empty removes)
  --add-label <label>        Add label (repeatable)
  --remove-label <label>     Remove label (repeatable)

Fields (single-item only):
  --title <string>           Set title
  --desc <string>            Set description (- for stdin)

Selection filters:
  --select-project <name>
  --select-status <status>
  --select-type <type>
  --select-label <label>
  --select-parent <id>
  --select-epic <id>         All descendants of epic

Other:
  --dry-run                  Preview changes
```

**Safety rules:**

| Field | Single | Multiple | Behavior |
|-------|--------|----------|----------|
| `--title` | ✓ | ✗ | Error: "--title can only be used with a single item" |
| `--desc` | ✓ | ✗ | Error: "--desc can only be used with a single item" |
| Others | ✓ | ✓ | Allowed |

**Removed commands:**
- `tpg set-status` — use `tpg start`, `tpg done`, `tpg cancel`, `tpg block`, `tpg reopen`
- `tpg parent` — use `tpg edit --parent`

**Examples:**

```bash
# Move tasks to epic
$ tpg edit --select-label feature-x --parent ep-abc123

# Set priority on all epic tasks
$ tpg edit --select-epic ep-abc123 --priority 1

# Multiple explicit IDs
$ tpg edit ts-1 ts-2 ts-3 --parent ep-abc123 --add-label in-epic

# Edit single item (title allowed)
$ tpg edit ts-abc --title "New title" --priority 1

# Preview changes
$ tpg edit --select-label old-feature --remove-label old-feature --add-label archived --dry-run
```

## Agent Workflow

### Orchestrator (from main repo)

```bash
# See all ready work, with epic annotations
$ tpg ready
[shows ready tasks, worktree info in output]

# Check specific epic
$ tpg plan ep-abc123
[shows epic details + worktree location]

# Delegate to agent
$ tpg show ts-def456
[confirms task is in epic with worktree]

# Launch agent in worktree
$ cd .worktrees/ep-abc123
$ @tpg-agent Work on ts-def456
```

### Task Agent (in worktree)

```bash
# Same commands work everywhere; context shown but not enforced
$ tpg ready
[shows all ready tasks]

# Filter to this epic explicitly
$ tpg ready --epic ep-abc123
[shows only ready tasks in epic ep-abc123]

# Confirm context
$ tpg show ts-def456
[shows epic path + worktree info]

# Start work
$ tpg start ts-def456

# Do work, run tests, etc.

# Complete
$ tpg done ts-def456
```

## Prime Template Integration

When running from a worktree, prime output includes worktree context:

```
## Status
**Worktree:** feature/ep-abc123-worktree-support → ep-abc123 "Implement worktree support"
  Location: .worktrees/ep-abc123/
  Use `tpg ready --epic ep-abc123` to see this epic's ready tasks.

**Your work:**
  • [ts-def456] Implement auto-detection
- 5 ready (use 'tpg ready')
...
```

**Setup needed section** (for epics with worktree metadata but no detected worktree):

```
## Setup Needed
Epics with worktree metadata but no worktree detected:
  • ep-abc123 "Big Feature" — run:
    git worktree add -b feature/ep-abc123-big-feature .worktrees/ep-abc123 main
```

## Database Migration

**Version 4:**

```sql
ALTER TABLE items ADD COLUMN worktree_branch TEXT;
ALTER TABLE items ADD COLUMN worktree_base TEXT;
```

Both nullable. Only meaningful when `type = 'epic'`.

## Configuration

Worktree defaults and branch naming policy are configurable in `.tpg/config.json`:

```json
{
  "worktree": {
    "branch_prefix": "feature",
    "require_epic_id": true,
    "root": ".worktrees"
  }
}
```

- **branch_prefix**: Prefix for auto-generated branches (e.g., `feature/`)
- **require_epic_id**: If true, any explicit `--branch` must include the epic id
- **root**: Default worktree root for printed instructions (relative to repo root)

The suggested worktree path is resolved as:

```
<worktree.root>/<epic-id>/
```

tpg never creates directories or worktrees; this is a hint shown to the user.

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Worktree deleted manually | `tpg show` displays "(not found)" with recreate command; `tpg prime` lists in "Setup Needed" |
| Not in git repo | Worktree feature disabled; commands work normally |
| Task moved to different epic | Worktree stays with original epic; task now in different epic without worktree |
| Nested epics | Each epic can have its own worktree; auto-detection finds the one matching current branch |
| Branch already exists with worktree | `tpg add -e --worktree` shows detected worktree location |
| Branch already exists without worktree | `tpg add -e --worktree` prints instructions to create a worktree |
| Dirty repo | tpg does not check; `git worktree add` may fail if there are uncommitted changes |
| Multiple epics same branch | Auto-detection disabled (shows all tasks); use `--epic` explicitly |
| On worktree branch but not in worktree | `tpg ready` shows hint, displays all tasks; `tpg show` shows "⚠ not in worktree" |
| Submodule directory | Submodule's `.git` is a file pointing to parent's `.git/modules/`. We follow that to find the real repo root. |

## Implementation Tasks

| # | Task | Dependencies |
|---|------|--------------|
| 1 | Schema migration v4: worktree columns | None |
| 2 | Model: Item fields, query updates | 1 |
| 3 | Git helpers package: find repo root (via .git, handling submodules), get current branch, list worktrees, detect if in worktree (file ops only) | None |
| 4 | DB queries: FindEpicByBranch, ReadyItemsForEpic, GetRootEpic, FindEpicsWithWorktreeButNoWorktree | 1, 2 |
| 5 | `tpg list --ids-only` | None |
| 6 | `tpg edit` expansion: multiple IDs, --select filters, remove set-status/parent | 5 |
| 7 | `tpg add -e --worktree` | 2, 3 |
| 8 | `tpg ready --epic` filtering | 3, 4 |
| 9 | `tpg show` epic context with status indicators | 3, 4 |
| 10 | `tpg start` worktree guidance | 3, 4 |
| 11 | `tpg epic worktree` subcommand | 3, 6 |
| 12 | `tpg epic finish` subcommand | 4 |
| 13 | `tpg plan` worktree header | 4 |
| 14 | Prime template updates with "Setup Needed" section | 3, 4 |
| 15 | Update CLI.md documentation | All |
| 16 | Update agent documentation | 6, 8, 9 |

## Migration Guide for Users

**Before:**
```bash
tpg parent ts-abc ep-xyz
tpg set-status ts-abc done
```

**After:**
```bash
tpg edit ts-abc --parent ep-xyz
tpg done ts-abc  # (no change)
```

**New capabilities:**
```bash
# Bulk move tasks to epic
tpg edit --select-label feature-x --parent ep-xyz

# Create epic with worktree metadata
tpg add "Big Feature" -e --worktree
# (then follow printed instructions to create worktree)

# Use existing branch/worktree
tpg add "Another epic" -e --worktree --branch feature-x
```
