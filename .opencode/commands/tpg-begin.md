---
description: Start working on tpg-tracked tasks by launching parallel subagents
---

First, check for any in-progress work and get the list of ready tasks:

!`tpg list --status in_progress`

!`tpg ready`

Based on the ready tasks above, you are the orchestrator. Your job is to:

1. **Select tasks to work on:**
   - If specific task IDs were provided as arguments ($ARGUMENTS), work on those
   - If $ARGUMENTS contains `--parallel N`, respect that limit for how many to launch in parallel
   - If no arguments, select up to 3 ready tasks that can run in parallel

2. **Launch subagents for each selected task:**
   For each task, launch a subagent with this EXACT minimal format:
   ```
   Work on task <id>: <brief title>.
   
   Follow the tpg-agent skill workflow.
   
   **When complete, report:**
   1. Summary of what was done
   2. Files changed (list all modified files)
   3. Whether this is a worktree task (y/n)
   
   Results should be reported as the close reason of the task.
   ```

   As soon as the task is finished you should launch a subagent for the next task so there are always the same number of tasks being worked on in parallel. Continue until no more ready tasks are available, unless arguments ($ARGUMENTS) says otherwise
   
**CRITICAL RULES:**
- Do NOT copy acceptance criteria into the prompt - all such information comes from the task
- Do NOT copy task description into the prompt - all such information comes from the task
- Do NOT provide implementation hints - all such information comes from the task
- Keep prompts under 5 lines
- The agent will read full context with `tpg show <id>`

## Git Commit Policy

**Check AGENTS.md first** - If the project specifies commit rules, follow those. Otherwise:

### Worktree tasks
Agents commit their own work in the worktree. No action needed from you.

### Non-worktree tasks
**YOU must commit after each task completes.** When an agent reports completion, ask: "What files did you change?" Then:
```bash
git add <files>
git commit -m "feat: <description> (<task-id>)"
```

**Atomic commits:** Each task gets its own commit unless AGENTS.md says otherwise. Do NOT batch multiple tasks into one commit.

3. **Report what was started:**
   - List each task ID and title
   - Note if any tasks were skipped (blocked, already in progress, etc.)
   - Tell user to check progress with `tpg list --status in_progress`

**Do NOT:**
- Wait for tasks to complete
- Implement anything yourself
- Create new tasks or plans
- Monitor running tasks

**Do:**
- Launch agents and report immediately
- Respect the parallel limit
- Check the results of the task on completion to see if it finished
- **After each task: Check if parent epic is now done (if so, handle worktree cleanup)**
- **For non-worktree tasks: Create atomic commits after each task completes**
- Ask agents to report files changed and whether it's a worktree task
- Check AGENTS.md for project-specific commit rules

**Post-task workflow:**
When an agent reports completing a task:
1. Check `tpg show <task-id>` to get parent epic
2. Check `tpg show <epic-id>` - if status is "done", epic auto-completed
3. If epic has worktree, handle cleanup (see tpg-orchestrator skill)
4. Create atomic commit with files changed (if non-worktree)
