# tpg Templates

Templates define **standardized ways to solve problems**. A "tdd" template encodes the standard approach for test-driven development. A "discovery" template defines how to investigate unknowns. A "bug-fix" template captures the proven method for diagnosing and resolving issues.

When you instantiate a template, tpg creates a parent epic with child tasks that follow the standardized approach, including proper dependencies between steps.

## Quick Start

```bash
# Create templates directory
mkdir -p .tpg/templates

# Create a template (see examples below)
cat > .tpg/templates/tdd.yaml << 'EOF'
title: "TDD"
description: "Standardized test-driven development approach"

variables:
  feature_name:
    description: "Name of the feature"

steps:
  - id: write-tests
    title: "Write tests: {{.feature_name}}"
    description: "Write tests before implementation"
  
  - id: implement
    title: "Implement: {{.feature_name}}"
    depends: [write-tests]
    description: "Implement to make tests pass"
EOF

# Use the TDD approach for a feature
tpg add "User Authentication" --template tdd \
  --var 'feature_name="user authentication"'
```

## Template Locations

Templates are searched in multiple locations, in priority order (most local first):

1. **Project:** `.tpg/templates/` (searched upward from current directory)
2. **User:** `~/.config/tpg/templates/`
3. **Global:** `~/.config/opencode/tpg-templates/`

Templates from more local locations override templates with the same ID from more global locations.

```
project/
├── .tpg/
│   ├── tpg.db
│   ├── config.json
│   └── templates/           # Project-specific task types
│       └── tdd.yaml
│       └── bug-fix.yaml

~/.config/
├── tpg/
│   └── templates/           # User task types (shared across projects)
│       └── discovery.yaml
└── opencode/
    └── tpg-templates/       # Global task types
        └── investigation.yaml
```

### Viewing Template Locations

Use `tpg template locations` to see which directories are being searched:

```bash
$ tpg template locations
Template search locations (highest priority first):

  [project] /path/to/project/.tpg/templates
  [user] /Users/you/.config/tpg/templates
```

## Managing Templates

### List Available Templates

```bash
$ tpg template list
tdd (project)
  Standardized test-driven development approach
  Variables:
    feature_name (required): Name of the feature
    constraints (optional): Hard constraints
  Steps: 4

discovery (user)
  Standardized approach for investigating unknowns
  Variables:
    topic (required): What to investigate
  Steps: 3
```

### Show Template Details

```bash
$ tpg template show tdd
Template: tdd
Source: /path/to/project/.tpg/templates/tdd.yaml (project)
Title: TDD
Description: Standardized test-driven development approach

Variables:
  feature_name (required)
    Name of the feature
  constraints (optional)
    Hard constraints

Steps:
  1. [write-tests] Write tests: {{.feature_name}}
       Write tests before implementation...
  2. [implement] Implement: {{.feature_name}}
       Implement to make tests pass...
       Depends: write-tests
  ...
```

## Template Format

