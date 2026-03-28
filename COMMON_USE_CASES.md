# Common Use Cases for Act

This document provides practical examples for common `act` usage scenarios. For complete documentation, please visit the [act user guide](https://nektosact.com).

## Table of Contents

- [Running Specific Jobs](#running-specific-jobs)
- [Testing Pull Request Workflows](#testing-pull-request-workflows)
- [Using Custom Secrets](#using-custom-secrets)
- [Running with Different Event Types](#running-with-different-event-types)
- [Debugging Failed Workflows](#debugging-failed-workflows)
- [Matrix Build Testing](#matrix-build-testing)

---

## Running Specific Jobs

When you have a workflow with multiple jobs, you often want to run just one job for faster feedback:

```bash
# List all available jobs
act -l

# Run a specific job by name
act -j build

# Run a specific job with specific workflow file
act -j test -W .github/workflows/ci.yml
```

### Example Workflow File

```yaml
name: CI
on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build
        run: make build

  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Test
        run: make test
```

Run only the build job:
```bash
act -j build
```

---

## Testing Pull Request Workflows

To test workflows triggered by pull request events, you can simulate the event with a JSON payload:

```bash
# Create a sample pull request event payload
cat > pr-event.json << 'EOF'
{
  "pull_request": {
    "number": 123,
    "head": {
      "ref": "feature-branch",
      "sha": "abc123"
    },
    "base": {
      "ref": "main"
    }
  }
}
EOF

# Run with the pull_request event
act pull_request -e pr-event.json
```

This is especially useful for testing:
- PR labeling automation
- CI checks that depend on PR metadata
- Workflows that comment on PRs

---

## Using Custom Secrets

Many workflows require secrets. You can provide them in several ways:

### Method 1: Command Line (one-time)

```bash
act -s GITHUB_TOKEN=your_token_here -s API_KEY=secret_key
```

### Method 2: Environment Variables

```bash
export GITHUB_TOKEN=your_token_here
act -s GITHUB_TOKEN
```

### Method 3: .secrets File (recommended for local development)

Create a `.secrets` file in your repository root:

```bash
# .secrets - Add this to your .gitignore!
GITHUB_TOKEN=ghp_xxxxxxxxxxxx
API_KEY=your_api_key
DOCKER_USERNAME=your_username
DOCKER_PASSWORD=your_password
```

Then simply run:
```bash
act
```

> ⚠️ **Security Note**: Never commit the `.secrets` file to your repository. Add it to `.gitignore`!

---

## Running with Different Event Types

Different GitHub events trigger different workflows. You can simulate any event:

```bash
# Simulate a push event (default)
act push

# Simulate a pull request event
act pull_request

# Simulate a release event
act release

# Simulate a workflow_dispatch (manual trigger)
act workflow_dispatch

# Simulate a schedule event (for cron-based workflows)
act schedule
```

### Testing workflow_dispatch with Inputs

For workflows that accept inputs:

```yaml
on:
  workflow_dispatch:
    inputs:
      environment:
        description: 'Environment to deploy'
        required: true
        default: 'staging'
      debug_mode:
        description: 'Enable debug mode'
        required: false
        default: 'false'
```

Run with inputs:
```bash
act workflow_dispatch \
  -e <(echo '{"inputs": {"environment": "production", "debug_mode": "true"}}')
```

---

## Debugging Failed Workflows

When a workflow fails locally, you can debug it:

### View Detailed Logs

```bash
# Run with verbose output
act -v

# Run with very verbose output (includes Docker commands)
act -vv
```

### Keep the Container Running for Inspection

```bash
# Keep container after job completes (even on failure)
act --bind

# Then inspect the container
docker ps -a
docker exec -it <container_id> /bin/bash
```

### Run Specific Steps

```bash
# Run only up to a specific step
act --step-debug
```

### Common Debugging Tips

1. **Check the environment**: Run `act -v` to see what environment variables are being set
2. **Inspect artifacts**: Use `--artifact-server-path` to persist artifacts locally
3. **Check container logs**: When using `--bind`, containers remain for inspection

---

## Matrix Build Testing

When working with matrix builds, you may want to test a specific combination:

```yaml
strategy:
  matrix:
    node-version: [16, 18, 20]
    os: [ubuntu-latest, windows-latest]
```

### Test Specific Matrix Combination

Unfortunately, `act` doesn't directly support filtering matrix jobs by combination. However, you can:

1. **Temporarily modify the workflow** for testing:
```yaml
strategy:
  matrix:
    node-version: [18]  # Test only one version
    os: [ubuntu-latest]
```

2. **Use environment variables** to control behavior:
```yaml
- name: Setup Node
  uses: actions/setup-node@v4
  with:
    node-version: ${{ matrix.node-version }}
```

Then test locally with a single version:
```bash
act -j test -e <(echo '{"matrix": {"node-version": "18"}}')
```

---

## Performance Tips

### Use Smaller Images

```bash
# For quick syntax validation
act -P ubuntu-latest=node:16-alpine

# For faster iteration during development
act -P ubuntu-latest=catthehacker/ubuntu:act-latest
```

### Cache Docker Layers

```bash
# Use buildkit for better caching
export DOCKER_BUILDKIT=1
act
```

### Run Jobs in Parallel (when safe)

```bash
# If jobs don't depend on each other, run them separately in parallel terminals
act -j build &
act -j lint &
wait
```

---

## Tips for CI/CD Integration

While `act` is primarily for local development, you can use it in CI for validation:

```yaml
name: Validate Workflows
on: [push, pull_request]

jobs:
  act-syntax-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install act
        run: |
          curl -fsSL https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash
      
      - name: Validate workflow syntax
        run: |
          act -l  # List jobs to validate YAML syntax
          
      - name: Dry run (list what would execute)
        run: |
          act -n  # Dry run mode
```

---

## Troubleshooting

### "Job not found" Error

Make sure the workflow file exists and the job name is correct:
```bash
# List all available workflows and jobs
act -l
```

### Docker Connection Issues

Ensure Docker is running:
```bash
docker ps
act --container-daemon-socket /var/run/docker.sock
```

### Out of Disk Space

Clean up act containers and images:
```bash
# Remove stopped act containers
docker container prune -f

# Remove unused images
docker image prune -f
```

---

## Getting Help

If you encounter issues not covered here:

1. Check the [act user guide](https://nektosact.com)
2. Search [existing discussions](https://github.com/nektos/act/discussions)
3. Join the [GitHub Discussions](https://github.com/nektos/act/discussions) for community support

Happy testing! 🚀
