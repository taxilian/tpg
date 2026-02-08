---
name: tpg-agent
description: >-
  FOR SUBAGENTS ONLY: Execute a single tpg task from start to completion.
  Use this skill when you are a subagent assigned a specific task ID to work on.
  NEVER use this skill as the primary agent - the orchestrator should delegate
  to you. This skill covers the complete task lifecycle including worktree handling.
---

# tpg-agent

**FOR SUBAGENTS ONLY**

You are a task executor subagent. You should only load this skill when:
- The orchestrator (@tpg-orchestrator) has delegated a specific task to you
- You are working on ONE task at a time from start to completion
- You need to handle worktree-based development

**If you're the primary agent and the user wants to work on tasks, load `tpg-orchestrator` instead and delegate to subagents.**

## ALWAYS use this skill when

- You are assigned a specific tpg task ID to work on (e.g., "Work on task ABC-123").
- You need to implement a feature or fix described in a tpg task.
- You are asked to complete, finish, or resolve a tpg task.
- You are instructed to start or claim a tpg task.
- You need to create follow-up tasks for temporary work (mocks, TODOs, placeholders).

## When to use

- Executing implementation tasks from tpg.
- Working on bug fixes tracked in tpg.
- Completing subtasks of larger epics.
- Handling worktree-based development tasks.
- Creating dependencies and follow-up tasks.

## Triggers

- The user provides a task ID (e.g., "ts-abc123", "ABC-456").
- The user says "work on task", "complete task", "finish task", or "start task".
- The user asks you to implement something described in a tpg issue.
- You need to log progress or mark a task complete.

## Task lifecycle workflow

**CRITICAL:** You MUST follow the full lifecycle for every task:
1. **START** - `tpg start <id>` when you begin (claims the task)
2. **LOG** - `tpg log <id>` throughout for milestones, decisions, blockers  
3. **DONE** - `tpg done <id>` when complete with results

This is your communication channel with the orchestrator. Without these steps, the orchestrator cannot track progress.

### 1. Read and understand

Always start by reading the full task context:

```bash
# Read the task details (includes shared context from parent epics)
tpg show <task-id>

# Check for blocking dependencies
tpg dep <task-id> list

# If the task is part of an epic, see other ready tasks
tpg ready --epic <epic-id>
```

**Important:** Task IDs are meaningless - don't infer order or relationships from ID patterns.
Always query tpg for the truth.

### 2. Claim the task

```bash
tpg start <task-id>
```

This marks the task as "in_progress" so others know you're working on it.

### 3. Work and log

As you work, log significant milestones and discoveries:

```bash
# Log decisions, blockers, discoveries, or milestones
tpg log <task-id> - <<EOF
Decided to use approach X because Y. Alternatives considered: A, B.
EOF
```

**Log immediately when:**
- You create a dependency or follow-up task
- You choose between alternatives
- You discover existing code that changes your plan
- You answer a key unknown that required searching/testing
- You hit something unexpected (error, missing API, wrong assumption)
- You finish a key milestone (core logic works, tests pass)

**Do NOT log routine actions** (opened a file, read docs, ran a command).

### 4. Create follow-up tasks (if needed)

When you do anything temporary (mock, disabled test, TODO, placeholder):

**CRITICAL: Always check templates first:**

```bash
# Step 1: Check available templates (MANDATORY)
tpg template list

# Step 2a: If a template fits, use it
tpg add "Replace payment mock with real integration" \
  --template <template-id> \
  --priority 1 \
  --blocks <parent-task-id> \
  --vars-yaml <<EOF
module: "payment"
EOF

# Step 2b: Or create a plain follow-up with full description
tpg add "Replace [X] mock with real implementation" \
  --priority 1 \
  --desc - <<EOF
Context: [what was done and why]. See <parent-task-id> for details.

## Current State
Currently using [explanation of temporary work].

## Required Changes
- [ ] Replace with real implementation
- [ ] Update tests
EOF
  --blocks <parent-task-id>
```

### 5. Verify completion

Before marking complete, verify:

```
- [ ] Task objective achieved
- [ ] Acceptance criteria met
- [ ] Tests written and passing
- [ ] Code follows project conventions
- [ ] Follow-up tasks created for ALL temporary work
- [ ] Quality gate labels handled (needs-review, needs-tests, etc.)
```

### 6. Mark complete

```bash
tpg done <task-id> "Completed: [brief summary]. Follow-ups: [list any]"
```

## Commands reference

### Task lifecycle

```bash
tpg show <id>                    # Read task details and shared context
tpg start <id>                   # Claim work (mark in_progress)
tpg log <id> - <<EOF             # Log milestone/discovery
message here
EOF
tpg done <id> "results"          # Mark complete with results
```

### Task creation and dependencies

```bash
# Create follow-up task with dependency (use HEREDOC for full description)
tpg add "title" --desc - <<EOF --blocks <id>
Description here with full context.

## Current State
...

## Required Changes
- [ ] ...
EOF

# Add dependency to existing task
tpg dep <id> blocks <other-id>

# Show dependencies
tpg dep <id> list

# Change parent
tpg edit <id> --parent <id>
```

