package main

import (
	"fmt"

	"github.com/Inspirify/corral/internal/config"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration without running anything",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			fmt.Printf("Configuration valid: %d agent(s) configured\n", len(cfg.Agents))
			for name := range cfg.Agents {
				fmt.Printf("  - %s\n", name)
			}
			return nil
		},
	}
}
