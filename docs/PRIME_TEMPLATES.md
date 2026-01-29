# Prime Template Customization

Tpg's `prime` command outputs context for AI agents at session start. You can customize this output using Go templates.

## Quick Start

Create a custom template:

```bash
tpg prime --customize
```

This creates `.tpg/PRIME.md` in your project and opens it in `$EDITOR`. Edit the template, save, and run `tpg prime` to see the result.

## Template Locations

Templates are searched in this order (first match wins):

1. **Project**: `.tpg/PRIME.md` (searched upward from current directory)
2. **User**: `~/.config/tpg/PRIME.md`
3. **Global**: `~/.config/opencode/tpg-prime.md`
4. **Default**: Built-in template (if no custom template found)

This allows project-specific templates to override user/global templates.

## Template Data

Your template has access to a `PrimeData` struct with these fields:

### Status Counts
```
.Open          int  - Count of open tasks
.InProgress    int  - Count of in-progress tasks (all agents)
.Blocked       int  - Count of blocked tasks
.Done          int  - Count of done tasks
.Canceled      int  - Count of canceled tasks
.Ready         int  - Count of ready tasks (unblocked, no dependencies)
```

### Agent Context
```
.AgentID       string  - Current agent ID (from $AGENT_ID)
.AgentType     string  - Agent type (from $AGENT_TYPE)
.IsSubagent    bool    - True if agent type != "general"

.MyInProgItems []PrimeItem  - Agent's in-progress tasks
.OtherInProgCount int       - Count of other agents' in-progress tasks
```

### Project & Config
```
.Project        string - Current project name
.TaskPrefix     string - Task ID prefix (e.g. "ts")
.EpicPrefix     string - Epic ID prefix (e.g. "ep")
.DefaultProject string - Default project from config
.HasDB          bool   - True if database is available
```

### Knowledge Base
```
.ConceptCount   int - Number of concepts in knowledge base
.LearningCount  int - Number of learnings in knowledge base
```

### PrimeItem Structure

Each item in `.MyInProgItems` has:
```
.ID       string - Task ID (e.g. "ts-abc123")
.Title    string - Task title
.Priority int    - Priority (1=high, 2=normal, 3=low)
```

## Template Functions

Three helper functions are available:

### `add`
Add two integers.
```
Total: {{add .Open .InProgress}}
```

### `sub`
Subtract two integers.
```
Remaining: {{sub .Open .Done}}
```

### `plural`
Choose singular or plural form based on count.
```
{{.Ready}} {{plural .Ready "task" "tasks"}}
```
Output: `1 task` or `5 tasks`

## Example Templates

### Minimal Template

```markdown
# Tpg

{{if .HasDB -}}
Project: {{.Project}}
Status: {{.Open}} open, {{.InProgress}} in progress, {{.Done}} done
{{.Ready}} ready to work on

Run 'tpg ready' to see tasks.
{{else -}}
No database - run 'tpg init'
{{end}}
```

### Agent-Focused Template

```markdown
# Tpg Status

{{if not .HasDB -}}
No database. Run 'tpg init' first.
{{else -}}

{{if gt (len .MyInProgItems) 0 -}}
## Your Active Work
{{range .MyInProgItems}}
- [{{.ID}}] {{.Title}}{{if eq .Priority 1}} ⚡{{end}}
{{end}}
{{end}}

## Available Work
- {{.Ready}} {{plural .Ready "task" "tasks"}} ready
{{if gt .OtherInProgCount 0}}- {{.OtherInProgCount}} in progress (other agents){{end}}
{{if gt .Blocked 0}}- {{.Blocked}} blocked{{end}}

Commands: `tpg ready` | `tpg start <id>` | `tpg done <id>`
{{end}}
```

### Detailed Template

```markdown
# Tpg Context for {{.Project}}

{{if not .HasDB -}}
Database not initialized. Run 'tpg init' to get started.
{{else -}}

## Current State

**Open**: {{.Open}} | **In Progress**: {{.InProgress}} | **Blocked**: {{.Blocked}} | **Done**: {{.Done}}

{{if gt (len .MyInProgItems) 0 -}}
### Your Work
{{range .MyInProgItems}}
  {{.ID}}: {{.Title}} (priority={{.Priority}})
{{end}}
{{else -}}
No active work assigned to you.
{{end}}

{{if gt .OtherInProgCount 0 -}}
*Note: {{.OtherInProgCount}} {{plural .OtherInProgCount "task" "tasks"}} assigned to other agents*
{{end}}

### Ready to Work On
{{if gt .Ready 0 -}}
There {{plural .Ready "is" "are"}} **{{.Ready}}** unblocked {{plural .Ready "task" "tasks"}}.
Run `tpg ready` to see {{plural .Ready "it" "them"}}.
{{else -}}
No tasks are currently ready. Check `tpg status` for details.
{{end}}

{{if gt (add .ConceptCount .LearningCount) 0 -}}
### Knowledge Base
- {{.ConceptCount}} {{plural .ConceptCount "concept" "concepts"}}
- {{.LearningCount}} {{plural .LearningCount "learning" "learnings"}}

Use `tpg concepts` to explore, `tpg context -c <name>` to load.
{{end}}

## Quick Reference

**Find work**: `tpg ready` | `tpg show <id>`
**Start work**: `tpg start <id>` | `tpg log <id> "message"`
**Complete**: `tpg done <id>` | `tpg block <id> "reason"`
**Context**: `tpg context -c <concept>` | `tpg learn "..." -c <concept>`

{{end}}
```

## Testing Templates