Templates use [Go's text/template](https://pkg.go.dev/text/template) syntax for variable interpolation.

### YAML Example

```yaml
title: "TDD Workflow"
description: "Test-driven development workflow"

variables:
  feature_name:
    description: "Name of the feature"
  constraints:
    description: "Hard constraints (optional)"
    optional: true

steps:
  - id: write-tests
    title: "Write tests: {{.feature_name}}"
    description: |
      Write tests for {{.feature_name}}.
      
      **Requirements:** {{.requirements}}
      {{- if hasValue .constraints}}
      **Constraints:** {{.constraints}}
      {{- end}}

  - id: implement
    title: "Implement: {{.feature_name}}"
    depends:
      - write-tests
    description: |
      Implement {{.feature_name}} to make tests pass.
```

### TOML Example

```toml
title = "Feature Request"
description = "Standard feature implementation workflow"

[variables.feature_name]
description = "Name of the feature"
required = true

[variables.acceptance_criteria]
description = "Acceptance criteria"
required = true

[[steps]]
id = "design"
title = "Design: {{.feature_name}}"
description = "Create design document"

[[steps]]
id = "implement"
title = "Implement: {{.feature_name}}"
depends = ["design"]
description = "Implement the feature"
```

## Variables

### Variable Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Explains what value to provide (shown in errors) |
| `optional` | bool | If `true`, variable is optional (default: `false`, meaning required) |
| `default` | string | Value used when optional variable is not provided (default: `""`) |

### Example

```yaml
variables:
  # Required variable (default) - must be provided
  feature_name:
    description: "Name of the feature (e.g., 'user authentication')"
  
  # Optional variable with custom default
  priority:
    description: "Priority level"
    optional: true
    default: "medium"
  
  # Optional variable (empty string if not provided)
  notes:
    description: "Additional notes"
    optional: true
```

## Template Syntax

Templates use Go's `text/template` syntax. Variables are accessed with `.` prefix.

### Basic Interpolation

```yaml
title: "Implement {{.feature_name}}"
description: "Requirements: {{.requirements}}"
```

### Conditionals

Use `{{if}}...{{end}}` to conditionally include content:

```yaml
description: |
  **Requirements:** {{.requirements}}
  {{if .constraints}}
  **Constraints:** {{.constraints}}
  {{end}}
```

**Note:** `{{if .var}}` is true if the variable exists and is non-empty.

### hasValue Helper

Use `hasValue` for explicit empty-string checking:

```yaml
description: |
  {{if hasValue .optional_field}}
  **Optional:** {{.optional_field}}
  {{end}}
```

`hasValue` returns `false` for empty strings and whitespace-only strings.

### default Helper

Use `default` to provide fallback values inline:

```yaml
description: |
  Priority: {{default "medium" .priority}}
```

### Whitespace Control

Use `{{-` and `-}}` to trim whitespace around template directives:

```yaml
description: |
  Required info
  {{- if hasValue .optional}}
  Optional: {{.optional}}
  {{- end}}
  More content
```

Without the `-`, you'd get extra blank lines when the conditional is false.

### Complete Example

```yaml
description: |
  ## Feature: {{.feature_name}}
  
  **Problem:** {{.problem}}
  **Scope:** {{.scope}}
  **Requirements:** {{.requirements}}
  {{- if hasValue .constraints}}
  **Constraints:** {{.constraints}}
  {{- end}}
  {{- if hasValue .concerns}}
  **Concerns:** {{.concerns}}
  {{- end}}
  {{- if hasValue .context}}
  
  ### Additional Context
  {{.context}}
  {{- end}}
  
  ## Acceptance Criteria
  
  - [ ] All requirements implemented
  {{- if hasValue .constraints}}
  - [ ] All constraints satisfied
  {{- end}}
  - [ ] Tests passing
```

## Steps

### Step Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique identifier (auto-generated if omitted) |
| `title` | string | Task title (supports template syntax) |
| `description` | string | Task description (supports template syntax) |
| `depends` | []string | List of step IDs this step depends on |

### Dependencies

Steps can depend on other steps. Dependencies are converted to tpg task dependencies.

```yaml
steps:
  - id: design
    title: "Design"
    description: "Create design document"
  
  - id: implement
    title: "Implement"
    depends: [design]  # Can't start until design is done
    description: "Implement the feature"
  
  - id: test
    title: "Test"
    depends: [implement]  # Can't start until implement is done
    description: "Write and run tests"
  
  - id: review
    title: "Review"
    depends: [implement, test]  # Depends on multiple steps
    description: "Code review"
```

### Auto-generated IDs

If you omit `id`, a random 3-character ID is generated:

```yaml
steps:
  - title: "First step"
    description: "..."
  
  - title: "Second step"
    depends: []  # Can't reference first step without explicit ID
    description: "..."
```

## Using Templates

### Basic Usage

```bash
tpg add "Epic Title" --template template-name \
  --var 'var1="value1"' \
  --var 'var2="value2"'
```

### Variable Format

Variables are passed as `name=json-string`:

```bash
# Simple string
--var 'feature_name="authentication"'

# String with quotes
--var 'message="He said \"hello\""'

# Multi-line string
--var 'requirements="1. First requirement\n2. Second requirement"'
```

### What Gets Created

Instantiating a template creates:

1. **Parent epic** with the title you provide
2. **Child tasks** for each step, with:
   - Rendered title and description
   - Dependencies between tasks (based on `depends`)
   - All child tasks as dependencies of the parent epic

```bash
$ tpg add "Auth Feature" --template tdd-workflow --var 'feature_name="auth"' ...
ep-abc123

$ tpg list --parent ep-abc123
ID           STATUS  TITLE
ts-def456    open    Write tests: auth
ts-ghi789    open    Implement: auth
ts-jkl012    open    Review: auth

$ tpg graph
ep-abc123 [open] Auth Feature
  ├── ts-def456 [open] Write tests: auth
  ├── ts-ghi789 [open] Implement: auth
  └── ts-jkl012 [open] Review: auth
ts-ghi789 [open] Implement: auth
  └── ts-def456 [open] Write tests: auth
ts-jkl012 [open] Review: auth
  └── ts-ghi789 [open] Implement: auth
```

### Viewing Templated Tasks

When you view a templated task, the template is rendered with the stored variables:

```bash
$ tpg show ts-def456
ID:          ts-def456
Type:        task
Project:     myproject
Title:       Write tests: auth
Status:      open
Priority:    2
Parent:      ep-abc123
Template:    tdd-workflow
Step:        1

Description:
Write tests for auth.

**Requirements:** validate email, hash passwords
...
```

### Template Changes

Each task stores a hash of the template at creation time. If you modify the template:

- `tpg show` renders using the **latest** template content
- A notice appears if the template has changed since creation

This allows templates to evolve without breaking existing tasks.

## Example Template

See `examples/templates/tdd-workflow.yaml` for a complete TDD workflow template with:

- Required and optional variables
- Conditional sections
- Multi-step workflow with dependencies
- Detailed task descriptions

```bash
# Copy the example to your project
mkdir -p .tpg/templates
cp examples/templates/tdd-workflow.yaml .tpg/templates/

# Use it
tpg add "User Authentication" --template tdd-workflow \
  --var 'feature_name="user authentication"' \
  --var 'problem="Users need to log in securely"' \
  --var 'scope="API endpoint with validation and storage"' \
  --var 'requirements="Validate email format, Hash passwords securely"'
```
