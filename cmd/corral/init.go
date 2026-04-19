package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const defaultConfig = `# Corral agent configuration
# See: https://github.com/Inspirify/corral

[defaults]
idle_timeout = "15m"
max_runtime = "3h"
lock = true
log_dir = "~/.config/corral/logs"
done_signal = "AGENT_DONE"

[defaults.env]
PATH = "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"

# Define agents below. Example:
# [agents.dev]
# command = "claude -p 'Pick up the next issue'"
# schedule = "*/30 * * * *"
`

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold a new corral.toml in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := filepath.Join(".", "corral.toml")
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("corral.toml already exists")
			}
			if err := os.WriteFile(path, []byte(defaultConfig), 0644); err != nil {
				return fmt.Errorf("writing corral.toml: %w", err)
			}
			fmt.Println("Created corral.toml")
			return nil
		},
	}
}
