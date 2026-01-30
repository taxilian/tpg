# CLI Reference

## Core Commands

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

## Work Commands

| Command | Description |
|---------|-------------|
| `tpg start <id>` | Set task to in_progress |
| `tpg done <id> [message]` | Mark task complete |
| `tpg cancel <id> [reason]` | Cancel task (close without completing) |
| `tpg block <id> <reason>` | Mark blocked (requires `--force`; prefer dependencies instead) |
| `tpg log <id> <message>` | Add timestamped log entry |
| `tpg append <id> <text>` | Append to task description |
| `tpg desc <id> <text>` | Replace task description |
| `tpg edit <id>` | Edit description in $TPG_EDITOR (defaults to nvim, nano, vi) |
| `tpg merge <source> <target>` | Merge duplicate tasks (requires `--yes-i-am-sure`) |

## Organization

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

ID length is configurable via `id_length` in `.tpg/config.json` (default: 3 characters, base-36 alphabet `[0-9a-z]`).

## Environment Variables

| Variable | Description |
|----------|-------------|
| `TPG_DB` | Override default database location |
| `TPG_EDITOR` | Editor for `tpg edit` command (defaults to nvim, nano, vi) |

## Data Model

- **Items**: Tasks or epics with title, description, status, priority
- **Status**: `open` -> `in_progress` -> `done` (or `blocked`, `canceled`)
- **Dependencies**: Task A can depend on Task B (A is blocked until B is done)
- **Labels**: Tags for categorization (bug, feature, refactor, etc), project-scoped
- **Logs**: Timestamped audit trail per item
- **Projects**: String tag to scope work (e.g., "gaia", "myapp")
- **Concepts**: Knowledge categories within a project (e.g., "auth", "database")
- **Learnings**: Specific insights tagged with concepts, with summary and detail

Database location: `.tpg/tpg.db` (in current directory, or override with `TPG_DB`).
