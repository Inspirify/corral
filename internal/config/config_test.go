package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "corral.toml")
	os.WriteFile(cfgPath, []byte(`
[agents.dev]
command = "echo hello"
`), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(cfg.Agents))
	}
	agent := cfg.Agents["dev"]
	if agent.Command() != "echo hello" {
		t.Errorf("command = %q, want %q", agent.Command(), "echo hello")
	}
}

func TestLoadWithDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "corral.toml")
	os.WriteFile(cfgPath, []byte(`
[defaults]
idle_timeout = "10m"
max_runtime = "2h"
lock = true
done_signal = "DONE"

[defaults.env]
FOO = "bar"

[agents.worker]
command = "do-work"
`), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	agent := cfg.Agents["worker"]
	if agent.IdleTimeout.Duration != 10*time.Minute {
		t.Errorf("idle_timeout = %v, want 10m", agent.IdleTimeout)
	}
	if agent.MaxRuntime.Duration != 2*time.Hour {
		t.Errorf("max_runtime = %v, want 2h", agent.MaxRuntime)
	}
	if !agent.Lock() {
		t.Error("lock should be true")
	}
	if agent.DoneSignal != "DONE" {
		t.Errorf("done_signal = %q, want %q", agent.DoneSignal, "DONE")
	}
	if agent.Env["FOO"] != "bar" {
		t.Errorf("env FOO = %q, want %q", agent.Env["FOO"], "bar")
	}
}

func TestAgentFileDiscovery(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "corral.toml")
	os.WriteFile(cfgPath, []byte(`
[defaults]
idle_timeout = "5m"
`), 0644)

	agentsDir := filepath.Join(dir, "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "cpo.toml"), []byte(`
command = "claude -p 'do CPO scan'"
schedule = "0 9 * * *"
description = "Morning CPO scan"
`), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	agent, ok := cfg.Agents["cpo"]
	if !ok {
		t.Fatal("expected cpo agent from agents/cpo.toml")
	}
	if agent.Command() != "claude -p 'do CPO scan'" {
		t.Errorf("command = %q", agent.Command())
	}
	if agent.Schedule() != "0 9 * * *" {
		t.Errorf("schedule = %q", agent.Schedule())
	}
	if agent.IdleTimeout.Duration != 5*time.Minute {
		t.Errorf("idle_timeout should inherit default, got %v", agent.IdleTimeout)
	}
}

func TestEnvInterpolation(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "corral.toml")
	os.WriteFile(cfgPath, []byte(`
[agents.dev]
command = "${MY_CMD} --flag"
working_dir = "${PROJECT_ROOT}/src"
`), 0644)

	t.Setenv("MY_CMD", "opencode")
	t.Setenv("PROJECT_ROOT", "/home/user/project")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	agent := cfg.Agents["dev"]
	if agent.Command() != "opencode --flag" {
		t.Errorf("command = %q, want %q", agent.Command(), "opencode --flag")
	}
	if agent.WorkingDir != "/home/user/project/src" {
		t.Errorf("working_dir = %q", agent.WorkingDir)
	}
}

func TestValidationMissingCommand(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "corral.toml")
	os.WriteFile(cfgPath, []byte(`
[agents.bad]
schedule = "* * * * *"
`), 0644)

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected validation error for missing command")
	}
}

func TestUnitFileForm(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "corral.toml")
	os.WriteFile(cfgPath, []byte(`
[agents.builder]
run = "make"
args = ["build", "--release"]
cron = "0 */6 * * *"
`), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	agent := cfg.Agents["builder"]
	if agent.Command() != "make" {
		t.Errorf("command = %q, want %q", agent.Command(), "make")
	}
	if agent.Schedule() != "0 */6 * * *" {
		t.Errorf("schedule = %q", agent.Schedule())
	}
	if len(agent.Args) != 2 || agent.Args[0] != "build" {
		t.Errorf("args = %v", agent.Args)
	}
}

func TestTildeExpansion(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "corral.toml")
	os.WriteFile(cfgPath, []byte(`
[agents.dev]
command = "echo hi"
working_dir = "~/projects"
log_dir = "~/.config/corral/logs"
`), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	agent := cfg.Agents["dev"]
	home, _ := os.UserHomeDir()
	if agent.WorkingDir != filepath.Join(home, "projects") {
		t.Errorf("working_dir = %q", agent.WorkingDir)
	}
	if agent.LogDir != filepath.Join(home, ".config/corral/logs") {
		t.Errorf("log_dir = %q", agent.LogDir)
	}
}
