# Contributing to EchoWarp

Thank you for your interest in contributing to EchoWarp! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, inclusive, and constructive in all interactions.

## Development Setup

### Prerequisites

- **Go 1.22+** (project uses Go 1.26)
- **CGo** enabled (required for Opus codec)
- **Opus library**:
  - macOS: `brew install opus`
  - Linux (Debian/Ubuntu): `sudo apt-get install libasound2-dev libopus-dev`

### Clone and Build

```bash
git clone https://github.com/lHumaNl/EchoWarp.git
cd EchoWarp
make build
```

### Run Tests

```bash
make test        # All tests with race detection
make test-short  # Skip integration tests
make test-cover  # Generate coverage report
```

### E2E Tests

E2E tests use build tag `//go:build e2e` and are located in `test/e2e/`:

```bash
go test -tags=e2e -v ./test/e2e/...
```

## Project Structure

```
cmd/echowarp/          → Entry point (main.go)
internal/              → Private application code
  cli/                 → Cobra commands (server, client, devices, config, daemon)
  tui/                 → Bubble Tea TUI (3 screens)
  app/                 → ServerApp, ClientApp, RunWithReconnect, IPRateLimiter
  config/              → YAML config, validation
  logging/             → charmbracelet/log + file logging
  api/                 → HTTP REST API + WebSocket events
  daemon/              → PID file, lifecycle management
pkg/echowarp/          → Public facade (Node, Options, Events)
  audio/               → malgo capture/playback, Opus codec, AudioMixer, VirtualMic
  transport/           → TCPSignaler, WebRTCPeer, MultiPeerManager, ICE
  auth/                → HMAC-SHA256 challenge-response
  ban/                 → File-based ban manager
  discovery/           → mDNS/DNS-SD via grandcat/zeroconf
test/e2e/              → End-to-end tests (build tag: e2e)
```

## Development Workflow

1. **Fork** the repository on GitHub
2. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feature/my-feature
   ```
3. **Make your changes** following coding standards below
4. **Run checks** before committing:
   ```bash
   make check  # Runs fmt, vet, lint, test-short
   ```
5. **Push and submit a Pull Request**

## Coding Standards

### Formatting

Run `gofmt` before committing:
```bash
make fmt  # gofmt -s -w .
```

### Code Comments

- Document exported functions and types
- Explain "why" not "what"
- Keep comments up-to-date with code changes

### Error Handling

- Always check and handle errors
- Wrap errors with context using `fmt.Errorf("operation failed: %w", err)`
- Never ignore errors with `_`

### Testing

- Write unit tests for new functionality
- Use `testify` for assertions (already a dependency)
- Table-driven tests preferred for multiple cases
- Aim for meaningful coverage, not 100%

## Commit Messages

Format: `type(scope): description`

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `chore`: Maintenance tasks
- `perf`: Performance improvements

Examples:
```
feat(audio): add adaptive bitrate controller
fix(transport): resolve data race in TCPSignaler
docs(readme): update installation instructions
```

## Pull Request Process

### PR Title

Use the same format as commit messages: `type(scope): description`

### PR Description

Include:
- **Summary**: What changes and why
- **Testing**: How you tested the changes
- **Related Issues**: Link any related issues

### Review Process

1. All PRs require at least one review
2. CI checks must pass (lint, test, build)
3. Address review feedback promptly
4. Squash commits before merge (if requested)
