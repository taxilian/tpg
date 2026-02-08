# Context Engine

The context engine captures tacit knowledge — things agents learn that aren't obvious from the code. This knowledge persists across sessions, helping future agents avoid rediscovering the same insights.

## Data Model

**Concepts** are knowledge categories within a project:
```
auth          - "Token lifecycle, refresh, session coupling"
database      - "SQLite patterns, schema migrations"
config        - "Environment loading, precedence rules"
```

**Learnings** are specific insights tagged with concepts:
```
lrn-abc123: Token refresh has race condition
  Detail: The mutex only protects token write, not refresh check. See PR #423.
  Concepts: auth, concurrency
  Files: auth/token.go
  Task: ts-def456
```

## Two-Phase Retrieval

Agents retrieve context in phases to minimize token usage:

```
Phase 1: Discovery
  tpg show <task>
    -> Task details, logs, deps
    -> Suggested concepts: auth (3), config (2)

  Agent decides which concepts are relevant

Phase 2: Scan
  tpg context -c auth --summary
    -> auth: Token lifecycle, refresh, session coupling
    -> lrn-abc: Token refresh has race condition
    -> lrn-def: Auth tokens expire after 1 hour

  Agent sees concept summary, then learning one-liners

Phase 3: Load
  tpg context --id lrn-abc
    -> Full detail, files, linked task
```

Each phase filters, so agents only load what's actually relevant.

## Commands

### Concepts

| Command | Description |
|---------|-------------|
| `tpg concepts` | List concepts for a project |
| `tpg concepts --recent` | Sort by last updated instead of learning count |
| `tpg concepts --stats` | Show statistics (count and oldest learning age) |
| `tpg concepts --related <task-id>` | Suggest concepts for a task |
| `tpg concepts <name> --summary <text>` | Update a concept's summary |
| `tpg concepts <name> --rename <new-name>` | Rename a concept |

### Context Retrieval

| Command | Description |
|---------|-------------|
| `tpg context -c <name>` | Retrieve learnings by concept(s) |
| `tpg context -q <query>` | Full-text search on learnings |
| `tpg context --summary` | Show one-liner per learning (no detail) |
| `tpg context --id <learning-id>` | Load specific learning by ID |
| `tpg context --include-stale` | Include stale learnings in results |
| `tpg context --json` | Output as JSON |

### Learning Management

| Command | Description |
|---------|-------------|
| `tpg learn <summary>` | Log a new learning |
| `tpg learn edit <id>` | Edit a learning's summary or detail |
| `tpg learn stale <id>` | Mark learning as outdated |
| `tpg learn rm <id>` | Delete a learning |

### Retrieval Examples

```bash
# List concepts to see what knowledge exists
tpg concepts -p myproject
# NAME          LEARNINGS  LAST UPDATED  SUMMARY
# auth                  3  2h ago        Token lifecycle, refresh
# database              2  1d ago        SQLite patterns

# Retrieve by concept (union of multiple concepts)
tpg context -c auth -c database -p myproject

# Full-text search when you don't know the concept
tpg context -q "race condition" -p myproject

# Include stale learnings for historical context
tpg context -c auth --include-stale -p myproject

# Get concept statistics
tpg concepts -p myproject --stats

# Suggest concepts for a task
tpg concepts --related ts-abc123 -p myproject
```

## Logging Learnings

Log learnings at the end of a session during reflection. This is more efficient than logging during work because:

- The learning is validated through implementation
- You can synthesize related discoveries into one insight
- You know what's signal vs noise

```bash
# Basic learning with concepts
tpg learn "Token refresh has race condition" -c auth -c concurrency -p myproject

# With related files
tpg learn "Config loads from env first, then file" -c config -p myproject -f config.go

# With full detail
tpg learn "summary" -c concept -p myproject --detail "full explanation..."

# Multi-line detail via stdin
tpg learn "summary" -c concept -p myproject --detail - <<EOF
Full explanation with multiple lines...
EOF
```

**Good learnings:**
- Things that aren't obvious from reading the code
- Gotchas, edge cases, "why" decisions
- Context that would help the next agent

**Avoid logging:**
- Things already documented in code comments
- Obvious behavior that code makes clear
- Temporary workarounds (mark as stale instead)

### Concept Hygiene

- **Reuse existing concepts** — check `tpg concepts` before creating new ones
- **Create sparingly** — prefer broader concepts over narrow ones
- **Use clear names** — `auth` not `authentication-and-authorization`

## Grooming and Compaction

Over time, learnings accumulate. Without periodic grooming:
- Redundant entries waste context tokens when retrieved
- Stale learnings mislead agents with outdated information
- Unclear summaries reduce retrieval effectiveness
- Fragmented insights are harder to discover and use

The `tpg prime` command automatically flags concepts that may need attention (5+ learnings, or learnings older than 7 days).

### The Compaction Workflow

Run `tpg compact` to get guided prompting for grooming. The workflow has two phases:

**Phase 1: Discovery**
```bash
tpg concepts -p myproject --stats    # See concept distribution
tpg context -p myproject --summary   # Scan all one-liners
```

Flag candidates: redundant (similar summaries), stale (old or outdated), low quality (vague, not actionable), fragmented (should be combined).

**Phase 2: Selection & Grooming**
```bash
tpg context --id lrn-abc123          # Load specific learning
tpg context -c auth -p myproject --json  # Load all for a concept
```

Then apply actions:
- **Archive**: `tpg learn stale lrn-a lrn-b --reason "Consolidated"`
- **Update**: `tpg learn edit lrn-abc --summary "Clearer summary"`
- **Consolidate**: Archive originals, create new combined learning

### Marking Learnings Stale

When a learning becomes outdated but is still useful for reference:

```bash
tpg learn stale lrn-abc123 --reason "Refactored in v2"
```

Stale learnings are excluded by default but can be included with `--include-stale`.

### Concept Grooming

```bash
# Update a concept's summary
tpg concepts auth -p myproject --summary "Token lifecycle and session management"

# Rename a fragmented concept
tpg concepts authn -p myproject --rename auth
```

## Resources & Inspiration

- **[CASS Memory System](https://github.com/Dicklesworthstone/cass_memory_system)** — Lesson extraction and rule validation through evidence gates. Influenced the approach to learning quality.
- **[AgentFS](https://github.com/tursodatabase/agentfs)** — SQLite-based agent memory with audit trails. Validated SQLite for durability and linking learnings to tasks.
- **[Dynamic Context Discovery](https://cursor.com/blog/dynamic-context-discovery)** (Cursor) — Two-phase retrieval with stubs in context, full content on-demand. Directly inspired the `--summary` -> `--id` pattern.
- **[Everything is Context](https://arxiv.org/abs/2512.05470)** (Xu et al., 2024) — File-system abstraction for context engineering. Reinforced concepts-over-files approach.
