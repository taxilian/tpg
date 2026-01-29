# TPG Context

This project uses **tpg** for cross-session task management with template support.
{{if .Project}}Project: {{.Project}}{{else if .DefaultProject}}Default: {{.DefaultProject}}{{end}}

## Status
{{if not .HasDB -}}
No database - run `tpg init`
{{else -}}
{{if gt (len .MyInProgItems) 0 -}}
**Your work:**
{{range .MyInProgItems}}  - [{{.ID}}] {{.Title}}{{if eq .Priority 1}} (HIGH){{end}}
{{end}}
{{end -}}
{{if gt .Ready 0}}- {{.Ready}} ready (use `tpg ready`)
{{end -}}
{{if gt .OtherInProgCount 0}}- {{.OtherInProgCount}} in progress (other agents)
{{end -}}
{{if gt .Blocked 0}}- {{.Blocked}} blocked
{{end -}}
- {{.Done}} done, {{.Open}} open
{{if gt .ConceptCount 0}}- {{.LearningCount}} {{plural .LearningCount "learning" "learnings"}} in {{.ConceptCount}} {{plural .ConceptCount "concept" "concepts"}}
{{end -}}
{{end}}

## Workflow

**Find work:** `tpg ready` -> `tpg show <id>` -> `tpg start <id>`
**During:** `tpg log <id> "progress"` -- log significant discoveries and milestones as you work
**Finish:** `tpg done <id> "results"` or `tpg block <id> "reason"`
**Context:** `tpg concepts` -> `tpg context -c <name>`

**Logging:** While working on a task, use `tpg log` to record:
- Significant discoveries (unexpected behavior, key insights, design decisions)
- Implementation milestones (tests passing, component complete, integration done)
This builds cross-session context so progress is never lost.

## Templates

Check available templates before creating tasks:
```
tpg template list              # See all templates
tpg template show <id>         # View template details
tpg add "Title" --template <id> --var 'key="value"'
```

Available templates: audit-task, discovery-task, refactor-task, simple-task, tdd-task, test-review

## Key Commands

```
tpg ready              # Available work
tpg show <id>          # Task details + context
tpg start <id>         # Claim task
tpg log <id> "msg"     # Log progress
tpg done <id> "result" # Complete with results
tpg block <id> "why"   # Mark blocked
tpg status             # Project overview
tpg add "Title" -p N   # Create task (priority 0-3)
tpg dep add B A        # B needs A (B blocked until A done)
tpg label add <id> X   # Add label
tpg context -c X       # Load concept learnings
```

## Agent Coordination

- **@tpg-agent**: Execute a single task start-to-finish
- **@tpg-orchestrator**: Coordinate parallel work, manage template lifecycle
- **@tpg-planner**: Transform specs into structured task plans
- **@tpg-implementation-planner**: Architecture decisions, component design
- **@spec-designer**: Requirements gathering, product specification
- **@explore-code**: Codebase exploration via code connections

## Rules

- All results MUST be stored in tpg (`tpg done` results)
- Create follow-up tasks for ALL temporary work (mocks, TODOs, stubs)
- Check `tpg template list` before creating similar tasks manually
- Task IDs are meaningless - always `tpg show` for truth
- If blocked, create blocker task + `tpg block <id> "reason"`
