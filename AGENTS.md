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
