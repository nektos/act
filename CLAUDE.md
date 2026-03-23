# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Act is a tool to run GitHub Actions locally. It reads `.github/workflows/` files, builds an execution plan, and uses Docker to run containers for each action. Written in Go 1.24+.

## Common Commands

- `make build` — build binary to `dist/local/act`
- `make test` — run `go test ./...` and the act CLI
- `make lint-go` — run `golangci-lint run`
- `make format` — run `go fmt ./...`
- `make tidy` — run `go mod tidy`
- `make pr` — full PR checklist: tidy, format-all, lint, test
- `go test ./pkg/runner/...` — run tests for a single package
- `go test ./pkg/runner/ -run TestRunEvent` — run a single test

## Architecture

### Execution Flow

1. **CLI** (`cmd/root.go`) — Cobra-based CLI parses flags into an `Input` struct
2. **Planner** (`pkg/model/planner.go`) — parses workflow YAML into a `Plan` containing `Stage`s (serial) with `Run`s (parallel jobs)
3. **Runner** (`pkg/runner/runner.go`) — converts the Plan into composable `Executor` chains
4. **RunContext** (`pkg/runner/run_context.go`) — holds all state for a job execution (env vars, matrix, containers, expressions)
5. **Steps** (`pkg/runner/step.go`) — each step type (action, docker, script) implements the `step` interface

### Core Abstraction: Executor Pattern

The `Executor` type (`pkg/common/executor.go`) is a `func(ctx context.Context) error` used throughout the codebase. Executors compose via:
- `.Then()`, `.Finally()`, `.OnError()` — chaining
- `NewPipelineExecutor()` — serial execution
- `NewParallelExecutor()` — parallel execution
- `.If()`, `.IfNot()` — conditional execution

### Key Packages

- **`pkg/model/`** — workflow YAML parsing, plan creation, action definitions
- **`pkg/runner/`** — core execution engine, expression evaluation, step types (local/remote/docker/composite actions, reusable workflows)
- **`pkg/container/`** — Docker API wrapper, container and host execution environments
- **`pkg/common/`** — Executor pattern, context utilities, logging
- **`pkg/exprparser/`** — GitHub Actions `${{ }}` expression language interpreter
- **`pkg/artifacts/`** and **`pkg/artifactcache/`** — artifact upload/download and caching server

## Linting Rules

Configured in `.golangci.yml`:

- Use `errors` from stdlib, not `github.com/pkg/errors`
- Use `github.com/sirupsen/logrus` (aliased as `log`), not stdlib `log`
- Use `github.com/stretchr/testify` for tests, not `gotest.tools/v3`
- Max cyclomatic complexity: 20
- Import aliases enforced: `logrus` → `log`, `testify/assert` → `assert`

## Testing

- Tests use `testify/assert` and `testify/mock`
- Table-driven tests are common in `pkg/model/` and `pkg/exprparser/`
- Test fixtures live in `testdata/` directories alongside their packages
- `pkg/runner/testdata/` contains extensive sample GitHub Actions workflows used as integration test fixtures
