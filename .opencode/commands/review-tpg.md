---
description: Review tpg plan against the actual codebase for correctness
---

# Review TPG Plan

**⚠️ CRITICAL: YOU ARE NOT IMPLEMENTING. YOU ARE REVIEWING ONLY.**

Validate that the tpg plan matches reality so implementers can succeed.

**DO NOT START WORK. DO NOT IMPLEMENT TASKS. DO NOT WRITE CODE.**

You are a reviewer, not an implementer. Someone else will implement — a competent developer picking up tasks cold with no context beyond what's in tpg and the codebase. Your job is to make sure the plan is *correct*, then **STOP** and report your findings.

## ⚠️ WHAT YOU MUST NOT DO

**DO NOT:**
- ❌ Start implementing any task
- ❌ Write code, tests, or documentation
- ❌ Run `tpg start` on any task
- ❌ Create new implementation work
- ❌ Delegate to other agents for implementation

**YOU ARE A REVIEWER. REVIEW ONLY.**

## What You're Doing

1. **Exploring the codebase** to understand actual state
2. **Reading the plan** using tpg discovery commands
3. **Validating** that tasks reflect reality
4. **FIXING PLAN ISSUES ONLY** — dependencies, descriptions, structure
5. **REPORTING** what was found and changed, then **STOPPING**

## What to Check

### Dependencies reflect actual needs
For each task, determine what it *actually needs* before it can start. Does it depend on schema changes, new packages, config additions, or code that another task creates? That's a real dependency.

- **False blockers** — tasks blocked by something they don't actually need
- **Missing blockers** — tasks missing real prerequisites

### Assumptions match reality
- "Extend X which currently supports Y" when X actually only does Z
- Creating something that already exists under a different name
- Referencing commands, flags, or functions that don't exist yet

### Existing code that would be duplicated
If there's already a function, pattern, or module that overlaps with what a task needs to build, the task should mention it.

### Design coverage
Cross-reference design docs against the task list. Are all specified features covered?

### Tasks describe problems, not solutions
A task should describe: what problem needs to be solved, why it matters, what constraints exist, and what "done" looks like. The developer decides *how* to solve it.

If a task dictates implementation details — function names, algorithms, internal structure — strip that out unless there's a concrete project-level reason it must be done a specific way.

### Epic structure and context

**Shared context** (`--context`): If child tasks need common guidelines, move repeated info to the epic's shared context.

**Closing instructions** (`--on-close`): Epics should have on-close instructions for cleanup (update changelog, merge PR, clean up worktree, etc.).

**Worktree integration**: For epics with worktrees, verify:
- Worktree metadata is set
- On-close includes merge/cleanup instructions
- Tasks are properly scoped to the worktree branch

**Auto-complete behavior**: Epics complete automatically. Delete any tasks like "Complete the epic" or "Mark epic done."

## Common Review Mistakes

- **Reviewing the plan only against itself** instead of the codebase
- **Assuming the name/summary is enough** without reading full task descriptions
- **Adding too much detail** when fixing tasks — over-constrains implementers
- **Comparing deps to description text** instead of actual needs
- **Not considering templates** — repeated patterns should use templates

## Process Overview

1. **Explore codebase** (FIRST - use @explore-code)
2. **Get plan structure** using tpg commands (see skill for details)
3. **Review each epic/task** against codebase
4. **Fix PLAN problems only** using tpg editing commands (deps, descriptions, structure)
5. **STOP and report your findings**

**After reporting: DO NOT CONTINUE TO IMPLEMENTATION.**

## How to Execute

Load the `tpg-review` skill for detailed guidance on:
- Which tpg commands to use for discovery
- How to restructure plans (split tasks into epics)
- Command syntax and workflows
- Best practices for plan modification

## Arguments

$ARGUMENTS

If arguments are provided, focus the review on those areas. Otherwise, perform a general review of all open work.

## Expected Output

Report what you found and what you changed:
- Dependencies added/removed
- Tasks restructured into epics
- Descriptions updated
- Templates identified/applied
- Issues that couldn't be fixed (and why)
