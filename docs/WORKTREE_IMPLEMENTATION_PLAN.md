# Worktree Implementation Plan

This document outlines how to implement the worktree support feature using multiple git worktrees for parallel development.

## Overview

The worktree feature itself will be the first use case of worktrees for its own implementation. We'll split the work into logical groups that can be developed in parallel in separate worktrees.

## Worktree Structure

### Main Epic: WORKTREE-1 (this worktree)
**Branch:** `feature/WORKTREE-1-worktree-support`
**Purpose:** Foundation and coordination
**Work:**
- Schema migration (v4)
- Model updates
- Configuration support
- Documentation

### Worktree 2: WORKTREE-2 (new worktree)
**Branch:** `feature/WORKTREE-2-git-helpers`
**Purpose:** Git detection utilities
**Depends on:** WORKTREE-1 (schema, model)
**Work:**
- Git helpers package
- Repo root detection
- Worktree detection
- Branch reading

### Worktree 3: WORKTREE-3 (new worktree)
**Branch:** `feature/WORKTREE-3-db-queries`
**Purpose:** Database queries for worktree support
**Depends on:** WORKTREE-1 (schema, model)
**Work:**
- FindEpicByBranch
- ReadyItemsForEpic
- GetRootEpic
- FindEpicsWithWorktreeButNoWorktree

### Worktree 4: WORKTREE-4 (new worktree)
**Branch:** `feature/WORKTREE-4-edit-refactor`
**Purpose:** tpg edit expansion (can be done in parallel)
**Depends on:** None (independent refactor)
**Work:**
- Multiple ID support
- --select filters
- Remove set-status/parent
- Bulk operations

### Worktree 5: WORKTREE-5 (new worktree)
**Branch:** `feature/WORKTREE-5-worktree-commands`
**Purpose:** Worktree-specific commands
**Depends on:** WORKTREE-2 (git helpers), WORKTREE-3 (db queries)
**Work:**
- tpg add -e --worktree
- tpg epic worktree
- tpg epic finish
- tpg ready --epic

### Worktree 6: WORKTREE-6 (new worktree)
**Branch:** `feature/WORKTREE-6-context-display`
**Purpose:** Context display in existing commands
**Depends on:** WORKTREE-2 (git helpers), WORKTREE-3 (db queries)
**Work:**
- tpg show worktree context
- tpg start guidance
- tpg plan header
- Prime template updates

## Issue Grouping

### Group 1: Foundation (WORKTREE-1)
Issues:
- WORKTREE-1.1: Schema migration v4 - add worktree_branch and worktree_base columns
- WORKTREE-1.2: Model updates - Item struct fields and scanning
- WORKTREE-1.3: Configuration - worktree config in config.json
- WORKTREE-1.4: Documentation - WORKTREE_SUPPORT.md finalization

### Group 2: Git Helpers (WORKTREE-2)
Issues:
- WORKTREE-2.1: Find repo root via .git (handles submodules)
- WORKTREE-2.2: List worktrees by reading .git/worktrees/
- WORKTREE-2.3: Detect if in worktree (check if .git is file)
- WORKTREE-2.4: Get current branch from .git/HEAD

### Group 3: Database Queries (WORKTREE-3)
Issues:
- WORKTREE-3.1: FindEpicByBranch query
- WORKTREE-3.2: ReadyItemsForEpic query (descendants + ready criteria)
- WORKTREE-3.3: GetRootEpic query (walk parent chain)
- WORKTREE-3.4: FindEpicsWithWorktreeButNoWorktree query

### Group 4: Edit Refactor (WORKTREE-4)
Issues:
- WORKTREE-4.1: tpg edit multiple IDs support
- WORKTREE-4.2: tpg edit --select filters
- WORKTREE-4.3: Remove tpg set-status command
- WORKTREE-4.4: Remove tpg parent command (migrate to edit)
- WORKTREE-4.5: tpg list --ids-only flag

### Group 5: Worktree Commands (WORKTREE-5)
Issues:
- WORKTREE-5.1: tpg add -e --worktree flag and metadata storage
- WORKTREE-5.2: Branch name generation with epic id
- WORKTREE-5.3: tpg epic worktree subcommand
- WORKTREE-5.4: tpg epic finish subcommand
- WORKTREE-5.5: tpg ready --epic filtering

### Group 6: Context Display (WORKTREE-6)
Issues:
- WORKTREE-6.1: tpg show epic ancestry and worktree info
- WORKTREE-6.2: Worktree status indicators (✓/✗/⚠)
- WORKTREE-6.3: tpg start worktree guidance
- WORKTREE-6.4: tpg plan worktree header
- WORKTREE-6.5: Prime template worktree context
- WORKTREE-6.6: Prime template "Setup Needed" section

## Dependencies

```
WORKTREE-1 (Foundation)
    ├── WORKTREE-2 (Git Helpers)
    │       └── WORKTREE-5 (Worktree Commands)
    │       └── WORKTREE-6 (Context Display)
    ├── WORKTREE-3 (DB Queries)
    │       └── WORKTREE-5 (Worktree Commands)
    │       └── WORKTREE-6 (Context Display)
    └── WORKTREE-4 (Edit Refactor) [independent]
```

## Merge Order

1. WORKTREE-1 (Foundation) - First, required by almost everything
2. WORKTREE-4 (Edit Refactor) - Can merge anytime, independent
3. WORKTREE-2 (Git Helpers) and WORKTREE-3 (DB Queries) - Can be parallel
4. WORKTREE-5 (Worktree Commands) - After 2, 3
5. WORKTREE-6 (Context Display) - After 2, 3

## Testing Strategy

Each worktree should:
1. Write tests first (TDD)
2. Ensure tests pass before merging
3. Integration tests in main worktree after merge

## Notes

- WORKTREE-4 (Edit Refactor) is independent and can be worked on immediately
- WORKTREE-1 must be first as it defines the schema
- WORKTREE-5 and WORKTREE-6 can be worked on in parallel after 2 and 3
- Each worktree should focus on its specific group; resist scope creep
