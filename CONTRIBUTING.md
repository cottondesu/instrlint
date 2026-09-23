# Contributing to InstrLint

InstrLint is a small Go CLI. Contributions should keep its behavior deterministic, offline, and free of external Go dependencies.

## Development setup

Install Go 1.23 or newer, clone the repository, and run:

```sh
go build ./cmd/instrlint
go test ./...
go vet ./...
```

Format changed Go files with `gofmt`. Run `go test -race ./...` when a change can affect concurrent or shared behavior, and `go test -bench=. ./...` when parser, duplicate-detection, or conflict-detection work may affect performance.

## Parser regressions

Add a minimal fixture under `internal/instrlint/testdata/regression/` for parser behavior or a bug fix. Keep one clear behavior per fixture when practical, sanitize real-world inputs, and assert the rule, diagnostic line, related line, and ordering when diagnostics are expected. Include false-positive cases as clean fixtures.

Parser bug reports should include a minimal, sanitized `AGENTS.md`, expected and actual results, the InstrLint version, and the operating system. Remove private repository names, paths, URLs, credentials, and proprietary instructions.

## Conflict rules

Keep conflict detection conservative and explainable. Any new conflict pattern must include both positive tests and false-positive regression fixtures, including different-scope or similar-but-compatible examples where applicable.

## Pull requests

Keep changes focused, preserve the documented exit codes and stdout/stderr behavior, and explain any parser behavior change. Before opening a pull request, run `gofmt`, `go vet ./...`, `go test ./...`, and `go build ./cmd/instrlint`. Pull requests are expected to pass the GitHub Actions CI workflow, including the race test.
