# Workflows Guide

This guide covers creating, managing, and using workflows in gitsavvy.

## What is a Workflow?

A workflow is a sequence of shell commands organized into reusable steps. Workflows are stored as YAML files in your Git repository, making them version-controlled, shareable, and reviewable.

### Key Concepts

- **Steps**: Individual commands that make up a workflow
- **Placeholders**: Parameters that prompt for input at runtime (e.g., `<environment>`)
- **Defaults**: Workflow-level settings for shell, working directory, and confirmation behavior
- **Tags**: Labels for organizing and discovering workflows

## Workflow Schema

```yaml
schema_version: 1          # Required: workflow format version
title: "Workflow Name"     # Required: human-readable name
description: "Description" # Optional: detailed description
tags:                      # Optional: list of tags
  - deployment
  - production

defaults:                  # Optional: default values for steps
  shell: "bash"            # Default shell (bash, zsh, sh, pwsh)
  cwd: "/path/to/dir"      # Default working directory
  confirm_each_step: true  # Prompt before each step

placeholders:              # Optional: runtime parameters
  param_name:              # Placeholder identifier
    prompt: "Question?"    # Prompt text for user
    default: "value"       # Optional default value
    validate: "regex"      # Optional regex validation
    secret: false          # Hide value in output if true

steps:                     # Required: at least one step
  - name: "Step name"      # Optional step identifier
    command: "echo 'hello'" # Required: command to execute
    shell: "zsh"           # Optional: override default shell
    cwd: "/tmp"            # Optional: override working directory
    env:                   # Optional environment variables
      VAR: "value"
    continue_on_error: false # Optional: continue if this step fails
    confirmation: "Ready?" # Optional: custom confirmation prompt
```

## Creating Workflows

### From Shell History

Create a workflow from your recent shell commands:

```bash
gitsavvy record history
```

This opens a TUI where you can:
- Browse your shell history (bash and zsh supported)
- Search and filter commands
- Select multiple commands with space
- Preview selected commands
- Edit step names and descriptions
- Add placeholders

### From a Live Session

Record commands as you execute them:

```bash
gitsavvy record
# Execute commands in the subshell
kubectl get pods
kubectl logs pod-123
exit
# Workflow editor opens to edit and save
```

### From Scratch (TUI Editor)

Create a workflow using the built-in editor:

```bash
gitsavvy edit new-workflow
```

The TUI editor provides:
- Workflow metadata editing (title, description, tags)
- Step management (add, edit, reorder, delete)
- Placeholder configuration
- Real-time YAML preview

### From Scratch (Manual)

Create a YAML file directly in your workflow repository:

```bash
cd ~/.local/share/gitsavvy/repo
mkdir -p workflows/chaz
vim workflows/chaz/deploy.yaml
git add workflows/chaz/deploy.yaml
git commit -m "Add deployment workflow"
```

## Using Placeholders

Placeholders allow you to parameterize workflows with runtime input.

### Basic Placeholder

```yaml
placeholders:
  environment:
    prompt: "Which environment?"
    default: "staging"

steps:
  - command: "kubectl apply -f deployment-<environment>.yaml"
```

At runtime, you'll be prompted:
```
Which environment? [staging]: production
```

### Validation

Use regex to validate input:

```yaml
placeholders:
  version:
    prompt: "Semantic version"
    validate: "^v\\d+\\.\\d+\\.\\d+$"
```

### Secret Values

Mask sensitive values in output:

```yaml
placeholders:
  api_key:
    prompt: "API Key"
    secret: true
```

### Per-Step vs Form Prompting

Configure how placeholders are collected in `config.toml`:

```toml
[placeholders]
prompt_style = "form"     # Collect all placeholders upfront
# or
prompt_style = "per-step" # Prompt as each step is reached
```

## Workflow Examples

### Deployment Workflow

```yaml
schema_version: 1
title: "Deploy Application"
description: "Build and deploy to specified environment"
tags: ["deployment", "docker"]

defaults:
  confirm_each_step: true

placeholders:
  environment:
    prompt: "Target environment"
    default: "staging"
    validate: "^(staging|production)$"
  tag:
    prompt: "Docker image tag"
    default: "latest"

steps:
  - name: "Run tests"
    command: "go test ./..."
    continue_on_error: false

  - name: "Build Docker image"
    command: "docker build -t myapp:<tag> ."

  - name: "Push to registry"
    command: "docker push myapp:<tag>"

  - name: "Deploy"
    command: |
      kubectl set image deployment/myapp \
        myapp=myapp:<tag> -n <environment>
    confirmation: "Deploy to <environment> with tag <tag>?"
```

### Database Migration

```yaml
schema_version: 1
title: "Run Database Migration"
description: "Run database migrations with backup"
tags: ["database", "migration"]

placeholders:
  migration_file:
    prompt: "Migration filename"
    validate: "^\\d+_[a-z_]+\\.sql$"

steps:
  - name: "Create backup"
    command: "pg_dump $DATABASE_URL > backup_$(date +%s).sql"
    confirmation: "Create database backup?"

  - name: "Run migration"
    command: "psql $DATABASE_URL -f migrations/<migration_file>"

  - name: "Verify migration"
    command: "psql $DATABASE_URL -c 'SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1;'"
```

