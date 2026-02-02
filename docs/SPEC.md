# tpg Update Specification

This document specifies the required behavior changes for the current prog CLI, renamed to tpg, plus template-driven tasks and Opencode-first integration. It avoids implementation detail and focuses on observable behavior.

## Current System (Observed)
- CLI name: `prog` with commands for tasks, epics, dependencies, logs, labels, learnings, and TUI.
- Storage: SQLite database at `~/.prog/prog.db` with backups in `~/.prog/backups/`.
- Projects: optional `-p` flag across commands; empty project allowed.
- Onboarding: `tpg onboard` writes a Task Tracking snippet to `AGENTS.md` and installs an OpenCode plugin.
- Task reading: `prog show` prints task details, logs, deps, and suggested concepts.
- Readiness: `prog ready` filters out tasks with unmet dependencies.
- Tasks vs Epics: Both are items. Epics (`ep-` prefix) are a grouping mechanism; tasks (`ts-` prefix) can have a parent epic. The parent relationship implies dependency inheritance - tasks within an epic are implicitly blocked by the epic's dependencies.

### Dependency Inheritance

When an epic has dependencies (i.e., it is blocked by other items), all tasks within that epic are also blocked. This is automatic - you don't need to manually add the epic's dependencies to each task.

**Example:**
- Epic A depends on Task X (Epic A is blocked until Task X is done)
- Task Y is a child of Epic A
- Task Y shows as "not ready" until Task X is done

**Visual Example:**
```
Epic A [blocked] → depends on → Task X [open]
  └─ Task Y [blocked - inherited from Epic A]
  └─ Task Z [blocked - inherited from Epic A]
```

When viewing a task's dependencies with `tpg show` or `tpg dep <id> list`, both direct dependencies and inherited dependencies from ancestor epics are displayed. Inherited dependencies are marked as such.

**Benefits:**
- No manual dependency management for child tasks
- Automatic coordination when epics have external dependencies
- Clear visibility into why tasks are blocked

## Scope of Change
- Rename CLI to `tpg`.
- Store all data under `$CWD/.tpg/` instead of `~/.prog/`.
- Keep multi-project support but require a default when `-p` is omitted.
- Make Opencode the primary agent integration (replace Claude Code emphasis).
- Add template-driven task creation with variable interpolation, step dependencies, and parent/child task generation.
- Add progress updates to support resuming in-progress tasks.
- Preserve existing task/label/learning/dep capabilities unless explicitly changed here.

## Functional Requirements

### 1) Command and Naming
- CLI executable and help text use `tpg` (not `prog`).
- All user-facing output and docs reference `tpg` commands.
- Any remaining `prog` mentions are removed or redirected as part of the update.

### 1a) Task ID Prefix
- Task IDs use a configurable prefix (default: `ts`) plus a random hash (e.g., `ts-4boe`).
- Epic IDs use a configurable prefix (default: `ep`) plus a random hash.
- `tpg init` allows setting the prefix; it can be changed at any time on a per-project basis.
- New tasks/epics use the current prefix; existing IDs are not renamed.

### 2) Storage Location (Per CWD)
- All persistent data is stored under `.tpg/` in the project root.
- When `tpg` is run from a subdirectory, it searches upward from `$CWD` to locate the nearest `.tpg/` directory and uses that.
- If no `.tpg/` directory is found, commands that require it fail with a clear error prompting `tpg init`.
- `tpg init` creates the `.tpg/` directory in the current working directory.
- The CLI must not create or write new data under `~/.prog/`.

### 3) Projects and Defaults
- Multi-project support via `-p` remains across all commands that currently accept it.
- When `-p` is omitted, the CLI uses a default project value.
- The chosen default must be deterministic and visible in output (so the user can tell what project scope was used).

### 4) Opencode-First Integration
- Opencode is the primary agent target in onboarding and documentation.
- `tpg onboard` (or equivalent) writes the Task Tracking snippet to the Opencode instruction file/location and configures a session-start hook to run `tpg prime` where Opencode supports hooks.
- OpenCode is the primary target; other agents can manually use the workflow.
- `tpg prime` output text references Opencode and `tpg` commands.

### 5) Templates

