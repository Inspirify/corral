package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Inspirify/corral/internal/config"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			if len(cfg.Agents) == 0 {
				fmt.Println("No agents configured.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSCHEDULE\tCOMMAND\tMAX RUNTIME")
			for name, agent := range cfg.Agents {
				schedule := agent.Schedule()
				if schedule == "" {
					schedule = "(manual)"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, schedule, agent.Command(), agent.MaxRuntime)
			}
			return w.Flush()
		},
	}
}
