---
name: tpg-context
description: |
  Knowledge management for TPG: capture insights, log learnings, find previous learnings,
  retrieve context by concept, and groom knowledge. Use when the user mentions learning,
  knowledge, context, insights, "what did we learn", "capture this", "find previous",
  "groom knowledge", "retrieve context", or "what do we know about X".
  
  ALWAYS use this skill when:
  - Logging learnings at end of session (tpg learn)
  - Retrieving context for a task (tpg context)
  - Managing knowledge concepts (tpg concepts)
  - Grooming/compacting accumulated learnings (tpg compact)
compatibility: opencode
metadata:
  category: knowledge
  audience: agents
  version: "1"
---

## What I do

I guide agents through TPG's knowledge management system: capturing insights as learnings,
organizing them by concept, retrieving relevant context for tasks, and maintaining
knowledge hygiene through grooming and compaction.

## Trigger phrases

Load this skill when the user asks for any of:
- "capture insights" / "log learnings" / "record what we learned"
- "find previous learnings" / "what do we know about X"
- "groom knowledge" / "compact learnings" / "clean up concepts"
- "retrieve context" / "get context for this task"
- "what did we learn" / "remember that thing about..."
- "knowledge" / "learnings" / "insights" / "context"

## ALWAYS-use triggers

**You MUST use this skill when:**

1. **End-of-session reflection** — Before ending any significant work session, check if there are insights worth logging with `tpg learn`
2. **Starting work on a task** — Retrieve relevant context with `tpg context -c <concept>` to avoid rediscovering known issues
3. **Encountering gotchas** — When you find non-obvious behavior, edge cases, or "why" decisions that would help the next agent
4. **Knowledge maintenance** — When `tpg prime` flags concepts needing attention (5+ learnings or old entries)

## Non-goals