#### Template Discovery
- Templates are stored as `.toml` or `.yaml` files under `.tgz/templates`.
- When `tpg` is run from a subdirectory, it must search upward from `$CWD` to locate the nearest ancestor containing `.tgz/templates` and use that as the template root.
- If no `.tgz/templates` directory is found up to the filesystem root, template-related commands fail with a clear error.

#### Template Definition
- A template can define variables, each with a description that guides the value the user must provide.
- A template contains an ordered list of steps.
- Each step may define an optional `id` and an optional `depends` list (referencing other step ids).
- If a step id is omitted, it is generated at instantiation time as a random 3-character hash that is unique within the template instance.
- A template may define `worktree: true` to indicate the parent epic should use a git worktree for isolated development.
- Alternatively, templates can use a `use_worktree` variable to allow dynamic worktree selection.

#### Template Instantiation
- Using templates is optional; simple tasks can be created without a template.
- When using a template, instantiation requires a value for every template variable.
- Variable values accept multi-line content; values are passed as JSON-encoded strings to support newlines and special characters.
- Instantiation creates a parent epic (for multi-step templates) or task/epic (for single-step templates based on -e flag) and child tasks for each template step using the existing dependency system.
- Multi-step templates always create a parent epic regardless of the -e flag.
- The parent epic depends on all child tasks via standard dependencies.
- Step dependencies are applied between child tasks based on the template `depends` lists, using standard dependencies.
- If a `depends` entry references a non-existent step id, instantiation fails with no partial task creation.

#### Storage of Templated Tasks
- Templated child tasks persist only:
  - The template identifier used.
  - The step index (based on template order).
  - The instantiated variable values.
- Templated parent tasks persist the template identifier and variable values required to render instance context.
- Each template instance stores a hash of the template content at instantiation time for change detection.
- If the current template content hash differs from the stored hash, the task view surfaces a notice but still renders using the latest template (template evolution is expected).
- No rendered step content is stored directly in the task body.

#### Reading and Rendering
- When a templated task is read (for example via `tpg show`), the CLI interpolates the template step with the stored variables and presents the rendered output as the task description.
- Listing and ready views show a meaningful title for templated tasks derived from the template step.
- Rendered output must provide sufficient context for an LLM to resume work without needing to inspect template files manually.

### 6) Progress Updates
- The system supports progress updates for major milestones on a task using the existing "log progress" mechanism.
- Progress updates are distinct from ordinary logs or are clearly labeled as such.
- When a task is in progress, the most recent progress update is surfaced prominently when the task is viewed so a resuming agent sees it first.
- Progress updates refresh the task's last-updated timestamp.

### 7) Stale Work Detection
- The system provides a `tpg stale` command that lists in-progress tasks with no updates within a threshold.
- The default threshold is 5 minutes; a CLI flag allows overriding the threshold.
- Stale detection uses the task's last-updated timestamp.

### 8) Results Message on Done
- Marking a task done requires a results message.
- Done is blocked when the task has unmet dependencies, unless an override message is explicitly provided at completion time.
- The results message captures what a dependent task implementer needs to proceed.
- For discovery tasks, results should summarize what was learned and/or where to find details.
- For implementation tasks, results should summarize what was done and how to use it.
- The tool does not enforce a rigid results format beyond requiring a message.
- Results are stored in a consistent place and are visible when viewing the task.

### 9) Task View Prioritization
- `tpg show` surfaces a "Latest Update" section that includes the most recent progress update, current blockers (if any), and the results message (if done).
- `tpg show` lists dependency task IDs but does NOT include their full results (to avoid excessive output). Use `tpg show <dep-id>` to view dependency details.
- Output should be concise; pagination or truncation may be needed for tasks with long histories.

## User Flows (Requirements-Level)
- Create tasks from a template by selecting the template and supplying values for all variables; the system creates parent + child tasks and applies step dependencies.
- Resume an in-progress task: view the task to see the latest progress update and the interpolated template content before other logs.
- Find stale in-progress tasks with `tpg stale`, using the default 5-minute threshold or a CLI override.
- Use `-p` to filter or create tasks for specific projects; omit `-p` to use the default project.

