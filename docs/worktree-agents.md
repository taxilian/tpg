# Worktree Support for Agents

This document explains how TPG's worktree feature helps agents work effectively on epics.

## What Agents Need to Know

When working on tasks under a worktree epic:

1. **Always check worktree context** when starting a task
2. **Navigate to the correct directory** before making changes
3. **Never execute git commands** - TPG only prints instructions
4. **Follow the printed instructions** for worktree setup and cleanup

## Agent Workflow

### 1. Discover Available Work

```bash
# See all ready tasks
tpg ready

# Or filter to a specific epic
tpg ready --epic ep-abc123
```

### 2. Start Work (Worktree Context Shown)

```bash
tpg start ts-xyz789
# Started ts-xyz789
#
# 📁 Worktree: ep-abc123 - Implement user authentication
#    Branch: feature/ep-abc123-implement-user-authentication
#    Location: .worktrees/ep-abc123
#
#    To work in the correct directory:
#    cd .worktrees/ep-abc123
```

**Important**: The agent must `cd` to the worktree directory before making changes.

### 3. Verify Context

```bash
# Confirm worktree details
tpg show ts-xyz789
# Shows worktree section with epic, branch, location, and path
```

### 4. Work in Correct Directory

```bash
# Navigate to worktree
cd .worktrees/ep-abc123

# Now make changes...
# Edit files, run tests, etc.
```

### 5. Complete Task

```bash
# Return to main directory if needed
cd ../..

# Mark task done
tpg done ts-xyz789 "Implemented OAuth2 provider configuration"
```

## Creating Epics with Worktrees

When planning work that needs isolation:

```bash
# Create epic with worktree
tpg add "Implement payment processing" -e --worktree

# Add tasks under the epic
tpg add "Set up Stripe integration" --parent ep-abc123
tpg add "Implement webhook handlers" --parent ep-abc123
tpg add "Add payment confirmation" --parent ep-abc123
```

## Finishing Epics

When all tasks are complete:

```bash
# Validate and finish epic
tpg epic finish ep-abc123

# Follow the printed instructions:
# git checkout main
# git merge feature/ep-abc123-implement-payment-processing
# git worktree remove .worktrees/ep-abc123
# git branch -d feature/ep-abc123-implement-payment-processing
```

## Nested Epics

For large features, create nested epics:

```bash
# Parent epic
tpg add "E-commerce Platform" -e --worktree
# ep-platform

# Child epic for payments
tpg add "Payment System" -e --parent ep-platform --worktree
# ep-payments

# Tasks under child
tpg add "Integrate Stripe" --parent ep-payments
```

When finishing `ep-payments`, it will merge to `ep-platform`'s branch, not main.

## Key Points for Agents

### TPG Never Executes Git Commands

TPG only **prints** git commands. Agents must:
- Copy and paste the commands if they want to execute them
- Or use the instructions as guidance for manual steps

### Nearest Ancestor Wins

If a task is under multiple nested epics with worktrees, TPG shows the nearest one (the one closest to the task in the hierarchy).

### Worktree May Not Exist

TPG stores worktree metadata in the database, but the actual git worktree may not be created yet. TPG always prints creation instructions in `tpg show` output.

### Branch Naming

Auto-generated branches follow: `feature/<epic-id>-<slug>`
- Slug is lowercase title with non-alphanumeric → hyphens
- Example: "Implement Auth" → `feature/ep-abc123-implement-auth`

## Common Scenarios

### Scenario 1: Task Without Worktree Context

```bash
tpg start ts-regular-task
# Started ts-regular-task
# (No worktree section shown - task is not under a worktree epic)
```

Work in the main repository directory.

### Scenario 2: Worktree Metadata But No Directory

```bash
tpg show ts-xyz789
# ...
# Worktree:
#   Epic:     ep-abc123 - Implement auth
#   Branch:   feature/ep-abc123-implement-auth
#   Location: .worktrees/ep-abc123
#   Status:   (check with: git worktree list)
#
#   To create worktree:
#     git worktree add -b feature/ep-abc123-implement-auth .worktrees/ep-abc123 main
#     cd .worktrees/ep-abc123
```

The epic has worktree metadata but the directory may not exist. Follow the instructions to create it.

### Scenario 3: Wrong Directory Warning

If an agent starts working on a task that's under a worktree epic but they're in the wrong directory:

```bash
# In main repo directory
tpg start ts-xyz789
# Started ts-xyz789
#
# 📁 Worktree: ep-abc123 - Implement user authentication
#    ...
#    To work in the correct directory:
#    cd .worktrees/ep-abc123
```

**Action**: `cd` to the worktree directory before proceeding.

## Integration with Agent Hooks

When using `tpg prime` for agent hooks, worktree context is included:

```bash
tpg prime
# Shows:
# - Current project
# - In-progress items
# - Worktree context for active tasks
# - Ready tasks
```

This helps agents understand the full context including which directory to work in.
