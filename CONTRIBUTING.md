# Contributing to Nexus Agentic Protocol

Thank you for your interest in contributing to NAP! This guide explains the process for contributing to this project.

## Getting Started

1. **Fork** the repository on GitHub.
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/<your-username>/nexus.git
   cd nexus
   ```
3. **Create a branch** for your change:
   ```bash
   git checkout -b feat/my-feature
   ```
4. **Install dependencies**:
   ```bash
   make tools
   ```

## Development Workflow

### Build

```bash
make build
```

### Run Tests

```bash
# Unit tests
make test

# Integration tests (requires Docker)
make test-integration

# Coverage report
make test-cover
```

### Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Run `make lint` before submitting.
- Keep functions focused and well-named; avoid deep nesting.
- Use table-driven tests where appropriate.

### Commit Messages

Use concise, imperative-mood messages:

```
feat: add rate limiting middleware
fix: wire pagination query params in ListAgents
test: add agent service unit tests
docs: update quickstart example
```

Prefix with `feat:`, `fix:`, `test:`, `docs:`, `refactor:`, or `chore:`.

## Pull Request Process

1. Ensure all tests pass: `make test`
2. Ensure the build is clean: `go build ./...`
3. Run the linter: `make lint`
4. Push your branch and open a PR against `main`.
5. Fill in the PR template with a summary and test plan.
6. A maintainer will review your PR. Address feedback promptly.

## Reporting Issues

Open a GitHub issue with:
- A clear title describing the problem.
- Steps to reproduce (if a bug).
- Expected vs. actual behavior.
- Go version and OS.

## Code of Conduct

All participants are expected to follow our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.
