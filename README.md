# tasks

Lightweight task management for AI agents. SQLite-backed, CLI-driven.

## Install

```bash
go install github.com/baiirun/dotworld-tasks/cmd/tasks@latest
```

Or build from source:

```bash
git clone https://github.com/baiirun/dotworld-tasks.git
cd dotworld-tasks
go build -o tasks ./cmd/tasks
```

## Quick Start

```bash
# Initialize database (creates ~/.world/tasks/tasks.db)
tasks init

# Create a task
tasks add "Implement user authentication" -p myproject --priority 1
# Output: ts-a1b2c3

# See what's ready to work on
tasks ready -p myproject

# Start working
tasks start ts-a1b2c3

# Log progress
tasks log ts-a1b2c3 "Added JWT token generation"

# Mark complete
tasks done ts-a1b2c3
```

## CLI Reference

### Core Commands

| Command | Description |
|---------|-------------|
| `tasks init` | Initialize the database |
| `tasks add <title>` | Create a task (returns ID) |
| `tasks list` | List all tasks |
| `tasks show <id>` | Show task details, logs, and dependencies |
| `tasks ready` | Show tasks ready for work (open + deps met) |
| `tasks status` | Project overview for agent spin-up |

### Work Commands

| Command | Description |
|---------|-------------|
| `tasks start <id>` | Set task to in_progress |
| `tasks done <id>` | Mark task complete |
| `tasks block <id> <reason>` | Mark blocked with reason |
| `tasks log <id> <message>` | Add timestamped log entry |
| `tasks append <id> <text>` | Append to task description |

### Organization

| Command | Description |
|---------|-------------|
| `tasks dep <id> --on <other>` | Add dependency (id depends on other) |
| `tasks add -e <title>` | Create an epic instead of task |

### Flags

| Flag | Commands | Description |
|------|----------|-------------|
| `-p, --project` | all | Filter/set project scope |
| `-e, --epic` | add | Create epic instead of task |
| `--priority` | add | Priority: 1=high, 2=medium (default), 3=low |
| `--status` | list | Filter by status |

## ID Format

IDs are auto-generated with type prefixes:
- `ts-XXXXXX` — tasks (e.g., `ts-a1b2c3`)
- `ep-XXXXXX` — epics (e.g., `ep-f0a20b`)

## Agent Workflow

### Spin-up (new agent joining)

```bash
# Get project overview
tasks status -p myproject

# Output:
# Project: myproject
#
# Summary: 3 open, 1 in progress, 0 blocked, 2 done (2 ready)
#
# In progress:
#   [ts-a1b2c3] Implement auth middleware
#
# Ready for work:
#   [ts-d4e5f6] Add login endpoint (pri 1)
#   [ts-g7h8i9] Write auth tests (pri 2)
```

### Pick up work

```bash
# See what's unblocked
tasks ready -p myproject

# Read full context
tasks show ts-d4e5f6

# Claim it
tasks start ts-d4e5f6
```

### While working

```bash
# Log progress (timestamped audit trail)
tasks log ts-d4e5f6 "Implemented POST /login"
tasks log ts-d4e5f6 "Added rate limiting"

# Update description with decisions/context
tasks append ts-d4e5f6 "Using bcrypt for password hashing"
```

### Finish or hand off

```bash
# Complete
tasks done ts-d4e5f6

# Or mark blocked for next agent
tasks block ts-d4e5f6 "Need API spec for OAuth flow"
```

## Data Model

- **Items**: Tasks or epics with title, description, status, priority
- **Status**: `open` → `in_progress` → `done` (or `blocked`)
- **Dependencies**: Task A can depend on Task B (A is blocked until B is done)
- **Logs**: Timestamped audit trail per item
- **Projects**: String tag to scope work (e.g., "gaia", "myapp")

Database location: `~/.world/tasks/tasks.db`

## Design

See [DESIGN.md](DESIGN.md) for architecture and schema details.

## Goals

1. Track tasks within larger work (epics)
2. Progress reports for current work and what's left
3. Split work for parallel agents
4. Track dependencies for ordering
5. Prioritize work
6. Store context so agents can resume where others left off

## Non-Goals

- Git sync (single-player, local only)
- Multiplayer / collaboration
- Complex workflows

## License

MIT
