package main

import (
	"time"

	"github.com/joshp123/github-triage/internal/config"
	"github.com/joshp123/github-triage/internal/llm"
	"github.com/spf13/cobra"
)

func newSweepCmd() *cobra.Command {
	var limit int
	var prNumbers []int
	var state string
	var order string
	var timeout time.Duration
	var skipExisting bool
	cmd := &cobra.Command{
		Use:          "sweep",
		Short:        "Run a slop sweep (slop vs needs-human)",
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
			return runner.Sweep(cmd.Context(), cfg, limit, prNumbers, concurrencyFlag, state, order, timeout, skipExisting)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Max PRs to sweep from triage/raw (0 = all)")
	cmd.Flags().IntSliceVar(&prNumbers, "pr", nil, "Specific PR number to sweep (repeatable)")
	cmd.Flags().StringVar(&state, "state", "open", "PR state filter: open|closed|all")
	cmd.Flags().StringVar(&order, "order", "updated-desc", "Order: updated-asc|updated-desc|number-asc|number-desc")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Per-PR timeout (e.g. 2m, 30s)")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", true, "Skip PRs with existing map cards")
	return cmd
}
