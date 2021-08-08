---
name: Issue
about: Use this template for reporting a bug/issue.
title: "Issue: <shortly describe issue>"
labels: kind/bug
assignees: ""
---

<!--
    - Make sure you are able to reproduce it on the [latest version](https://github.com/nektos/act/releases)
    - Search the existing issues.
    - Refer to [README](https://github.com/nektos/act/blob/master/README.md).
-->

## System information

<!--
    - Operating System: < Windows | Linux | macOS | etc... >
    - Architecture: < x64 (64-bit) | x86 (32-bit) | arm64 (64-bit) | arm (32-bit) | etc... >
    - Apple M1: < yes | no >
    - Docker version: < output of `docker system info -f "{{.ServerVersion}}"` >
    - Docker image used in `act`: < can be omitted if it's included in log >
    - `act` version: < output of `act --version`, if you've built `act` yourself, please provide commit hash >
-->

- Operating System:
- Architecture:
- Apple M1:
- Docker version:
- Docker image used in `act`:
- `act` version:

## Expected behaviour

<!--
    - Describe how whole process should go and finish
-->

## Actual behaviour

<!--
    - Describe the issue
-->

## Workflow and/or repository

<!--
    - Provide workflow with which we can reproduce the issue
      OR
    - Provide link to your GitHub repository that contains the workflow

<details>
  <summary>workflow</summary>

```none
name: example workflow

on: [push]

jobs:
  [...]
```

</details>

## Steps to reproduce

<!--
    - Make sure to include full command with parameters you used to run `act`, example:
      1. Clone example repo (https://github.com/cplee/github-actions-demo)
      2. Enter cloned repo directory
      3. Run `act -s SUPER_SECRET=im-a-value`
-->

## `act` output

<!--
    - Use `act` with `-v`/`--verbose` and paste output from your terminal in code block below
-->

<details>
  <summary>Log</summary>

```none
PASTE YOUR LOG HERE
```

</details>
