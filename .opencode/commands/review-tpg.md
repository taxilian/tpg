---
description: Review tpg plan against the actual codebase for correctness
---

# Review TPG Plan

You are not going to implement this plan. Someone else will — a competent developer picking up tasks cold with no context beyond what's in tpg and the codebase. Your job is to make sure the plan is *correct* so they can succeed.

Do NOT review the plan against itself. Review it against the actual codebase. Start by exploring the code to understand what actually exists, then check whether the plan reflects reality.

## Start with the codebase

Before looking at any task, use an explore agent to understand the actual state of things: project structure, existing functions, current command implementations, schema version, config shape, etc. This is the foundation — everything else is checking the plan against what you learned here.

## What to check

### 1. Dependencies reflect actual needs
For each task, think about what it *actually needs* before it can start. Does it depend on schema changes, new packages, config additions, or code that another task creates? That's a real dependency — enforce it with `tpg dep`.

Don't look at dependency lists in descriptions. Look at what the task needs to do and work backwards to what must exist first:
- **False blockers** — tasks blocked by something they don't actually need. This directly kills parallelism and is the most common plan bug.
- **Missing blockers** — tasks that could start before a real prerequisite is done, leading to failures or rework.

### 2. Assumptions match reality
Does any task assume something about the codebase that isn't true? Common traps:
- "Extend X which currently supports Y" when X actually only does Z
- Creating something that already exists under a different name
- Referencing commands, flags, or functions that don't exist yet and aren't created by a predecessor task

### 3. Existing code that would be duplicated
If there's already a function, pattern, or module that overlaps with what a task needs to build, the task should mention it. The developer doesn't need hand-holding — but they shouldn't have to accidentally discover that the work is half done already.

### 4. Design coverage
If there's a design doc, cross-reference it against the task list. Look for specified features that have no corresponding task.

### 5. Tasks describe problems, not solutions
A task should describe: what problem needs to be solved, why it matters, what constraints exist (from project decisions or integration contracts), and what "done" looks like. The developer decides *how* to solve it.

If a task dictates implementation details — function names, algorithms, internal structure, step-by-step instructions — strip that out unless there's a concrete project-level reason it must be done a specific way. When fixing tasks during this review, resist the urge to add more detail. Simplify.

## Common reviewer mistakes

- **Reviewing the plan only against itself** instead of against the codebase. If you haven't explored the actual code, you haven't reviewed anything.
- **Assuming that the name / summary is all you need to see** instead of doing a full `tpg show`
- **Adding too much detail when "fixing" tasks.** The instinct is to make descriptions more thorough by adding line numbers, function signatures, and step-by-step guides. This makes things worse — it over-constrains the implementer and creates more things that can be wrong. Fix by simplifying. You aren't doing the job, you're just making sure the implementer will have the knowledge needed to do the job.
- **Comparing deps to description text** instead of to actual needs. Descriptions may list deps, may not, may be wrong. Ignore them. Think about what the task needs.
- **Not considering tpg templates** If there are a lot of tasks which follow the same pattern then they should be using a template; if there isn't an appropriate template, add one and then update the tasks to use it. Make sure the template variables encourage / enforce good practices. Remember: planners are sometimes lazy and don't remember to use the templates, part of your job is to catch that and fix it if it happens, as well as identifying new patterns that can be made into good templates.

## Process

1. Explore the actual codebase thoroughly (use an explore agent — do this FIRST)
2. Get the lay of the land:
   - `tpg tree` — see all epics and their task counts
   - `tpg list` — see active items across the project
   - Identify which epics have open work that needs review
3. For each epic with open work:
   - `tpg plan <epic-id>` — tree structure, progress, blockers
   - `tpg export --parent <epic-id>` — full task descriptions and deps
   - For each task: read it, think about what it needs, check deps, check assumptions
4. Fix problems: `tpg dep` for dependencies, `tpg desc` for descriptions, `tpg add` for missing tasks
5. Verify the final state and report what was found and changed

## Special concerns

$ARGUMENTS

If arguments are provided, pay extra attention to those areas. Otherwise, do a general review.
