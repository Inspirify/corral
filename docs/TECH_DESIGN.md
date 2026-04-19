# Corral — Technical Design

## Overview

Corral is a single Go binary (~5-10MB) with zero runtime dependencies. It provides agent scheduling, lifecycle management, and OS service integration for autonomous AI coding agents.

**Language:** Go 1.22+
**Config:** TOML via `pelletier/go-toml`
**CLI:** `spf13/cobra`
**Cron:** `robfig/cron/v3`

---

## 1. Project Structure

```
corral/
├── cmd/
│   └── corral/
│       └── main.go              — Entry point, cobra root command
├── internal/
│   ├── config/
│   │   ├── config.go            — Root config types, Load(), Validate()
│   │   ├── merge.go             — Merge defaults + agent overrides
│   │   ├── env.go               — Environment variable interpolation
│   │   └── config_test.go
│   ├── scheduler/
│   │   ├── scheduler.go         — Cron-based scheduler, jitter support
│   │   ├── scheduler_test.go
│   │   └── cron.go              — Cron expression parsing helpers
│   ├── harness/
│   │   ├── harness.go           — Core run loop: lock → exec → watch → cleanup
│   │   ├── lock.go              — File-based PID lock
│   │   ├── watchdog.go          — Idle timeout + max runtime enforcement
│   │   ├── continuation.go      — Done signal detection + session resume
│   │   └── harness_test.go
│   ├── process/
│   │   ├── manager.go           — Process tracking, signal forwarding
│   │   └── manager_test.go
│   ├── service/
│   │   ├── service.go           — Interface for OS service install/uninstall
│   │   ├── launchd.go           — macOS launchd plist generation
│   │   ├── systemd.go           — Linux systemd unit generation
│   │   └── service_test.go
│   └── logging/
│       ├── logger.go            — Per-agent, per-run log file management
│       └── logger_test.go
├── docs/
│   ├── PRD.md
│   └── TECH_DESIGN.md
├── testdata/                    — Test fixtures (sample configs)
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── AGENTS.md
└── LICENSE
```

---

## 2. Configuration System

### 2.1 Types

```go
type Config struct {
    Defaults AgentDefaults          `toml:"defaults"`
    Agents   map[string]AgentConfig `toml:"agents"`
}

type AgentDefaults struct {
    IdleTimeout      Duration          `toml:"idle_timeout"`
    MaxRuntime       Duration          `toml:"max_runtime"`
    Lock             bool              `toml:"lock"`
    LogDir           string            `toml:"log_dir"`
    DoneSignal       string            `toml:"done_signal"`
    Env              map[string]string `toml:"env"`
}

type AgentConfig struct {
    // Metadata
    Description string `toml:"description"`

    // Command
    Command     string            `toml:"command"`     // inline shorthand
    Run         string            `toml:"run"`         // unit file form
    Args        []string          `toml:"args"`
    WorkingDir  string            `toml:"working_dir"`
    Env         map[string]string `toml:"env"`

    // Schedule
    Schedule    string `toml:"schedule"`    // inline shorthand
    Cron        string `toml:"cron"`        // unit file form
    Jitter      Duration `toml:"jitter"`

    // Harness
    IdleTimeout      Duration `toml:"idle_timeout"`
    MaxRuntime       Duration `toml:"max_runtime"`
    MaxContinuations int      `toml:"max_continuations"`
    DoneSignal       string   `toml:"done_signal"`
    Lock             *bool    `toml:"lock"`              // pointer to distinguish unset from false
    LogDir           string   `toml:"log_dir"`
}
```

### 2.2 Loading Order

1. Load `corral.toml` from working directory (or `--config` flag)
2. Discover all `agents/*.toml` files
3. For each agent file, parse and merge with `[defaults]`
4. Inline agents from `corral.toml` are also merged with defaults
5. Per-agent values override defaults (explicit wins)
6. Interpolate environment variables in all string fields
7. Validate: required fields present, cron expressions valid, timeouts positive

### 2.3 Environment Variable Interpolation

All string values undergo interpolation before use:

- `${VAR}` — replaced with `os.Getenv("VAR")`, error if unset
- `${VAR:-default}` — replaced with env value or default if unset
- `${CORRAL_SECRET_*}` — same as `${VAR}` but flagged as sensitive (redacted in logs)

---

## 3. Scheduler

### 3.1 Design

The scheduler is a long-lived goroutine that:

1. Loads config and builds a schedule table
2. For each agent with a cron expression, registers a job with `robfig/cron`
3. Each job fires by calling `harness.Run(agentName)`
4. The harness checks the lock before starting — if locked, the run is skipped and logged

### 3.2 Jitter

When `jitter` is configured, the scheduler adds a random delay between 0 and the jitter duration before starting the agent. This prevents multiple agents on the same schedule from all starting simultaneously.

### 3.3 Graceful Shutdown

On SIGINT/SIGTERM:
1. Stop accepting new scheduled runs
2. Send SIGTERM to all running agent processes
3. Wait up to 30 seconds for graceful exit
4. SIGKILL any remaining processes
5. Release all locks
6. Exit 0

---

## 4. Harness Engine

The harness is the core of Corral. Each `corral run <agent>` invocation (whether manual or scheduled) goes through this pipeline:

```
┌─────────┐    ┌──────────┐    ┌───────────┐    ┌────────────┐    ┌─────────┐
│ Acquire  │───▶│  Start   │───▶│  Watchdog │───▶│ Continuation│───▶│ Cleanup │
│  Lock    │    │ Process  │    │  Monitor  │    │   Loop     │    │         │
└─────────┘    └──────────┘    └───────────┘    └────────────┘    └─────────┘
```

