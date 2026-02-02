package main

import (
	"github.com/joshp123/github-triage/internal/config"
	"github.com/joshp123/github-triage/internal/llm"
	"github.com/spf13/cobra"
)

func newReduceCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "reduce",
		Short:        "Run inventory snapshot over classification cards",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(repoFlag)
			if err != nil {
				return err
			}
			if err := cfg.EnsureDirs(); err != nil {
				return err
			}
			ensureSelfInPath()

			runner, err := llm.NewRunner(cfg, modelFlag)
			if err != nil {
				return err
			}
			return runner.Reduce(cmd.Context())
		},
	}
}
