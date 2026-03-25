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

**OpenCode Plugin:**
The TPG OpenCode plugin (`internal/plugin/opencode.ts`) automatically injects `AGENT_ID` and `AGENT_TYPE` environment variables into all `tpg` bash commands. This enables proper agent tracking and attribution. The plugin:
- Injects env vars before tpg command execution
- Removes any duplicates if agents mistakenly include them
- Determines agent type (primary/subagent) from session parentID
- Provides subagent inspection tools (metadata, messages, errors, work summary)

**When to use:**
- New feature/product → spec-designer → tpg-implementation-planner → tpg-planner
- Start implementation → tpg-orchestrator
- Single task → tpg-agent
- Understand codebase → explore-code

## Worktree Merge Protocol

Worktree-enabled epics use dedicated git worktrees for isolated development. Each worktree epic has its own branch and directory (`.worktrees/<epic-id>`). This protocol governs how worktree epics merge back to their parent branch.

### Epic Lifecycle

**Worktree epics stay OPEN until explicitly merged** via `tpg epic merge`. Unlike regular epics that auto-close when all children complete, worktree epics require manual merge because:

1. The git merge step is distinct from task completion
2. Merge order must follow git's ancestry graph
3. The merge may fail if the parent branch has moved

**Ready to merge:** A worktree epic shows "ready to merge" when all child tasks are done. This does NOT automatically merge - it signals that the epic is eligible for merge.

### Merge Order

**Child epics must be merged before parent epics.** The git graph enforces this: if epic B is based on epic A's branch, you must merge A before you can merge B.

tpg tracks this via worktree parent relationships. When you run `tpg epic merge`, it verifies that all child worktree epics are already merged.

**Example:**
```
main
 └─ feature/ep-auth      (Epic A: Auth system)
     └─ feature/ep-oauth (Epic B: OAuth integration)
```

Merge order: ep-oauth → ep-auth → main

If you try to merge ep-auth before ep-oauth, tpg will error.

### The Merge Protocol

**Command:** `tpg epic merge <epic-id>`

**What it does:**

1. **Verify worktree is clean** - No uncommitted changes allowed
2. **Rebase worktree onto parent branch** - `git rebase <parent-branch>` in worktree directory
3. **Checkout parent branch** - Switch to the parent branch
4. **Fast-forward merge** - `git merge --ff-only <worktree-branch>`
5. **Mark merged in database** - Epic status becomes "merged"
6. **Auto-close epic** - Epic transitions to "done" after successful merge

**If step 4 fails** (parent branch has moved since rebase), the protocol automatically retries from step 2. This handles race conditions when multiple epics merge concurrently.

**Error cases:**
- Uncommitted changes in worktree → Manual resolution required
- Child worktree epics not merged → Merge children first
- Rebase conflicts → Manual conflict resolution required

### Commands

```bash
# Check if epic is ready to merge
tpg show <epic-id>

# Execute merge protocol
tpg epic merge <epic-id>

# View worktree status
tpg show <task-id>  # Shows worktree context for tasks
```

**Agent workflow:**

- **tpg-agent (subagent):** Completes tasks, commits changes to worktree branch, reports when last task done
- **tpg-orchestrator (primary):** Runs `tpg epic merge` after all children complete

**CRITICAL:** Subagents do NOT merge. Only the orchestrator handles merges via `tpg epic merge`.