### 4.1 Locking

```go
type Lock struct {
    Path string // e.g., ~/.config/corral/locks/dev.lock
    PID  int
}
```

- Lock file contains the PID of the owning process
- On acquire: check if file exists → if yes, check if PID is alive → if dead, steal lock
- On release: remove lock file
- Locks are always released in a `defer` + signal handler

### 4.2 Watchdog

The watchdog runs as a goroutine alongside the agent process:

- **Idle detection:** Wraps stdout/stderr in a `io.Writer` that records last-write timestamp. A ticker checks every 10 seconds if `time.Since(lastWrite) > idleTimeout`.
- **Max runtime:** A `time.After(maxRuntime)` channel fires unconditionally.
- **Action on timeout:** Send SIGTERM, wait 10s, then SIGKILL if still alive.

### 4.3 Continuation Loop

After the agent process exits:

1. Check if `done_signal` was found in stdout
2. If found → agent completed, stop
3. If not found and `continuations < maxContinuations` → restart the process (continuation)
4. If max continuations reached → stop and log warning

The continuation counter resets on each scheduled invocation.

### 4.4 Output Handling

- stdout and stderr are tee'd to both the log file and an in-memory ring buffer (last 1000 lines)
- The ring buffer is used for done-signal detection and `corral status` last-output display
- Log files are named: `{log_dir}/{agent}/{YYYY-MM-DD-HHMMSS}.log`

---

## 5. Process Manager

Tracks all running agent processes:

```go
type Manager struct {
    mu       sync.Mutex
    agents   map[string]*RunningAgent
}

type RunningAgent struct {
    Name       string
    PID        int
    StartedAt  time.Time
    Cmd        *exec.Cmd
    Cancel     context.CancelFunc
    ExitCode   int
    State      AgentState  // Running, Idle, Error
}
```

Provides:
- `Start(name, cmd)` — track a new process
- `Stop(name)` — graceful stop with SIGTERM → SIGKILL escalation
- `StopAll()` — called on daemon shutdown
- `Status()` — snapshot of all agents for `corral status`

---

## 6. OS Service Integration

### 6.1 Interface

```go
type ServiceInstaller interface {
    Install(corralBinary string, configPath string) error
    Uninstall() error
    IsInstalled() bool
}
```

### 6.2 macOS (launchd)

Generates a plist at `~/Library/LaunchAgents/com.corral.scheduler.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" ...>
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.corral.scheduler</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/corral</string>
        <string>start</string>
        <string>--config</string>
        <string>/Users/tom/Projects/myapp/corral.toml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>~/.config/corral/corral.log</string>
    <key>StandardErrorPath</key>
    <string>~/.config/corral/corral.log</string>
</dict>
</plist>
```

### 6.3 Linux (systemd)

Generates a unit at `~/.config/systemd/user/corral.service`:

```ini
[Unit]
Description=Corral Agent Scheduler
After=network.target

[Service]
ExecStart=/usr/local/bin/corral start --config %h/Projects/myapp/corral.toml
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
```

---

## 7. CLI Commands (Cobra)

```
corral
├── init          — Scaffold corral.toml
├── start         — Start scheduler (--daemon for background)
├── stop          — Stop scheduler daemon
├── run <agent>   — Run agent immediately
├── status        — Show agent states
├── logs <agent>  — Tail agent logs
├── list          — List agents and schedules
├── install       — Install system service
├── uninstall     — Remove system service
└── validate      — Check config validity
```

Each command is a separate file in `cmd/corral/`:
- `cmd/corral/start.go`
- `cmd/corral/run.go`
- etc.

---

## 8. Testing Strategy

### Unit Tests
- Config loading, merging, validation (table-driven tests with testdata fixtures)
- Cron expression parsing and jitter calculation
- Lock acquire/release/steal
- Watchdog timeout detection
- Done signal parsing

### Integration Tests
- Full harness run with a mock agent (a script that prints output and exits)
- Idle timeout triggering kill
- Continuation loop resuming after exit
- Lock preventing concurrent runs

### Manual Testing
- End-to-end: `corral init` → edit config → `corral run` → `corral start` → `corral install`
- Service survives reboot (macOS: `launchctl`, Linux: `systemctl`)

---

## 9. Dependencies (Minimal)

| Dependency | Purpose |
|-----------|---------|
| `spf13/cobra` | CLI framework |
| `pelletier/go-toml/v2` | TOML parsing |
| `robfig/cron/v3` | Cron scheduling |

No other external dependencies. Standard library for everything else (os/exec, io, sync, time, os/signal).

---

## 10. Build & Release

```makefile
build:
	go build -o corral ./cmd/corral

test:
	go test ./... -race

install:
	go install ./cmd/corral

release:
	goreleaser release --clean
```

Binary releases via GoReleaser for:
- macOS arm64
- macOS amd64
- Linux arm64
- Linux amd64

Distribution: `go install`, Homebrew tap, GitHub Releases.

---

## 11. Future Considerations (Post-v1)

- **TUI dashboard** — Real-time view of all agents with `corral dashboard`
- **Webhook notifications** — POST to a URL on agent completion/failure
- **Agent templates** — Shareable config snippets for common agent setups
- **Remote execution** — SSH-based agent runs on remote machines
- **Metrics export** — Prometheus endpoint for agent run counts, durations, failures
