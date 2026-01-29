# tpg

SQLite-backed task and tacit knowledge management for AI agents. Agents lose context between sessions. tpg persists tasks, progress logs, and learnings so agents can pick up where others left off.

**Opencode-first** — tpg is designed primarily for Opencode.

## Install

### Go install

Requires Go 1.24+.

```bash
go install github.com/taxilian/tpg/cmd/tpg@v0.4.1
```

### Build from source

Requires Go 1.24+.

```bash
git clone https://github.com/taxilian/tpg.git
cd tpg
go build -o tpg ./cmd/tpg
./tpg --help
```

Or install to your `$GOBIN`:

```bash
git clone https://github.com/taxilian/tpg.git
cd tpg
go install ./cmd/tpg
tpg --help
```

### Development mode

For development, install so changes are reflected immediately:

```bash
git clone https://github.com/taxilian/tpg.git
cd tpg
go install ./cmd/tpg
```

After making code changes, reinstall:

```bash
go install ./cmd/tpg
```

This builds and installs to `$GOBIN` (or `$GOPATH/bin`). Make sure it's in your `$PATH`.

## Quick Start

```bash
# Initialize database (creates .tpg/tpg.db in current directory)
tpg init

# Set up Opencode integration (recommended)
tpg onboard

# Create a task
tpg add "Implement user authentication"
# Output: ts-a1b2c3

# See what's ready to work on
tpg ready

# Start working
tpg start ts-a1b2c3

# Log progress
tpg log ts-a1b2c3 "Added JWT token generation"

# Mark complete (requires results message)
tpg done ts-a1b2c3 "Implemented JWT auth with refresh tokens"
```

## CLI Reference

### Core Commands

| Command | Description |
|---------|-------------|
| `tpg init` | Initialize the database (supports `--prefix`, `--epic-prefix`) |
| `tpg onboard` | Set up tpg integration for Opencode |
| `tpg add <title>` | Create a task (returns ID) |
| `tpg list` | List all tasks |
| `tpg show <id>` | Show task details, logs, deps, suggested concepts |
| `tpg ready` | Show tasks ready for work (open + deps met) |
| `tpg status` | Project overview for agent spin-up |
| `tpg prime` | Output context for agent hooks |
| `tpg compact` | Output compaction workflow guidance |
| `tpg tui` | Launch interactive terminal UI (alias: `tpg ui`) |

### Work Commands

| Command | Description |
|---------|-------------|
| `tpg start <id>` | Set task to in_progress |
| `tpg done <id>` | Mark task complete |
| `tpg cancel <id> [reason]` | Cancel task (close without completing) |
| `tpg block <id> <reason>` | Mark blocked with reason |
| `tpg log <id> <message>` | Add timestamped log entry |
| `tpg append <id> <text>` | Append to task description |
| `tpg desc <id> <text>` | Replace task description |
| `tpg edit <id>` | Edit description in $TPG_EDITOR (defaults to nvim, nano, vi) |

### Organization

| Command | Description |
|---------|-------------|
| `tpg parent <id> <epic-id>` | Set task's parent epic |
| `tpg dep <id> blocks <other>` | Add blocking relationship (other blocked until id done) |
| `tpg dep <id> after <other>` | Add dependency (id depends on other) |
| `tpg dep <id> list` | Show all dependencies for a task |
| `tpg dep <id> remove <other>` | Remove dependency between tasks |
| `tpg graph` | Show dependency graph |
| `tpg projects` | List all projects |
| `tpg add -e <title>` | Create an epic instead of task |

### Labels

| Command | Description |
|---------|-------------|
| `tpg labels` | List all labels for a project |
| `tpg labels add <name>` | Create a new label |
| `tpg labels rm <name>` | Delete a label |
| `tpg labels rename <old> <new>` | Rename a label |
| `tpg label <id> <name>` | Add label to task (creates if needed) |
| `tpg unlabel <id> <name>` | Remove label from task |

### Flags

