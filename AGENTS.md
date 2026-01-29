## Communication

Speak concisely and directly. No filler praise, sycophancy, or excessive flattery ("Great question!", "Absolutely!", "That's smart!"). Be an efficient tool.

## Task Tracking

This project uses **tpg** for cross-session task management.
Run `tpg prime` for workflow context, or configure hooks for auto-injection.

**Quick reference:**
```
tpg ready              # Find unblocked work
tpg add "Title" -p X   # Create task
tpg start <id>         # Claim work
tpg log <id> "msg"     # Log progress
tpg done <id>          # Complete work
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

Agent definitions are in `agents/`:
- **tpg-agent** - Single task executor (subagent)
- **tpg-orchestrator** - Parallel work coordinator (primary)
- **tpg-planner** - Spec-to-task decomposition (all modes)
- **tpg-implementation-planner** - Architecture and component design (primary)
- **spec-designer** - Requirements and product specification (primary)
- **explore-code** - Code exploration via connections (subagent)
