# Agent-Aware Task Tracking

Tpg supports agent-aware task tracking when running in AI agent environments. This feature allows multiple agents to work on tasks concurrently while maintaining visibility into who is working on what.

## Overview

When the `$AGENT_ID` environment variable is set, tpg tracks which agent is working on which tasks. This enables:

- **Task ownership tracking**: See which agent claimed which task
- **Multi-agent coordination**: Distinguish between "my work" and "other agents' work"
- **Agent project history**: Remember which projects each agent last accessed
- **Silent takeover**: Agents can claim tasks from each other without errors

## Environment Variables

### `AGENT_ID` (required for tracking)
Unique identifier for the agent. When set, task assignments are tracked.

Example: `agent-abc123`, `session-2024-01-28-user`

### `AGENT_TYPE` (optional)
Type of agent. Used to distinguish between main agents and subagents.

Values:
- `general` - Main agent (default if type not specified)
- `explore` - Subagent for code exploration
- Other values are treated as subagents

## How It Works

### Task Assignment

When an agent runs `tpg start <id>`, the task's `agent_id` field is set to the current `$AGENT_ID`:

```bash
export AGENT_ID="agent-123"
tpg start ts-abc123
# Task ts-abc123 is now assigned to agent-123
```

### Task Release

When a task reaches a terminal state, the agent assignment is cleared:

- `tpg done <id>` - clears assignment
- `tpg block <id>` - clears assignment  
- `tpg cancel <id>` - clears assignment

### Silent Takeover

If Agent A is working on a task and Agent B runs `tpg start` on the same task, Agent B silently takes over. No error is raised - this is intentional to allow agents to freely pick up work.

```bash
# Agent A claims task
export AGENT_ID="agent-a"
tpg start ts-abc123

# Agent B takes over (no error)
export AGENT_ID="agent-b"
tpg start ts-abc123
# Task is now assigned to agent-b
```

## Status Reports

The `tpg status` command separates in-progress tasks by agent:

```bash
export AGENT_ID="agent-123"
tpg status
```

Output includes:
- **Your work**: Tasks assigned to `$AGENT_ID`
- **Other agents**: Count of tasks assigned to other agents
- **Unassigned**: Tasks in progress without agent assignment

## Prime Context

When `$AGENT_ID` is set, `tpg prime` includes agent-specific context:

```markdown
## Your Work
- [ts-abc123] Implement authentication
- [ts-def456] Fix bug in parser

2 in progress (other agents)
```

This helps agents understand the current work distribution at session start.

## Project Access History

Tpg maintains a history of which projects each agent accessed. This is used internally for context retrieval and may be exposed in future features.

History is limited to the last 20 project accesses per agent and is cleaned up automatically.

### Recording Access

Project access is recorded when agents run:
- `tpg start <id>`
- `tpg show <id>`
- `tpg ready [-p project]`
- `tpg status [-p project]`
- `tpg prime`

## Graceful Degradation

When `$AGENT_ID` is not set:
- All agent tracking features become no-ops
- Tasks can still be started, completed, etc.
- Status reports show all in-progress tasks without separation
- Prime output omits agent-specific sections

This ensures tpg works identically for human users and non-agent environments.

## Database Schema

Agent tracking uses two new fields on the `items` table:

- `agent_id` (TEXT, nullable) - ID of agent currently working on task
- `agent_last_active` (DATETIME, nullable) - When agent last touched task

And a new `agent_sessions` table:

```sql
CREATE TABLE agent_sessions (
    agent_id TEXT NOT NULL,
    project TEXT NOT NULL,
    last_active DATETIME NOT NULL,
    PRIMARY KEY (agent_id, project)
);
```

## Implementation Notes

### Why Silent Takeover?

We chose silent takeover (no error when claiming an assigned task) because:

1. **Agent autonomy**: Agents should be free to pick up work without coordination
2. **Stale state**: Previous agent may have crashed or been interrupted
3. **Simplicity**: No complex locking or reservation system needed
4. **Trust**: Assumes agents are working in good faith

If strict task exclusivity is needed in the future, it can be added via a flag or separate mechanism.

### Subagent Detection

Agents with `AGENT_TYPE != "general"` are considered subagents. This distinction may be used for:

- Filtering subagent work from status reports
- Different context delivery strategies
- Analytics on subagent usage patterns

Currently, subagent detection is implemented but not actively used in core commands.

## Future Enhancements

Potential future additions:

- `tpg agents` - List all active agents and their current work
- Agent handoff protocols (explicit task transfer)
- Agent activity timeout (auto-release stale tasks)
- Agent collaboration features (shared context, notes)
- Subagent work aggregation in reports

## Example Session

```bash
# Agent starts session
export AGENT_ID="agent-alpha"
export AGENT_TYPE="general"

# See available work
tpg ready
# 5 tasks ready

# Claim task
tpg start ts-abc123
# Started ts-abc123

# Check status
tpg status
# Your work:
#   [ts-abc123] Implement authentication
# 
# 2 in progress (other agents)
# 3 ready

# Complete work
tpg done ts-abc123 "Implemented OAuth flow"
# Completed ts-abc123

# Check status again
tpg status
# (no active work shown)
# 2 in progress (other agents)
# 4 ready
```

## See Also

- [PRIME_TEMPLATES.md](PRIME_TEMPLATES.md) - Customizing prime output for agents
- [AGENTS.md](../AGENTS.md) - Instructions for AI agents using tpg
