# Corral — Product Requirements Document

## Vision

Corral is a general-purpose, open-source tool for running, scheduling, and managing autonomous AI coding agents. It is backend-agnostic, cross-platform, and ships as a single binary with zero runtime dependencies.

## Problem Statement

Teams and individuals running autonomous coding agents face the same set of operational problems:

1. **No standard way to schedule agents** — People cobble together cron jobs, launchd plists, and shell scripts
2. **No lifecycle management** — Agents hang, run forever, or crash without cleanup
3. **Tight coupling to specific AI backends** — Every harness is written for one tool
4. **No observability** — Hard to know what agents are doing, when they last ran, or why they failed
5. **Safety gaps** — No single-instance enforcement, no timeouts, no graceful shutdown

## Target Users

1. **Solo developers** running 1-3 agents on their local machine to automate development tasks
2. **Small teams** running agents across projects with shared configuration patterns
3. **Open-source AI tool authors** who want a standard way to deploy their agents

## Core Requirements

### R1: Backend-Agnostic Agent Execution

- Corral wraps any CLI command as an "agent"
- No knowledge of the underlying AI tool is required
- The agent is defined by: a command to run, environment variables, and a working directory
- Corral manages the process lifecycle, not the AI conversation

### R2: Layered TOML Configuration

- A single `corral.toml` file defines global defaults and optionally inline agents
- Per-agent `agents/*.toml` files provide overrides for complex agents
- Per-agent files are auto-discovered and merged with defaults
- Environment variable interpolation via `${VAR}` syntax
- Secret interpolation via `${CORRAL_SECRET_*}` pattern (sourced from env, not stored in config)

### R3: Built-in Scheduler

- Cron expression support (standard 5-field: minute, hour, day, month, weekday)
- Optional jitter to prevent thundering herd when multiple agents share a schedule
- The scheduler runs as a long-lived daemon via `corral start`
- Respects lock state: won't start an agent if a previous run is still active

### R4: Agent Harness (Lifecycle Management)

Each agent run is wrapped by the harness, which provides:

- **Single-instance locking** — File-based lock with PID tracking. Prevents concurrent runs of the same agent.
- **Activity-based idle timeout** — Monitors stdout/stderr. If no output for N minutes, kill the agent.
- **Maximum runtime** — Hard ceiling on total agent run time.
- **Continuation loop** — After the agent completes (or emits a done signal), optionally resume the session up to N times.
- **Done signal detection** — Poll stdout for a configurable string (e.g., `AGENT_DONE`) to detect completion.
- **Cleanup** — On exit (normal, timeout, or signal), release lock, write summary, log duration.

### R5: Observability

- Structured logs per agent, per run (stored in configurable log directory)
- `corral status` shows: agent name, state (idle/running/error), last run time, next scheduled run, last exit code
- `corral logs <agent>` tails the most recent log file
- Run history: last N runs with duration, exit code, and trigger (scheduled vs manual)

### R6: OS Service Integration

- `corral install` generates and installs a system service that runs `corral start` at boot
  - macOS: launchd plist in `~/Library/LaunchAgents/`
  - Linux: systemd user unit in `~/.config/systemd/user/`
- `corral uninstall` removes the service cleanly
- Service runs as the current user (not root)

### R7: Cross-Platform

- macOS (primary — most AI coding agent users are on macOS)
- Linux (secondary — for CI/CD and server deployments)
- Windows support is not planned for v1

## CLI Interface

```
corral init               Scaffold a new corral.toml in the current directory
corral start              Start the scheduler daemon (foreground by default)
corral start --daemon     Start the scheduler in the background
corral stop               Stop the scheduler daemon
corral run <agent>        Run a single agent immediately, ignoring schedule
corral status             Show all agents and their state
corral logs <agent>       Tail logs for an agent
corral list               List all configured agents with schedule info
corral install            Install corral as a system service
corral uninstall          Remove the system service
corral validate           Validate configuration without running anything
```

## Non-Goals (v1)

- **Skill/prompt management** — Corral doesn't manage agent prompts or skills. That's the agent's job.
- **Multi-agent orchestration** — No DAG execution, no agent-to-agent communication. Each agent is independent.
- **Web dashboard** — CLI only for v1. A TUI or web UI could come later.
- **Remote execution** — Agents run locally. No SSH, no containers, no cloud deployment.
- **Agent marketplace** — No sharing/publishing of agent configs.

## Success Criteria

1. A user can go from zero to a scheduled agent in under 5 minutes
2. Migrating from a hand-rolled shell harness takes less than 30 minutes
3. The tool works reliably unattended for weeks without intervention
4. Configuration is understandable without reading documentation

## Open Questions

1. **Daemon mode:** Should `corral start` default to foreground or background? Foreground is simpler and more debuggable. Background requires PID file management.
2. **Log rotation:** Should corral handle log rotation itself, or defer to the OS (logrotate/newsyslog)?
3. **Notifications:** Should v1 include any notification mechanism (e.g., desktop notification on agent failure)? Or defer to v2?
4. **Secrets management:** Is env-var interpolation sufficient, or do we need a secrets file/keychain integration?
