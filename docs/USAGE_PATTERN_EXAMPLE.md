# Usage Pattern Example: Building a Feature with tpg

This document walks through using `tpg` to coordinate multiple AI agents building a significant feature from idea to completion.

## The Scenario

You want to add "template support" to a CLI tool. This involves understanding requirements, designing the solution, implementing multiple components, and verifying everything works together. Multiple agents will work on this, potentially in parallel, over multiple sessions.

## Phase 1: Understand the Problem

Create a Discovery task to clarify what we're building. Provide detailed variable values that capture the full context: what question we're answering, what constraints exist, what success looks like, and what open questions remain.

An agent claims the discovery task, investigates, and completes it with a results message that tells downstream tasks where to find the spec and what key decisions were made.

## Phase 2: Break Down the Work

Based on the discovery results, create implementation tasks. Use templates when they fit the work type, or create simple tasks when they don't. Templates are optional - they help structure certain kinds of work but aren't required.

- **Storage task (TDD template)** - Store template reference, step index, and variables on tasks. No dependencies, can start immediately.
- **Parsing task (TDD template)** - Parse .toml/.yaml template files. No dependencies, can start immediately.
- **Instantiation task** - Depends on storage AND parsing completing first.
- **Rendering task** - Depends on storage AND parsing completing first.
- **Integration audit (Audit template)** - Depends on instantiation AND rendering. Verifies the whole feature works end-to-end.

This creates a task graph where storage and parsing can run in parallel, instantiation and rendering wait for both, and the audit waits for everything.

## Phase 3: Parallel Execution

Multiple agents check `tpg ready` and see different available work.

When an agent starts a task (e.g., `tpg start ts-4boe`), they first review:
1. The task itself via `tpg show` - see the interpolated template, description, and any existing progress
2. The results of dependency tasks - understand what was built and how to use it

Each agent works through their task steps, logging progress at meaningful milestones to keep the task fresh and recoverable.

When done, the results message captures what a dependent task implementer needs: what was built, where to find it, how to use it.

## Phase 4: Dependent Work Unlocks

Once storage and parsing are both done, `tpg ready` shows instantiation and rendering tasks as available.

A new agent picks up instantiation. Before starting work, they read the results from the storage and parsing tasks to understand what's available to them. They proceed without needing to re-read specs or ask questions - the dependency results provide the context.

## Phase 5: Handling Discovered Blockers

Agent working on rendering discovers a gap: template step titles can contain variables but the parser doesn't expand them.

Instead of marking the task "blocked", they create a new task to address the gap and set it as a blocker:
```
tpg add "Add variable expansion to template parser"
tpg blocks ts-newid ts-rendering
```

Now ts-rendering won't appear in `tpg ready` until the new task completes. Another agent (or the same one) can pick up the blocker task, complete it with a results message explaining how expansion now works, and ts-rendering automatically becomes ready again.

## Phase 6: Stale Work Recovery

An agent crashes or abandons work mid-task. Use `tpg stale` to find in-progress tasks with no recent updates.

A new agent reviews the stale task via `tpg show`, sees the latest progress update showing where work stopped, adds a fresh progress update to indicate they're resuming, and continues from where it left off.

## Phase 7: Final Verification

All implementation tasks complete. The audit task becomes ready.

An agent claims it and works through the audit steps, checking that everything integrates correctly. If issues are found, new tasks are created with appropriate dependencies to fix them before the audit can complete.

## Phase 8: Project Completion

When the audit passes, its results message summarizes the verification and points to any documentation created.

If a parent task was created to track the whole feature, it now has all dependencies satisfied and can be marked done with a summary of the entire effort.

---

## Key Principles

1. **Results messages are the handoff** - When you complete a task, write what the next person needs to know. They shouldn't have to dig through logs or re-read specs.

2. **Dependencies gate work** - Don't manually coordinate. Let `tpg ready` show what's actually unblocked.

3. **Templates are optional but useful** - Discovery, TDD, Audit are reusable patterns that force you to think through what matters. Use them when they fit; skip them for straightforward work.

4. **Progress updates prevent lost work** - Log milestones. If you crash, the next agent knows where you were.

5. **Blockers are tasks, not status** - When you discover missing work, create a task that blocks the current one. Don't just mark things "blocked" with a reason.

6. **Parallel by default** - Structure dependencies so independent work can happen simultaneously. Don't serialize unnecessarily.

7. **Read your dependencies before starting** - The results from upstream tasks tell you what you need to know. Start there.

8. **The task graph is the plan** - Looking at `tpg list` and `tpg graph` shows what's done, what's in progress, what's blocked, and what's waiting.
