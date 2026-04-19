package config

import (
	"os"
	"regexp"
)

var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// interpolateString replaces ${VAR} references with environment variable values.
func interpolateString(s string) string {
	return envPattern.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1] // strip ${ and }
		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		return match // leave unresolved vars as-is
	})
}

// interpolateMap processes all values in a string map.
func interpolateMap(m map[string]string) {
	for k, v := range m {
		m[k] = interpolateString(v)
	}
}

// interpolateAll processes all interpolatable fields in the config.
func interpolateAll(cfg *Config) {
	cfg.Defaults.LogDir = interpolateString(cfg.Defaults.LogDir)
	interpolateMap(cfg.Defaults.Env)

	for name, agent := range cfg.Agents {
		agent.Cmd = interpolateString(agent.Cmd)
		agent.Run = interpolateString(agent.Run)
		agent.WorkingDir = interpolateString(agent.WorkingDir)
		agent.LogDir = interpolateString(agent.LogDir)
		agent.DoneSignal = interpolateString(agent.DoneSignal)
		interpolateMap(agent.Env)
		for i, arg := range agent.Args {
			agent.Args[i] = interpolateString(arg)
		}

		// Expand ~ in paths
		agent.WorkingDir = expandPath(agent.WorkingDir)
		agent.LogDir = expandPath(agent.LogDir)

		cfg.Agents[name] = agent
	}

	// Also expand ~ in defaults
	cfg.Defaults.LogDir = expandPath(cfg.Defaults.LogDir)
}
