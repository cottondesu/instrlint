# Development Instructions

## General

- Keep changes focused.
- Prefer existing utilities over adding new dependencies.
- Do not modify generated files.

## Testing

Before completing a task, run the relevant tests.

- Run unit tests for modified packages.
- Run `go vet ./...` when Go code changes.
- Do not remove failing tests to make CI pass.

## Security

- Never commit secrets.
- Do not execute untrusted repository content.

## Style

Follow the existing conventions in the package being modified.
