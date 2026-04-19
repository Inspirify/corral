# Corral

A general-purpose agent harness and scheduler. Run, schedule, and manage autonomous AI coding agents with any backend.

## What is Corral?

Corral is a single Go binary that manages the full lifecycle of autonomous AI coding agents:

- **Schedule** agents on cron expressions with jitter support
- **Harness** each run with locking, timeouts, continuation loops, and health checks
- **Backend-agnostic** — works with Claude Code, OpenCode, Codex CLI, Aider, or any CLI tool
- **Cross-platform** — macOS (launchd) and Linux (systemd) service installation built in

## Quick Start

```bash
# Install
go install github.com/Inspirify/corral@latest

# Initialize a config
corral init

# Define an agent in corral.toml
cat >> corral.toml << 'EOF'
[agents.dev]
command = "claude -p 'Pick up the next issue and implement it'"
schedule = "0 */1 * * *"
EOF

# Run it once now
corral run dev

# Or start the scheduler daemon
corral start
```

## Configuration

Corral uses a layered TOML configuration:

- **`corral.toml`** — Global defaults and simple inline agents
- **`agents/*.toml`** — Per-agent unit files for complex agents (auto-discovered)

Per-agent files inherit from `[defaults]` in `corral.toml` and can override any setting.

### Example `corral.toml`

```toml
[defaults]
idle_timeout = "15m"
max_runtime = "3h"
lock = true
log_dir = "~/.config/corral/logs"
done_signal = "AGENT_DONE"

[defaults.env]
PATH = "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"

# Simple agents can be defined inline
[agents.cpo]
command = "claude -p 'Run morning board scan'"
schedule = "0 6,12,18 * * *"
max_runtime = "1h"
```

### Example `agents/dev.toml`

```toml
[agent]
description = "Autonomous developer agent"

[command]
run = "opencode run --attach"
args = ["--continue"]
env = { OPENCODE_SERVER_PASSWORD = "${CORRAL_SECRET_OPENCODE}" }

[schedule]
cron = "*/30 * * * *"
jitter = "5m"

[harness]
max_continuations = 20
idle_timeout = "15m"
max_runtime = "3h"
done_signal = "AGENT_DONE"
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `corral start` | Start the scheduler daemon |
| `corral run <agent>` | Run a single agent immediately |
| `corral status` | Show all agents, next scheduled run, last result |
| `corral logs <agent>` | Tail agent logs |
| `corral list` | List all configured agents |
| `corral init` | Scaffold a new `corral.toml` |
| `corral install` | Install as a system service (launchd/systemd) |
| `corral uninstall` | Remove system service |

## Why Corral?

If you're running autonomous coding agents, you need:

1. **Scheduling** — Agents should run at the right times without manual intervention
2. **Lifecycle management** — Locking, timeouts, continuation, cleanup
3. **Observability** — Logs, status, health checks
4. **Safety** — Single-instance enforcement, graceful shutdown, activity-based timeouts

Corral handles all of this so your agent configuration stays declarative and your agents stay reliable.

## License

MIT
