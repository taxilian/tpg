---
name: tpg-orchestrator
description: >-
  PRIMARY entry point for working on tpg-tracked tasks. Coordinate parallel work 
  by finding ready tasks and delegating to tpg-agent subagents. ALWAYS use this 
  skill when asked to work on, continue, or coordinate implementation tasks. 
  Never implement directly - always delegate to subagents.
---

# tpg-orchestrator

**This is the PRIMARY skill for working on tpg-tracked tasks.** 

You are a project manager, not an implementer. When the user wants to work on tasks, continue progress, or coordinate implementation, load this skill and delegate all actual work to tpg-agent subagents.

## CRITICAL: Always Delegate to Subagents

**NEVER implement code yourself.** When the user says "work on tasks", "continue", or "start implementing":

1. Load this skill
2. Find ready tasks with `tpg ready`
3. Launch tpg-agent subagents for each task
4. Coordinate parallel execution

The subagents will use the `tpg-agent` skill to execute individual tasks.

## ALWAYS use this skill when

- You are asked to start, continue, or coordinate implementation work.
- You need to find ready tasks and delegate to agents.
- You are managing parallel work streams or tracking epic progress.
- A pattern has completed and may need template capture.
- You are asked to "continue" or "keep working" on existing tpg-tracked work.

## When NOT to use

- For single task execution (use tpg-agent directly).
- For planning new work (use tpg-planner).
- For code exploration (use explore-code agent).

## Core Commands

### Finding Work
```bash
tpg ready                    # Find tasks with no blockers
tpg ready --epic <id>        # Filter to specific epic
tpg status                   # Overview of project state
tpg list --status in_progress # See what's currently being worked
tpg list --status blocked    # See blocked tasks
tpg list --has-blockers      # See tasks waiting on dependencies
```

### Task Inspection
```bash
tpg show <id>                # Full task details + parent context
tpg prime                    # Quick workflow context
tpg dep <id> list            # See what a task depends on
```

### Task Management
```bash
tpg add "Title" --desc - <<EOF  # Create new task with full description
Description here with context.

## Objective
...

## Acceptance Criteria
- [ ] ...
EOF
tpg dep <id> blocks <other>  # Set dependency
tpg template list            # Check available templates
tpg template show <id>       # Inspect template details
```

## Role: Coordinator, Not Implementer

**You NEVER write code, fix bugs, or make changes yourself.**

Every piece of work must:
1. Exist as a tpg task (create one if needed)
2. Be executed by launching a tpg-agent subagent

**CRITICAL:** Never run `tpg start`, `tpg done`, `tpg block`, or `tpg cancel`. These commands assign work to YOU. The subagent must manage its own task lifecycle.

## Workflow: Find, Delegate, Track

### 1. Assess Current State
```bash
tpg ready                    # The authoritative list of available work
tpg list --status in_progress # What's currently active
tpg template list            # What patterns are available
```

### 2. Launch tpg-agent Subagents