Unless explicitly requested, I do NOT:
- Implement the context engine itself (this is for using TPG's existing context commands)
- Invent concepts without checking existing ones first
- Log learnings during active work (wait for end-of-session reflection)
- Delete learnings without archiving them first (mark stale instead)

## Core commands

| Command | Purpose |
|---------|---------|
| `tpg concepts` | List knowledge categories for the project |
| `tpg context -c <name>` | Retrieve learnings by concept |
| `tpg learn "summary" -c <concept>` | Log a new learning |
| `tpg compact` | Guided grooming workflow for accumulated learnings |

## Two-phase retrieval workflow

To minimize token usage, retrieve context in phases:

### Phase 1: Discovery (summary view)
```bash
# See what concepts exist and their learning counts
tpg concepts

# Get one-liner summaries for a concept
tpg context -c auth --summary

# Result: concept summary + learning IDs with one-line descriptions
# auth: Token lifecycle, refresh, session coupling
#   lrn-abc: Token refresh has race condition
#   lrn-def: Auth tokens expire after 1 hour
```

### Phase 2: Load (full detail)
```bash
# Load specific learning when you need full detail
tpg context --id lrn-abc

# Result: full detail, linked files, task references
```

**Why two phases?** Most context needs are satisfied by summaries. Only load full detail
for learnings that are clearly relevant to your current task.

## When to log learnings

**Log at end of session, not during work:**

Logging during work is inefficient because:
- The learning isn't yet validated through implementation
- You can't yet distinguish signal from noise
- You may need to synthesize multiple discoveries into one insight

**Log these kinds of insights:**
- Things not obvious from reading the code
- Gotchas, edge cases, "why" decisions
- Context that would help the next agent avoid pitfalls

**Avoid logging:**
- Things already in code comments
- Obvious behavior the code makes clear
- Temporary workarounds (mark as stale when fixed)

### Logging workflow

```bash
# Basic learning with concept
tpg learn "Token refresh has race condition" -c auth -c concurrency

# With related files
tpg learn "Config loads from env first, then file" -c config -f config.go

# With full detail
tpg learn "summary" -c concept --detail "full explanation..."
```

## Concept hygiene

Concepts are knowledge categories. Good hygiene keeps the system useful:

### Reuse existing concepts

**Always check before creating:**
```bash
tpg concepts  # See what exists
```

Prefer broader concepts over narrow ones:
- ✅ `auth` — not `authentication-and-authorization`
- ✅ `database` — not `sqlite-migrations`
- ✅ `config` — not `environment-variables`

### Create sparingly

Only create a new concept when:
- No existing concept reasonably covers the knowledge
- The topic is distinct enough to be searched independently
- You expect multiple learnings in this area

### Update concept summaries

Keep concept summaries current as learnings evolve:
```bash
tpg concepts auth --summary "Token lifecycle, refresh, session coupling"
```

## Grooming and compaction workflow

Over time, learnings accumulate and need maintenance:

### When to groom

- `tpg prime` flags concepts with 5+ learnings or entries older than 7 days
- You notice redundant or outdated learnings during retrieval
- Concept names have become fragmented or unclear

### The compaction workflow

**Phase 1: Discovery**
```bash
# See concept distribution
tpg concepts --stats

# Scan all one-liners for quality issues
tpg context --summary
```

Flag candidates:
- **Redundant**: Similar summaries that should be combined
- **Stale**: Outdated or superseded by newer code
- **Low quality**: Vague summaries, not actionable
- **Fragmented**: Multiple small learnings that should be one

**Phase 2: Selection & grooming**
```bash
# Load specific learning for review
tpg context --id lrn-abc123

# Load all for a concept (as JSON for easier processing)
tpg context -c auth --json
```

Apply actions:
```bash
# Archive outdated learnings
tpg learn stale lrn-a lrn-b --reason "Consolidated into lrn-c"

# Update unclear summaries
tpg learn edit lrn-abc --summary "Clearer, more specific summary"

# Consolidate: archive originals, create new combined learning
tpg learn stale lrn-x lrn-y --reason "Combined"
tpg learn "Combined insight covering X and Y" -c concept
```

### Marking learnings stale

When a learning becomes outdated but is useful for reference:
```bash
tpg learn stale lrn-abc123 --reason "Refactored in v2, no longer applies"
```

Stale learnings are excluded by default but can be included:
```bash
tpg context -c auth --include-stale
```

## Full command reference

### Concepts

| Command | Description |
|---------|-------------|
| `tpg concepts` | List concepts for current project |
| `tpg concepts --recent` | Sort by last updated |
| `tpg concepts --stats` | Show statistics (count and oldest age) |
| `tpg concepts --related <task-id>` | Suggest concepts for a task |
| `tpg concepts <name> --summary <text>` | Update concept summary |
| `tpg concepts <name> --rename <new-name>` | Rename a concept |

### Context retrieval

| Command | Description |
|---------|-------------|
| `tpg context -c <name>` | Retrieve learnings by concept(s) |
| `tpg context -q <query>` | Full-text search on learnings |
| `tpg context --summary` | Show one-liner per learning |
| `tpg context --id <learning-id>` | Load specific learning |
| `tpg context --include-stale` | Include stale learnings |
| `tpg context --json` | Output as JSON |

### Learning management

| Command | Description |
|---------|-------------|
| `tpg learn <summary>` | Log a new learning |
| `tpg learn edit <id>` | Edit summary or detail |
| `tpg learn stale <id>` | Mark learning as outdated |
| `tpg learn rm <id>` | Delete a learning |

## Examples

### Example 1: Starting work on authentication

```bash
# Discover what we know about auth
tpg concepts
# NAME          LEARNINGS  LAST UPDATED  SUMMARY
# auth                  3  2h ago        Token lifecycle, refresh
# database              2  1d ago        SQLite patterns

# Get summaries for auth
tpg context -c auth --summary
# auth: Token lifecycle, refresh, session coupling
#   lrn-abc: Token refresh has race condition
#   lrn-def: Auth tokens expire after 1 hour

# Load the race condition detail
tpg context --id lrn-abc
# Detail: The mutex only protects token write, not refresh check. See PR #423.
# Files: auth/token.go
# Task: ts-def456
```

### Example 2: End-of-session learning capture

```bash
# After implementing the fix, log the insight
tpg learn "Token refresh race: mutex only protects write, not check" \
  -c auth -c concurrency \
  -f auth/token.go \
  --detail "The refresh check happens outside the mutex, allowing concurrent refreshes. Fixed by moving check inside."
```

### Example 3: Knowledge grooming session

```bash
# Check for concepts needing attention
tpg concepts --stats

# Review all learnings at summary level
tpg context --summary

# Found two similar learnings about auth, load details
tpg context --id lrn-old
tpg context --id lrn-new

# Archive the old one, keep the new
tpg learn stale lrn-old --reason "Superseded by lrn-new with better detail"
```

## How to test

- Verify `tpg concepts` lists concepts for your project
- Test retrieval: `tpg context -c <concept> --summary` returns one-liners
- Test full load: `tpg context --id <learning-id>` returns complete detail
- Test learning: `tpg learn "test" -c test` creates a learning
- Test grooming: `tpg learn stale <id>` marks it stale, excluded from default queries

## Troubleshooting

**No concepts found:**
- Check you're in a project directory with `.tpg/` folder
- Run `tpg concepts -p <project>` to specify project explicitly

**Learnings not showing up:**
- Stale learnings are excluded by default; use `--include-stale`
- Check concept name matches exactly (case-sensitive)

**Concept fragmentation:**
- Use `tpg concepts <name> --rename <new-name>` to consolidate
- Prefer broader concept names

## Quality rubric

Good knowledge management:
- [ ] Concepts are reused rather than duplicated
- [ ] Learnings logged at end of session, not during
- [ ] Two-phase retrieval used (summary → detail)
- [ ] Stale learnings marked, not deleted
- [ ] Concept summaries kept current
- [ ] Grooming done when `tpg prime` flags accumulation
