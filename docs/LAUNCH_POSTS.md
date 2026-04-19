# Launch Posts for Corral v0.1.0

## Hacker News — Show HN

**Title:** Show HN: Corral – A scheduler and harness for autonomous AI coding agents

**URL:** https://github.com/Inspirify/corral

**Text (if self-post):**

I built Corral because I was running 3–5 autonomous coding agents (Claude Code, OpenCode) across multiple repos and the shell script situation got out of hand — each script was 160+ lines of locking, watchdog, cleanup, and log management.

Corral is a single Go binary that handles the full lifecycle:

- **Cron scheduling** with jitter support
- **Single-instance locking** — no overlapping runs
- **Idle timeout + max runtime** — agents don't run forever
- **Continuation loops** — agents can signal "not done yet"
- **Per-run log capture** with automatic rotation
- **Service install** — launchd (macOS) and systemd (Linux)
- **Backend-agnostic** — works with Claude Code, OpenCode, Codex CLI, Aider, or any CLI tool

Config is a simple TOML file:

```toml
[agents.dev]
command = "claude -p 'Pick the next issue and implement it'"
schedule = "0 */2 * * *"
max_runtime = "3h"
idle_timeout = "15m"
```

Install: `brew tap inspirify/tap && brew install corral`

Would love feedback on the design. The codebase is ~2k lines of Go.

---

## Reddit r/golang

**Title:** Corral — a general-purpose scheduler and harness for autonomous AI coding agents (single Go binary)

**Body:**

Hey r/golang,

I just open-sourced [Corral](https://github.com/Inspirify/corral), a CLI tool I built to manage autonomous AI coding agents. It's a single Go binary, ~2k lines, built with Cobra + go-toml.

**The problem:** I was running multiple AI agents (Claude Code, OpenCode) on cron, and each agent's shell script grew to 160+ lines — locking, watchdog processes, cleanup, log rotation, idle detection. Every agent duplicated the same boilerplate.

**What Corral does:**
- Cron scheduling with jitter
- Single-instance locking (no overlapping runs)
- Idle timeout + max runtime enforcement
- Continuation loops (agent signals "not done yet")
- Per-run log capture
- launchd/systemd service installation
- TOML configuration with defaults inheritance

**Stack:** Go 1.26, Cobra, robfig/cron, pelletier/go-toml. Cross-compiled for darwin/linux × amd64/arm64.

Install: `brew tap inspirify/tap && brew install corral`

GitHub: https://github.com/Inspirify/corral

Feedback welcome — especially on the harness design and whether the TOML config schema makes sense.

---

## Reddit r/LocalLLaMA

**Title:** Corral — open-source scheduler for autonomous coding agents (Claude Code, Codex, Aider, etc.)

**Body:**

If you're running autonomous AI coding agents, you probably have a mess of cron jobs, shell scripts, and log files. I built [Corral](https://github.com/Inspirify/corral) to fix that.

It's a single binary that handles scheduling, locking, timeouts, continuation loops, and log management. Works with any CLI-based agent — Claude Code, OpenCode, Codex CLI, Aider, etc.

Quick example:
```toml
[agents.dev]
command = "claude -p 'Pick the next GitHub issue and implement it'"
schedule = "0 */2 * * *"
max_runtime = "3h"
idle_timeout = "15m"
```

Then: `corral start` runs your agents on schedule. `corral status` shows what's running. `corral logs dev` tails output.

Install: `brew tap inspirify/tap && brew install corral`

GitHub: https://github.com/Inspirify/corral

---

## Reddit r/ChatGPTCoding

**Title:** Open-sourced a tool to schedule and manage autonomous coding agents (works with Claude Code, Codex, Aider, etc.)

**Body:**

I've been running autonomous coding agents overnight and on schedules — pick up GitHub issues, run code review, do refactoring. The problem is each agent needs: scheduling, locking (don't run two at once), timeouts (don't let it run forever), log management, and a way to handle "I'm not done yet" continuations.

I built [Corral](https://github.com/Inspirify/corral) to handle all of that. It's a single Go binary with TOML config:

```toml
[agents.dev]
command = "claude -p 'Pick the next issue and implement it'"
schedule = "0 */2 * * *"
max_runtime = "3h"
```

- `corral start` — runs agents on their cron schedules
- `corral run dev` — run one agent immediately
- `corral status` — see what's running, next scheduled time
- `corral install` — install as a system service (survives reboots)

Works with Claude Code, OpenCode, Codex CLI, Aider, or literally any CLI command.

Install: `brew tap inspirify/tap && brew install corral`

GitHub: https://github.com/Inspirify/corral

---

## X/Twitter

Running autonomous AI coding agents? I built Corral — an open-source scheduler and harness that handles cron, locking, timeouts, continuations, and logs for any CLI agent.

Single Go binary. TOML config. Works with Claude Code, Codex, Aider, etc.

brew tap inspirify/tap && brew install corral

https://github.com/Inspirify/corral
