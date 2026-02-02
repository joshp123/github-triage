package main

import (
	"path/filepath"

	"github.com/joshp123/github-triage/internal/cluster"
	"github.com/joshp123/github-triage/internal/config"
	"github.com/spf13/cobra"
)

func newClusterExportCmd() *cobra.Command {
	var output string
	var state string
	cmd := &cobra.Command{
		Use:          "cluster-export",
		Short:        "Export PR items for clustering",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(repoFlag)
			if err != nil {
				return err
			}
			if output == "" {
				output = filepath.Join(cfg.TriageDir, "cluster", "items.json")
			}
			return cluster.Export(cfg, output, state)
		},
	}
	cmd.Flags().StringVar(&output, "output", "", "Output JSON path (default: <data-root>/triage/cluster/items.json)")
	cmd.Flags().StringVar(&state, "state", "open", "PR state filter: open|closed|all")
	return cmd
}
