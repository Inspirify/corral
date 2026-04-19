# AGENTS.md — Corral

## Project Overview

Corral is a general-purpose agent harness and scheduler — a single Go binary that runs, schedules, and manages autonomous AI coding agents with any backend.

- **Language:** Go
- **Config format:** TOML (layered: `corral.toml` + `agents/*.toml`)
- **Platforms:** macOS (launchd), Linux (systemd)
- **License:** MIT

## Architecture

```
corral/
├── cmd/corral/        — CLI entry point (cobra commands)
├── internal/
│   ├── config/        — TOML loader, validation, merging, env interpolation
│   ├── scheduler/     — Cron engine, jitter, next-run calculation
│   ├── harness/       — Lifecycle manager: lock, watchdog, continuation, cleanup
│   ├── process/       — Agent process management, signal handling
│   ├── service/       — OS service install/uninstall (launchd/systemd)
│   └── logging/       — Structured log management, rotation
├── docs/
│   ├── PRD.md         — Product requirements
│   └── TECH_DESIGN.md — Technical design document
├── corral.toml        — Example configuration
├── agents/            — Example per-agent unit files
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Development

```bash
# Build
go build -o corral ./cmd/corral

# Run tests
go test ./...

# Install locally
go install ./cmd/corral
```

## Verification Steps

1. `go build ./...` — must compile cleanly
2. `go test ./...` — all tests must pass
3. `go vet ./...` — no vet warnings

## Design Documents

- [Product Requirements](docs/PRD.md)
- [Technical Design](docs/TECH_DESIGN.md)
