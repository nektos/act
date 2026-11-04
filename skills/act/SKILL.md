---
name: act
description: Run GitHub Actions workflows locally with the act CLI. Use when the user wants to test or debug a workflow or action without pushing, reproduce a CI failure locally, simulate an event (push, pull_request, workflow_dispatch) against .github/workflows/, or mentions act.
---

# Running workflows locally with act

act runs the jobs in `.github/workflows/` inside Docker containers that imitate GitHub's hosted runners. Docker must be running. `act --help` is the authoritative flag reference — this file carries only the workflow and the gotchas `--help` does not confess.

## Install

If `act` is not on the PATH, install it with the system package manager when one carries it (`brew install act`, `winget install nektos.act`, `choco install act-cli`), else with the official script: `curl --proto '=https' --tlsv1.2 -sSfL https://raw.githubusercontent.com/nektos/act/master/install.sh | bash` — it installs to `./bin` by default; pass `-b <dir>` (e.g. `-b ~/.local/bin`) to choose a directory already on the PATH. As a fallback with a Go toolchain: `go install github.com/nektos/act@latest`. Verify with `act --version`.

## Workflow

1. **Scout before running.** `act -l` lists every job with its workflow file and trigger event. `act -n` (dry run) validates the workflows without creating containers. Know what will run before you run it.
2. **Run the target, not everything.** The first positional argument is the event, default `push`: `act pull_request`. Narrow with `-j <job-id>` for one job, `-W <path/to/workflow.yml>` for one file, `--matrix key:value` for one matrix leg.
3. **Supply what GitHub would supply.** Secrets: `-s NAME=value`, or `-s NAME` alone to read it from the shell environment, or `--secret-file .secrets`. Steps that call the GitHub API or check out private repos need a real token: `-s GITHUB_TOKEN="$(gh auth token)"`. `workflow_dispatch` inputs: `--input name=value`. To control the event payload (PR base branch, issue number, pushed ref), write the event JSON to a file and pass `-e event.json`.
4. **Diagnose with `-v`.** Verbose output shows expression evaluation, the images used, and container setup — read it before blaming the workflow.

## Gotchas

- **Runner images are not GitHub's images.** The default medium image (`catthehacker/ubuntu:act-latest`) lacks many tools GitHub preinstalls. A "command not found" failure that CI does not show is an image gap, not a workflow bug. Fix by mapping a fuller image, `-P ubuntu-latest=ghcr.io/catthehacker/ubuntu:full-latest` (large download), or by installing the tool as a step.
- **First run prompts for an image size** (Micro/Medium/Large) and stores the choice in `~/.config/act/actrc`. In a non-interactive session, pass `-P` explicitly or create that file first.
- **`.actrc` is the config file**: one flag per line, read from the home config and the current directory. Pin project-specific `-P` mappings or `--container-architecture` there instead of retyping them.
- **Workflows can detect act**: act sets `ACT=true` in the environment. Guard steps that must not run locally with `if: ${{ !env.ACT }}`.
- **Apple Silicon and other non-amd64 hosts**: pass `--container-architecture linux/amd64` when images or tools misbehave.
- **`upload-artifact`/`download-artifact` fail silently absent an artifact server.** Enable it with `--artifact-server-path /tmp/artifacts`. The cache server (`actions/cache`) runs by default.
- **`--bind` writes into your working directory.** The default copies the checkout into the container; `-b/--bind` mounts it, so the job's writes land on the host.
- **Faster iteration**: `--action-offline-mode` and `-p=false` skip re-pulling actions and images; `-r/--reuse` keeps containers between runs to preserve state.
