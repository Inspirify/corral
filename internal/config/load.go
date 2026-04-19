package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// loadFile reads and parses a TOML config file into a Config.
func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		Agents: make(map[string]AgentConfig),
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing TOML: %w", err)
	}
	return cfg, nil
}

// loadAgentFile reads a per-agent TOML file.
// The file is expected to contain a single AgentConfig (no [agents.*] wrapper).
func loadAgentFile(path string) (string, AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", AgentConfig{}, err
	}
	var agent AgentConfig
	if err := toml.Unmarshal(data, &agent); err != nil {
		return "", AgentConfig{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	// Agent name derived from filename without extension
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return name, agent, nil
}

// discoverAgents looks for agents/*.toml and loads them into cfg.
// Per-agent files override inline agents of the same name.
func discoverAgents(dir string, cfg *Config) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil // no agents/ directory is fine
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		name, agent, err := loadAgentFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		if cfg.Agents == nil {
			cfg.Agents = make(map[string]AgentConfig)
		}
		cfg.Agents[name] = agent
	}
	return nil
}
