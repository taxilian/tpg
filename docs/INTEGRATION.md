# Agent Integration

tpg is designed to be used by AI coding agents. The `tpg onboard` command automates setup for Opencode, but tpg works with any agent that can run CLI commands.

## Opencode Setup

```bash
tpg onboard
```

This does two things:

1. **Adds a Task Tracking section to `AGENTS.md`** with workflow instructions and key commands
2. **Installs an Opencode plugin** (`.opencode/plugins/tpg.ts`) that:
   - Injects `tpg prime` context into the system prompt for each session
   - Re-injects context during compaction so task state survives
   - Adds `AGENT_ID` and `AGENT_TYPE` environment variables to tpg commands

This ensures agents maintain context about tasks across sessions and compaction boundaries.

## Other Agents

For Cursor, Claude Code, Codex, Gemini, or any other tool:

1. Copy the Task Tracking snippet from `AGENTS.md` to your agent's instruction file
2. If your tool supports hooks, add `tpg prime` to session start
3. If no hooks, run `tpg prime` and paste output into agent context

## What `tpg prime` Outputs

- **Session close protocol**: Mandatory checklist for logging progress and updating status before ending sessions
- **Core rules**: When to use `tpg` (strategic, cross-session) vs local task tracking (tactical, within-session)
- **Essential commands**: Quick reference grouped by workflow phase
- **Current state**: Live summary of open, in-progress, and blocked tasks

This ensures agents never forget the workflow, even after context compaction.

## Agent Workflow

### Spin-up (new agent session)

```bash
tpg status              # Project overview: what's open, in progress, blocked
tpg ready               # What's unblocked and available
tpg show ts-a1b2c3      # Full context for a specific task
```

### While working

```bash
tpg start ts-a1b2c3                           # Claim work
tpg log ts-a1b2c3 "Implemented POST /login"   # Log progress
tpg append ts-a1b2c3 "Using bcrypt for hashing"  # Add context to description
```

### Finish or hand off

```bash
tpg done ts-a1b2c3 "Implemented JWT auth"              # Complete
tpg cancel ts-a1b2c3 "Requirements changed"             # Cancel if no longer needed

# Hit a blocker? Create a task for it with --blocks:
tpg add "Blocker: Need OAuth API spec" -p 1 --blocks ts-a1b2c3
# System auto-reverts ts-a1b2c3 to open and logs the reason
```

### Logging guidelines

Log things that would matter if someone else picks up the task:
- Discoveries (answered key questions, found existing code, unexpected constraints)
- Design decisions with rationale (chose X because Y)
- Finishing key parts (core logic done, tests passing, integration verified)

Don't log routine actions (started file, read docs, ran tests).

## Multi-Agent Coordination

For parallel work with multiple agents:

1. Create an epic with subtasks and dependencies
2. Each agent runs `tpg ready` to find unblocked work
3. `tpg start` claims a task (prevents duplicate work)
4. Dependencies ensure correct ordering automatically

```bash
tpg add "Auth system" -e                    # Epic
tpg add "DB schema" --parent ep-abc         # Subtask 1
tpg add "API endpoints" --parent ep-abc     # Subtask 2
tpg add "Integration tests" --parent ep-abc # Subtask 3
tpg dep ts-api blocks ts-tests              # Tests wait for API
tpg dep ts-schema blocks ts-api             # API waits for schema
```

Agents only see `ts-schema` in `tpg ready` until it's done, then `ts-api` appears, then `ts-tests`.
