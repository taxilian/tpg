---
name: tpg-review
description: >-
  Review tpg plans against the codebase for correctness. Use when asked to review
  plans, verify dependencies, or validate tasks against reality. Teaches effective
  use of tpg discovery and editing commands for plan review and restructuring.
---

# tpg-review

Review tpg plans against the actual codebase to ensure implementers can succeed.
This skill teaches you how to use tpg commands effectively for discovery and plan modification.

## ALWAYS use this skill when

- Asked to review a tpg plan or task list for correctness.
- Need to verify dependencies, assumptions, or templates against the codebase.
- Running the `review-tpg` command.
- Need to restructure plans (split tasks into epics, fix dependencies, etc.).

## Discovery: Getting the Full Picture

Unless reviewing a specific epic, you need a complete view of the project.

### Quick overview
```bash
tpg list                    # Tree view of all epics and tasks
tpg list --flat             # Flat list for easier scanning
tpg status                  # Project health summary
```

### Full details for analysis
```bash
# Export everything as JSONL for programmatic analysis
tpg export --jsonl > plan.jsonl

# Or export to a file for reading
tpg export -o plan.md

# Export specific epics
tpg export --parent <epic-id> --jsonl
```

### Focused epic review
```bash
tpg plan <epic-id>          # Visual tree with status
tpg export --parent <epic-id>  # Full details
```

## Restructuring Plans

When you find tasks that need to become epics, or epics that need restructuring:

### Convert a task to an epic

Use `--from-yaml` to set multiple fields at once (context, on-close, priority):

```bash
tpg epic replace <task-id> "New Epic Title" --from-yaml <<EOF
context: |
  Context shared with all descendant tasks.
  Include API versions, patterns, constraints.
on_close: |
  Remember to update the changelog.
  Run integration tests.
priority: 1
EOF
```

For simpler cases, use individual flags:
```bash
tpg epic replace <task-id> "New Epic Title" --context - <<EOF
Context shared with all children.
EOF
```

### Update descriptions with HEREDOC
Always prefer HEREDOC for multi-line descriptions:

```bash
tpg desc <task-id> - <<EOF
## Objective
Clear goal statement.

## Context
Why this matters and background info.

## Acceptance Criteria
- [ ] Specific criterion 1
- [ ] Specific criterion 2

## Dependencies
Blocked by: <id> (reason)
EOF
```

### Fix dependencies
```bash
# Add dependency
tpg dep <blocker-id> blocks <blocked-id>

# Remove incorrect dependency
tpg dep <id> remove <other-id>

# View dependencies
tpg dep <id> list
```

## Review Workflow

1. **Explore codebase** - Use @explore-code to understand actual state
2. **Get plan data** - Use `tpg export --jsonl` or `tpg list`
3. **Validate** - Check against codebase:
   - Dependencies reflect actual prerequisites
   - Assumptions match current code
   - No duplication of existing code
   - Tasks describe problems, not solutions
4. **Fix issues** - Use `tpg desc`, `tpg epic replace`, `tpg dep`
5. **Verify** - Re-export and confirm fixes

## What to Check

- **Dependencies**: Real prerequisites vs false blockers
- **Assumptions**: Code references actually exist
- **Duplication**: Existing code referenced appropriately
- **Epic structure**: Shared context used, on-close instructions set
- **Templates**: Repeated patterns use templates
- **Task scope**: Describes problem/constraints, not step-by-step solutions

## Commands Reference

```bash
# Discovery
tpg list
tpg list --flat
tpg status
tpg export --jsonl
tpg export --parent <epic-id>
tpg plan <epic-id>

# Restructuring
tpg epic replace <task-id> "Title" --context -
tpg epic add "Title" --context -
tpg edit <id> --parent <new-parent>
tpg desc <id> -
tpg dep <id> blocks <other>
tpg dep <id> remove <other>

# Templates
tpg template list
tpg template show <id>
```

## Gotchas

- Don't review the plan in isolation - always check against codebase
- Don't over-constrain tasks with implementation details
- Don't create "close the epic" tasks - epics auto-complete
- Don't duplicate context across tasks - use epic shared context
- Always use HEREDOC for multi-line text