## Acceptance Criteria
- Running `tpg init` in two different directories creates separate `.tpg/` stores.
- `tpg` commands do not write to `~/.prog/` for new data.
- `-p` continues to filter projects; omitted `-p` uses a deterministic default that is visible in output.
- `tpg prime` and onboarding instructions are Opencode-first and use `tpg` commands exclusively.
- Template instantiation creates a parent epic plus child tasks with correct dependencies.
- Templated tasks persist only the template reference, step index, and variable values; rendered content appears on read.
- If a template changes after instantiation, `tpg show` indicates the hash mismatch but renders using the latest template.
- Progress updates are visible and prioritized for in-progress tasks.
- Progress updates refresh the task's last-updated timestamp.
- `tpg stale` lists in-progress tasks with no updates in the last 5 minutes by default and supports a threshold override flag.
- Marking done requires a results message and surfaces it on task view.
- `tpg done` is blocked when unmet dependencies exist unless an override message is provided.
- Running `tpg` from a subdirectory locates the nearest ancestor `.tgz/templates` directory for templates.
- When no `.tgz/templates` directory exists in any ancestor, template commands fail with a clear error.
- `tpg show` includes a "Latest Update" section with latest progress, blockers, and results (if done).

## Out of Scope
- New non-functional requirements (performance, uptime, security) beyond what is needed for functional changes.
- Cloud sync or multi-user collaboration.
- Changes to learnings/concepts/labels beyond command renaming or storage relocation.

## Open Decisions (Required for Implementation)
- Default project selection strategy when `-p` is omitted (fixed string vs directory-derived vs user-configured).
- How templates are identified/selected by users within `.tgz/templates`.
- Backward compatibility for `prog` command and whether existing `~/.prog` data should be migrated.
- Opencode hook/config file locations and naming for onboarding.
- How the override message is provided when completing a task with unmet dependencies (flag vs prompt).
- Whether environment variables should be renamed from `PROG_*` to `TPG_*`, and if aliases are required.
- Pagination or truncation strategy for long `tpg show` output.

---

## Implementation Analysis: What Exists vs What Needs to Change

### Already Works As Needed (No Changes Required)

| Feature | Location | Notes |
|---------|----------|-------|
| Task/Epic distinction | `model/item.go` | ItemType with "task"/"epic" values |
| Dependency system | `db/deps.go` | AddDep, GetDeps, HasUnmetDeps |
| `blocks` command | `main.go:864-886` | `tpg blocks A B` makes B depend on A |
| Ready filtering | `db/queries.go:102-147` | Excludes tasks with unmet deps |
| Parent/child hierarchy | `db/items.go:104-129` | SetParent for epic grouping |
| Logs system | `db/logs.go` | AddLog, GetLogs for progress tracking |
| Labels system | `db/labels.go` | Full CRUD, item associations |
| Learnings/concepts | `db/learnings.go` | Context engine intact |
| TUI | `tui/tui.go` | Works with db package |
| Backup/restore | `db/backup.go` | Core logic reusable |
| Schema migrations | `db/db.go:117-258` | Version-based migration system |

### Requires Renaming Only (String Replacements)

| Item | Files | Current | New |
|------|-------|---------|-----|
| CLI name | `main.go:96` | `"prog"` | `"tpg"` |
| Binary directory | `cmd/prog/` | `prog` | `tpg` |
| Module path | `go.mod:1` | `github.com/baiirun/prog` | TBD |
| Help text | `main.go` (many) | `prog <cmd>` | `tpg <cmd>` |
| prime output | `main.go:2502-2608` | `prog` references | `tpg` references |
| compact output | `main.go:2634-2691` | `prog` references | `tpg` references |
| CLAUDE.md snippet | `main.go:1528-1543` | `prog` commands | `tpg` commands |
| Temp file pattern | `main.go:721` | `prog-edit-*.md` | `tpg-edit-*.md` |
| Backup filename | `backup.go:43,87,139` | `prog-%s.db` | `tpg-%s.db` |
| README.md | entire file | `prog` everywhere | `tpg` everywhere |
| AGENTS.md | entire file | `prog` references | `tpg` references |
| TUI header | `tui/tui.go:702` | `"prog"` | `"tpg"` |

### Requires Logic Changes

#### 1. Storage Path (High Impact)

**Current:**
```go
// db/db.go:150-161
func DefaultPath() (string, error) {
    if envPath := os.Getenv("PROG_DB"); envPath != "" {
        return envPath, nil
    }
    home, err := os.UserHomeDir()
    return filepath.Join(home, ".prog", "prog.db"), nil
}

// db/backup.go:19-26
func BackupPath() (string, error) {
    home, err := os.UserHomeDir()
    return filepath.Join(home, ".prog", "backups"), nil
}
```

