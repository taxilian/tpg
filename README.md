# tpg

An issue tracker for AI agents. Agents lose context between sessions — tpg gives them a persistent backlog so work survives across sessions, handoffs, and context compaction.

## The Problem

AI coding agents are stateless. Every new session starts from scratch. When work spans multiple sessions — or multiple agents — there's no shared record of what's been done, what's next, or why decisions were made. Progress logs, blockers, and dependencies live only in chat context that gets compacted away.

## What tpg Does

**Issue tracking** — Tasks, epics, priorities, dependencies, labels. `tpg ready` shows unblocked work. `tpg status` gives the full picture. Agents (or humans) create, claim, log progress, and close tasks through a simple CLI.

**Context preservation** — Every task carries a description, timestamped progress logs, and dependency relationships. When a new agent spins up, `tpg status` and `tpg show <id>` give it everything it needs to continue where the last session left off.

**Dependency management** — Tasks can block other tasks. `tpg ready` only surfaces work whose dependencies are met, so agents don't start things out of order.

**Agent integration** — `tpg onboard` wires up your agent tool (Opencode, etc.) so that `tpg prime` context is injected into every session automatically. Agents get the backlog on spin-up and follow a close protocol on shutdown.

## Install

Requires Go 1.24+.

```bash
go install github.com/taxilian/tpg/cmd/tpg@v0.10.0
```

Or build from source:

```bash
git clone https://github.com/taxilian/tpg.git
cd tpg
go install ./cmd/tpg
```

## Quick Start

```bash
tpg init                  # Create .tpg/tpg.db in current directory
tpg onboard               # Set up agent integration (recommended)

tpg add "Implement auth"  # Create a task → ts-a1b
tpg ready                 # See unblocked work
tpg start ts-a1b          # Claim it
tpg log ts-a1b "Added JWT generation"
tpg done ts-a1b "JWT auth with refresh tokens"
```

### Organize with dependencies and epics

```bash
tpg add "Auth system" --type epic           # Create an epic → ep-c3d
tpg add "Login endpoint" --parent ep-c3d    # Task under epic
tpg add "Auth tests" --parent ep-c3d        # Another task
tpg dep ts-login blocks ts-tests            # Tests wait for login
tpg ready                                   # Only shows unblocked work
```

### Preserve context across sessions

```bash
tpg log ts-a1b "Chose bcrypt over argon2 — simpler, sufficient for our scale"
tpg status          # Agent spin-up: see everything in flight
tpg show ts-a1b     # Full context: description, logs, deps
```

### Use templates for repeatable workflows

```bash
tpg template list
tpg add "User Auth" --template tdd --var 'feature_name="user auth"'
# Creates epic with child tasks and dependencies
```

## Core Concepts

- **Types** — Arbitrary work item types (task, epic, bug, story, etc.). Any type can have children.
- **IDs** — Auto-generated with configurable prefixes (default: `ts-xxx` for tasks, `ep-xxx` for epics)
- **Dependencies** — Task A blocks Task B; `ready` respects this
- **Labels** — Tags for categorization (bug, feature, etc.)
- **Logs** — Timestamped progress entries per task
- **Templates** — Reusable workflows that expand into parent + child tasks

## Key Commands

| Command | Purpose |
|---------|---------|
| `tpg ready` | What can be worked on right now |
| `tpg status` | Full project overview |
| `tpg show <id>` | Task details, logs, dependencies |
| `tpg add <title>` | Create task (or use `--type epic` for epics) |
| `tpg start <id> [--resume]` | Claim work (use `--resume` if already in progress) |
| `tpg log <id> <msg>` | Record progress |
| `tpg done <id> [msg]` | Complete task |
| `tpg dep <id> blocks <other>` | Set dependency |
| `tpg history <id>` | Chronological task timeline |
| `tpg prime` | Output context for agent hooks |
| `tpg tui` | Interactive terminal UI |

## Documentation

| Doc | Contents |
|-----|----------|
| [CLI Reference](docs/CLI.md) | All commands, flags, environment variables |
| [Context Engine](docs/CONTEXT.md) | Concepts, learnings, knowledge preservation |
| [Templates](docs/TEMPLATES.md) | Template format, variables, examples |
| [TUI](docs/TUI.md) | Interactive terminal UI navigation and actions |
| [Agent Integration](docs/INTEGRATION.md) | Opencode setup, other agents, `tpg prime` |
| [Spec](docs/SPEC.md) | Design specification |

## Goals

1. Persistent issue tracking that survives agent session boundaries
2. Dependency-aware work queue (`ready` only shows unblocked tasks)
3. Context preservation through logs and descriptions
4. Support for parallel agent coordination via epics and dependencies
5. Simple CLI that agents can use without special tooling

## Non-Goals

- Git sync (single-machine, local-only)
- Multiplayer / collaboration
- Complex workflow engines

## License

MIT