| Flag | Commands | Description |
|------|----------|-------------|
| `--project` | all | Filter/set project scope |
| `-p, --priority` | add | Priority: 1=high, 2=medium (default), 3=low |
| `-e, --epic` | add | Create epic instead of task |
| `-l, --label` | add, list, ready, status | Attach label at creation / filter by label (repeatable, AND logic) |
| `--parent` | add, list | Set parent epic at creation / filter by parent |
| `--blocks` | add | Set task this will block at creation |
| `--status` | list | Filter by status |
| `--type` | list | Filter by item type (task, epic) |
| `--blocking` | list | Show items that block the given ID |
| `--blocked-by` | list | Show items blocked by the given ID |
| `--has-blockers` | list | Show only items with unresolved blockers |
| `--no-blockers` | list | Show only items with no blockers |
| `--all` | status | Show all ready tasks (default: limit to 10) |

## ID Format

IDs are auto-generated with type prefixes:
- `ts-XXXXXX` — tasks (e.g., `ts-a1b2c3`)
- `ep-XXXXXX` — epics (e.g., `ep-f0a20b`)

## Agent Workflow

### Spin-up (new agent joining)

```bash
# Get project overview
tpg status

# Output:
# Summary: 3 open, 1 in progress, 0 blocked, 2 done (2 ready)
#
# In progress:
#   [ts-a1b2c3] Implement auth middleware
#
# Ready for work:
#   [ts-d4e5f6] Add login endpoint (pri 1)
#   [ts-g7h8i9] Write auth tests (pri 2)

# Or filter by project if you use them:
# tpg status -p myproject
```

### Pick up work

```bash
# See what's unblocked
tpg ready

# Read full context
tpg show ts-d4e5f6

# Claim it
tpg start ts-d4e5f6
```

### While working

```bash
# Log progress (timestamped audit trail)
tpg log ts-d4e5f6 "Implemented POST /login"
tpg log ts-d4e5f6 "Added rate limiting"

# Append to description with decisions/context
tpg append ts-d4e5f6 "Using bcrypt for password hashing"

# Replace description entirely
tpg desc ts-d4e5f6 "Implement login endpoint with JWT auth and rate limiting"

# Or edit in your editor
tpg edit ts-d4e5f6
```

### Finish or hand off

```bash
# Complete
tpg done ts-d4e5f6

# Or cancel if no longer needed
tpg cancel ts-d4e5f6 "Requirements changed"

# Or mark blocked for next agent
tpg block ts-d4e5f6 "Need API spec for OAuth flow"

# If task is part of an epic, update the epic too
tpg append ep-a1b2c3 "Completed auth endpoint, next: write tests"
```

### Dependencies

Use dependencies to enforce task ordering. A task with unmet dependencies won't appear in `tpg ready`.

```bash
# Create a task that blocks another (at creation time)
tpg add "Build API" --blocks ts-frontend
# New task blocks ts-frontend, so ts-frontend can't start until the new task is done

# Add dependency to existing tasks
tpg dep ts-backend blocks ts-frontend
# ts-frontend cannot start until ts-backend is done

# View dependencies for a task
tpg dep ts-frontend list

# Remove a dependency
tpg dep ts-backend remove ts-frontend

# View all dependencies
tpg graph

# Output:
# ts-frontend [open] Build frontend components
#   └── ts-backend [in_progress] Implement API endpoints
```

The `ready` command automatically filters out tasks with unmet dependencies, so agents only see work they can actually start.

### Labels

Labels are tags for categorizing tasks (bug, feature, refactor, etc). They're project-scoped and identified by name.

```bash
# Create labels (or they're auto-created on first use)
tpg labels add bug -p myproject
tpg labels add feature -p myproject

# Attach labels to tasks
tpg label ts-a1b2c3 bug
tpg label ts-a1b2c3 urgent

# Or attach at creation
tpg add "Fix login crash" -p myproject -l bug -l urgent

# Filter by labels (AND logic - must have all specified)
tpg list -p myproject -l bug
tpg list -p myproject -l bug -l urgent
tpg ready -p myproject -l feature

# Remove a label
tpg unlabel ts-a1b2c3 urgent

# List all labels in a project
tpg labels -p myproject
```

Labels appear in list output and task details as `[bug] [urgent]`.

### Epics

Group related tasks under an epic for organization:

```bash
# Create an epic
tpg add "Authentication system" -p myproject -e
# Output: ep-a1b2c3

# Create task under epic (at creation time)
tpg add "Implement login" -p myproject --parent ep-a1b2c3

# Or assign existing tasks to the epic
tpg parent ts-d4e5f6 ep-a1b2c3
tpg parent ts-g7h8i9 ep-a1b2c3

# View task with parent
tpg show ts-d4e5f6
# Output includes: Parent: ep-a1b2c3
```

