package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of corral",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("corral %s (commit: %s, built: %s)\n", version, commit, date)
		},
	}
}