Use the `--render` flag to test a template file without installing it:

```bash
tpg prime --render /path/to/template.md
```

This renders the template and outputs the result, useful for:
- Testing syntax before committing
- Comparing different template versions
- Debugging template issues

## Template Syntax

Templates use Go's `text/template` syntax. Key constructs:

### Conditionals
```
{{if .HasDB}}
  Database is available
{{else}}
  No database
{{end}}
```

### Range (loops)
```
{{range .MyInProgItems}}
  Task: {{.ID}}
{{end}}
```

### Comparisons
```
{{if gt .Ready 5}}
  Many tasks available!
{{end}}
```

Operators: `eq` (==), `ne` (!=), `lt` (<), `le` (<=), `gt` (>), `ge` (>=)

### Comments
```
{{/* This is a comment */}}
```

## Common Patterns

### Show section only if items exist
```markdown
{{if gt (len .MyInProgItems) 0 -}}
## Your Tasks
{{range .MyInProgItems}}
- {{.ID}}: {{.Title}}
{{end}}
{{end}}
```

The `-` after `{{if` removes trailing whitespace, preventing blank lines.

### Conditional formatting
```markdown
{{range .MyInProgItems}}
- [{{.ID}}] {{.Title}}{{if eq .Priority 1}} ⚡HIGH{{end}}
{{end}}
```

### Plural with custom text
```markdown
You have {{.Ready}} {{plural .Ready "unblocked task" "unblocked tasks"}}
```

### Calculations
```markdown
Total work: {{add (add .Open .InProgress) .Blocked}}
Work remaining: {{sub (add .Open .InProgress) 0}}
```

## Validation

When you save a template via `--customize`, tpg validates the syntax. If there are errors, you'll see:

```
Warning: Template has syntax errors: template: prime:5: unexpected "}"
Fix errors and run 'tpg prime' to test.
```

The template is saved even if invalid, so you can fix and re-test.

## Fallback Behavior

If a custom template exists but fails to render (runtime error, not syntax), tpg falls back to the default template and prints an error:

```
Error rendering prime template from .tpg/PRIME.md: ...
Falling back to default output.
```

This ensures `tpg prime` always produces output, even with a broken template.

## Default Template

The built-in default template is condensed and focused on commands:

```markdown
# Tpg Context

This project uses 'tpg' for cross-session task management.
Project: myproject

## Status
Your work:
  • [ts-abc123] Fix authentication bug

- 5 ready (use 'tpg ready')
- 2 in progress (other agents)
- 1 blocked
- 10 done, 3 open
- 5 learnings in 3 concepts

## Workflow

**Start:** 'tpg ready' → 'tpg show <id>' → 'tpg start <id>'
**During:** 'tpg log <id> "progress"'
**Finish:** 'tpg done <id>' or 'tpg block <id> "reason"'
**Context:** 'tpg concepts' → 'tpg context -c <name>'

## Key Commands

  tpg ready         # Available work
  tpg show <id>     # Task details
  tpg start <id>    # Claim task
  tpg log <id> "msg"  # Log progress
  tpg done <id>     # Complete
  tpg status        # Overview
  tpg context -c X  # Load learnings
```

You can see the full default template source in `internal/prime/prime.go`.

## Best Practices

### Keep It Concise
Prime output is injected at session start. Keep it scannable - avoid walls of text.

### Focus on Next Steps
Emphasize what the agent should do next, not comprehensive documentation.

### Use Conditionals Wisely
Show sections only when relevant (e.g., "Your Work" only if you have work).

### Test Before Committing
Use `--render` to test templates before committing them to version control.

### Consider Your Audience
Templates for subagents might differ from templates for main agents. Use `.IsSubagent` to branch.

### Preserve Key Commands
Always include at least: `tpg ready`, `tpg start`, `tpg done`, `tpg status`

## Debugging

If your template isn't working:

1. **Syntax errors**: Run `tpg prime --customize`, save, check for validation errors
2. **Missing data**: Add `{{printf "%#v" .}}` to dump all available data
3. **Wrong logic**: Use `--render` to test with current database state
4. **Comparison issues**: Remember Go template comparisons are strict (use `eq`, not `==`)

## Examples by Use Case

### For Daily Standup Context
```markdown
# Daily Status for {{.Project}}

{{if gt (len .MyInProgItems) 0 -}}
**Yesterday**: Worked on {{len .MyInProgItems}} {{plural (len .MyInProgItems) "task" "tasks"}}
{{range .MyInProgItems}}- {{.Title}}
{{end}}
{{end}}

**Today**: {{.Ready}} {{plural .Ready "task" "tasks"}} ready
**Blockers**: {{.Blocked}} blocked
```

### For Knowledge-Heavy Projects
```markdown
# {{.Project}} - Knowledge First

{{if gt .LearningCount 0 -}}
📚 {{.LearningCount}} learnings across {{.ConceptCount}} concepts
Use `tpg concepts` to explore before starting work.
{{end}}

{{if gt .Ready 0 -}}
🚀 {{.Ready}} tasks ready - `tpg ready`
{{end}}
```

### For Subagents
```markdown
{{if .IsSubagent -}}
# Subagent Context

Quick task lookup: `tpg show <id>`
Return context via structured output.
{{else -}}
# Main Agent Context
[...full template...]
{{end}}
```

## See Also

- [AGENT_AWARE.md](AGENT_AWARE.md) - Agent tracking features
- [AGENTS.md](../AGENTS.md) - Instructions for AI agents
- Go text/template docs: https://pkg.go.dev/text/template
