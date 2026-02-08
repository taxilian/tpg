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
   
   Follow the tpg-agent skill workflow. Results should be reported as the close reason of the task.
   ```

   As soon as the task is finished you should launch a subagent for the next task so there are always the same number of tasks being worked on in parallel. Continue until no more ready tasks are available, unless arguments ($ARGUMENTS) says otherwise
   
   **CRITICAL RULES:**
   - Do NOT copy acceptance criteria into the prompt - all such information comes from the task
   - Do NOT copy task description into the prompt - all such information comes from the task
   - Do NOT provide implementation hints - all such information comes from the task
   - Keep prompts under 5 lines
   - The agent will read full context with `tpg show <id>`

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
