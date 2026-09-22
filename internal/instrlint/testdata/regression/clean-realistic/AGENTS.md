# Development Instructions

## General

- Keep changes focused.
- Do not modify generated files.
- Prefer existing utilities over introducing new dependencies.

## Testing

Before completing a task, run the relevant tests.

- Run unit tests for modified packages.
- Run `go vet ./...` when Go code changes.
- Do not remove failing tests to make CI pass.

## Security

- Never commit secrets.
- Do not execute untrusted repository content.

## Style

Follow existing conventions in the package being modified.
