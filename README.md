# InstrLint

[![CI](https://github.com/cottondesu/instrlint/actions/workflows/ci.yml/badge.svg)](https://github.com/cottondesu/instrlint/actions/workflows/ci.yml)

Fast, zero-dependency linter for `AGENTS.md` and AI coding-agent instruction files.

[GitHub repository](https://github.com/cottondesu/instrlint)

## Why InstrLint?

AI coding agents increasingly rely on repository-level instructions such as `AGENTS.md`. As these files grow, duplicated and directly conflicting instructions can become hard to notice. InstrLint checks for duplicates and a limited set of high-confidence conflicts quickly and deterministically, without an LLM, network connection, or runtime dependencies.

## Features

- Duplicate instruction detection in `AGENTS.md`
- High-confidence conflict detection for conservative English patterns
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

## Build from source

With Go 1.23 or newer, from the repository root:

```sh
go build ./cmd/instrlint
```

The Go module is `github.com/cottondesu/instrlint`.

To install the latest published release:

```sh
go install github.com/cottondesu/instrlint/cmd/instrlint@latest
```

## Quick Start

```sh
instrlint .
instrlint . --exclude .omx
instrlint . --exclude .omx --exclude generated
instrlint AGENTS.md
instrlint --help
```

If built locally and not on your `PATH`, use `./instrlint` instead. A directory with no `AGENTS.md` prints `no supported instruction files found` and exits successfully. Clean files produce no output.

`--exclude` may be repeated to skip directories before recursive scanning enters them. A directory name such as `.omx` matches at any depth; a relative directory path such as `tools/cache` matches only from the scan root. Absolute paths and paths escaping the scan root are rejected. The option is ignored when scanning a file directly. This is not glob or gitignore syntax; negation is not supported.

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
| 0 | No lint violations |
| 1 | Lint violations found |
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

## License

MIT. See [LICENSE](LICENSE).
