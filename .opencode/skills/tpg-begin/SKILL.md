---
name: tpg-begin
description: Start working on tpg-tracked tasks by launching parallel subagents. Lists ready tasks and delegates 2-4 at a time to tpg-agent subagents with correct minimal context.
---

# tpg-begin

**Primary entry point for starting tpg-tracked work.**

This skill coordinates the beginning of work sessions by finding ready tasks and delegating them to parallel subagents.

## What I do

1. Find ready tasks with `tpg ready`
2. Select 2-4 tasks that can safely run in parallel
3. Launch tpg-agent subagents for each task
4. Report which tasks were started

## Trigger phrases

- `/tpg-begin`
- `tpg begin`
- `start working on tpg tasks`
- `continue tpg work`
- `work on ready tasks`

## Non-goals

- I do NOT implement tasks myself
- I do NOT wait for tasks to complete
- I do NOT monitor running tasks (that's for later)
- I do NOT create new tasks or plans

## Arguments

```
/tpg-begin [task-id ...] [--parallel N]
```

- **task-id** (optional): Specific task ID(s) to work on instead of auto-selecting
- **--parallel N** (optional): Max parallel tasks (default: 4, max: 6)

## Process

### Step 1: Find Ready Work

```bash
# Check for in-progress work first
tpg list --status in_progress

# Get ready tasks
tpg ready
```

### Step 2: Select Tasks

**If specific task IDs provided:**
- Verify each task exists and is ready
- Check for blockers with `tpg dep <id> list`
- Warn if any selected task is blocked

**If auto-selecting:**
- Select up to `--parallel` tasks (default 4)
- Prefer tasks without worktrees (can run anywhere)
- Mix of worktree and non-worktree tasks is fine
- Skip tasks that depend on each other

### Step 3: Launch Subagents

For each selected task, launch a tpg-agent subagent:

```
@tpg-agent Work on task <id>: <brief title from tpg show>.

Follow the tpg-agent skill workflow. Full context in `tpg show <id>`.
```

**CRITICAL: Minimal context only!**
- Do NOT copy acceptance criteria
- Do NOT copy task description
- Do NOT provide implementation hints
- The agent will read the task with `tpg show`

### Step 4: Report

Output summary:
```
Started N parallel tasks:
- <id>: <title> (epic: <epic-id>)
- <id>: <title>
...

Check progress with: tpg list --status in_progress
```

## Guardrails

- **Never exceed --parallel limit** (default 4, max 6)
- **Never launch agents for blocked tasks** - check `tpg dep <id> list` first
- **Never duplicate task context** in delegation prompts
- **Always verify task exists** before launching agent

## Examples

### Example 1: Auto-select ready tasks

```
User: /tpg-begin

I check `tpg ready`, find 6 tasks, select 4, launch agents:

Started 4 parallel tasks:
- ts-abc: Fix auth middleware
- ts-def: Add user validation
- ts-ghi: Update API docs
- ts-jkl: Refactor config

Check progress: tpg list --status in_progress
```

### Example 2: Specific tasks

```
User: /tpg-begin ts-123 ts-456

I verify both are ready, check no blockers, launch:

Started 2 tasks:
- ts-123: Implement rate limiting (epic: ep-789)
- ts-456: Add caching layer

Check progress: tpg list --status in_progress
```

### Example 3: Limit parallelism

```
User: /tpg-begin --parallel 2

I select only 2 ready tasks:

Started 2 parallel tasks:
- ts-abc: Fix auth middleware
- ts-def: Add user validation

Check progress: tpg list --status in_progress
```

## How to test

1. Ensure tpg has ready tasks: `tpg ready`
2. Invoke: `/tpg-begin`
3. Verify subagents launched: check `tpg list --status in_progress`
4. Verify minimal prompts: subagent prompts should be < 5 lines

## Troubleshooting

**"No ready tasks"**
- Run `tpg ready` manually to confirm
- Check for blocked tasks: `tpg list --has-blockers`

**"Task X is blocked"**
- The task has unresolved dependencies
- Run `tpg dep <id> list` to see what's blocking it

**"Too many parallel tasks requested"**
- Max is 6, default is 4
- Use `--parallel N` to adjust

## Output contract

When invoked, I produce:
1. List of selected tasks with IDs and titles
2. Confirmation that subagents were launched
3. Command to check progress

I do NOT:
- Wait for completion
- Show real-time progress
- Create follow-up tasks
