# Contributing to Codencer

Thank you for your interest in contributing to Codencer! As a **Public Beta (v0.1.0-beta)** project, we are actively looking for feedback on our orchestration protocols, CLI ergonomics, and adapter reliability.

## 🏛 The Relay Philosophy
Before contributing, please remember that Codencer is a **Defensive Relay**, not a "Brain." We prioritize:
- **Local-First Safety**: No data should ever leave the user's machine via the daemon.
- **Auditability**: Every action must be recorded with high-fidelity evidence (logs, diffs, hashes).
- **Simplicity**: Favor standard Go patterns and CLI-based toolchains.

## 🛠 Local Development Setup
1. **Fork & Clone**: Standard GitHub fork/clone workflow.
2. **Prerequisites**: Ensure you have Go 1.21+, SQLite3, and Git installed.
3. **Build**: Run `make setup build` to initialize the `.codencer/` directory and compile binaries.
4. **Test**: Run `make test` and `make smoke` to verify the orchestrator state machine.

## 🧪 Testing Guidelines
- **Unit Tests**: Use `t.TempDir` for filesystem isolation.
- **Simulation**: Use `make simulate` to test orchestrator logic without requiring LLM agents.
- **Benchmarks**: If adding a new feature, ensure it is covered by a benchmark in the `internal/benchmarks/` suite.

## 📝 Pull Request Process
1. Create a feature branch from `main`.
2. Ensure your code is formatted with `go fmt` and passes `golangci-lint` (if installed).
3. Update specific trackers in `docs/internal/` (e.g., `TASKS.md`, `PROGRESS.md`) if appropriate for larger changes.
4. Submit the PR with a clear description of the problem solved.

## ⚖ License
By contributing, you agree that your contributions will be licensed under the project's **MIT License**.
