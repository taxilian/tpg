## Communication

Speak concisely and directly. No filler praise, sycophancy, or excessive flattery ("Great question!", "Absolutely!", "That's smart!"). Be an efficient tool.

## Task Tracking

This project uses **tpg** for cross-session task management.
Run `tpg prime` for workflow context, or configure hooks for auto-injection.

**Quick reference:**
```
tpg ready                        # Find unblocked work
tpg ready --epic <id>            # Filter to epic's tasks
tpg start <id>                   # Claim work
tpg done <id>                    # Complete work
tpg dep <id> blocks <other-id>   # Set dependency
tpg dep <id> list                # Show dependencies
tpg edit <id> --parent <id>      # Change parent

# Creating tasks — always use heredoc for full context:
tpg add "Title" -p 1 --desc - <<EOF
What to do, why it matters, constraints, acceptance criteria.
Future agents won't have your current context—be thorough.
EOF

# Logging progress — always use heredoc for detail:
tpg log <id> - <<EOF
Decisions made, alternatives considered, blockers found,
milestones reached. Skip routine actions (opened file, ran cmd).
EOF
```

For full workflow: `tpg prime`

## Templates

**MANDATORY:** Before creating ANY task, you MUST run `tpg template list` and check if a template applies. No exceptions.

Check `.tpg/templates/` for reusable task templates before creating tasks manually:
```
tpg template list                                    # List available templates (MANDATORY FIRST STEP)
tpg template show <id>                               # View template details
tpg add "Title" --template <id> --var 'key="value"'  # Create from template
```

**Rule:** If a template exists that fits the work, use it. Only create ad-hoc tasks when no template is appropriate.

## Unit Testing

**CRITICAL:** Unit tests MUST NOT write to `.tpg/tpg.db` in the project root.

This project dogfoods tpg for its own task management. The `.tpg/tpg.db` database contains real development tasks. Tests that accidentally write to or read from this database will corrupt project state.

**Required test patterns:**
- Use `t.TempDir()` for test databases
- Set `TPG_DB` env var to temp path if testing CLI commands
- Never `os.Chdir` to the project root without restoring afterward
- Never call `db.Open()` without an explicit temp path

**Example:**
```go
func setupTestDB(t *testing.T) *db.DB {
    t.Helper()
    dir := t.TempDir()
    path := filepath.Join(dir, "test.db")
    // ... open and init db at path
}
```

## Git Safety

**NEVER** run destructive git commands that discard uncommitted work:
- `git restore` — discards working tree changes
- `git reset --hard` — resets working tree and index
- `git checkout -- <file>` — reverts file to last commit
- `git clean` — removes untracked files
- `git stash drop` / `git stash clear` — destroys stash entries

If you made an error, **stop and inform the user.** Let them decide how to recover.

Allowed: `git add`, `git commit`, `git diff`, `git status`, `git log`, `git show`, `git blame`, `git branch`, `git switch`, `git push`, etc.

## Agents

Agent definitions are in `.opencode/agent/`:

**Planning & Design:**
- **spec-designer** - Create product specifications from business requirements (primary)
- **tpg-implementation-planner** - Design technical architecture and components (primary)
- **tpg-planner** - Break specs into tpg tasks with dependencies (all modes)

**Execution:**
- **tpg-orchestrator** - Coordinate parallel work, manage templates (primary)
- **tpg-agent** - Execute individual tpg tasks (subagent)
- **explore-code** - Explore codebase via code connections (subagent)

**When to use:**
- New feature/product → spec-designer → tpg-implementation-planner → tpg-planner
- Start implementation → tpg-orchestrator
- Single task → tpg-agent
- Understand codebase → explore-code
