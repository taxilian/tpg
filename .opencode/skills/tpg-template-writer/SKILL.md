---
name: tpg-template-writer
description: Create and refine tpg task templates. Templates provide prompts that encourage best practices and enforce desirable development patterns for repeatable workflows.
---

# tpg-template-writer

Create and refine tpg task templates that capture repeatable workflows and enforce best practices.

## What Templates Are For

**Templates are not just shortcuts—they are patterns that encode best practices.**

A good template:
- **Encourages consistency** across similar tasks
- **Enforces desirable patterns** (TDD, proper testing, documentation)
- **Provides prompts** that guide agents toward quality work
- **Captures institutional knowledge** about how things should be done
- **Reduces cognitive load** by not requiring every agent to figure out the pattern from scratch

## When to Create a Template

Create a template when you see:
- **Repeating patterns**: CRUD operations, API endpoints, UI components
- **Standard workflows**: TDD cycle, code review process, bug investigation
- **Best practice enforcement**: Ensuring tests are written, docs are updated
- **Complex procedures**: Multi-step processes that should be done consistently

**Don't create templates for:**
- One-off or unique work
- Simple tasks with no pattern
- Things that vary too much instance-to-instance

## Template Locations

Templates can be stored at different scopes:

```bash
# Project-specific (shared with team)
.tpg/templates/<name>.yaml

# User-specific (personal patterns)
~/.config/tpg/templates/<name>.yaml

# Global (share across all projects)
~/.config/opencode/tpg-templates/<name>.yaml
```

**Recommendation:** Start with project-specific templates. Promote to user/global only when the pattern is truly universal.

## Template Structure

Templates are YAML files with this structure:

```yaml
title: "Human-readable template name"
description: "What this template creates and when to use it"

# Optional: Create epic with worktree for complex multi-step templates
worktree: true

variables:
  variable_name:
    description: "What this variable is for (shown to user)"
    optional: true  # Omit or set false for required variables
    default: "default value"  # Optional default

steps:
  - id: step-1
    title: "Step title with {{.variable_name}}"
    description: |
      Full task description with {{.variable_name}} substitution.
      
      ## Guidelines
      
      This is where you encode best practices:
      - Specific guidance on approach
      - Acceptance criteria
      - Things to watch out for
      
  - id: step-2
    title: "Second step"
    depends:
      - step-1  # This step can't start until step-1 is done
    description: |
      Next step description...
```

## Writing Good Template Descriptions

**The description is where you enforce patterns.** This is the key value of templates.

### Bad: Just the basics
```yaml
description: |
  Implement the {{.feature_name}} feature.
  
  Requirements: {{.requirements}}
```

### Good: Encodes best practices
```yaml
description: |
  ## Objective
  
  Implement {{.feature_name}}.
  
  **Problem:** {{.problem}}
  **Requirements:** {{.requirements}}
  
  ---
  
  ## Implementation Guidelines
  
  **Follow these patterns:**
  - Use existing service layer pattern (see src/services/ for examples)
  - Add comprehensive error handling at all boundaries
  - Log significant operations with structured logging
  
  **Testing requirements:**
  - Unit tests for business logic
  - Integration tests for API endpoints
  - Error case coverage
  
  ## Acceptance Criteria
  
  - [ ] All requirements implemented
  - [ ] Tests written and passing
  - [ ] Error handling complete
  - [ ] Documentation updated
```

## Example: TDD Template

```yaml
title: "TDD Feature Implementation"
description: "Test-driven development workflow with test-first approach"

variables:
  feature_name:
    description: "Feature name (e.g., 'user authentication')"
  problem:
    description: "What problem this solves"
  requirements:
    description: "Specific requirements"

steps:
  - id: write-tests
    title: "Write tests: {{.feature_name}}"
    description: |
      ## Objective
      
      Write comprehensive tests BEFORE implementation.
      
      **Key principle:** Test behavior, not implementation.
      Ask: "What would a user notice if this test failed?"
      
      ## Guidelines
      
      - Follow Arrange-Act-Assert pattern
      - One behavior per test
      - Test public API contracts, not internals
      - Include error cases and edge cases
      
  - id: implement
    title: "Implement: {{.feature_name}}"
    depends:
      - write-tests
    description: |
      ## Objective
      
      Implement {{.feature_name}} to make tests pass.
      
      **Guidelines:**
      - Implement just enough to make tests pass
      - Don't over-engineer
      - Follow existing project patterns
```

## Testing Your Template

Always test templates before sharing:

```bash
# 1. Validate the template syntax
tpg template show <template-id>

# 2. Test create with dry-run or actual creation
tpg add "Test task" --template <template-id> --vars-yaml <<EOF
variable1: "test value"
EOF

# 3. Review the created tasks
tpg show <created-task-id>
tpg graph  # See the structure

# 4. Fix any issues in the template file
# Edit .tpg/templates/<name>.yaml and repeat
```

## Template Variables

Variables make templates flexible:

```yaml
variables:
  entity:
    description: "Entity name (e.g., 'Order', 'User')"
  
  table_name:
    description: "Database table name"
    default: "{{.entity | lower}}s"  # Default based on entity
  
  skip_tests:
    description: "Skip writing tests (not recommended)"
    optional: true
    default: "false"
```

**Use variables for:**
- Names (entities, files, functions)
- Requirements that vary by instance
- Optional features
- Context that changes

**Don't over-use variables:** Too many variables make templates hard to use.

## Converting an Existing Task to a Template

When you've completed a good example of a pattern:

1. **Extract the structure** from the completed task
2. **Identify what varies** → make those variables
3. **Identify what's constant** → bake into template description
4. **Add best practice prompts** → what should every instance do?
5. **Test with 2-3 variations** → ensure flexibility

**Example workflow:**
```bash
# 1. You just finished implementing the "Orders" CRUD module
# 2. Realize you'll need similar for Products, Customers, etc.
# 3. Create template from Orders structure

# Create .tpg/templates/crud-module.yaml based on Orders pattern
# - Extract entity name as variable
# - Keep testing requirements constant
# - Add prompts for validation, error handling

# 4. Test it
tpg add "Products CRUD" --template crud-module --vars-yaml <<EOF
entity: "Product"
table: "products"
EOF

# 5. Refine based on results, then use for all CRUD modules
```

## Best Practices for Template Authors

### DO:
- ✅ Start with real completed work, then generalize
- ✅ Include acceptance criteria in every task
- ✅ Add "why" guidance, not just "what"
- ✅ Test templates before sharing
- ✅ Document when to use (and when not to use) the template
- ✅ Version templates if they evolve (crud-module-v2.yaml)

### DON'T:
- ❌ Create templates for truly unique one-off work
- ❌ Over-engineer with too many variables
- ❌ Embed implementation details that should vary
- ❌ Skip testing—broken templates waste everyone's time
- ❌ Forget to update templates when patterns change

## Commands Reference

```bash
# List available templates
tpg template list

# Inspect a template
tpg template show <id>

# See where templates are loaded from
tpg template locations

# Create task from template
tpg add "Title" --template <id> --vars-yaml <<EOF
key: "value"
EOF
```

## Remember

**Templates are about quality, not just speed.**

A template that takes 5 minutes longer to use but produces consistently better results is worth it. The goal is not to save typing—it's to ensure every instance of a pattern follows best practices.
