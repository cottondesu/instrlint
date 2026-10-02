# InstrLint

[![CI](https://github.com/cottondesu/instrlint/actions/workflows/ci.yml/badge.svg)](https://github.com/cottondesu/instrlint/actions/workflows/ci.yml)

Fast, zero-dependency linter for `AGENTS.md` and AI coding-agent instruction files.

[GitHub repository](https://github.com/cottondesu/instrlint)

## Why InstrLint?

AI coding agents increasingly rely on repository-level instructions such as `AGENTS.md`. As these files grow, duplicated and directly conflicting instructions can become hard to notice. InstrLint checks for duplicates and a limited set of high-confidence conflicts quickly and deterministically, without an LLM, network connection, or runtime dependencies.

## Features

- Duplicate instruction detection in `AGENTS.md`
- High-confidence conflict detection for conservative English patterns
- `AGENTS.md` hierarchy inspection with `instrlint scope`, as a tree or JSON
- Nested and multiline list item support
- Fenced code block and HTML comment exclusion
- UTF-8 and Japanese instruction support
- Deterministic, offline analysis
- Native Go binary with zero external Go dependencies
- CI-friendly exit codes

## Supported scope

InstrLint scans `AGENTS.md` files for duplicate and conflicting instructions. It accepts an individual file path directly, regardless of its name. Directory scans find `AGENTS.md` recursively and skip `.git`, `node_modules`, `vendor`, `dist`, `build`, `out`, `coverage`, `tmp`, and `.cache`. Directory symlinks are not followed. Duplicates and conflicts are compared within each file, not across files.

The parser recognizes unordered and ordered list items, nested lists, indented continuation lines within list items, and common imperative paragraphs in English and Japanese. List items ending in `:` are treated as introductions rather than instructions. It skips headings, backtick and tilde fenced code, indented code outside lists, HTML comments, horizontal rules, blockquotes, and blank lines. It is a small Markdown subset, not a full Markdown parser. Invalid UTF-8 is an error.

Normalization removes list markers, folds ASCII case outside inline code, collapses whitespace, and ignores light sentence-ending punctuation. Inline code remains case-sensitive. Emphasis markers are retained to avoid broad Markdown rewriting, so emphasis-only variations are syntax-sensitive. Emphasis-only paragraphs that begin with Markdown punctuation are outside the recognized imperative-paragraph subset. Other Unicode characters are left as-is; semantically similar wording is not considered equal.

## Conflict detection

InstrLint detects a limited set of high-confidence conflicting English instructions. The rule is `conflicting-instruction`, its severity is `warning`, and each diagnostic identifies both the later instruction and the earlier related instruction.

Supported patterns are deliberately narrow:

- The same imperative with opposite explicit polarity, such as `Always use npm.` and `Never use npm.`
- A bare positive imperative and its explicit prohibition, such as `Edit generated files directly.` and `Do not edit generated files directly.`
- Different supported package managers (`npm`, `pnpm`, `yarn`, or `bun`) selected with `use` for the same explicit `for ...` scope, such as `Use npm for frontend dependencies.` and `Use pnpm for frontend dependencies.`

Opposite-polarity matching is limited to the action verbs `add`, `check`, `commit`, `delete`, `edit`, `execute`, `follow`, `include`, `install`, `keep`, `modify`, `remove`, `run`, `use`, and `write`. Matching is ASCII-case-insensitive and requires the normalized action, object, and scope to agree exactly where applicable. Different scopes remain clean. For example, `Use npm for publishing.` and `Use pnpm for local development.` do not conflict. Unscoped alternatives such as `Use npm.` and `Use pnpm.` are intentionally not reported because the surrounding scope is unknown.

## Installation

### Go install

With Go 1.23 or newer, install the latest published version:

```sh
go install github.com/cottondesu/instrlint/cmd/instrlint@latest
```

Make sure your Go binary directory (usually `$(go env GOPATH)/bin`) is on your `PATH`.

### Prebuilt binaries

Starting with v0.2.1, download the archive for your platform from [GitHub Releases](https://github.com/cottondesu/instrlint/releases). Builds cover both `amd64` and `arm64`:

- macOS and Linux: `instrlint_vX.Y.Z_<os>_<arch>.tar.gz`
- Windows: `instrlint_vX.Y.Z_windows_<arch>.zip`

Each archive contains `instrlint` (or `instrlint.exe`) and `LICENSE`. Use the release's `checksums.txt` to verify the archive's SHA256 checksum before extracting it.

To build from source instead, run `go build ./cmd/instrlint` at the repository root.

## Quick Start

```sh
instrlint .
instrlint . --exclude .omx
instrlint . --exclude .omx --exclude generated
instrlint AGENTS.md
instrlint scope .
instrlint scope . --format json
instrlint --help
```

If built locally and not on your `PATH`, use `./instrlint` instead. A directory with no `AGENTS.md` prints `no supported instruction files found` and exits successfully. Clean files produce no output.

`--exclude` may be repeated to skip directories before recursive scanning enters them. A directory name such as `.omx` matches at any depth; a relative directory path such as `tools/cache` matches only from the scan root. Absolute paths and paths escaping the scan root are rejected. The option is ignored when scanning a file directly. This is not glob or gitignore syntax; negation is not supported.

## Inspecting AGENTS.md hierarchy

Use `scope` to show the discovered `AGENTS.md` ancestry under a directory:

```sh
instrlint scope .
instrlint scope . --exclude tools/cache
```

Example output:

```text
AGENTS.md
├── backend/AGENTS.md
└── frontend/AGENTS.md
```

Each file is shown under its nearest discovered ancestor `AGENTS.md`. If there is no ancestor file, it appears as a separate top-level root. Paths are relative to the scanned directory and use `/` separators. Names containing control characters are quoted to keep each node on one line. `scope` uses the same directory exclusions and symlink behavior as lint scanning. It reads file paths only, so even an `AGENTS.md` with invalid UTF-8 appears in the tree. It does not lint, merge instructions, infer overrides, or report cross-file conflicts. Use `instrlint scope --help` for command-specific help.

### JSON output

Use `--format json` for machine-readable hierarchy output. `--format tree` is the default and prints the tree shown above. `--format` is accepted only by `scope`.

```sh
instrlint scope . --format json
instrlint scope . --exclude .omx --format json
```

```json
{
  "schema_version": 1,
  "entries": [
    {"path": "AGENTS.md", "parent": null},
    {"path": "backend/AGENTS.md", "parent": "AGENTS.md"},
    {"path": "frontend/AGENTS.md", "parent": "AGENTS.md"}
  ]
}
```

The CLI prints the same document as compact JSON on one line, followed by a newline. The contract for `schema_version` 1:

- `schema_version` is the integer `1`.
- `entries` is always an array; it is `[]` when no `AGENTS.md` is found, and no human-readable message is printed.
- `path` is relative to the scanned directory and uses `/` separators, the same path the tree shows.
- `parent` is the `path` of the nearest discovered ancestor `AGENTS.md`, or `null` for a top-level root. A rootless forest has several `null` parents.
- Entries are ordered depth-first, with roots and siblings in lexical order, so every parent appears before its children. Output bytes are deterministic.

Control and other non-graphic characters in paths, including Unicode bidirectional controls, are written as standard JSON escapes such as `\n`, `\u001b`, or `\u202e`, so decoded values match the original paths exactly. A path that is not valid UTF-8 cannot be represented losslessly in JSON; it is reported as an error with exit code 2 and no output on stdout. On success, stdout holds exactly one JSON document and stderr is empty. On usage, filesystem, or rendering errors, stdout is empty; if writing to stdout itself fails, the exit code is 2 and the output may be incomplete.

JSON output reports filesystem ancestry only. It does not model effective instructions, inheritance semantics, or overrides.

## Example output

```text
AGENTS.md:2:1: warning duplicate-instruction: Duplicate instruction: always run tests (first at AGENTS.md:1)

1 warning
```

A conflict uses the same one-line format and reports its related location explicitly:

```text
AGENTS.md:2:1: warning conflicting-instruction: Conflicting instruction: Never use npm. (conflicts with AGENTS.md:1)

1 warning
```

Diagnostics go to stdout. Usage and filesystem errors go to stderr.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | No lint violations, or successful scope inspection |
| 1 | Lint violations found (lint command only) |
| 2 | Usage or runtime error |

## Development

```sh
go test ./...
go vet ./...
go test -run '^$' -bench=. ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the regression-fixture workflow and pull request expectations.

## Reporting parser bugs

Open a [GitHub issue](https://github.com/cottondesu/instrlint/issues) with a minimal, sanitized `AGENTS.md`, the expected and actual results, the InstrLint version, and the operating system. Remove repository names, private paths, URLs, credentials, and proprietary instructions before sharing the input.

## Limitations

Conflict detection is conservative and pattern-based. InstrLint does not detect semantic conflicts, cross-file conflicts, Japanese conflict patterns, ambiguous instructions, synonyms, or complex paraphrases. It is not a full CommonMark parser and has no LLM integration, configuration, or autofix. Paragraph recognition is intentionally conservative and may miss less common imperative forms. Blockquotes are excluded because they may be quotations, and emphasis-only variations are not analyzed as equivalent instructions. Japanese duplicate detection remains supported. Linting is local and deterministic: InstrLint does not use the network, execute Markdown content, or collect telemetry.

Scope inspection does not model effective agent instructions, tool-specific precedence, or a target-specific instruction chain. It does not produce semantic hierarchy warnings, and it does not add any new symlink-following behavior. JSON output is available only for `scope`; lint diagnostics are not available as JSON.

## License

MIT. See [LICENSE](LICENSE).
