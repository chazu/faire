# CI/CD Integration

This guide covers using gitsavvy in automated environments like CI/CD pipelines.

## Non-Interactive Mode

For automation, gitsavvy can run workflows without interactive prompts.

### Basic Non-Interactive Execution

```bash
gitsavvy run <workflow> --non-interactive
```

### Passing Parameters

Provide placeholder values via command-line flags:

```bash
gitsavvy run deploy --non-interactive \
  --param environment=production \
  --param tag=v1.2.3
```

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success - all steps completed |
| `1` | Failure - one or more steps failed |
| `2` | Configuration error |
| `3` | Workflow not found |
| `4` | Validation error |
| `130` | Interrupted by user (SIGINT) |

### Dry Run

Preview what would execute without running commands:

```bash
gitsavvy run deploy --dry-run --non-interactive
```

This prints the steps that would run and exits.

## GitHub Actions

### Basic Workflow

```yaml
name: Deploy Application

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout workflows repo
        uses: actions/checkout@v4
        with:
          repository: my-org/workflows
          path: workflows-repo

      - name: Install gitsavvy
        run: |
          wget https://github.com/chazu/faire/releases/latest/download/gitsavvy-linux-amd64 -O gitsavvy
          chmod +x gitsavvy

      - name: Configure gitsavvy
        run: |
          gitsavvy config repo.path "${{ github.workspace }}/workflows-repo"
          gitsavvy config identity.path "ci"

      - name: Run deployment workflow
        run: |
          gitsavvy run deploy-production --non-interactive \
            --param environment=production \
            --param version=${{ github.sha }}
        env:
          # Environment variables for secrets in workflow
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
          API_KEY: ${{ secrets.API_KEY }}
```

### Matrix Strategy

Run workflows across multiple environments:

```yaml
name: Multi-Environment Deploy

on:
  workflow_dispatch:

jobs:
  deploy:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        environment: [staging, production]
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Deploy to ${{ matrix.environment }}
        run: |
          gitsavvy run deploy --non-interactive \
            --param environment=${{ matrix.environment }} \
            --param version=${{ github.sha }}
```

### Conditional Execution

Skip steps based on conditions:

```yaml
- name: Deploy with dry-run check
  run: |
    if [ "${{ github.event_name }}" = "pull_request" ]; then
      gitsavvy run deploy --dry-run --non-interactive
    else
      gitsavvy run deploy --non-interactive
    fi
```

## GitLab CI

### Basic Pipeline

```yaml
stages:
  - deploy

deploy:production:
  stage: deploy
  image: golang:latest
  script:
    # Install gitsavvy
    - go install github.com/chazu/faire/cmd/gitsavvy@latest

    # Configure
    - gitsavvy config repo.path "$CI_PROJECT_DIR/workflows"
    - gitsavvy config identity.path "gitlab-ci"

    # Run workflow
    - gitsavvy run deploy-production --non-interactive
      --param environment=production
      --param version=$CI_COMMIT_SHA
  environment:
    name: production
  only:
    - main
```

### Multi-Environment Pipeline

```yaml
stages:
  - deploy

.deploy_template: &deploy_template
  stage: deploy
  image: golang:latest
  before_script:
    - go install github.com/chazu/faire/cmd/gitsavvy@latest
    - gitsavvy config repo.path "$CI_PROJECT_DIR/workflows"
    - gitsavvy config identity.path "gitlab-ci"

deploy:staging:
  <<: *deploy_template
  script:
    - gitsavvy run deploy --non-interactive --param environment=staging
  environment:
    name: staging
  only:
    - develop

deploy:production:
  <<: *deploy_template
  script:
    - gitsavvy run deploy --non-interactive --param environment=production
  environment:
    name: production
  when: manual
  only:
    - main
```

## Jenkins

### Declarative Pipeline

```groovy
pipeline {
    agent any

    environment {
        GITSAVVY_REPO_PATH = "${WORKSPACE}/workflows"
    }

    stages {
        stage('Setup') {
            steps {
                sh 'go install github.com/chazu/faire/cmd/gitsavvy@latest'
                sh "gitsavvy config repo.path ${GITSAVVY_REPO_PATH}"
                sh 'gitsavvy config identity.path jenkins'
            }
        }

        stage('Deploy') {
            steps {
                script {
                    def env = params.ENVIRONMENT ?: 'staging'
                    sh "gitsavvy run deploy --non-interactive --param environment=${env}"
                }
            }
        }
    }

    parameters {
        choice(name: 'ENVIRONMENT', choices: ['staging', 'production'], description: 'Deployment environment')
    }
}
```

## CircleCI

### Configuration

```yaml
version: 2.1

jobs:
  deploy:
    docker:
      - image: golang:latest
    steps:
      - checkout
      - run:
          name: Install gitsavvy
          command: go install github.com/chazu/faire/cmd/gitsavvy@latest
      - run:
          name: Configure
          command: |
            gitsavvy config repo.path "${PWD}/workflows"
            gitsavvy config identity.path "circleci"
      - run:
          name: Deploy
          command: |
            gitsavvy run deploy-production --non-interactive \
              --param environment=production \
              --param version=${CIRCLE_SHA1}

workflows:
  deploy-workflow:
    jobs:
      - deploy:
          filters:
            branches:
              only: main
```

## Docker

### Dockerfile with gitsavvy

```dockerfile
FROM golang:alpine AS builder
RUN go install github.com/chazu/faire/cmd/gitsavvy@latest

FROM alpine:latest
COPY --from=builder /go/bin/gitsavvy /usr/local/bin/
WORKDIR /app
COPY . .

# Configure at runtime
ENTRYPOINT ["gitsavvy"]
CMD ["run", "deploy"]
```

