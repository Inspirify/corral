// Package config handles loading, merging, and validating corral configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config is the root configuration loaded from corral.toml and agents/*.toml.
type Config struct {
	Defaults AgentDefaults          `toml:"defaults"`
	Agents   map[string]AgentConfig `toml:"agents"`
}

// AgentDefaults defines default values inherited by all agents.
type AgentDefaults struct {
	IdleTimeout Duration          `toml:"idle_timeout"`
	MaxRuntime  Duration          `toml:"max_runtime"`
	Lock        bool              `toml:"lock"`
	LogDir      string            `toml:"log_dir"`
	DoneSignal  string            `toml:"done_signal"`
	Env         map[string]string `toml:"env"`
}

// AgentConfig defines the configuration for a single agent.
// Fields can be set inline in corral.toml or in a per-agent agents/*.toml file.
type AgentConfig struct {
	// Metadata
	Description string `toml:"description"`

	// Command — inline form
	Cmd string `toml:"command"`
	// Command — unit file form
	Run  string   `toml:"run"`
	Args []string `toml:"args"`

	WorkingDir string            `toml:"working_dir"`
	Env        map[string]string `toml:"env"`

	// Schedule — inline form
	Sched string `toml:"schedule"`
	// Schedule — unit file form
	Cron   string   `toml:"cron"`
	Jitter Duration `toml:"jitter"`

	// Harness
	IdleTimeout      Duration `toml:"idle_timeout"`
	MaxRuntime       Duration `toml:"max_runtime"`
	MaxContinuations int      `toml:"max_continuations"`
	DoneSignal       string   `toml:"done_signal"`
	LockEnabled      *bool    `toml:"lock"`
	LogDir           string   `toml:"log_dir"`
}

// Command returns the resolved command string, preferring inline "command" over unit-file "run".
func (a AgentConfig) Command() string {
	if a.Cmd != "" {
		return a.Cmd
	}
	return a.Run
}

// Schedule returns the resolved cron expression.
func (a AgentConfig) Schedule() string {
	if a.Sched != "" {
		return a.Sched
	}
	return a.Cron
}

// Lock returns whether single-instance locking is enabled.
func (a AgentConfig) Lock() bool {
	if a.LockEnabled != nil {
		return *a.LockEnabled
	}
	return true // default
}

// Load reads the configuration from the given path (or default ./corral.toml),
// discovers agents/*.toml files, merges defaults, interpolates env vars, and validates.
func Load(path string) (*Config, error) {
	if path == "" {
		path = "corral.toml"
	}

	cfg, err := loadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", path, err)
	}

	// Discover and merge per-agent files
	agentsDir := filepath.Join(filepath.Dir(path), "agents")
	if err := discoverAgents(agentsDir, cfg); err != nil {
		return nil, fmt.Errorf("discovering agents: %w", err)
	}

	// Apply defaults to all agents
	applyDefaults(cfg)

	// Interpolate environment variables
	interpolateAll(cfg)

	// Validate
	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	return cfg, nil
}

// validate checks that the configuration is internally consistent.
func validate(cfg *Config) error {
	for name, agent := range cfg.Agents {
		if agent.Command() == "" {
			return fmt.Errorf("agent %q: no command specified", name)
		}
	}
	return nil
}

// Duration wraps time.Duration for TOML string parsing (e.g., "15m", "3h").
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", string(text), err)
	}
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// expandPath replaces ~ with the user's home directory.
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