### Multi-Environment Service Restart

```yaml
schema_version: 1
title: "Restart Services"
description: "Restart services across multiple environments"
tags: ["operations"]

defaults:
  shell: "bash"

placeholders:
  service:
    prompt: "Service name"
  environments:
    prompt: "Environments (comma-separated)"
    default: "staging,production"

steps:
  - name: "Restart in each environment"
    command: |
      for env in $(echo "<environments>" | tr "," "\\n"); do
        kubectl rollout restart deployment/<service> -n $env
        echo "Restarted in $env"
      done
    confirmation: "Restart <service> in: <environments>?"
```

## Running Workflows

### Interactive Mode

```bash
gitsavvy run deploy-application
```

Interactive mode:
1. Shows workflow description and steps
2. Collects placeholder values
3. Prompts before each step
4. Displays command output
5. Allows stopping at any point

### Non-Interactive Mode

For CI/CD or scripting:

```bash
gitsavvy run deploy-application --non-interactive \
  --param environment=production \
  --param tag=v1.2.3
```

### Running Specific Steps

Run only specific steps by index or name:

```bash
# Run step 2 only
gitsavvy run deploy --step 2

# Run steps 1-3
gitsavvy run deploy --steps 1-3

# Run named steps
gitsavvy run deploy --step "build" --step "test"
```

### Dry Run

Preview what would run without executing:

```bash
gitsavvy run deploy --dry-run
```

## Editing Workflows

### TUI Editor

```bash
gitsavvy edit my-workflow
```

Features:
- Edit workflow metadata
- Add/remove/reorder steps
- Configure placeholders
- Live YAML validation
- Save and commit changes

### Manual Editing

Edit the YAML file directly:

```bash
vim ~/.local/share/gitsavvy/repo/workflows/chaz/my-workflow.yaml
```

After editing, reindex to update the search cache:

```bash
gitsavvy index rebuild
```

## Organizing Workflows

### Directory Structure

```
repo/
├── workflows/
│   ├── chaz/              # Personal workflows
│   │   ├── deploy.yaml
│   │   └── diagnostics.yaml
│   └── platform/          # Team workflows
│       ├── chaz/
│       │   └── k8s-ops.yaml
│       └── shared/
│           └── incident-response.yaml
├── shared/                # Curated/shared workflows
│   ├── onboarding.yaml
│   └── emergency.yaml
└── .gitsavvy/
    └── index.json         # Search index
```

### Identity Path Claiming

Your identity path determines where workflows are saved by default:

```toml
[identity]
path = "chaz"              # Saves to workflows/chaz/
path = "platform/chaz"     # Saves to workflows/platform/chaz/
```

Team prefixes can be validated:

```toml
[identity]
path = "platform/chaz"
team_prefix = "platform/"  # Only allows paths under platform/
```

### Tags and Discovery

Use tags to organize workflows:

```yaml
tags:
  - deployment
  - kubernetes
  - production
```

Find workflows by tag:

```bash
gitsavvy list --tag deployment
gitsavvy list --tag kubernetes --tag production
```

## Best Practices

### Workflow Design

1. **Make steps idempotent** - Steps should be safe to re-run
2. **Use confirmations for destructive operations** - Explicit prompts before deletions, deployments
3. **Provide good descriptions** - Help others understand what the workflow does
4. **Tag appropriately** - Make workflows discoverable
5. **Keep workflows focused** - Single responsibility per workflow

### Placeholder Usage

1. **Use descriptive names** - `api_key` not `key`
2. **Provide sensible defaults** - Reduce typing for common cases
3. **Add validation** - Catch errors early with regex patterns
4. **Mark secrets** - Use `secret: true` for sensitive values

### Collaboration

1. **Commit frequently** - Small, focused changes are easier to review
2. **Use PRs** - Even for personal workflows, PRs provide a review checkpoint
3. **Document in README.md** - Add workflow documentation alongside the YAML file
4. **Share via `shared/`** - Team workflows go in the shared directory

### Error Handling

```yaml
steps:
  - name: "Check prerequisites"
    command: "command -v kubectl"
    continue_on_error: false  # Stop if kubectl is missing

  - name: "Optional cleanup"
    command: "rm -rf /tmp/cache"
    continue_on_error: true   # Don't fail if cleanup fails
```

## Exporting Workflows

Convert workflows to other formats:

```bash
# Export to Markdown
gitsavvy export my-workflow --format markdown > my-workflow.md

# Export to JSON
gitsavvy export my-workflow --format json > my-workflow.json

# Export YAML (canonical form)
gitsavvy export my-workflow --format yaml > my-workflow.yaml
```

## Troubleshooting

### Workflow Not Found

```bash
# Rebuild the index
gitsavvy index rebuild

# List all available workflows
gitsavvy list
```

### Placeholder Not Substituted

Ensure placeholder syntax uses angle brackets: `<param_name>`

### Step Fails Silently

Check the step output logs and enable `stream_output` in config:

```toml
[runner]
stream_output = true
```

### Permission Denied

Ensure the shell script has execute permissions or use explicit shell:

```yaml
steps:
  - command: "bash ./script.sh"  # Explicit shell
```