### Templates

Templates define **standardized ways to solve problems**. A "tdd" template encodes the standard approach for test-driven development. A "discovery" template defines how to investigate unknowns. A "bug-fix" template captures the proven method for diagnosing and resolving issues.

When you instantiate a template, tpg creates a parent epic with child tasks that follow the standardized approach, including proper dependencies between steps.

#### Template Locations

Templates are searched in multiple locations, in priority order:

1. **Project:** `.tpg/templates/` (searched upward from current directory)
2. **User:** `~/.config/tpg/templates/`
3. **Global:** `~/.config/opencode/tpg-templates/`

Templates from more local locations override templates with the same ID from global locations.

```bash
# List available templates
tpg template list

# Show template details
tpg template show tdd

# See all template search locations
tpg template locations

# Create project-local templates directory
mkdir -p .tpg/templates
```

#### Template Format

Templates use [Go's text/template](https://pkg.go.dev/text/template) syntax.

```yaml
# .tpg/templates/tdd.yaml
title: "TDD"
description: "Test-driven development"

variables:
  feature_name:
    description: "Name of the feature"
  constraints:
    description: "Hard constraints (optional)"
    optional: true

steps:
  - id: write-tests
    title: "Write tests: {{.feature_name}}"
    description: |
      Write tests for {{.feature_name}}.
      {{- if hasValue .constraints}}
      **Constraints:** {{.constraints}}
      {{- end}}

  - id: implement
    title: "Implement: {{.feature_name}}"
    depends: [write-tests]
    description: "Implement to make tests pass."
```

#### Key Features

- **Required vs optional variables**: Variables are required by default; set `optional: true` for optional ones
- **Conditionals**: `{{if hasValue .var}}...{{end}}` to omit sections when empty
- **Whitespace control**: `{{-` and `-}}` to trim newlines
- **Step dependencies**: `depends: [step-id]` creates task dependencies

See **[docs/TEMPLATES.md](docs/TEMPLATES.md)** for complete documentation.

#### Using Templates

```bash
# Instantiate a template
tpg add "User Auth Feature" --template my-workflow \
  --var 'feature_name="user authentication"' \
  --var 'constraints="must use bcrypt"'

# Output: ep-abc123 (parent epic with child tasks)

# View created tasks
tpg list --parent ep-abc123
tpg show ts-def456
```

**Variable values are JSON-encoded strings** to support multi-line content:

```bash
# Multi-line variable value
tpg add "Complex Feature" --template tdd-workflow \
  --var 'requirements="1. Validate email format\n2. Hash passwords with bcrypt\n3. Generate JWT tokens"'
```

#### Template Change Detection

Each instantiated task stores a hash of the template at creation time. If you modify the template later:

- `tpg show` renders using the **latest** template content
- A notice appears if the template has changed since instantiation
- This allows templates to evolve without breaking existing tasks

## Context Engine

The context engine captures tacit knowledge—things agents learn that aren't obvious from the code. This knowledge persists across sessions, helping future agents avoid rediscovering the same insights.

### Data Model

**Concepts** are knowledge categories within a project:
```
auth          - "Token lifecycle, refresh, session coupling"
database      - "SQLite patterns, schema migrations"
config        - "Environment loading, precedence rules"
```

**Learnings** are specific insights tagged with concepts:
```
lrn-abc123: Token refresh has race condition
  Detail: The mutex only protects token write, not refresh check. See PR #423.
  Concepts: auth, concurrency
  Files: auth/token.go
  Task: ts-def456
```

### Two-Phase Retrieval

Agents retrieve context in phases to minimize token usage:

```
┌─────────────────────────────────────────────────────────────┐
│ PHASE 1: Discovery                                          │
│   tpg show <task>                                           │
│     → Task details, logs, deps                              │
│     → Suggested concepts: auth (3), config (2)              │
│                                                             │
│   Agent decides which concepts are relevant                 │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ PHASE 2: Scan                                               │
│   tpg context -c auth --summary                             │
│     → auth: Token lifecycle, refresh, session coupling      │
│     → lrn-abc: Token refresh has race condition             │
│     → lrn-def: Auth tokens expire after 1 hour              │
│                                                             │
│   Agent sees concept summary, then learning one-liners      │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ PHASE 3: Load                                               │
│   tpg context --id lrn-abc                                  │
│     → Full detail, files, linked task                       │
└─────────────────────────────────────────────────────────────┘
```

Each phase filters, so agents only load what's actually relevant.

### Commands

| Command | Description |
|---------|-------------|
| `tpg concepts` | List concepts for a project |
| `tpg context -c <name>` | Retrieve learnings by concept(s) |
| `tpg context -q <query>` | Full-text search on learnings |
| `tpg learn <summary>` | Log a new learning |
| `tpg learn edit <id>` | Edit a learning's summary or detail |
| `tpg learn stale <id>` | Mark learning as outdated |
| `tpg learn rm <id>` | Delete a learning |

#### Retrieval Examples

```bash
# List concepts to see what knowledge exists
tpg concepts -p myproject
# NAME          LEARNINGS  LAST UPDATED  SUMMARY
# auth                  3  2h ago        Token lifecycle, refresh
# database              2  1d ago        SQLite patterns

# Retrieve by concept (union of multiple concepts)
tpg context -c auth -c database -p myproject

# Full-text search when you don't know the concept
tpg context -q "race condition" -p myproject

# Include stale learnings for historical context
tpg context -c auth --include-stale -p myproject
```

### Logging Learnings (Reflection)

Log learnings at the end of a session during reflection. This is more efficient than logging during work because:

- The learning is validated through implementation
- You can synthesize related discoveries into one insight
- You know what's signal vs noise

```bash
# Basic learning with concepts
tpg learn "Token refresh has race condition" -c auth -c concurrency -p myproject

# With related files
tpg learn "Config loads from env first, then file" -c config -p myproject -f config.go

# With full detail
tpg learn "summary" -c concept -p myproject --detail "full explanation..."
```

**What makes a good learning?**

- Things that aren't obvious from reading the code
- Gotchas, edge cases, "why" decisions
- Context that would help the next agent

**What to avoid logging:**

- Things already documented in code comments
- Obvious behavior that code makes clear
- Temporary workarounds (mark as stale instead)

#### Concept Hygiene

- **Reuse existing concepts** — check `tpg concepts` before creating new ones
- **Create sparingly** — prefer broader concepts over narrow ones
- **Use clear names** — `auth` not `authentication-and-authorization`

### Grooming and Compaction

Over time, learnings accumulate. Without periodic grooming:
- **Redundant entries** waste context tokens when retrieved
- **Stale learnings** mislead agents with outdated information
- **Unclear summaries** reduce retrieval effectiveness
- **Fragmented insights** are harder to discover and use

The `tpg prime` command automatically flags concepts that may need attention (5+ learnings, or learnings older than 7 days).

#### The Compaction Workflow

Run `tpg compact` to get guided prompting for grooming. The workflow has two phases:

**Phase 1: Discovery**
```bash
tpg concepts -p myproject --stats    # See concept distribution
tpg context -p myproject --summary   # Scan all one-liners
```

Flag candidates: redundant (similar summaries), stale (old or outdated), low quality (vague, not actionable), fragmented (should be combined).

**Phase 2: Selection & Grooming**
```bash
tpg context --id lrn-abc123          # Load specific learning
tpg context -c auth -p myproject --json  # Load all for a concept
```

Then apply actions:
- **Archive**: `tpg learn stale lrn-a lrn-b --reason "Consolidated"`
- **Update**: `tpg learn edit lrn-abc --summary "Clearer summary"`
- **Consolidate**: Archive originals, create new combined learning

#### Marking Learnings Stale

When a learning becomes outdated but is still useful for reference:

```bash
tpg learn stale lrn-abc123 --reason "Refactored in v2"
```

Stale learnings are excluded by default but can be included with `--include-stale`.

#### Concept Grooming

```bash
# Update a concept's summary
tpg concepts auth -p myproject --summary "Token lifecycle and session management"

# Rename a fragmented concept
tpg concepts authn -p myproject --rename auth
```

### Resources & Inspiration

The context engine design draws from several projects and papers:

- **[CASS Memory System](https://github.com/Dicklesworthstone/cass_memory_system)** — Lesson extraction and rule validation through evidence gates. Influenced our approach to learning quality (actionable, specific, pattern-based).

- **[AgentFS](https://github.com/tursodatabase/agentfs)** — SQLite-based agent memory with audit trails. Validated our choice of SQLite for durability and the importance of linking learnings to tasks.

- **[Dynamic Context Discovery](https://cursor.com/blog/dynamic-context-discovery)** (Cursor) — Two-phase retrieval with stubs in context, full content on-demand. Directly inspired our `--summary` → `--id` pattern for 46%+ token reduction.

- **[Everything is Context](https://arxiv.org/abs/2512.05470)** (Xu et al., 2024) — File-system abstraction for context engineering. Reinforced concepts-over-files approach and the value of structured knowledge retrieval.

## Opencode Integration

The `tpg onboard` command sets up Opencode integration:

1. Adds a Task Tracking section to `AGENTS.md`
2. Installs an Opencode plugin (`.opencode/plugins/tpg.ts`) that:
   - Injects `tpg prime` context into the system prompt for each session
   - Re-injects context during compaction so task state survives
   - Adds `AGENT_ID` and `AGENT_TYPE` environment variables to tpg commands

This ensures agents maintain context about tasks across sessions and compaction boundaries.

**Using a different agent?** (Cursor, Claude Code, Codex, Gemini, etc.)

1. Copy the Task Tracking snippet from `AGENTS.md` to your agent's instruction file
2. If your tool supports hooks, add `tpg prime` to session start
3. If no hooks, run `tpg prime` and paste output into agent context

### What `tpg prime` outputs

- **SESSION CLOSE PROTOCOL**: Mandatory checklist for logging progress and updating status before ending sessions
- **Core Rules**: When to use `tpg` (strategic, cross-session) vs TodoWrite (tactical, within-session)
- **Essential Commands**: Quick reference grouped by workflow phase
- **Current State**: Live summary of open, in-progress, and blocked tasks

This ensures agents never forget the workflow, even after context compaction.

## Interactive TUI

Launch with `tpg tui` (or `tpg ui`):

```
tpg  12/47 items  status:oib

◐ ts-234d9f  Set up Bubble Tea scaffold       [tasks]
○ ts-9566cd  Task list view with indicators   [tasks]
○ ts-f39592  Vim keybind navigation           [tasks]
```

### Navigation

| Key | Action |
|-----|--------|
| `j/k` or arrows | Move up/down |
| `g/G` or Home/End | Jump to first/last |
| `enter` or `l` | View task details |
| `esc` or `h` | Go back to list |
| `q` | Quit |

### Actions

| Key | Action |
|-----|--------|
| `s` | Start task |
| `d` | Mark done |
| `b` | Block (prompts for reason) |
| `L` | Log progress (prompts for message) |
| `c` | Cancel task |
| `D` | Delete task |
| `a` | Add dependency |
| `r` | Refresh |

### Filtering

| Key | Action |
|-----|--------|
| `/` | Search by title/ID/description |
| `p` | Filter by project (partial match) |
| `t` | Filter by label (partial match while typing, repeat to add more) |
| `1-5` | Toggle status: 1=open 2=in_progress 3=blocked 4=done 5=canceled |
| `0` | Show all statuses |
| `esc` | Clear filters |

## Data Model

- **Items**: Tasks or epics with title, description, status, priority
- **Status**: `open` → `in_progress` → `done` (or `blocked`, `canceled`)
- **Dependencies**: Task A can depend on Task B (A is blocked until B is done)
- **Labels**: Tags for categorization (bug, feature, refactor, etc), project-scoped
- **Logs**: Timestamped audit trail per item
- **Projects**: String tag to scope work (e.g., "gaia", "myapp")
- **Concepts**: Knowledge categories within a project (e.g., "auth", "database")
- **Learnings**: Specific insights tagged with concepts, with summary and detail

Database location: `.tpg/tpg.db` (in current directory, or override with `TPG_DB` environment variable)

## Environment Variables

| Variable | Description |
|----------|-------------|
| `TPG_DB` | Override default database location |
| `TPG_EDITOR` | Editor for `tpg edit` command (defaults to nvim, nano, vi) |

## Goals

1. Track tasks within larger work (epics)
2. Progress reports for current work and what's left
3. Split work for parallel agents
4. Track dependencies for ordering
5. Prioritize work
6. Store context so agents can resume where others left off
7. Capture tacit knowledge that persists across sessions

## Non-Goals

- Git sync (single-player, local only)
- Multiplayer / collaboration
- Complex workflows

## License

MIT