### Docker Compose

```yaml
version: '3.8'

services:
  deploy:
    image: golang:alpine
    command: |
      sh -c "
        go install github.com/chazu/faire/cmd/gitsavvy@latest &&
        gitsavvy config repo.path /workflows &&
        gitsavvy run deploy --non-interactive
      "
    environment:
      - ENVIRONMENT=production
      - VERSION=${GIT_SHA:-latest}
    volumes:
      - ./workflows:/workflows
```

## Kubernetes

### CronJob for Scheduled Tasks

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nightly-backup
spec:
  schedule: "0 2 * * *"  # 2 AM daily
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: gitsavvy
            image: gitsavvy:latest
            command:
              - gitsavvy
              - run
              - backup-database
              - --non-interactive
            env:
              - name: BACKUP_RETENTION_DAYS
                value: "30"
          restartPolicy: OnFailure
```

### Init Container for Setup

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app-with-setup
spec:
  initContainers:
  - name: workflow-setup
    image: gitsavvy:latest
    command:
      - gitsavvy
      - run
      - k8s-init
      - --non-interactive
    env:
      - name: NAMESPACE
        value: "production"
  containers:
  - name: app
    image: myapp:latest
```

## Systemd Timer

### Service Unit

```ini
# /etc/systemd/system/gitsavvy-backup.service
[Unit]
Description=Run gitsavvy backup workflow
After=network.target

[Service]
Type=oneshot
User=backup
Environment="GITSAVVY_REPO__PATH=/var/lib/workflows"
ExecStart=/usr/local/bin/gitsavvy run backup-database --non-interactive
```

### Timer Unit

```ini
# /etc/systemd/system/gitsavvy-backup.timer
[Unit]
Description=Run gitsavvy backup daily

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

Enable and start:

```bash
sudo systemctl enable gitsavvy-backup.timer
sudo systemctl start gitsavvy-backup.timer
```

## AWS Lambda

### Running Workflows in Lambda

```python
import subprocess
import json
import os

def lambda_handler(event, context):
    # Get parameters from event
    environment = event.get('environment', 'staging')
    version = event.get('version', 'latest')

    # Configure gitsavvy
    os.environ['GITSAVVY_REPO__PATH'] = '/opt/workflows'
    os.environ['GITSAVVY_IDENTITY__PATH'] = 'lambda'

    # Run workflow
    result = subprocess.run([
        '/opt/gitsavvy',
        'run', 'deploy',
        '--non-interactive',
        '--param', f'environment={environment}',
        '--param', f'version={version}'
    ], capture_output=True, text=True)

    return {
        'statusCode': result.returncode,
        'stdout': result.stdout,
        'stderr': result.stderr
    }
```

## Terraform

### Provisioning with Workflows

```hcl
resource "null_resource" "run_workflow" {
  triggers = {
    workflow_version = var.workflow_version
  }

  provisioner "local-exec" {
    command = "gitsavvy run infrastructure-provision --non-interactive --param environment=${var.environment}"
  }
}
```

## Error Handling Patterns

### Retry Logic

```bash
#!/bin/bash
MAX_RETRIES=3
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
  if gitsavvy run deploy --non-interactive; then
    exit 0
  fi

  RETRY_COUNT=$((RETRY_COUNT + 1))
  echo "Retry $RETRY_COUNT/$MAX_RETRIES"
  sleep 10
done

exit 1
```

### Fallback Workflow

```bash
#!/bin/bash
# Try primary deployment
if ! gitsavvy run deploy --non-interactive; then
  echo "Primary deployment failed, running rollback..."

  # Run rollback workflow
  gitsavvy run rollback --non-interactive
  exit 1
fi
```

## Best Practices

### 1. Version Pinning

Pin workflow versions or commits in CI:

```yaml
- name: Checkout specific workflow commit
  uses: actions/checkout@v4
  with:
    ref: 'abc123'  # Workflow commit hash
```

### 2. Secrets Management

Never pass secrets as command-line arguments. Use environment variables:

```yaml
# WRONG
run: gitsavvy run deploy --param api_key=${{ secrets.API_KEY }}

# RIGHT
run: gitsavvy run deploy --non-interactive
env:
  API_KEY: ${{ secrets.API_KEY }}
```

### 3. Workflow Validation

Validate workflows before production use:

```yaml
- name: Validate workflow
  run: gitsavvy validate deploy-production

- name: Dry run
  run: gitsavvy run deploy-production --dry-run --non-interactive

- name: Deploy
  run: gitsavvy run deploy-production --non-interactive
```

### 4. Output Capture

Capture and archive workflow output:

```yaml
- name: Run workflow with output
  run: |
    gitsavvy run deploy --non-interactive 2>&1 | tee deployment.log

- name: Upload logs
  uses: actions/upload-artifact@v4
  with:
    name: deployment-logs
    path: deployment.log
```

### 5. Timeout Configuration

Set appropriate timeouts for workflow execution:

```yaml
- name: Deploy with timeout
  run: timeout 30m gitsavvy run deploy --non-interactive
```

## Troubleshooting CI/CD Integration

### Workflow Not Found

```bash
# Verify repo path
gitsavvy config repo.path

# Rebuild index
gitsavvy index rebuild

# List available workflows
gitsavvy list
```

### Permission Issues

Ensure the CI runner has appropriate permissions:

```bash
# Check config location
ls -la ~/.config/gitsavvy/

# Verify repo access
git -C ~/.local/share/gitsavvy/repo fetch --dry-run
```

### Shell Issues

Explicitly set the shell in CI:

```bash
# In workflow YAML
steps:
  - name: Deploy
    shell: bash
    run: gitsavvy run deploy --non-interactive
```
