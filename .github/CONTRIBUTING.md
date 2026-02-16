# Contributing

Thank you for your interest in contributing to Updater Service.

## Getting Started

Only Docker is required locally. All build and test targets run inside containers.

```bash
make help   # list all available targets
make check  # format + vet + test
```

See the [README](../README.md) for a full development quickstart.

## Reporting Bugs

Use the [bug report template](ISSUE_TEMPLATE/bug_report.yml) when opening a new issue. Include:

- Steps to reproduce
- Expected vs actual behaviour
- Version, storage backend, and OS
- Relevant log output

## Suggesting Features

Use the [feature request template](ISSUE_TEMPLATE/feature_request.yml).
Describe the problem you are trying to solve, not just the solution.

## Pull Request Process

1. Branch from `main`.
2. Keep each PR to one logical change.
3. All checks must pass: `make check`.
4. Add or update tests for any changed behaviour.
5. Update the OpenAPI spec (`internal/api/openapi/openapi.yaml`) if the API changes.
6. Reference any related issues with `Closes #N` in the PR description.

PRs are reviewed on a best-effort basis. Small, focused changes are merged faster.

## Code Standards

These mirror the rules in [CLAUDE.md](../CLAUDE.md):

- Tests must be table-driven and co-located with the code they test.
- No CGO. Exception: the race detector is permitted in test targets only.
- No emojis in code, comments, or documentation.
- Consult context7 documentation before using any third-party library.
- Update the OpenAPI spec whenever the HTTP API changes — this is a manual step.
- Use `log/slog` structured logging; tag security audit events with `"event", "security_audit"`.

## Running Checks Locally

```bash
make check     # fmt + vet + unit tests
make security  # gosec (high severity, medium confidence)
make secrets   # gitleaks scan of git history
```

All three must exit clean before a PR is ready for review.
