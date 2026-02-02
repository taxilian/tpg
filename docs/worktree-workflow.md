# Worktree Workflow Guide

This guide explains how to use TPG's worktree feature for isolated epic development.

## Overview

Git worktrees allow you to have multiple branches checked out simultaneously in separate directories. TPG integrates worktrees with epics to provide:

- **Isolated development**: Each epic gets its own working directory
- **Branch management**: Automatic branch naming and tracking
- **Context awareness**: Tasks know which worktree they belong to
- **Cleanup guidance**: Instructions for merging and removing worktrees

## Creating Epics with Worktrees

### New Epic with Worktree

```bash
# Create epic with auto-generated branch
tpg add "Implement user authentication" -e --worktree
# Output:
# ep-abc123
#
# Worktree setup instructions:
#   git worktree add -b feature/ep-abc123-implement-user-authentication .worktrees/ep-abc123 main
#   cd .worktrees/ep-abc123
```

### Custom Branch and Base

```bash
# Create epic with custom branch and base
tpg add "Fix API bugs" -e --worktree --branch feature/api-fixes --base develop
# Output:
# ep-def456
#
# Worktree setup instructions:
#   git worktree add -b feature/api-fixes .worktrees/ep-def456 develop
#   cd .worktrees/ep-def456
```

## Adding Worktree to Existing Epic

```bash
# Add worktree metadata to existing epic
tpg epic worktree ep-abc123

# With custom branch
tpg epic worktree ep-abc123 --branch feature/custom-name

# With custom base
tpg epic worktree ep-abc123 --base release/v2.0
```

## Working with Tasks

### Viewing Ready Tasks

```bash
# Show all ready tasks for an epic
tpg ready --epic ep-abc123
# Ready tasks for epic ep-abc123 - Implement user authentication:
# (3 ready)
#
# ID         STATUS  PRI  TITLE
# ts-xyz789  open    2    Set up OAuth2 provider config
# ts-uvw456  open    2    Implement token refresh
# ts-rst123  open    2    Add logout endpoint
```

### Starting Work

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

### Viewing Task Details

```bash
tpg show ts-xyz789
# ID:          ts-xyz789
# Type:        task
# Title:       Set up OAuth2 provider config
# Status:      in_progress
#
# Worktree:
#   Epic:     ep-abc123 - Implement user authentication
#   Branch:   feature/ep-abc123-implement-user-authentication
#   Location: .worktrees/ep-abc123
#   Path:     ep-abc123 -> ts-xyz789
#
#   To create worktree:
#     git worktree add -b feature/ep-abc123-implement-user-authentication .worktrees/ep-abc123 main
#     cd .worktrees/ep-abc123
```

## Nested Epics

TPG supports nested epics (epics within epics). The nearest ancestor with a worktree is used.

```bash
# Create parent epic
tpg add "Authentication System" -e --worktree
# ep-parent123

# Create child epic under parent
tpg add "OAuth2 Implementation" -e --parent ep-parent123 --worktree
# ep-child456

# Create task under child epic
tpg add "Set up OAuth2 config" --parent ep-child456
# ts-task789

# When starting the task, the child epic's worktree is shown
tpg start ts-task789
# 📁 Worktree: ep-child456 - OAuth2 Implementation (nearest)
```

## Completing Epics

```bash
tpg epic finish ep-abc123
```

This command:
1. Validates all descendants are done or canceled
2. Marks the epic as done
3. Prints cleanup instructions

### Standard Epic (merges to main)

```
Epic ep-abc123 marked as done.

Cleanup instructions:
  # Merge to main:
  git checkout main
  git merge feature/ep-abc123-implement-user-authentication
  git worktree remove .worktrees/ep-abc123
  git branch -d feature/ep-abc123-implement-user-authentication
```

### Nested Epic (merges to parent)

```
Epic ep-child456 marked as done.

Cleanup instructions:
  # Merge to parent epic branch (feature/ep-parent123-auth-system):
  git checkout feature/ep-parent123-auth-system
  git merge feature/ep-child456-oauth2-implementation
  git worktree remove .worktrees/ep-child456
  git branch -d feature/ep-child456-oauth2-implementation
```

## JSON/YAML Output

Worktree information is included in structured output:

```bash
tpg show ts-xyz789 --json
```

```json
{
  "item": { ... },
  "worktree": {
    "epic_id": "ep-abc123",
    "epic_title": "Implement user authentication",
    "branch": "feature/ep-abc123-implement-user-authentication",
    "base": "main",
    "location": ".worktrees/ep-abc123",
    "path": ["ep-abc123", "ts-xyz789"]
  }
}
```

## Best Practices

1. **Create worktree immediately**: Use `--worktree` when creating epics to get setup instructions
2. **Use auto-generated branches**: They include the epic ID for easy reference
3. **Check worktree context**: Use `tpg show` to confirm which worktree a task belongs to
4. **Finish properly**: Use `tpg epic finish` to get correct cleanup instructions
5. **Nested epics**: Child epics merge to parent branches, not main

## Troubleshooting

### Worktree already exists

If the branch already has a worktree, TPG will show instructions but won't execute git commands. Check with:

```bash
git worktree list
```

### Wrong worktree shown

TPG finds the nearest ancestor epic with a worktree. If you have nested epics, the closest one wins.

### Missing worktree metadata

If an epic should have worktree metadata but doesn't:

```bash
tpg epic worktree ep-abc123
```