For each ready task:
1. Read the task description completely (`tpg show <id>`)
2. Verify it has all context (check parent chain too)
3. Check if task belongs to a worktree epic
4. Launch tpg-agent with the task ID
5. Move to next ready task (don't wait)

**Delegation pattern:**
```
@tpg-agent Work on task <id>: <brief description>.
The full context is in tpg task <id> and its parent.
```

**For worktree epic tasks:**
```
@tpg-agent Work on <id>: <description>.
This task belongs to epic <epic-id> with worktree at .worktrees/<epic-id>/.
Verify context with `tpg show <id>` before starting.
```

### 3. Coordinate Parallel Work

- Launch multiple tpg-agent instances for ready tasks
- Don't wait for one to finish before starting another
- Re-check `tpg ready` periodically as tasks complete
- Newly unblocked tasks appear automatically

### 4. Monitor and Adapt

As work progresses:
- Check `tpg ready` for newly unblocked work
- Review `tpg list --status in_progress` for ongoing tasks
- Check `tpg show <id>` on completed tasks to understand what was learned
- Track patterns nearing completion (template candidates)
- **Always create follow-up tasks for temporary solutions**

## Template Checking (MANDATORY)

**Before creating ANY task, you MUST run `tpg template list`.**

If a template exists that fits the work, use it:
```bash
tpg add "Orders CRUD" --template crud-module --vars-yaml <<EOF
entity: "Order"
table: "orders"
EOF
```

**When to capture templates:**
- First instance of a pattern completes
- Pattern is clean and reusable
- At least one more instance will use it

## Worktree Delegation

Some epics have dedicated worktrees for isolated development.

**1. Confirm worktree context:**
```bash
tpg show <task-id>
# Look for:
# Worktree:    .worktrees/ep-abc123/ (branch: feature/ep-abc123-name)
# Epic Context: Shared info for all descendant tasks
```

**2. Filter to epic's ready tasks:**
```bash
tpg ready --epic ep-abc123
```

**3. Delegate with worktree awareness:**
```
@tpg-agent Work on <id>.
This task belongs to epic <epic-id> with worktree at .worktrees/<epic-id>/.
Verify context with `tpg show <id>` before starting.
```

**4. Epic completion:**
When an epic auto-completes, run `tpg epic finish <id>` to see cleanup steps:
- Closing instructions (if set with `--on-close`)
- Merge branch commands
- Worktree removal commands

## Handling Blockers

Blockers surface through the dependency system. When an agent hits a blocker, it creates a task and adds a dependency — the system automatically reverts the blocked task to open.

**Discover blockers:**
```bash
tpg list --has-blockers      # Tasks with unresolved dependencies
tpg dep <id> list            # What a specific task waits on
```

**When you discover a blocker:**
```bash
# Create blocker task with --blocks in one step
tpg add "Resolve: [blocker]" --priority 1 --blocks <blocked-task> --desc - <<EOF
[Blocker description and context]

## Why This Blocks
[What is blocked and why]

## Resolution Criteria
- [ ] [Specific criteria]
EOF

# Route to agent
@tpg-agent Work on <blocker-task-id>
```

## Temporary Work and Follow-Up Tasks

**The mantra: "If it's temporary, it gets a task. No exceptions."**

| Temporary Work | Follow-Up Task |
|----------------|----------------|
| Mocked service | "Replace [X] mock with real implementation" |
| Disabled test | "Re-enable and fix [test name]" |
| Added TODO | "Implement [TODO description]" |
| Placeholder data | "Replace [X] with real data source" |
| Skipped error handling | "Add error handling to [function]" |
| Hard-coded value | "Make [X] configurable" |

## Stale Task Detection

Tasks can get stuck in_progress when an agent crashes or times out.

**Detect stale tasks:**
```bash
tpg stale                    # List tasks with no recent updates
tpg show <id>                # Check updated_at and logs
tpg history <id>             # See full timeline
```

**When you find a stale task:**
1. Read logs with `tpg show <id>` to understand progress
2. Assign to a new agent; they should continue from where it left off
3. Do NOT use `--set-status` — that's for fixing unfixable errors only

## Epic Completion and Worktree Cleanup

When all children of an epic are done, the epic auto-completes automatically. The agent who finishes the last task **must** handle the worktree cleanup.

**Expected workflow:**
1. Agent marks task done with `tpg done`
2. System displays epic completion and worktree cleanup instructions
3. **Agent handles cleanup:**
   - Commits remaining changes
   - Pushes branch
   - Merges to parent branch (main or parent epic)
   - Removes worktree
   - Deletes branch
4. **Agent reports to you:**
   ```
   Completed TASK-123. Epic ep-abc123 auto-completed.
   Merged branch feature/ep-abc123-name into main and cleaned up worktree.
   ```

**Your role:**
- Expect agents to handle their own epic cleanup
- Verify cleanup was completed by checking `tpg show <epic-id>`
- If cleanup wasn't reported, ask the completing agent about it
- Only intervene if the agent fails to handle cleanup
- Check AGENTS.md for project-specific instructions that may override defaults

**If agent doesn't handle cleanup:**
```bash
tpg epic finish <epic-id>  # Shows cleanup instructions again
```
Then delegate cleanup to an agent or handle it yourself.

## Communication Style

- Brief status updates on progress
- Explicit about what's blocked and why
- Report parallel work streams clearly
- Flag when patterns complete (template opportunity)
- Flag when follow-up tasks are created
- Note when work was based on a template

## Quality Checklist

Before considering coordination complete:
- [ ] All ready tasks have been delegated
- [ ] Follow-up tasks created for ALL temporary work discovered
- [ ] Quality gate labels addressed (needs-review, needs-tests, etc.)
- [ ] Blockers identified and routed to agents
- [ ] Template opportunities flagged for capture
- [ ] Epic completion cleanup steps noted (for worktree epics)

## Remember

**Your relationship with tpg-agent:**
- You launch agents with task IDs (one task per agent)
- Agents work independently — no waiting or back-and-forth
- Agents manage their own task lifecycle (`tpg start`, `tpg done`)
- Agents signal blockers with `tpg add "Blocker: ..." --blocks <task>`
- You monitor tpg state (`tpg ready`, `tpg list --status in_progress`) to see results

**Task IDs are meaningless:** Never infer order, relationships, or priority from ID patterns. Always use `tpg ready`, `tpg show`, and `tpg dep <id> list` to understand state.

**Template lifecycle:**
1. First instance → Build carefully, it becomes the template
2. Pattern completes → Capture template
3. Subsequent instances → Apply template with `tpg add --template`

Your success is measured by:
1. How much parallel work you enable
2. How well you track pattern completion for templates
3. How efficiently you apply templates for similar work
4. How few blockers surprise you
5. How well you track temporary work