**Needed:**
- Search upward from CWD for `.tpg/` directory
- If not found, error with "run tpg init"
- `tpg init` creates `.tpg/` in current directory
- Env var rename: `PROG_DB` → `TPG_DB`
- Backup path relative to found `.tpg/`

#### 2. Configurable ID Prefix (Medium Impact)

**Current:**
```go
// model/item.go:15-25
func GenerateID(itemType ItemType) string {
    prefix := "ts-"
    if itemType == ItemTypeEpic {
        prefix = "ep-"
    }
    b := make([]byte, 3)
    rand.Read(b)
    return prefix + hex.EncodeToString(b)
}
```

**Needed:**
- Store prefix config in `.tpg/` (e.g., `.tpg/config.json`)
- `tpg init` accepts `--prefix` flag
- `tpg config prefix <value>` to change later
- GenerateID reads from config

#### 3. Onboarding (Medium Impact)

**Current:**
- Writes to `AGENTS.md`
- Installs OpenCode plugin at `.opencode/plugins/tpg.ts`
- Plugin injects `tpg prime` context automatically

#### 4. `tpg done` Requires Results Message (Medium Impact)

**Current:**
```go
// main.go:380-410
var doneCmd = &cobra.Command{
    Use:   "done <id>",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // Just updates status, no message required
        database.UpdateStatus(id, model.StatusDone)
    },
}
```

**Needed:**
- Change to `cobra.MinimumNArgs(2)` or prompt if missing
- Store results message (new column or special log entry)
- Block if unmet deps unless override flag provided
- Surface results in `tpg show`

#### 5. `tpg stale` Command (New Feature)

**Current:** Does not exist

**Needed:**
- New command listing in_progress tasks with old `updated_at`
- Default threshold: 5 minutes
- `--threshold` flag to override
- Requires `updated_at` to be refreshed on progress logs

#### 6. `tpg show` Enhancements (Medium Impact)

**Current:**
```go
// main.go:290-370 (showCmd)
// Shows: item details, logs, deps (IDs), suggested concepts
```

**Needed:**
- "Latest Update" section with most recent progress log
- List dependency IDs (already does this)
- Show results message if done
- Pagination/truncation for long output

#### 7. Template System (New Feature - High Impact)

**Current:** Does not exist

**Needed:**
- Template file parsing (TOML/YAML from `.tgz/templates/`)
- Template discovery (search upward for `.tgz/templates/`)
- Variable validation and JSON-encoded input
- Instantiation: create epic + child tasks with deps
- New DB columns: `template_id`, `step_index`, `variables` (JSON), `template_hash`
- Rendering: interpolate template on read
- Schema migration for new columns

### New Database Columns Required

| Table | Column | Type | Purpose |
|-------|--------|------|---------|
| items | template_id | TEXT | Reference to template file |
| items | step_index | INTEGER | Which step in template (NULL for non-templated) |
| items | variables | TEXT (JSON) | Instantiated variable values |
| items | template_hash | TEXT | Hash of template at instantiation |
| items | results | TEXT | Results message when done |

Or alternatively, results could be a specially-tagged log entry.

### Environment Variables

| Current | New | Used In |
|---------|-----|---------|
| `PROG_DB` | `TPG_DB` | db/db.go:153 |
| `PROG_EDITOR` | `TPG_EDITOR` | main.go:708 |

### Files to Create

| File | Purpose |
|------|---------|
| `.tgz/templates/*.toml` or `.yaml` | Template definitions (user-created) |
| `.tpg/config.json` | ID prefix and other settings |

### Files to Rename

| Current | New |
|---------|-----|
| `cmd/prog/` | `cmd/tpg/` |
| `cmd/prog/main.go` | `cmd/tpg/main.go` |
| `cmd/prog/*_test.go` | `cmd/tpg/*_test.go` |

### Test Files Requiring Updates

| File | Changes |
|------|---------|
| `cmd/prog/prime_test.go` | String assertions for "tpg" |
| `cmd/prog/onboard_test.go` | Path assertions, hook command |
| `cmd/prog/add_test.go` | Minimal changes |
| `internal/db/db_test.go` | Path assertions |
| `internal/db/*_test.go` | Minimal changes |
