## Communication

Speak concisely and directly. No filler praise, sycophancy, or excessive flattery ("Great question!", "Absolutely!", "That's smart!"). Be an efficient tool.

## Task Tracking

This project uses **tpg** for cross-session task management.
Run `tpg prime` for workflow context, or configure hooks for auto-injection.

**Quick reference:**
```
tpg ready                        # Find unblocked work
tpg add "Title" -p 1             # Create task (priority 1)
tpg start <id>                   # Claim work
tpg log <id> "msg"               # Log progress
tpg done <id>                    # Complete work
tpg dep <id> blocks <other-id>   # Set dependency
tpg dep <id> list                # Show dependencies
```

For full workflow: `tpg prime`

## Templates

Check `.tpg/templates/` for reusable task templates before creating tasks manually:
```
tpg template list                                    # List available templates
tpg template show <id>                               # View template details
tpg add "Title" --template <id> --var 'key="value"'  # Create from template
```

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
