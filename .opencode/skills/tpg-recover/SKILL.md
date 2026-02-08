---
name: tpg-recover
description: Recover stalled or interrupted tpg work by auditing stale tasks, logs, history, and worktree state.
---

# tpg-recover

Recover from crashes or partial work by finding stale tasks, reviewing logs and history, resuming safely, and handling worktree cleanup. This skill is for recovery and audit, not new feature work.

## ALWAYS use this skill when

- You need to resume or audit a task that looks abandoned.
- You need to verify what happened before continuing work.
- You need worktree resume or cleanup guidance for a worktree epic.

## When to use

- Tasks are stuck in `in_progress` after an agent crash/timeout.
- You need to audit what happened before resuming work.
- Worktree epics need resume or cleanup guidance.

## Triggers

- `tpg stale` shows tasks older than the stale threshold.
- `tpg list --status in_progress` shows items that look abandoned.
- A task was partially done and needs verification before continuing.
- An epic just auto-completed and you need its worktree cleanup steps.

## Recovery workflow

1. Detect stale tasks

```bash
tpg stale
tpg stale --threshold 10m
tpg list --status in_progress
```

2. Inspect the candidate task

```bash
tpg show <task-id>
tpg history <task-id>
tpg dep <task-id> list
```

Read logs for last actions, check dependencies, and note any worktree context shown by `tpg show`.

3. Choose the correct recovery path

- If you are a **subagent** resuming work:
  - Resume the task and continue safely.

```bash
tpg start <task-id>          # if status is open
tpg start <task-id> --resume
```

- If you are the **orchestrator**:
  - Do **not** change status. Assign a new `tpg-agent` to resume.
  - Provide the task ID and remind the agent to read `tpg show <task-id>`.

4. Worktree resume or cleanup

- If `tpg show` lists a worktree, use its instructions to enter the right directory.
- Verify the worktree exists (for example, with `git worktree list`) before editing files.
- When an epic is fully complete (auto-completed), run:

```bash
tpg epic finish <epic-id>
```

This shows cleanup steps (merge branch, remove worktree). `tpg epic finish` does not complete the epic.

5. Record recovery actions

- Add a brief `tpg log` entry describing what was found and how the resume proceeded.
- If the task can’t proceed, create a blocker task and link it with `--blocks`.

## Audit and history guidance

- `tpg show <id>` is the primary audit view (logs, deps, and worktree context).
- `tpg history <id>` shows the full timeline of state changes.
- Prefer audit-driven decisions: confirm what was done before resuming or closing work.

## Commands quick reference

```bash
tpg stale
tpg list --status in_progress
tpg show <id>
tpg history <id>
tpg start <id> --resume
tpg epic finish <epic-id>
```

## Gotchas and rules

- Only **subagents** should change task status (`tpg start`, `tpg done`, `tpg log`).
- Orchestrators **delegate** recovery; they do not take ownership of tasks.
- `tpg block` requires `--force` and should be avoided in favor of dependencies.
- Epics with children cannot be started; recover the child tasks instead.
- Avoid destructive git commands during recovery.
- Do not use removed commands (for example, `tpg set-status`) unless explicitly instructed to repair a corrupted state.
