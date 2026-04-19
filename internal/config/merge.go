package config

// applyDefaults merges default values into agents that don't specify them.
func applyDefaults(cfg *Config) {
	d := cfg.Defaults
	for name, agent := range cfg.Agents {
		if agent.IdleTimeout.Duration == 0 && d.IdleTimeout.Duration != 0 {
			agent.IdleTimeout = d.IdleTimeout
		}
		if agent.MaxRuntime.Duration == 0 && d.MaxRuntime.Duration != 0 {
			agent.MaxRuntime = d.MaxRuntime
		}
		if agent.LockEnabled == nil {
			lock := d.Lock
			agent.LockEnabled = &lock
		}
		if agent.LogDir == "" && d.LogDir != "" {
			agent.LogDir = d.LogDir
		}
		if agent.DoneSignal == "" && d.DoneSignal != "" {
			agent.DoneSignal = d.DoneSignal
		}
		// Merge env: defaults are base, agent-specific overrides
		if len(d.Env) > 0 {
			merged := make(map[string]string, len(d.Env)+len(agent.Env))
			for k, v := range d.Env {
				merged[k] = v
			}
			for k, v := range agent.Env {
				merged[k] = v
			}
			agent.Env = merged
		}
		cfg.Agents[name] = agent
	}
}
