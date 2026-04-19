package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

func main() {
	root := &cobra.Command{
		Use:   "corral",
		Short: "A general-purpose agent harness and scheduler",
		Long:  "Corral runs, schedules, and manages autonomous AI coding agents with any backend.",
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./corral.toml)")

	root.AddCommand(
		newInitCmd(),
		newRunCmd(),
		newListCmd(),
		newValidateCmd(),
		newStartCmd(),
		newStopCmd(),
		newStatusCmd(),
		newLogsCmd(),
		newInstallCmd(),
		newUninstallCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
