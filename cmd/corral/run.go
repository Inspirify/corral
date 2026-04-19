package main

import (
	"fmt"

	"github.com/Inspirify/corral/internal/config"
	"github.com/Inspirify/corral/internal/harness"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <agent>",
		Short: "Run a single agent immediately",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			agentName := args[0]
			agent, ok := cfg.Agents[agentName]
			if !ok {
				return fmt.Errorf("agent %q not found in configuration", agentName)
			}
			h := harness.New(agentName, agent)
			return h.Run(cmd.Context())
		},
	}
}
