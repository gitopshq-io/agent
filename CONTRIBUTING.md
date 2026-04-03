# Contributing to GitOpsHQ Agent

Thanks for contributing.

## Development Setup

- Go 1.25+
- Helm 3+
- Buf CLI

## Run Checks Locally

```bash
make test
make lint
make chart-template
```

## Pull Request Guidelines

- Keep changes scoped and explain the reason in the PR description.
- Add or update tests for behavior changes.
- Update documentation when configuration, protocol, or security behavior changes.
- Ensure CI passes before requesting review.

## Commit and Review Expectations

- Prefer small, reviewable commits.
- Avoid unrelated refactors in feature or bugfix PRs.
- Keep security-sensitive changes explicit and documented.