### Templates

```bash
tpg template list                # List available templates (ALWAYS check first)
tpg template show <id>           # View template details
tpg add "title" --template <id>  # Create from template
```

### Epics

```bash
tpg epic finish <id>             # Complete epic when all children done
tpg ready --epic <id>            # Filter to epic's tasks
```

### Quality gates

```bash
tpg label <id> needs-review      # Code needs review
tpg label <id> needs-tests       # Tests incomplete
tpg label <id> needs-docs        # Docs need updating
tpg unlabel <id> needs-review    # Remove label when addressed
```

## Worktree handling

Tasks may belong to epics with dedicated worktrees. `tpg show` displays worktree context:

```
Worktree:
  Epic:     ep-abc123 - Implement worktree support
  Branch:   feature/ep-abc123-implement-worktree-support
  Location: .worktrees/ep-abc123
  Path:     ep-abc123 -> ts-xyz789

  To create worktree:
    git worktree add -b feature/ep-abc123-implement-worktree-support .worktrees/ep-abc123 main
    cd .worktrees/ep-abc123
```

### When working in a worktree:

1. Verify context with `tpg show <id>` before starting
2. Use `tpg ready --epic <id>` to filter to the epic's tasks
3. All tpg commands work the same regardless of location
4. Navigate to the worktree directory if work needs to be done there
5. Run git commands as needed (commit, push, etc.)

**Note:** The `tpg` CLI tool prints git instructions but doesn't execute them - you must run git commands yourself.

### When completing the last task in an epic:

When you run `tpg done` on the final task in an epic, the system automatically shows epic completion info. **You must handle the worktree cleanup yourself:**

**After seeing the cleanup instructions, you should:**

1. Commit any remaining changes in the worktree
2. Push your branch: `git push -u origin <branch-name>`
3. Merge into the parent branch (main or parent epic branch):
   ```bash
   git checkout <parent-branch>
   git merge <worktree-branch>
   ```
4. Remove the worktree: `git worktree remove <path>`
5. Delete the branch: `git branch -d <branch-name>`

**CRITICAL:** Always follow the workflow:
1. **Mark start:** `tpg start <task-id>` when you begin
2. **Log progress:** `tpg log <task-id>` for milestones, decisions, blockers
3. **Mark done:** `tpg done <task-id>` when complete with results

**Report to orchestrator:**
After completing the task (and handling any epic cleanup), report what was done:
```
Completed TASK-123. Epic ep-abc123 auto-completed. 
Merged branch feature/ep-abc123-name into main and cleaned up worktree.
```

**Note:** Check AGENTS.md for any project-specific merge instructions that override the default workflow.

## Epic shared context

Tasks under epics inherit shared context - guidelines that apply to all tasks:

```
Shared Context (from ep-abc123):
  Use Stripe API v3. All handlers need idempotency keys.
  See docs/stripe.md for patterns.
```

**Always read and follow shared context:**
- It contains decisions already made (API versions, patterns, constraints)
- It may reference documentation or examples
- Ignoring shared context leads to inconsistent implementations

## Handling blockers

When something blocks you that's outside your task scope:

```bash
# Create the blocker task with --blocks to set the dependency in one step
tpg add "Blocker: [description]" \
  -p 1 \
  --blocks <your-task-id> \
  --desc - <<EOF
[What's blocking and why]

## Impact
[What work is blocked]

## Resolution Criteria
- [ ] [Specific criteria for resolving]
EOF

# That's it - exit. The system automatically:
# - Reverts your task to open (it was in_progress with an unmet dep)
# - Logs the reason on your task
# - The orchestrator sees the new blocker in tpg ready
```

**Do NOT use `tpg block`** - use `--blocks` when creating tasks, or `tpg dep`
when linking existing tasks.

## Template-first guidance

When creating follow-up tasks, **always check templates first**:

1. Run `tpg template list` to see available templates
2. If a template fits the work, use it with `--template <id>`
3. Templates enforce good structure automatically
4. Only create ad-hoc tasks when no template is appropriate

## Key principles

1. **Read the task fully** - Never assume from task IDs
2. **Claim before working** - Use `tpg start` to mark in_progress
3. **Log as you go** - Document decisions, blockers, milestones
4. **Create follow-ups** - Never leave temporary work without a task
5. **Check templates first** - Use template structure when available
6. **Complete with results** - Store all results in tpg via `tpg done`
7. **Compact between tasks** - If reused, treat each task as fresh

## Remember

- **You have no external memory** - Everything needed is in the tpg issue
- **Work on ONE task at a time** - Complete fully before moving on
- **TPG is your communication channel** - The orchestrator reads task state from tpg
- **You signal completion via `tpg done`** - No need to "report back"
- **Task IDs are meaningless** - Always query tpg for relationships

**Mantra: Read the task. Claim it. Do it. Track temporary work. Complete it. (Compact if continuing.)**
